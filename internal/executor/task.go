package executor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"m31labs.dev/reeve/internal/taskplan"
)

type TaskRunOptions struct {
	Command    string
	Workspace  string
	GreenCheck string
	Model      string
	Timeout    time.Duration
}

func PromptForSpec(spec taskplan.CreateSpec, greenCheck string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are Buckley executing a Reeve maintenance task.\n\n")
	fmt.Fprintf(&b, "Task: %s\n", spec.Title)
	if spec.Workspace != "" {
		fmt.Fprintf(&b, "Workspace: %s\n", spec.Workspace)
	}
	if greenCheck != "" {
		fmt.Fprintf(&b, "Green check: %s\n", greenCheck)
	}
	fmt.Fprintf(&b, "Dedup key: %s\n\n", spec.DedupKey)
	fmt.Fprintf(&b, "Task description:\n%s\n\n", spec.Description)
	fmt.Fprintf(&b, "Constraints:\n")
	fmt.Fprintf(&b, "- Keep the change scoped to the task.\n")
	fmt.Fprintf(&b, "- Leave unrelated user changes intact.\n")
	fmt.Fprintf(&b, "- Run the green check when practical and report the result.\n")
	fmt.Fprintf(&b, "- End with a machine-readable approval gate line: approval_gate: allow or approval_gate: ask.\n")
	return b.String()
}

func RunSpec(ctx context.Context, spec taskplan.CreateSpec, opts TaskRunOptions, execFn BuckleyExecFunc) BuckleyRun {
	return RunBuckley(ctx, BuckleyInvocation{
		Command: opts.Command,
		Workdir: opts.Workspace,
		Prompt:  PromptForSpec(spec, opts.GreenCheck),
		Model:   opts.Model,
		Timeout: opts.Timeout,
	}, execFn)
}
