package runner

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"m31labs.dev/reeve/internal/config"
	"m31labs.dev/reeve/internal/coord"
	"m31labs.dev/reeve/internal/executor"
	"m31labs.dev/reeve/internal/taskplan"
	_ "modernc.org/sqlite"
)

type execFakeHypha struct {
	done []executor.TraceDone
}

func (f *execFakeHypha) AssessTask(context.Context, coord.Task) (executor.Assessment, error) {
	return executor.Assessment{Category: "aligned"}, nil
}

func (f *execFakeHypha) StartTrace(context.Context, executor.TraceStart) (string, error) {
	return "trace-1", nil
}

func (f *execFakeHypha) TickTrace(context.Context, string, string, string) error {
	return nil
}

func (f *execFakeHypha) DoneTrace(_ context.Context, done executor.TraceDone) error {
	f.done = append(f.done, done)
	return nil
}

func (f *execFakeHypha) SubmitSpore(context.Context, executor.SporeSubmission) (string, error) {
	return "spore-1", nil
}

type execFakeWorktree struct {
	heads      []string
	dirty      bool
	green      executor.GreenRun
	quarantine string
}

func (f *execFakeWorktree) Head(context.Context, string) (string, error) {
	if len(f.heads) == 0 {
		return "same", nil
	}
	head := f.heads[0]
	f.heads = f.heads[1:]
	return head, nil
}

func (f *execFakeWorktree) Dirty(context.Context, string) (bool, error) {
	return f.dirty, nil
}

func (f *execFakeWorktree) GreenCheck(context.Context, string, string) executor.GreenRun {
	if f.green.Command == "" {
		f.green = executor.GreenRun{Command: "go test ./...", ExitCode: 0}
	}
	return f.green
}

func (f *execFakeWorktree) Quarantine(context.Context, string, string) (string, error) {
	if f.quarantine == "" {
		f.quarantine = "/tmp/quarantine/task"
	}
	return f.quarantine, nil
}

func TestExecuteOnceDryRunSelectsHighestManagedTask(t *testing.T) {
	tmp := t.TempDir()
	dbPath := writeExecutionIndex(t, tmp, `{"uri":"hypha://m31labs/reeve","mode":"maintenance","priority":0.9}`)
	descLow := managedDescription(t, "low", "hypha://m31labs/reeve", 0.2, 0)
	descHigh := managedDescription(t, "high", "hypha://m31labs/reeve", 0.95, 0)
	cfg := executionConfig(t, tmp, dbPath, []string{
		taskJSON("task-low", "Low", "pending", descLow),
		taskJSON("task-high", "High", "pending", descHigh),
	})
	report, err := ExecuteOnce(context.Background(), cfg, ExecutionOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Selected == nil || report.Selected.TaskID != "task-high" {
		t.Fatalf("selected=%#v", report.Selected)
	}
	if len(report.Updates) != 0 || report.Result != nil {
		t.Fatalf("dry run should not mutate or execute: %#v", report)
	}
}

func TestExecuteOnceUpdatesStatusAroundLifecycle(t *testing.T) {
	tmp := t.TempDir()
	dbPath := writeExecutionIndex(t, tmp, `{"uri":"hypha://m31labs/reeve","mode":"maintenance","priority":0.8}`)
	desc := managedDescription(t, "build", "hypha://m31labs/reeve", 0.9, 0)
	cfg := executionConfig(t, tmp, dbPath, []string{taskJSON("task-1", "Fix build", "pending", desc)})
	var updates []string
	report, err := ExecuteOnce(context.Background(), cfg, ExecutionOptions{
		Hypha:                    &execFakeHypha{},
		Worktree:                 &execFakeWorktree{heads: []string{"abc", "def"}},
		AllowRegisteredWorkspace: true,
		RunBuckley: func(context.Context, taskplan.CreateSpec, executor.TaskRunOptions) executor.BuckleyRun {
			return executor.BuckleyRun{ExitCode: 0, Approval: executor.ApprovalAllow}
		},
		UpdateExec: func(_ context.Context, _ string, args []string) ([]byte, error) {
			updates = append(updates, strings.Join(args, " "))
			return []byte(`{"ok":true}`), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Result == nil || report.Result.Decision.State != executor.StateLanded {
		t.Fatalf("result=%#v", report.Result)
	}
	joined := strings.Join(updates, "\n")
	for _, want := range []string{"--status in_progress", "--status completed"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("updates missing %q:\n%s", want, joined)
		}
	}
}

func TestExecuteOnceRetriesFailedLifecycle(t *testing.T) {
	tmp := t.TempDir()
	dbPath := writeExecutionIndex(t, tmp, `{"uri":"hypha://m31labs/reeve","mode":"maintenance","priority":0.8}`)
	desc := managedDescription(t, "build", "hypha://m31labs/reeve", 0.9, 0)
	cfg := executionConfig(t, tmp, dbPath, []string{taskJSON("task-1", "Fix build", "pending", desc)})
	var terminalDescription string
	report, err := ExecuteOnce(context.Background(), cfg, ExecutionOptions{
		Hypha:                    &execFakeHypha{},
		Worktree:                 &execFakeWorktree{heads: []string{"abc", "abc"}, dirty: true},
		AllowRegisteredWorkspace: true,
		RunBuckley: func(context.Context, taskplan.CreateSpec, executor.TaskRunOptions) executor.BuckleyRun {
			return executor.BuckleyRun{ExitCode: 1, Approval: executor.ApprovalUnknown, Error: "failed"}
		},
		UpdateExec: func(_ context.Context, _ string, args []string) ([]byte, error) {
			for i, arg := range args {
				if arg == "--description" && i+1 < len(args) {
					terminalDescription = args[i+1]
				}
			}
			return []byte(`{"ok":true}`), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Result == nil || report.Result.Decision.State != executor.StateFailed {
		t.Fatalf("result=%#v", report.Result)
	}
	trailer, err := coord.ParseTrailer(terminalDescription)
	if err != nil {
		t.Fatal(err)
	}
	if trailer.RetryCount != 1 || report.Updates[len(report.Updates)-1].Status != "pending" {
		t.Fatalf("retry=%d updates=%#v", trailer.RetryCount, report.Updates)
	}
}

func writeExecutionIndex(t *testing.T, dir string, metadata string) string {
	t.Helper()
	dbPath := dir + "/hyphae.db"
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
		"space.m31labs-reeve", "space", "m31labs/reeve", "Reeve", metadata); err != nil {
		t.Fatal(err)
	}
	return dbPath
}

func executionConfig(t *testing.T, dir, dbPath string, tasks []string) config.Config {
	t.Helper()
	workspacePath := dir + "/state/worktrees/reeve"
	hypha := writeExecutable(t, dir, "hypha", `#!/bin/sh
case "$1 $2" in
  "trace list") echo '{"ok":true,"data":[],"warnings":[],"errors":[]}' ;;
  "spore list") echo '{"ok":true,"data":[],"warnings":[],"errors":[]}' ;;
  *) echo '{"ok":true,"data":[],"warnings":[],"errors":[]}' ;;
esac
`)
	graftBody := `{"workspaces":[{"name":"reeve","path":"` + workspacePath + `"}],"tasks":[` + strings.Join(tasks, ",") + `]}`
	graft := writeExecutable(t, dir, "graft", `#!/bin/sh
case "$1 $2 $3" in
  "workspace list --json") printf '%s\n' '`+graftBody+`' ;;
  "coord task list") printf '%s\n' '`+graftBody+`' ;;
  *) echo '{"ok":true}' ;;
esac
`)
	cfg, err := config.Normalize(config.Config{
		AgentURI:       "agent://reeve/conductor",
		HyphaIndexPath: dbPath,
		StateDir:       dir + "/state",
		QuarantineDir:  dir + "/quarantine",
		MaxRetries:     3,
		Commands: config.Commands{
			Hypha:   hypha,
			Graft:   graft,
			Buckley: "buckley",
			Go:      "go",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func managedDescription(t *testing.T, key, space string, severity float64, retry int) string {
	t.Helper()
	desc, err := coord.AppendTrailer("Generated task.", coord.Trailer{
		DedupKey:   key,
		SpaceURI:   space,
		SignalKind: "red-build",
		Target:     "go build ./...",
		Severity:   severity,
		CreatedBy:  "agent://reeve/conductor",
		RetryCount: retry,
	})
	if err != nil {
		t.Fatal(err)
	}
	return desc
}

func taskJSON(id, title, status, description string) string {
	return `{"id":"` + id + `","title":"` + title + `","status":"` + status + `","description":` + quoteJSON(description) + `}`
}

func quoteJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return `"` + s + `"`
}
