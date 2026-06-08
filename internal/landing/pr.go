package landing

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type ExecFunc func(context.Context, string, []string, string) ([]byte, error)

type Options struct {
	GHCommand  string
	BaseBranch string
	Draft      bool
	Title      string
	Body       string
	Workdir    string
}

type Result struct {
	Branch      string   `json:"branch,omitempty"`
	BaseBranch  string   `json:"base_branch,omitempty"`
	PRURL       string   `json:"pr_url,omitempty"`
	PushCommand []string `json:"push_command,omitempty"`
	PRCommand   []string `json:"pr_command,omitempty"`
	Output      string   `json:"output,omitempty"`
}

func LandPR(ctx context.Context, opts Options, execFn ExecFunc) (Result, error) {
	if strings.TrimSpace(opts.Workdir) == "" {
		return Result{}, errors.New("workdir is required")
	}
	if strings.TrimSpace(opts.GHCommand) == "" {
		opts.GHCommand = "gh"
	}
	if strings.TrimSpace(opts.BaseBranch) == "" {
		opts.BaseBranch = "main"
	}
	if strings.TrimSpace(opts.Title) == "" {
		return Result{}, errors.New("PR title is required")
	}
	if strings.TrimSpace(opts.Body) == "" {
		opts.Body = "Automated Reeve maintenance change."
	}
	if execFn == nil {
		execFn = defaultExec
	}
	branch, err := currentBranch(ctx, opts.Workdir, execFn)
	if err != nil {
		return Result{}, err
	}
	result := Result{Branch: branch, BaseBranch: opts.BaseBranch}
	push := []string{"push", "-u", "origin", "HEAD"}
	result.PushCommand = append([]string{"git"}, push...)
	if out, err := execFn(ctx, "git", push, opts.Workdir); err != nil {
		return result, commandErr(out, err)
	} else {
		result.Output = strings.TrimSpace(string(out))
	}
	ghBase := strings.Fields(opts.GHCommand)
	if len(ghBase) == 0 {
		return result, errors.New("gh command is empty")
	}
	args := append([]string{}, ghBase[1:]...)
	args = append(args, "pr", "create",
		"--base", opts.BaseBranch,
		"--head", branch,
		"--title", opts.Title,
		"--body", opts.Body,
	)
	if opts.Draft {
		args = append(args, "--draft")
	}
	result.PRCommand = append([]string{ghBase[0]}, args...)
	out, err := execFn(ctx, ghBase[0], args, opts.Workdir)
	result.Output = strings.TrimSpace(result.Output + "\n" + string(out))
	if err != nil {
		return result, commandErr(out, err)
	}
	result.PRURL = firstURL(string(out))
	if result.PRURL == "" {
		result.PRURL = strings.TrimSpace(string(out))
	}
	return result, nil
}

func currentBranch(ctx context.Context, dir string, execFn ExecFunc) (string, error) {
	out, err := execFn(ctx, "git", []string{"rev-parse", "--abbrev-ref", "HEAD"}, dir)
	if err != nil {
		return "", commandErr(out, err)
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" || branch == "HEAD" {
		return "", fmt.Errorf("worktree is not on a named branch: %q", branch)
	}
	return branch, nil
}

func defaultExec(ctx context.Context, name string, args []string, dir string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

func commandErr(out []byte, err error) error {
	msg := strings.TrimSpace(string(out))
	if msg == "" {
		msg = err.Error()
	}
	return errors.New(msg)
}

func firstURL(s string) string {
	for _, field := range strings.Fields(s) {
		if strings.HasPrefix(field, "http://") || strings.HasPrefix(field, "https://") {
			return strings.TrimSpace(field)
		}
	}
	return ""
}
