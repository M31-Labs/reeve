package executor

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type ShellWorktree struct {
	QuarantineDir string
	WorktreeRoot  string
}

func (w ShellWorktree) Head(ctx context.Context, path string) (string, error) {
	out, err := git(ctx, path, "rev-parse", "HEAD")
	return strings.TrimSpace(string(out)), err
}

func (w ShellWorktree) Dirty(ctx context.Context, path string) (bool, error) {
	out, err := git(ctx, path, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) != "", nil
}

func (w ShellWorktree) GreenCheck(ctx context.Context, path string, command string) GreenRun {
	if strings.TrimSpace(command) == "" {
		command = "go build ./... && go test ./..."
	}
	cmd := exec.CommandContext(ctx, "bash", "-lc", command)
	cmd.Dir = path
	out, err := cmd.CombinedOutput()
	result := GreenRun{Command: command, Output: trimOutput(string(out))}
	if err == nil {
		return result
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
	} else {
		result.ExitCode = -1
	}
	if result.Output != "" {
		result.Error = result.Output
	} else {
		result.Error = err.Error()
	}
	return result
}

func (w ShellWorktree) Quarantine(ctx context.Context, path string, taskID string) (string, error) {
	if strings.TrimSpace(w.QuarantineDir) == "" {
		return "", errors.New("quarantine dir is required")
	}
	if strings.TrimSpace(w.WorktreeRoot) == "" {
		return "", errors.New("worktree root is required for quarantine safety")
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	absRoot, err := filepath.Abs(w.WorktreeRoot)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." {
		return "", errors.New("refusing to quarantine path outside isolated worktree root")
	}
	if err := os.MkdirAll(w.QuarantineDir, 0o755); err != nil {
		return "", err
	}
	target := filepath.Join(w.QuarantineDir, sanitizePathPart(taskID)+"-"+time.Now().UTC().Format("20060102T150405Z"))
	if err := os.Rename(absPath, target); err != nil {
		return "", err
	}
	_ = ctx
	return target, nil
}

func git(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		return out, nil
	}
	msg := strings.TrimSpace(string(out))
	if msg == "" {
		msg = err.Error()
	}
	return out, errors.New(msg)
}

func sanitizePathPart(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		default:
			return '-'
		}
	}, s)
	s = strings.Trim(s, "-")
	if s == "" {
		return "task"
	}
	return s
}
