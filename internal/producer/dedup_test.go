package producer

import (
	"testing"

	"m31labs.dev/reeve/internal/coord"
)

func TestPlanSkipsDuplicateOpenTask(t *testing.T) {
	signal := Signal{SpaceURI: "hypha://m31labs/reeve", Axis: "maintenance", Kind: "red-build", Target: "./...", Severity: 0.9, CreatedBy: "agent://reeve/conductor"}
	key := DedupKey(signal)
	existing := []coord.Task{{ID: "task-1", Status: "pending", Managed: true, Trailer: coord.Trailer{DedupKey: key}}}
	actions := Plan([]Signal{signal}, existing)
	if len(actions) != 1 {
		t.Fatalf("len=%d", len(actions))
	}
	if actions[0].Kind != ActionSkip || actions[0].PriorTask != "task-1" {
		t.Fatalf("action=%#v", actions[0])
	}
}

func TestPlanReopensCompletedRegression(t *testing.T) {
	signal := Signal{SpaceURI: "hypha://m31labs/reeve", Kind: "red-build", Target: "./...", Severity: 0.9, CreatedBy: "agent://reeve/conductor"}
	key := DedupKey(signal)
	existing := []coord.Task{{ID: "task-1", Status: "completed", Managed: true, Trailer: coord.Trailer{DedupKey: key}}}
	actions := Plan([]Signal{signal}, existing)
	if actions[0].Kind != ActionReopen {
		t.Fatalf("kind=%s", actions[0].Kind)
	}
	if actions[0].Trailer.PreviousTaskID != "task-1" {
		t.Fatalf("previous=%q", actions[0].Trailer.PreviousTaskID)
	}
}

func TestPlanSkipsDuplicateSignalsInSameBatch(t *testing.T) {
	signal := Signal{SpaceURI: "hypha://m31labs/reeve", Kind: "doctor", Target: "finding-1", Severity: 0.5, CreatedBy: "agent://reeve/conductor"}
	actions := Plan([]Signal{signal, signal}, nil)
	if actions[0].Kind != ActionCreate {
		t.Fatalf("first=%s", actions[0].Kind)
	}
	if actions[1].Kind != ActionSkip {
		t.Fatalf("second=%s", actions[1].Kind)
	}
}
