package hypha

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type Telemetry struct {
	ActiveTraces     []Trace  `json:"active_traces"`
	UnreviewedSpores []Spore  `json:"unreviewed_spores"`
	Warnings         []string `json:"warnings,omitempty"`
}

type Trace struct {
	ID       string `json:"id"`
	Space    string `json:"space"`
	Agent    string `json:"agent"`
	Status   string `json:"status"`
	Started  string `json:"started"`
	LastTick string `json:"last_tick"`
	Ticks    int    `json:"ticks"`
	Phase    string `json:"phase,omitempty"`
	Task     string `json:"task,omitempty"`
	Path     string `json:"path,omitempty"`
}

type Spore struct {
	ID          string `json:"id"`
	Space       string `json:"space"`
	Status      string `json:"status"`
	Path        string `json:"path"`
	SubmittedAt string `json:"submitted_at,omitempty"`
}

type envelope[T any] struct {
	OK       bool     `json:"ok"`
	Data     T        `json:"data"`
	Warnings []string `json:"warnings"`
	Errors   []any    `json:"errors"`
}

func LoadTelemetry(ctx context.Context, command, spaceURI string) Telemetry {
	var out Telemetry
	if strings.TrimSpace(command) == "" {
		out.Warnings = append(out.Warnings, "hypha command is empty; telemetry skipped")
		return out
	}
	traces, warnings, err := listActiveTraces(ctx, command, spaceURI)
	out.Warnings = append(out.Warnings, warnings...)
	if err != nil {
		out.Warnings = append(out.Warnings, err.Error())
	} else {
		out.ActiveTraces = traces
	}
	spores, warnings, err := listUnreviewedSpores(ctx, command, spaceURI)
	out.Warnings = append(out.Warnings, warnings...)
	if err != nil {
		out.Warnings = append(out.Warnings, err.Error())
	} else {
		out.UnreviewedSpores = spores
	}
	return out
}

func listActiveTraces(ctx context.Context, command, spaceURI string) ([]Trace, []string, error) {
	var env envelope[[]Trace]
	if err := runJSON(ctx, command, []string{"trace", "list", "--active", "--space", spaceURI, "--format", "json"}, &env); err != nil {
		return nil, nil, fmt.Errorf("hypha trace telemetry skipped: %w", err)
	}
	return env.Data, env.Warnings, envelopeError(env)
}

func listUnreviewedSpores(ctx context.Context, command, spaceURI string) ([]Spore, []string, error) {
	var env envelope[[]Spore]
	if err := runJSON(ctx, command, []string{"spore", "list", "--space", spaceURI, "--status", "unreviewed", "--format", "json", "--limit", "100"}, &env); err != nil {
		return nil, nil, fmt.Errorf("hypha spore telemetry skipped: %w", err)
	}
	return env.Data, env.Warnings, envelopeError(env)
}

func runJSON(ctx context.Context, command string, args []string, dest any) error {
	parts := append(strings.Fields(command), args...)
	if len(parts) == 0 {
		return errors.New("empty command")
	}
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return errors.New(msg)
	}
	if err := json.Unmarshal(out, dest); err != nil {
		return err
	}
	return nil
}

func envelopeError[T any](env envelope[T]) error {
	if env.OK {
		return nil
	}
	if len(env.Errors) == 0 {
		return errors.New("hypha returned ok=false")
	}
	body, err := json.Marshal(env.Errors)
	if err != nil {
		return errors.New("hypha returned errors")
	}
	return errors.New(string(body))
}
