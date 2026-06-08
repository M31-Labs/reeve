package graftcoord

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"m31labs.dev/reeve/internal/taskplan"
)

type ExecFunc func(context.Context, string, []string) ([]byte, error)

type ApplyOptions struct {
	DryRun bool
	Exec   ExecFunc
}

type ApplyResult struct {
	Action    string   `json:"action"`
	Title     string   `json:"title"`
	Workspace string   `json:"workspace,omitempty"`
	PriorTask string   `json:"prior_task,omitempty"`
	TaskID    string   `json:"task_id,omitempty"`
	Command   []string `json:"command"`
	DryRun    bool     `json:"dry_run"`
	Output    string   `json:"output,omitempty"`
	Error     string   `json:"error,omitempty"`
}

func ApplyPlan(ctx context.Context, command string, specs []taskplan.CreateSpec, opts ApplyOptions) []ApplyResult {
	if opts.Exec == nil {
		opts.Exec = defaultExec
	}
	results := make([]ApplyResult, 0, len(specs))
	for _, spec := range specs {
		result := ApplyResult{
			Action:    spec.Action,
			Title:     spec.Title,
			Workspace: spec.Workspace,
			PriorTask: spec.PriorTask,
			DryRun:    opts.DryRun,
		}
		argv, err := CommandForSpec(command, spec)
		result.Command = argv
		if err != nil {
			result.Error = err.Error()
			results = append(results, result)
			continue
		}
		if opts.DryRun {
			results = append(results, result)
			continue
		}
		out, err := opts.Exec(ctx, argv[0], argv[1:])
		result.Output = strings.TrimSpace(string(out))
		if err != nil {
			msg := strings.TrimSpace(string(out))
			if msg == "" {
				msg = err.Error()
			}
			result.Error = msg
			results = append(results, result)
			continue
		}
		result.TaskID = extractTaskID(out)
		results = append(results, result)
	}
	return results
}

func CommandForSpec(command string, spec taskplan.CreateSpec) ([]string, error) {
	base := strings.Fields(command)
	if len(base) == 0 {
		return nil, errors.New("graft command is empty")
	}
	switch spec.Action {
	case "create":
		argv := append([]string{}, base...)
		argv = append(argv, "coord", "task", "create",
			"--title", spec.Title,
			"--description", spec.Description,
			"--priority", spec.Priority,
		)
		if spec.Workspace != "" {
			argv = append(argv, "--workspace", spec.Workspace)
		}
		for _, tag := range spec.Tags {
			if strings.TrimSpace(tag) != "" {
				argv = append(argv, "--tag", tag)
			}
		}
		return append(argv, "--json"), nil
	case "reopen":
		if spec.PriorTask == "" {
			return nil, fmt.Errorf("reopen action for %q is missing prior task id", spec.Title)
		}
		argv := append([]string{}, base...)
		return append(argv, "coord", "task", "reopen", spec.PriorTask, "--reason", spec.Description, "--json"), nil
	default:
		return nil, fmt.Errorf("unsupported plan action %q", spec.Action)
	}
}

func defaultExec(ctx context.Context, name string, args []string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func extractTaskID(body []byte) string {
	var root any
	if err := json.Unmarshal(body, &root); err != nil {
		return ""
	}
	return firstID(root)
}

func firstID(v any) string {
	switch x := v.(type) {
	case map[string]any:
		for _, key := range []string{"id", "task_id", "name"} {
			if s, ok := x[key].(string); ok && s != "" {
				return s
			}
		}
		for _, child := range x {
			if id := firstID(child); id != "" {
				return id
			}
		}
	case []any:
		for _, child := range x {
			if id := firstID(child); id != "" {
				return id
			}
		}
	}
	return ""
}
