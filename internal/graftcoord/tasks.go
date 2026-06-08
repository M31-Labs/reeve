package graftcoord

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"m31labs.dev/reeve/internal/coord"
)

type Summary struct {
	Tasks    []coord.Task   `json:"tasks"`
	Counts   map[string]int `json:"counts"`
	Warnings []string       `json:"warnings,omitempty"`
}

func LoadSummary(ctx context.Context, command string) Summary {
	out := Summary{Tasks: []coord.Task{}, Counts: map[string]int{}}
	if strings.TrimSpace(command) == "" {
		out.Warnings = append(out.Warnings, "graft command is empty; coord status skipped")
		return out
	}
	args := append(strings.Fields(command), "coord", "task", "list", "--all", "--json")
	if len(args) == 0 {
		out.Warnings = append(out.Warnings, "graft command is empty; coord status skipped")
		return out
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	body, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			out.Warnings = append(out.Warnings, fmt.Sprintf("%s not found; coord status skipped", args[0]))
			return out
		}
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = err.Error()
		}
		out.Warnings = append(out.Warnings, "graft coord status skipped: "+msg)
		return out
	}
	tasks, err := ParseTasks(body)
	if err != nil {
		out.Warnings = append(out.Warnings, "graft coord status parse failed: "+err.Error())
		return out
	}
	out.Tasks = tasks
	out.Counts = CountByStatus(tasks)
	return out
}

func ParseTasks(body []byte) ([]coord.Task, error) {
	var root any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, err
	}
	var tasks []coord.Task
	collectTasks(root, &tasks)
	return dedupeTasks(tasks), nil
}

func CountByStatus(tasks []coord.Task) map[string]int {
	counts := map[string]int{}
	for _, task := range tasks {
		status := task.Status
		if status == "" {
			status = "unknown"
		}
		counts[status]++
		if task.Blocked {
			counts["unmanaged_blocked"]++
		}
	}
	return counts
}

func dedupeTasks(tasks []coord.Task) []coord.Task {
	seen := map[string]bool{}
	out := make([]coord.Task, 0, len(tasks))
	for _, task := range tasks {
		key := task.ID
		if key == "" {
			key = task.Title + "\x00" + task.Description
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, task)
	}
	return out
}

func collectTasks(v any, out *[]coord.Task) {
	switch x := v.(type) {
	case []any:
		for _, item := range x {
			collectTasks(item, out)
		}
	case map[string]any:
		if task, ok := taskFromMap(x); ok {
			*out = append(*out, task)
			return
		}
		for _, child := range x {
			collectTasks(child, out)
		}
	}
}

func taskFromMap(m map[string]any) (coord.Task, bool) {
	id := firstString(m, "id", "task_id", "name")
	title := firstString(m, "title", "summary")
	status := firstString(m, "status", "state")
	description := firstString(m, "description", "body", "details")
	if id == "" || (title == "" && description == "") {
		return coord.Task{}, false
	}
	if status == "" {
		status = "unknown"
	}
	return coord.ClassifyTask(id, title, description, status), true
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}
