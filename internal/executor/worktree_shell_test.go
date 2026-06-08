package executor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestShellWorktreeQuarantineRefusesPathOutsideRoot(t *testing.T) {
	tmp := t.TempDir()
	outside := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := (ShellWorktree{
		WorktreeRoot:  filepath.Join(tmp, "worktrees"),
		QuarantineDir: filepath.Join(tmp, "quarantine"),
	}).Quarantine(context.Background(), outside, "task-1")
	if err == nil {
		t.Fatal("expected quarantine refusal")
	}
}

func TestShellWorktreeQuarantineMovesPathInsideRoot(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "worktrees")
	path := filepath.Join(root, "task-worktree")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	target, err := (ShellWorktree{
		WorktreeRoot:  root,
		QuarantineDir: filepath.Join(tmp, "quarantine"),
	}).Quarantine(context.Background(), path, "task/1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("target missing: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("source still exists or unexpected err: %v", err)
	}
}
