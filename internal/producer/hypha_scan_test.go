package producer

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestScanHyphaMaintenanceEmitsStructuredSignals(t *testing.T) {
	now := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	signals := ScanHyphaMaintenance(context.Background(), "hypha://m31labs/reeve", "/repo/reeve", HyphaScanOptions{
		AgentURI: "agent://reeve/conductor",
		Command:  "hypha",
		Now:      now,
		SporeAge: 24 * time.Hour,
	}, fakeRunner{
		"hypha spore list --space hypha://m31labs/reeve --status unreviewed --limit 100 --format json": {
			Command: "hypha spore list",
			Output:  `{"ok":true,"data":[{"id":"spore.old","status":"unreviewed","submitted_at":"2026-06-06T08:00:00Z"},{"id":"spore.new","status":"unreviewed","submitted_at":"2026-06-08T07:00:00Z"}]}`,
		},
		"hypha analyze list --space hypha://m31labs/reeve --format json": {
			Command: "hypha analyze list",
			Output:  `{"ok":true,"data":[{"id":"analysis.dead","kind":"dead","status":"STALE"},{"id":"analysis.fresh","kind":"refs","status":"fresh"}]}`,
		},
		"hypha doctor --format json": {
			Command: "hypha doctor",
			Output:  `{"ok":true,"data":{"spaces":[{"uri":"hypha://m31labs/reeve","parse_errors":[{"path":"bad.md","error":"bad yaml"},{"path":"inbox","error":"inbox excluded"}]}]}}`,
		},
		"hypha analyze dead --space hypha://m31labs/reeve --source /repo/reeve --format json": {
			Command: "hypha analyze dead",
			Output:  `{"ok":true,"data":{"dead":[{"symbol":"unusedFunc"}]}}`,
		},
	})
	kinds := signalKinds(signals)
	for _, want := range []string{"aging-spore", "stale-analysis", "hypha-doctor", "dead-code"} {
		if !strings.Contains(kinds, want) {
			t.Fatalf("missing %s in %#v", want, signals)
		}
	}
	if strings.Contains(kinds, "spore.new") || strings.Contains(kinds, "inbox") {
		t.Fatalf("unexpected non-actionable signal: %#v", signals)
	}
}

func TestScanHyphaMaintenanceIgnoresCommandFailures(t *testing.T) {
	signals := ScanHyphaMaintenance(context.Background(), "hypha://m31labs/reeve", "/repo/reeve", HyphaScanOptions{}, fakeRunner{
		"hypha spore list --space hypha://m31labs/reeve --status unreviewed --limit 100 --format json": {ExitCode: 1, Output: "boom"},
		"hypha analyze list --space hypha://m31labs/reeve --format json":                               {ExitCode: 1, Output: "boom"},
		"hypha doctor --format json": {ExitCode: 1, Output: "boom"},
		"hypha analyze dead --space hypha://m31labs/reeve --source /repo/reeve --format json": {ExitCode: 1, Output: "boom"},
	})
	if len(signals) != 0 {
		t.Fatalf("signals=%#v", signals)
	}
}

func signalKinds(signals []Signal) string {
	var b strings.Builder
	for _, signal := range signals {
		b.WriteString(signal.Kind)
		b.WriteByte(':')
		b.WriteString(signal.Target)
		b.WriteByte('\n')
	}
	return b.String()
}
