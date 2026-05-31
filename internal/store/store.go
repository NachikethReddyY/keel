package store

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"keel/internal/ledger"
	"keel/internal/parser"
)

type Snapshot struct {
	Path   string
	Raw    []byte
	Hash   [32]byte
	Ledger ledger.Ledger
	Errors []parser.ParseError
	Mode   os.FileMode
}

var ErrConflict = errors.New("ledger changed on disk")

func ExpandPath(path string) (string, error) {
	if path == "" {
		return "", errors.New("empty path")
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			path = home
		} else {
			path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return filepath.Abs(path)
}

func Load(path string) (Snapshot, error) {
	expanded, err := ExpandPath(path)
	if err != nil {
		return Snapshot{}, err
	}
	raw, err := os.ReadFile(expanded)
	if errors.Is(err, os.ErrNotExist) {
		return Snapshot{
			Path: expanded,
			Hash: sha256.Sum256(nil),
			Mode: 0600,
		}, nil
	}
	if err != nil {
		return Snapshot{}, err
	}
	info, err := os.Stat(expanded)
	if err != nil {
		return Snapshot{}, err
	}
	l, errs := parser.Parse(string(raw))
	return Snapshot{
		Path:   expanded,
		Raw:    raw,
		Hash:   sha256.Sum256(raw),
		Ledger: l,
		Errors: errs,
		Mode:   info.Mode().Perm(),
	}, nil
}

func Save(path string, l ledger.Ledger, expectedHash [32]byte) error {
	expanded, err := ExpandPath(path)
	if err != nil {
		return err
	}
	current, err := os.ReadFile(expanded)
	if err == nil && sha256.Sum256(current) != expectedHash {
		return ErrConflict
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	mode := os.FileMode(0600)
	if info, statErr := os.Stat(expanded); statErr == nil {
		mode = info.Mode().Perm()
	}
	return atomicWrite(expanded, []byte(parser.Render(l)), mode)
}

func Repair(path string, l ledger.Ledger) (string, error) {
	expanded, err := ExpandPath(path)
	if err != nil {
		return "", err
	}
	if err := rejectSymlink(expanded); err != nil {
		return "", err
	}
	if _, err := os.Stat(expanded); err == nil {
		backup := expanded + ".bak"
		if err := copyFile(expanded, backup); err != nil {
			return "", err
		}
		return backup, atomicWrite(expanded, []byte(parser.Render(l)), 0600)
	}
	return "", atomicWrite(expanded, []byte(parser.Render(l)), 0600)
}

func rejectSymlink(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to repair symlink ledger: %s", path)
	}
	return nil
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".keel-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	if dirHandle, err := os.Open(dir); err == nil {
		_ = dirHandle.Sync()
		_ = dirHandle.Close()
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
