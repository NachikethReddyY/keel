package editor

import (
	"errors"
	"os"
	"os/exec"
	"strings"
)

func Resolve() (string, []string, error) {
	if raw := strings.TrimSpace(os.Getenv("EDITOR")); raw != "" {
		parts := strings.Fields(raw)
		if len(parts) > 0 {
			return parts[0], parts[1:], nil
		}
	}
	for _, candidate := range []string{"nvim", "vim", "vi"} {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil, nil
		}
	}
	return "", nil, errors.New("no editor found: set $EDITOR or install nvim, vim, or vi")
}

func Open(path string) error {
	bin, args, err := Resolve()
	if err != nil {
		return err
	}
	args = append(args, path)
	cmd := exec.Command(bin, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
