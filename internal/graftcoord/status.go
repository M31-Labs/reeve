package graftcoord

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const (
	StatusPending    = "pending"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
	StatusBlocked    = "blocked"
)

type TaskUpdate struct {
	TaskID      string
	Status      string
	Assign      string
	Workspace   string
	Description string
}

type UpdateOptions struct {
	DryRun bool
	Exec   ExecFunc
}

type UpdateResult struct {
	TaskID  string   `json:"task_id"`
	Status  string   `json:"status,omitempty"`
	Command []string `json:"command"`
	DryRun  bool     `json:"dry_run"`
	Output  string   `json:"output,omitempty"`
	Error   string   `json:"error,omitempty"`
}

func UpdateTask(ctx context.Context, command string, update TaskUpdate, opts UpdateOptions) UpdateResult {
	if opts.Exec == nil {
		opts.Exec = defaultExec
	}
	result := UpdateResult{TaskID: update.TaskID, Status: update.Status, DryRun: opts.DryRun}
	argv, err := CommandForUpdate(command, update)
	result.Command = argv
	if err != nil {
		result.Error = err.Error()
		return result
	}
	if opts.DryRun {
		return result
	}
	out, err := opts.Exec(ctx, argv[0], argv[1:])
	result.Output = strings.TrimSpace(string(out))
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		result.Error = msg
	}
	return result
}

func CommandForUpdate(command string, update TaskUpdate) ([]string, error) {
	base := strings.Fields(command)
	if len(base) == 0 {
		return nil, errors.New("graft command is empty")
	}
	if strings.TrimSpace(update.TaskID) == "" {
		return nil, errors.New("task id is required")
	}
	argv := append([]string{}, base...)
	argv = append(argv, "coord", "task", "update", update.TaskID)
	if update.Status != "" {
		if !validStatus(update.Status) {
			return nil, fmt.Errorf("unsupported coord status %q", update.Status)
		}
		argv = append(argv, "--status", update.Status)
	}
	if update.Assign != "" {
		argv = append(argv, "--assign", update.Assign)
	}
	if update.Workspace != "" {
		argv = append(argv, "--workspace", update.Workspace)
	}
	if update.Description != "" {
		argv = append(argv, "--description", update.Description)
	}
	return append(argv, "--json"), nil
}

func validStatus(status string) bool {
	switch status {
	case StatusPending, StatusInProgress, StatusCompleted, StatusBlocked:
		return true
	default:
		return false
	}
}
