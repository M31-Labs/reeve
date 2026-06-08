package executor

import (
	"strings"
	"testing"

	"m31labs.dev/reeve/internal/coord"
)

func TestRenderProposalSporeIncludesRequiredFields(t *testing.T) {
	task := managedTask(t)
	body := renderProposalSpore("spore.test", "identity://m31labs/reeve-conductor", SporeSubmission{
		Task:     task,
		Decision: Decision{State: StateProposed, Reason: "review required"},
		Buckley:  BuckleyRun{Approval: ApprovalAsk},
		Green:    GreenRun{Command: "go test ./...", ExitCode: 1},
	})
	for _, want := range []string{
		"id: spore.test",
		"type: spore",
		"space: hypha://m31labs/reeve",
		"source_refs:",
		"hypha://m31labs/reeve/object/spec.reeve-conductor-v1",
		"Decision: proposed",
		"Signed by: identity://m31labs/reeve-conductor",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("spore missing %q:\n%s", want, body)
		}
	}
}

func TestSporeIDSanitizesTaskID(t *testing.T) {
	got := sporeID("Task 123/ABC")
	if !strings.HasPrefix(got, "spore.") || !strings.HasSuffix(got, ".reeve.task-123-abc") {
		t.Fatalf("spore id=%q", got)
	}
}

var _ = coord.Task{}
