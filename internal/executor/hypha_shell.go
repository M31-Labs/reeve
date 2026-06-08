package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"m31labs.dev/reeve/internal/coord"
)

type ShellHypha struct {
	Command         string
	SigningIdentity string
	SporeDir        string
}

func (h ShellHypha) AssessTask(ctx context.Context, task coord.Task) (Assessment, error) {
	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Alignment      string `json:"alignment"`
			Recommendation string `json:"recommendation"`
		} `json:"data"`
		Errors []any `json:"errors"`
	}
	prompt := strings.TrimSpace(task.Title + "\n\n" + task.Description)
	err := h.runJSON(ctx, []string{"assess", "task", "--task", prompt, "--space", task.Trailer.SpaceURI, "--format", "json"}, &env)
	if err != nil {
		return Assessment{}, err
	}
	if !env.OK {
		return Assessment{}, envelopeErrors(env.Errors)
	}
	category := strings.TrimSpace(env.Data.Alignment)
	if category == "" {
		category = strings.TrimSpace(env.Data.Recommendation)
	}
	return Assessment{Category: category, Rationale: env.Data.Recommendation}, nil
}

func (h ShellHypha) StartTrace(ctx context.Context, start TraceStart) (string, error) {
	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			ID string
		} `json:"data"`
		Errors []any `json:"errors"`
	}
	err := h.runJSON(ctx, []string{"trace", "start", "--agent", start.AgentURI, "--task", start.TaskID, "--phase", start.Phase, "--space", start.SpaceURI, "--format", "json"}, &env)
	if err != nil {
		return "", err
	}
	if !env.OK {
		return "", envelopeErrors(env.Errors)
	}
	if env.Data.ID == "" {
		return "", errors.New("hypha trace start returned no id")
	}
	return env.Data.ID, nil
}

func (h ShellHypha) TickTrace(ctx context.Context, traceID string, spaceURI string, message string) error {
	_, err := h.run(ctx, []string{"trace", "tick", traceID, message, "--space", spaceURI})
	return err
}

func (h ShellHypha) DoneTrace(ctx context.Context, done TraceDone) error {
	args := []string{"trace", "done", done.TraceID, "--status", done.Status, "--space", done.SpaceURI}
	if done.SporeID != "" {
		args = append(args, "--link-spore", done.SporeID)
	}
	_, err := h.run(ctx, args)
	return err
}

func (h ShellHypha) SubmitSpore(ctx context.Context, sub SporeSubmission) (string, error) {
	if strings.TrimSpace(h.SigningIdentity) == "" {
		return "", errors.New("signing identity is required to submit proposal spores")
	}
	id := sporeID(sub.Task.ID)
	body := renderProposalSpore(id, h.SigningIdentity, sub)
	dir := h.SporeDir
	if dir == "" {
		dir = os.TempDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, id+".md")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return "", err
	}
	defer os.Remove(path)
	_, err := h.run(ctx, []string{"spore", "submit", path, "--sign", "--as", h.SigningIdentity, "--format", "text"})
	if err != nil {
		return "", err
	}
	return id, nil
}

func (h ShellHypha) runJSON(ctx context.Context, args []string, dest any) error {
	out, err := h.run(ctx, args)
	if err != nil {
		return err
	}
	return json.Unmarshal(out, dest)
}

func (h ShellHypha) run(ctx context.Context, args []string) ([]byte, error) {
	parts := append(strings.Fields(h.Command), args...)
	if len(parts) == 0 {
		return nil, errors.New("hypha command is empty")
	}
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
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

func renderProposalSpore(id, signer string, sub SporeSubmission) string {
	now := time.Now().UTC().Format(time.RFC3339)
	return fmt.Sprintf(`---
mdpp: "0.1"
id: %s
type: spore
space: hypha://m31labs/reeve
status: unreviewed
created: %s
agent:
    id: agent://reeve/conductor
    kind: conductor
    model: buckley
confidence: medium
source_refs:
    - %s
    - hypha://m31labs/reeve/object/spec.reeve-conductor-v1
---

# Reeve propose-only result for %s

Task: %s

Decision: %s

Reason: %s

Buckley approval: %s

Green check: %s exited %d

Signed by: %s
`, id, now, sub.Task.Trailer.SpaceURI, sub.Task.ID, sub.Task.Title, sub.Decision.State, sub.Decision.Reason, sub.Buckley.Approval, sub.Green.Command, sub.Green.ExitCode, signer)
}

func sporeID(taskID string) string {
	safe := strings.Map(func(r rune) rune {
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
	}, taskID)
	safe = strings.Trim(safe, "-")
	if safe == "" {
		safe = "task"
	}
	return "spore." + time.Now().UTC().Format("2006-01-02") + ".reeve." + safe
}

func envelopeErrors(items []any) error {
	if len(items) == 0 {
		return errors.New("hypha returned ok=false")
	}
	body, err := json.Marshal(items)
	if err != nil {
		return errors.New("hypha returned errors")
	}
	return errors.New(string(body))
}
