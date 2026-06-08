package executor

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

type BuckleyExecFunc func(context.Context, string, []string, string) ([]byte, error)

type BuckleyInvocation struct {
	Command string
	Workdir string
	Prompt  string
	Model   string
	Timeout time.Duration
}

type BuckleyRun struct {
	Command  []string `json:"command"`
	Workdir  string   `json:"workdir,omitempty"`
	ExitCode int      `json:"exit_code"`
	Output   string   `json:"output,omitempty"`
	Approval Approval `json:"approval"`
	Error    string   `json:"error,omitempty"`
}

func CommandForBuckley(inv BuckleyInvocation) ([]string, error) {
	base := strings.Fields(inv.Command)
	if len(base) == 0 {
		return nil, errors.New("buckley command is empty")
	}
	if strings.TrimSpace(inv.Prompt) == "" {
		return nil, errors.New("buckley prompt is empty")
	}
	argv := append([]string{}, base...)
	argv = append(argv, "--plain", "--no-color")
	if inv.Model != "" {
		argv = append(argv, "-m", inv.Model)
	}
	argv = append(argv, "-p", inv.Prompt)
	return argv, nil
}

func RunBuckley(ctx context.Context, inv BuckleyInvocation, execFn BuckleyExecFunc) BuckleyRun {
	argv, err := CommandForBuckley(inv)
	result := BuckleyRun{Command: argv, Workdir: inv.Workdir}
	if err != nil {
		result.ExitCode = -1
		result.Error = err.Error()
		result.Approval = ApprovalUnknown
		return result
	}
	if execFn == nil {
		execFn = defaultBuckleyExec
	}
	if inv.Timeout == 0 {
		inv.Timeout = 30 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, inv.Timeout)
	defer cancel()
	out, err := execFn(ctx, argv[0], argv[1:], inv.Workdir)
	result.Output = trimOutput(string(out))
	result.Approval = ParseApproval(result.Output)
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

func defaultBuckleyExec(ctx context.Context, name string, args []string, dir string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

func trimOutput(s string) string {
	s = strings.TrimSpace(s)
	const max = 4000
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n[truncated]"
}
