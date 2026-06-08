package executor

import (
	"context"
	"testing"

	"m31labs.dev/reeve/internal/coord"
	"m31labs.dev/reeve/internal/taskplan"
)

type fakeHypha struct {
	assessment Assessment
	traceID    string
	ticks      []string
	done       []TraceDone
	sporeID    string
}

func (f *fakeHypha) AssessTask(context.Context, coord.Task) (Assessment, error) {
	if f.assessment.Category == "" {
		f.assessment.Category = "aligned"
	}
	return f.assessment, nil
}

func (f *fakeHypha) StartTrace(context.Context, TraceStart) (string, error) {
	if f.traceID == "" {
		f.traceID = "trace-1"
	}
	return f.traceID, nil
}

func (f *fakeHypha) TickTrace(_ context.Context, _ string, _ string, message string) error {
	f.ticks = append(f.ticks, message)
	return nil
}

func (f *fakeHypha) DoneTrace(_ context.Context, done TraceDone) error {
	f.done = append(f.done, done)
	return nil
}

func (f *fakeHypha) SubmitSpore(context.Context, SporeSubmission) (string, error) {
	if f.sporeID == "" {
		f.sporeID = "spore-1"
	}
	return f.sporeID, nil
}

type fakeWorktree struct {
	heads      []string
	dirty      bool
	green      GreenRun
	quarantine string
}

func (f *fakeWorktree) Head(context.Context, string) (string, error) {
	if len(f.heads) == 0 {
		return "same", nil
	}
	head := f.heads[0]
	f.heads = f.heads[1:]
	return head, nil
}

func (f *fakeWorktree) Dirty(context.Context, string) (bool, error) {
	return f.dirty, nil
}

func (f *fakeWorktree) GreenCheck(context.Context, string, string) GreenRun {
	if f.green.Command == "" {
		f.green = GreenRun{Command: "go test ./...", ExitCode: 0}
	}
	return f.green
}

func (f *fakeWorktree) Quarantine(context.Context, string, string) (string, error) {
	if f.quarantine == "" {
		f.quarantine = "/quarantine/task"
	}
	return f.quarantine, nil
}

func TestExecuteLifecycleNoOpClosesTrace(t *testing.T) {
	h := &fakeHypha{}
	wt := &fakeWorktree{heads: []string{"abc", "abc"}}
	result, err := ExecuteLifecycle(context.Background(), managedTask(t), LifecycleOptions{
		WorkspacePath:  "/repo",
		BuckleyCommand: "buckley",
	}, h, wt, func(context.Context, taskplan.CreateSpec, TaskRunOptions) BuckleyRun {
		return BuckleyRun{ExitCode: 0, Approval: ApprovalAllow}
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Decision.State != StateNoOp || !result.TraceDone {
		t.Fatalf("result=%#v", result)
	}
	if len(h.done) != 1 || h.done[0].Status != "succeeded" {
		t.Fatalf("done=%#v", h.done)
	}
}

func TestExecuteLifecycleProposesOnAsk(t *testing.T) {
	h := &fakeHypha{}
	wt := &fakeWorktree{heads: []string{"abc", "def"}, green: GreenRun{Command: "go test ./...", ExitCode: 0}}
	result, err := ExecuteLifecycle(context.Background(), managedTask(t), LifecycleOptions{
		WorkspacePath:  "/repo",
		BuckleyCommand: "buckley",
	}, h, wt, func(context.Context, taskplan.CreateSpec, TaskRunOptions) BuckleyRun {
		return BuckleyRun{ExitCode: 0, Approval: ApprovalAsk}
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Decision.State != StateProposed || result.SporeID != "spore-1" {
		t.Fatalf("result=%#v", result)
	}
	if h.done[0].SporeID != "spore-1" {
		t.Fatalf("done=%#v", h.done)
	}
}

func TestExecuteLifecycleQuarantinesDirtyFailure(t *testing.T) {
	h := &fakeHypha{}
	wt := &fakeWorktree{heads: []string{"abc", "abc"}, dirty: true}
	result, err := ExecuteLifecycle(context.Background(), managedTask(t), LifecycleOptions{
		WorkspacePath:  "/repo",
		BuckleyCommand: "buckley",
	}, h, wt, func(context.Context, taskplan.CreateSpec, TaskRunOptions) BuckleyRun {
		return BuckleyRun{ExitCode: 1, Approval: ApprovalUnknown, Error: "failed"}
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Decision.State != StateFailed || result.Quarantine == "" {
		t.Fatalf("result=%#v", result)
	}
	if h.done[0].Status != "failed" {
		t.Fatalf("done=%#v", h.done)
	}
}

func TestExecuteLifecycleSkipsBelowAssessmentThreshold(t *testing.T) {
	h := &fakeHypha{assessment: Assessment{Category: "misaligned"}}
	wt := &fakeWorktree{}
	result, err := ExecuteLifecycle(context.Background(), managedTask(t), LifecycleOptions{
		WorkspacePath:   "/repo",
		AssessThreshold: "aligned",
		BuckleyCommand:  "buckley",
		HardAssessGate:  true,
	}, h, wt, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Decision.State != StateSkipped {
		t.Fatalf("result=%#v", result)
	}
	if len(h.done) != 0 {
		t.Fatalf("trace should not start for skipped assessment: %#v", h.done)
	}
}

func TestExecuteLifecycleAssessmentIsAdvisoryByDefault(t *testing.T) {
	h := &fakeHypha{assessment: Assessment{Category: "misaligned"}}
	wt := &fakeWorktree{heads: []string{"abc", "abc"}}
	result, err := ExecuteLifecycle(context.Background(), managedTask(t), LifecycleOptions{
		WorkspacePath:   "/repo",
		AssessThreshold: "aligned",
		BuckleyCommand:  "buckley",
	}, h, wt, func(context.Context, taskplan.CreateSpec, TaskRunOptions) BuckleyRun {
		return BuckleyRun{ExitCode: 0, Approval: ApprovalAllow}
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Decision.State != StateNoOp || !result.TraceDone {
		t.Fatalf("result=%#v", result)
	}
}

func managedTask(t *testing.T) coord.Task {
	t.Helper()
	trailer := coord.Trailer{
		DedupKey:   "dedup-1",
		SpaceURI:   "hypha://m31labs/reeve",
		SignalKind: "red-test",
		Target:     "go test ./...",
		Severity:   0.9,
		CreatedBy:  "agent://reeve/conductor",
	}
	description, err := coord.AppendTrailer("Fix tests", trailer)
	if err != nil {
		t.Fatal(err)
	}
	return coord.ClassifyTask("task-1", "Fix tests", description, "pending")
}
