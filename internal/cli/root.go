package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"keel/internal/config"
	"keel/internal/editor"
	"keel/internal/ledger"
	"keel/internal/parser"
	"keel/internal/store"
	"keel/internal/tui"
)

type options struct {
	ConfigPath string
	LedgerPath string
}

func NewRootCommand() *cobra.Command {
	var opts options
	root := &cobra.Command{
		Use:   "keel",
		Short: "Local-first task tracking for terminal workflows",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(opts)
			if err != nil {
				return err
			}
			snap, err := store.Load(cfg.LedgerPath)
			if err != nil {
				return err
			}
			model := tui.New(snap, cfg)
			_, err = tea.NewProgram(model, tea.WithAltScreen()).Run()
			return err
		},
	}
	root.PersistentFlags().StringVar(&opts.ConfigPath, "config", "", "path to keel TOML config")
	root.PersistentFlags().StringVar(&opts.LedgerPath, "ledger", "", "path to Markdown ledger")

	root.AddCommand(addCommand(&opts))
	root.AddCommand(inboxCommand(&opts))
	root.AddCommand(editCommand(&opts))
	root.AddCommand(doctorCommand(&opts))
	root.AddCommand(exportCommand(&opts))
	root.AddCommand(configCommand(&opts))
	root.AddCommand(aiCommand(&opts))
	return root
}

func addCommand(opts *options) *cobra.Command {
	var status string
	var priority string
	var due string
	var category string
	var tags []string
	cmd := &cobra.Command{
		Use:   "add [title]",
		Short: "Add a task to the ledger",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(*opts)
			if err != nil {
				return err
			}
			snap, err := store.Load(cfg.LedgerPath)
			if err != nil {
				return err
			}
			if len(snap.Errors) > 0 {
				return fmt.Errorf("ledger has parse errors; run keel doctor")
			}
			task := ledger.Task{
				ID:       ledger.NewID(time.Now()),
				Title:    strings.Join(args, " "),
				Status:   ledger.Status(status),
				Priority: ledger.Priority(priority),
				Due:      due,
				Category: strings.ReplaceAll(strings.TrimSpace(category), " ", "-"),
				Tags:     tags,
			}
			if _, errs := parser.Parse(task.CanonicalLine()); len(errs) > 0 {
				return fmt.Errorf("invalid task metadata: %s", errs[0].Msg)
			}
			snap.Ledger.Tasks = append(snap.Ledger.Tasks, task)
			if err := store.Save(cfg.LedgerPath, snap.Ledger, snap.Hash); err != nil {
				if errors.Is(err, store.ErrConflict) {
					return fmt.Errorf("ledger changed while adding task; reload and retry")
				}
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", task.ID, task.Title)
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", string(ledger.StatusTodo), "todo, doing, done, or blocked")
	cmd.Flags().StringVar(&priority, "priority", string(ledger.PriorityP2), "p0, p1, p2, or p3")
	cmd.Flags().StringVar(&due, "due", "", "due date as YYYY-MM-DD")
	cmd.Flags().StringVar(&category, "category", "", "category name")
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "tag without the leading plus")
	return cmd
}

func inboxCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "inbox",
		Short: "Print open tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, snap, err := snapshot(*opts)
			if err != nil {
				return err
			}
			_ = cfg
			for _, task := range snap.Ledger.Tasks {
				if !task.IsDone() {
					fmt.Fprintln(cmd.OutOrStdout(), task.CanonicalLine())
				}
			}
			return nil
		},
	}
}

func editCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Open the ledger in $EDITOR",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(*opts)
			if err != nil {
				return err
			}
			path, err := store.ExpandPath(cfg.LedgerPath)
			if err != nil {
				return err
			}
			if _, err := os.Stat(path); os.IsNotExist(err) {
				if err := store.Save(path, ledger.Ledger{}, [32]byte{}); err != nil {
					return err
				}
			}
			return editor.Open(path)
		},
	}
}

func doctorCommand(opts *options) *cobra.Command {
	var fix bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Validate and optionally repair the ledger",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, snap, err := snapshot(*opts)
			if err != nil {
				return err
			}
			if len(snap.Errors) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "ok")
				return nil
			}
			for _, parseErr := range snap.Errors {
				fmt.Fprintln(cmd.ErrOrStderr(), parseErr.Error())
			}
			if !fix {
				return fmt.Errorf("%d parse error(s)", len(snap.Errors))
			}
			backup, err := store.Repair(cfg.LedgerPath, snap.Ledger)
			if err != nil {
				return err
			}
			if backup != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "repaired ledger; backup: %s\n", backup)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "repaired ledger")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&fix, "fix", false, "rewrite ledger from parsed canonical tasks")
	return cmd
}

func exportCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "export",
		Short: "Export canonical Markdown",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, snap, err := snapshot(*opts)
			if err != nil {
				return err
			}
			if len(snap.Errors) > 0 {
				return fmt.Errorf("ledger has parse errors; run keel doctor")
			}
			fmt.Fprint(cmd.OutOrStdout(), parser.Render(snap.Ledger))
			return nil
		},
	}
}

func configCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Print resolved config",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(*opts)
			if err != nil {
				return err
			}
			path, _ := store.ExpandPath(cfg.LedgerPath)
			fmt.Fprintf(cmd.OutOrStdout(), "ledger = %q\n", path)
			fmt.Fprintf(cmd.OutOrStdout(), "theme = %q\n", cfg.Theme)
			return nil
		},
	}
}

func aiCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "ai [prompt]",
		Short: "Ask OpenCode to work with the Keel task ledger",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(*opts)
			if err != nil {
				return err
			}
			ledgerPath, err := store.ExpandPath(cfg.LedgerPath)
			if err != nil {
				return err
			}
			if _, err := os.Stat(ledgerPath); os.IsNotExist(err) {
				if err := store.Save(ledgerPath, ledger.Ledger{}, [32]byte{}); err != nil {
					return err
				}
			}
			prompt := strings.TrimSpace(strings.Join(args, " "))
			if prompt == "" {
				fmt.Fprint(cmd.ErrOrStderr(), "AI prompt> ")
				line, err := bufio.NewReader(os.Stdin).ReadString('\n')
				if err != nil {
					return err
				}
				prompt = strings.TrimSpace(line)
			}
			if prompt == "" {
				return fmt.Errorf("empty AI prompt")
			}
			opencode, err := exec.LookPath("opencode")
			if err != nil {
				return fmt.Errorf("opencode not found on PATH")
			}
			root, err := projectRoot()
			if err != nil {
				return err
			}
			skillPath := filepath.Join(root, "docs", "keel-agent-skill.md")
			ledgerDir := filepath.Dir(ledgerPath)
			message := fmt.Sprintf("Use the attached Keel skill. The canonical ledger is %s. User request: %s", ledgerPath, prompt)
			run := exec.Command(opencode, "run", "--dir", ledgerDir, "--file", skillPath, "--file", ledgerPath, "--title", "Keel task agent", message)
			run.Stdin = os.Stdin
			run.Stdout = os.Stdout
			run.Stderr = os.Stderr
			return run.Run()
		},
	}
}

func snapshot(opts options) (config.Config, store.Snapshot, error) {
	cfg, err := loadConfig(opts)
	if err != nil {
		return cfg, store.Snapshot{}, err
	}
	snap, err := store.Load(cfg.LedgerPath)
	return cfg, snap, err
}

func loadConfig(opts options) (config.Config, error) {
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return cfg, err
	}
	if opts.LedgerPath != "" {
		cfg.LedgerPath = opts.LedgerPath
	}
	return cfg, nil
}

func projectRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd, nil
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			return "", fmt.Errorf("could not find project root")
		}
		wd = parent
	}
}
