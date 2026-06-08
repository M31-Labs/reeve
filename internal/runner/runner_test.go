package runner

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"m31labs.dev/reeve/internal/config"
	_ "modernc.org/sqlite"
)

func TestBuildReportWithPathShimsSelectsEligibleCandidate(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "hyphae.db")
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Exec(`CREATE TABLE objects (
		id TEXT, type TEXT, space_id TEXT, title TEXT, metadata_json TEXT
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`INSERT INTO objects VALUES (?, ?, ?, ?, ?)`,
		"space.m31labs-reeve", "space", "m31labs/reeve", "Reeve", `{"uri":"hypha://m31labs/reeve","mode":"maintenance","priority":0.7}`); err != nil {
		t.Fatal(err)
	}
	hypha := writeExecutable(t, tmp, "hypha", `#!/bin/sh
case "$1 $2" in
  "trace list") echo '{"ok":true,"data":[],"warnings":[],"errors":[]}' ;;
  "spore list") echo '{"ok":true,"data":[],"warnings":[],"errors":[]}' ;;
  *) echo '{"ok":true,"data":[],"warnings":[],"errors":[]}' ;;
esac
`)
	graft := writeExecutable(t, tmp, "graft", `#!/bin/sh
case "$1 $2 $3 $4" in
  "workspace list --json ") echo '{"workspaces":[{"name":"reeve","path":"/repo/reeve"}]}' ;;
  "coord task list --all") echo '{"tasks":[]}' ;;
  *) echo '{"tasks":[]}' ;;
esac
`)
	cfg, err := config.Normalize(config.Config{
		HyphaIndexPath: dbPath,
		StateDir:       filepath.Join(tmp, "state"),
		Commands: config.Commands{
			Hypha:   hypha,
			Graft:   graft,
			Buckley: "buckley",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	report, err := BuildReport(context.Background(), cfg, true)
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.EligibleSpaces != 1 {
		t.Fatalf("eligible=%d report=%#v", report.Summary.EligibleSpaces, report)
	}
	if report.Selected == nil || report.Selected.SpaceID != "m31labs/reeve" {
		t.Fatalf("selected=%#v", report.Selected)
	}
	if len(report.Warnings) != 0 {
		t.Fatalf("warnings=%#v", report.Warnings)
	}
}

func TestBuildReportScanAggregatesDeterministicProducerSignals(t *testing.T) {
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module example.com/reeve\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(tmp, "hyphae.db")
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Exec(`CREATE TABLE objects (
		id TEXT, type TEXT, space_id TEXT, title TEXT, metadata_json TEXT
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`INSERT INTO objects VALUES (?, ?, ?, ?, ?)`,
		"space.m31labs-reeve", "space", "m31labs/reeve", "Reeve", `{"uri":"hypha://m31labs/reeve","mode":"maintenance","priority":0.7}`); err != nil {
		t.Fatal(err)
	}
	hypha := writeExecutable(t, tmp, "hypha", `#!/bin/sh
case "$1 $2" in
  "trace list") echo '{"ok":true,"data":[],"warnings":[],"errors":[]}' ;;
  "spore list") echo '{"ok":true,"data":[{"id":"spore.old","status":"unreviewed","submitted_at":"2026-06-01T00:00:00Z"}],"warnings":[],"errors":[]}' ;;
  "analyze list") echo '{"ok":true,"data":[{"id":"analysis.dead","kind":"dead","status":"STALE"}]}' ;;
  "analyze dead") echo '{"ok":true,"data":{"dead":[{"symbol":"unusedFunc"}]}}' ;;
  "doctor --format") echo '{"ok":true,"data":{"spaces":[{"uri":"hypha://m31labs/reeve","parse_errors":[{"path":"bad.md","error":"bad yaml"}]}]}}' ;;
  *) echo '{"ok":true,"data":[],"warnings":[],"errors":[]}' ;;
esac
`)
	graft := writeExecutable(t, tmp, "graft", `#!/bin/sh
case "$1 $2 $3 $4" in
  "workspace list --json ") echo '{"workspaces":[{"name":"reeve","path":"`+repo+`"}]}' ;;
  "coord task list --all") echo '{"tasks":[]}' ;;
  *) echo '{"tasks":[]}' ;;
esac
`)
	goBin := writeExecutable(t, tmp, "go", `#!/bin/sh
case "$1 $2 $3 $4" in
  "build ./...  ") exit 0 ;;
  "test ./...  ") exit 0 ;;
  "list -m -u -json") printf '%s\n%s\n' '{"Path":"example.com/reeve","Version":"v0.0.0"}' '{"Path":"golang.org/x/tools","Version":"v0.1.0","Update":{"Path":"golang.org/x/tools","Version":"v0.2.0"}}' ;;
  *) exit 0 ;;
esac
`)
	cfg, err := config.Normalize(config.Config{
		HyphaIndexPath: dbPath,
		StateDir:       filepath.Join(tmp, "state"),
		Commands: config.Commands{
			Hypha:   hypha,
			Graft:   graft,
			Buckley: "buckley",
			Go:      goBin,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	report, err := BuildReportWithOptions(context.Background(), cfg, Options{DryRun: true, Scan: true})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, item := range report.Plan {
		got[item.Spec.Action+":"+item.Spec.DedupKey] = true
		for _, tag := range item.Spec.Tags {
			if len(tag) > len("signal:") && tag[:len("signal:")] == "signal:" {
				got[tag] = true
			}
		}
	}
	for _, want := range []string{"signal:outdated-dep", "signal:aging-spore", "signal:stale-analysis", "signal:hypha-doctor", "signal:dead-code"} {
		if !got[want] {
			t.Fatalf("missing %s in plan=%#v", want, report.Plan)
		}
	}
}

func writeExecutable(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
