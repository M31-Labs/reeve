package worktree

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type ExecFunc func(context.Context, string, []string, string) ([]byte, error)

type Worktree struct {
	SourcePath string   `json:"source_path"`
	Root       string   `json:"root"`
	Branch     string   `json:"branch"`
	Path       string   `json:"path"`
	Command    []string `json:"command"`
	Output     string   `json:"output,omitempty"`
}

func BranchForTask(spaceID, taskID string) string {
	return "reeve/" + sanitize(spaceID) + "/" + sanitize(taskID)
}

func CreateWithBuckley(ctx context.Context, command string, sourcePath string, root string, branch string, execFn ExecFunc) (Worktree, error) {
	base := strings.Fields(command)
	if len(base) == 0 {
		return Worktree{}, errors.New("buckley command is empty")
	}
	if strings.TrimSpace(sourcePath) == "" {
		return Worktree{}, errors.New("source path is required")
	}
	if strings.TrimSpace(root) == "" {
		return Worktree{}, errors.New("worktree root is required")
	}
	if strings.TrimSpace(branch) == "" {
		return Worktree{}, errors.New("branch is required")
	}
	argv := append([]string{}, base...)
	argv = append(argv, "worktree", "create", "--root", root, branch)
	if execFn == nil {
		execFn = defaultExec
	}
	out, err := execFn(ctx, argv[0], argv[1:], sourcePath)
	result := Worktree{
		SourcePath: sourcePath,
		Root:       root,
		Branch:     branch,
		Command:    argv,
		Output:     strings.TrimSpace(string(out)),
	}
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return result, errors.New(msg)
	}
	path, err := ParseCreatedPath(out)
	if err != nil {
		return result, err
	}
	result.Path = path
	if !pathWithin(root, path) {
		return result, fmt.Errorf("buckley returned worktree outside root: %s", path)
	}
	return result, nil
}

func ParseCreatedPath(out []byte) (string, error) {
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		const marker = "Worktree created at "
		i := strings.Index(line, marker)
		if i < 0 {
			continue
		}
		rest := line[i+len(marker):]
		if j := strings.Index(rest, " (branch "); j >= 0 {
			rest = rest[:j]
		}
		rest = strings.TrimSpace(rest)
		if rest != "" {
			return rest, nil
		}
	}
	return "", errors.New("buckley worktree create output did not include created path")
}

func defaultExec(ctx context.Context, name string, args []string, dir string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

func sanitize(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		case r >= '0' && r <= '9':
			return r
		default:
			return '-'
		}
	}, s)
	s = strings.Trim(s, "-")
	if s == "" {
		return "item"
	}
	return s
}

func pathWithin(root, path string) bool {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
