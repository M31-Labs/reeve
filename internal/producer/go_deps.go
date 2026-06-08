package producer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type GoDepScanOptions struct {
	AgentURI string
	GoBin    string
	Timeout  time.Duration
}

func ScanGoDependencies(ctx context.Context, spaceURI, workspacePath string, opts GoDepScanOptions, runner Runner) []Signal {
	if runner == nil {
		runner = ExecRunner{}
	}
	if opts.AgentURI == "" {
		opts.AgentURI = "agent://reeve/conductor"
	}
	if opts.GoBin == "" {
		opts.GoBin = "go"
	}
	if opts.Timeout == 0 {
		opts.Timeout = 2 * time.Minute
	}
	if _, err := os.Stat(filepath.Join(workspacePath, "go.mod")); err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	res := runner.Run(ctx, workspacePath, opts.GoBin, "list", "-m", "-u", "-json", "all")
	if res.ExitCode != 0 {
		return nil
	}
	var signals []Signal
	for _, mod := range parseGoModules(res.Output) {
		path := firstStringAny(mod, "Path")
		if path == "" || path == "command-line-arguments" {
			continue
		}
		update, ok := mod["Update"].(map[string]any)
		if !ok {
			continue
		}
		version := firstStringAny(update, "Version")
		if version == "" {
			continue
		}
		target := path + "@" + version
		signals = append(signals, Signal{
			SpaceURI:  spaceURI,
			Axis:      "maintenance",
			Kind:      "outdated-dep",
			Target:    target,
			Severity:  0.7,
			Title:     "Bump " + path + " to " + version,
			Snapshot:  res.Snapshot(),
			CreatedBy: opts.AgentURI,
		})
	}
	return signals
}

func parseGoModules(body string) []map[string]any {
	dec := json.NewDecoder(strings.NewReader(body))
	dec.UseNumber()
	var out []map[string]any
	for {
		var item map[string]any
		err := dec.Decode(&item)
		if errors.Is(err, io.EOF) {
			return out
		}
		if err != nil {
			return out
		}
		out = append(out, item)
	}
}
