package graftcoord

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"m31labs.dev/reeve/internal/taskplan"
)

func TestCommandForCreateSpec(t *testing.T) {
	spec := taskplan.CreateSpec{
		Action:      "create",
		Title:       "Fix failing Go tests",
		Description: "body",
		Priority:    "p1",
		Workspace:   "reeve",
		Tags:        []string{"reeve", "signal:red-test"},
	}
	got, err := CommandForSpec("graft", spec)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"graft", "coord", "task", "create", "--title", "Fix failing Go tests", "--description", "body", "--priority", "p1", "--workspace", "reeve", "--tag", "reeve", "--tag", "signal:red-test", "--json"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestCommandForReopenSpec(t *testing.T) {
	spec := taskplan.CreateSpec{
		Action:      "reopen",
		Title:       "Fix failing Go tests",
		Description: "regressed",
		PriorTask:   "task-123",
	}
	got, err := CommandForSpec("/bin/graft --profile prod", spec)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/bin/graft", "--profile", "prod", "coord", "task", "reopen", "task-123", "--reason", "regressed", "--json"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestApplyPlanDryRunDoesNotExec(t *testing.T) {
	called := false
	results := ApplyPlan(context.Background(), "graft", []taskplan.CreateSpec{{
		Action:      "create",
		Title:       "Fix build",
		Description: "body",
		Priority:    "p2",
	}}, ApplyOptions{
		DryRun: true,
		Exec: func(context.Context, string, []string) ([]byte, error) {
			called = true
			return nil, nil
		},
	})
	if called {
		t.Fatal("dry-run executed command")
	}
	if len(results) != 1 || !results[0].DryRun || results[0].Error != "" {
		t.Fatalf("unexpected results: %#v", results)
	}
}

func TestApplyPlanExecCapturesTaskIDAndErrors(t *testing.T) {
	results := ApplyPlan(context.Background(), "graft", []taskplan.CreateSpec{
		{Action: "create", Title: "A", Description: "body", Priority: "p2"},
		{Action: "create", Title: "B", Description: "body", Priority: "p2"},
	}, ApplyOptions{
		Exec: func(_ context.Context, _ string, args []string) ([]byte, error) {
			if args[4] == "A" {
				return []byte(`{"task":{"id":"task-9"}}`), nil
			}
			return []byte("boom"), errors.New("exit 1")
		},
	})
	if results[0].TaskID != "task-9" || results[0].Error != "" {
		t.Fatalf("first result=%#v", results[0])
	}
	if results[1].Error != "boom" {
		t.Fatalf("second result=%#v", results[1])
	}
}
