package graftcoord

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestCommandForUpdate(t *testing.T) {
	got, err := CommandForUpdate("graft", TaskUpdate{
		TaskID:      "task-1",
		Status:      StatusInProgress,
		Assign:      "agent://reeve/conductor",
		Workspace:   "reeve",
		Description: "body",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"graft", "coord", "task", "update", "task-1", "--status", "in_progress", "--assign", "agent://reeve/conductor", "--workspace", "reeve", "--description", "body", "--json"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestCommandForUpdateRejectsUnsupportedStatus(t *testing.T) {
	if _, err := CommandForUpdate("graft", TaskUpdate{TaskID: "task-1", Status: "failed"}); err == nil {
		t.Fatal("expected unsupported status error")
	}
}

func TestUpdateTaskDryRunAndErrorCapture(t *testing.T) {
	called := false
	dry := UpdateTask(context.Background(), "graft", TaskUpdate{TaskID: "task-1", Status: StatusCompleted}, UpdateOptions{
		DryRun: true,
		Exec: func(context.Context, string, []string) ([]byte, error) {
			called = true
			return nil, nil
		},
	})
	if called || !dry.DryRun || dry.Error != "" {
		t.Fatalf("dry=%#v called=%v", dry, called)
	}
	fail := UpdateTask(context.Background(), "graft", TaskUpdate{TaskID: "task-1", Status: StatusBlocked}, UpdateOptions{
		Exec: func(context.Context, string, []string) ([]byte, error) {
			return []byte("boom"), errors.New("exit 1")
		},
	})
	if fail.Error != "boom" {
		t.Fatalf("fail=%#v", fail)
	}
}
