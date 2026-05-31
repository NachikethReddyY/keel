package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"keel/internal/ledger"
)

func TestSaveDetectsConflict(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.md")
	if err := os.WriteFile(path, []byte("- [ ] id:K-1 status:todo priority:p2 First\n"), 0600); err != nil {
		t.Fatal(err)
	}
	snap, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("- [ ] id:K-2 status:todo priority:p2 Second\n"), 0600); err != nil {
		t.Fatal(err)
	}
	err = Save(path, ledger.Ledger{}, snap.Hash)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestRepairCreatesBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.md")
	if err := os.WriteFile(path, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	backup, err := Repair(path, ledger.Ledger{})
	if err != nil {
		t.Fatal(err)
	}
	if backup == "" {
		t.Fatalf("expected backup path")
	}
	if _, err := os.Stat(backup); err != nil {
		t.Fatal(err)
	}
}
