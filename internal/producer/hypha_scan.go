package producer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type HyphaScanOptions struct {
	AgentURI string
	Command  string
	SporeAge time.Duration
	Now      time.Time
	Timeout  time.Duration
}

func ScanHyphaMaintenance(ctx context.Context, spaceURI, workspacePath string, opts HyphaScanOptions, runner Runner) []Signal {
	if runner == nil {
		runner = ExecRunner{}
	}
	if opts.AgentURI == "" {
		opts.AgentURI = "agent://reeve/conductor"
	}
	if opts.Command == "" {
		opts.Command = "hypha"
	}
	if opts.SporeAge == 0 {
		opts.SporeAge = 72 * time.Hour
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	if opts.Timeout == 0 {
		opts.Timeout = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	var signals []Signal
	signals = append(signals, scanAgingSpores(ctx, spaceURI, opts, runner)...)
	signals = append(signals, scanStaleAnalyses(ctx, spaceURI, opts, runner)...)
	signals = append(signals, scanDoctorFindings(ctx, spaceURI, opts, runner)...)
	signals = append(signals, scanDeadCode(ctx, spaceURI, workspacePath, opts, runner)...)
	return signals
}

func scanAgingSpores(ctx context.Context, spaceURI string, opts HyphaScanOptions, runner Runner) []Signal {
	res := runner.Run(ctx, "", opts.Command, "spore", "list", "--space", spaceURI, "--status", "unreviewed", "--limit", "100", "--format", "json")
	if res.ExitCode != 0 {
		return nil
	}
	var env struct {
		Data []struct {
			ID          string `json:"id"`
			Status      string `json:"status"`
			SubmittedAt string `json:"submitted_at"`
			Path        string `json:"path"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(res.Output), &env); err != nil {
		return nil
	}
	cutoff := opts.Now.Add(-opts.SporeAge)
	var signals []Signal
	for _, spore := range env.Data {
		if spore.ID == "" || spore.Status != "unreviewed" {
			continue
		}
		submitted, err := time.Parse(time.RFC3339, spore.SubmittedAt)
		if err == nil && submitted.After(cutoff) {
			continue
		}
		signals = append(signals, Signal{
			SpaceURI:  spaceURI,
			Axis:      "maintenance",
			Kind:      "aging-spore",
			Target:    spore.ID,
			Severity:  0.5,
			Title:     "Triage unreviewed spore " + spore.ID,
			Snapshot:  res.Snapshot(),
			CreatedBy: opts.AgentURI,
		})
	}
	return signals
}

func scanStaleAnalyses(ctx context.Context, spaceURI string, opts HyphaScanOptions, runner Runner) []Signal {
	res := runner.Run(ctx, "", opts.Command, "analyze", "list", "--space", spaceURI, "--format", "json")
	if res.ExitCode != 0 {
		return nil
	}
	var root any
	if err := json.Unmarshal([]byte(res.Output), &root); err != nil {
		return nil
	}
	var signals []Signal
	for _, item := range collectMaps(root) {
		id := firstStringAny(item, "id", "analysis_id", "name")
		kind := firstStringAny(item, "kind", "type")
		status := strings.ToLower(firstStringAny(item, "status", "state", "freshness"))
		stale, _ := item["stale"].(bool)
		if id == "" || (!stale && !strings.Contains(status, "stale")) {
			continue
		}
		target := id
		if kind != "" {
			target = kind + ":" + id
		}
		signals = append(signals, Signal{
			SpaceURI:  spaceURI,
			Axis:      "maintenance",
			Kind:      "stale-analysis",
			Target:    target,
			Severity:  0.4,
			Title:     "Refresh stale analysis " + target,
			Snapshot:  res.Snapshot(),
			CreatedBy: opts.AgentURI,
		})
	}
	return signals
}

func scanDoctorFindings(ctx context.Context, spaceURI string, opts HyphaScanOptions, runner Runner) []Signal {
	res := runner.Run(ctx, "", opts.Command, "doctor", "--format", "json")
	if res.ExitCode != 0 {
		return nil
	}
	var root any
	if err := json.Unmarshal([]byte(res.Output), &root); err != nil {
		return nil
	}
	var signals []Signal
	for _, space := range collectMaps(root) {
		if firstStringAny(space, "uri", "space", "space_uri") != spaceURI {
			continue
		}
		for _, finding := range nestedMaps(space, "parse_errors", "warnings", "errors") {
			target := firstStringAny(finding, "path", "name", "id")
			message := firstStringAny(finding, "error", "message", "detail")
			if target == "" && message == "" {
				continue
			}
			if strings.EqualFold(message, "inbox excluded") {
				continue
			}
			if target == "" {
				target = message
			}
			signals = append(signals, Signal{
				SpaceURI:  spaceURI,
				Axis:      "maintenance",
				Kind:      "hypha-doctor",
				Target:    target,
				Severity:  0.6,
				Title:     "Fix Hypha doctor finding " + target,
				Snapshot:  message + "\n\n" + res.Snapshot(),
				CreatedBy: opts.AgentURI,
			})
		}
	}
	return signals
}

func scanDeadCode(ctx context.Context, spaceURI, workspacePath string, opts HyphaScanOptions, runner Runner) []Signal {
	if strings.TrimSpace(workspacePath) == "" {
		return nil
	}
	res := runner.Run(ctx, "", opts.Command, "analyze", "dead", "--space", spaceURI, "--source", workspacePath, "--format", "json")
	if res.ExitCode != 0 {
		return nil
	}
	var root any
	if err := json.Unmarshal([]byte(res.Output), &root); err != nil {
		return nil
	}
	var signals []Signal
	for _, item := range collectMaps(root) {
		target := firstStringAny(item, "symbol", "name", "id", "target")
		if target == "" {
			continue
		}
		signals = append(signals, Signal{
			SpaceURI:  spaceURI,
			Axis:      "maintenance",
			Kind:      "dead-code",
			Target:    target,
			Severity:  0.4,
			Title:     "Remove dead code " + target,
			Snapshot:  res.Snapshot(),
			CreatedBy: opts.AgentURI,
		})
	}
	return signals
}

func collectMaps(v any) []map[string]any {
	var out []map[string]any
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case map[string]any:
			out = append(out, t)
			for _, child := range t {
				walk(child)
			}
		case []any:
			for _, child := range t {
				walk(child)
			}
		}
	}
	walk(v)
	return out
}

func nestedMaps(m map[string]any, keys ...string) []map[string]any {
	var out []map[string]any
	for _, key := range keys {
		if v, ok := m[key]; ok {
			for _, item := range collectMaps(v) {
				out = append(out, item)
			}
		}
	}
	return out
}

func firstStringAny(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			switch x := v.(type) {
			case string:
				return x
			case json.Number:
				return x.String()
			case float64:
				return fmt.Sprintf("%.0f", x)
			}
		}
	}
	return ""
}

func jsonLineObjects(body string) []map[string]any {
	dec := json.NewDecoder(bytes.NewBufferString(body))
	dec.UseNumber()
	var out []map[string]any
	for dec.More() {
		var item map[string]any
		if err := dec.Decode(&item); err != nil {
			return out
		}
		out = append(out, item)
	}
	return out
}
