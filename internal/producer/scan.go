package producer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Runner interface {
	Run(ctx context.Context, dir string, name string, args ...string) Result
}

type Result struct {
	Command  string `json:"command"`
	ExitCode int    `json:"exit_code"`
	Output   string `json:"output,omitempty"`
	Err      string `json:"err,omitempty"`
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, dir string, name string, args ...string) Result {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	res := Result{Command: strings.Join(append([]string{name}, args...), " "), Output: trimOutput(string(out))}
	if err == nil {
		return res
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		res.ExitCode = exitErr.ExitCode()
	} else {
		res.ExitCode = -1
	}
	res.Err = err.Error()
	return res
}

type ScanOptions struct {
	AgentURI string
	Timeout  time.Duration
	GoBin    string
}

func ScanGoMaintenance(ctx context.Context, spaceURI, workspacePath string, opts ScanOptions, runner Runner) []Signal {
	if runner == nil {
		runner = ExecRunner{}
	}
	if opts.AgentURI == "" {
		opts.AgentURI = "agent://reeve/conductor"
	}
	if opts.Timeout == 0 {
		opts.Timeout = 2 * time.Minute
	}
	if opts.GoBin == "" {
		opts.GoBin = "go"
	}
	if _, err := os.Stat(filepath.Join(workspacePath, "go.mod")); err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	var signals []Signal
	if res := runner.Run(ctx, workspacePath, opts.GoBin, "build", "./..."); res.ExitCode != 0 {
		signals = append(signals, Signal{
			SpaceURI:  spaceURI,
			Axis:      "maintenance",
			Kind:      "red-build",
			Target:    "go build ./...",
			Severity:  0.9,
			Title:     "Fix failing Go build",
			Snapshot:  res.Snapshot(),
			CreatedBy: opts.AgentURI,
		})
	}
	if res := runner.Run(ctx, workspacePath, opts.GoBin, "test", "./..."); res.ExitCode != 0 {
		signals = append(signals, Signal{
			SpaceURI:  spaceURI,
			Axis:      "maintenance",
			Kind:      "red-test",
			Target:    "go test ./...",
			Severity:  0.9,
			Title:     "Fix failing Go tests",
			Snapshot:  res.Snapshot(),
			CreatedBy: opts.AgentURI,
		})
	}
	return signals
}

func (r Result) Snapshot() string {
	if r.Err != "" {
		return fmt.Sprintf("%s exited %d: %s\n%s", r.Command, r.ExitCode, r.Err, r.Output)
	}
	return fmt.Sprintf("%s exited %d\n%s", r.Command, r.ExitCode, r.Output)
}

func trimOutput(s string) string {
	s = strings.TrimSpace(s)
	const max = 2000
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n[truncated]"
}
