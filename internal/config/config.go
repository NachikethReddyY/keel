package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	LedgerPath string
	Theme      string
}

func Default() Config {
	return Config{
		LedgerPath: "~/tasks.md",
		Theme:      "harbor",
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		path = defaultPath()
	}
	file, err := os.Open(expandHome(path))
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if unquoted, err := strconv.Unquote(value); err == nil {
			value = unquoted
		}
		switch key {
		case "ledger":
			cfg.LedgerPath = value
		case "theme":
			cfg.Theme = value
		}
	}
	return cfg, scanner.Err()
}

func defaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".keel.toml"
	}
	return filepath.Join(home, ".config", "keel", "config.toml")
}

func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			if path == "~" {
				return home
			}
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}
