package landing

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestLandPRPushesBranchAndCreatesDraftPR(t *testing.T) {
	var calls []struct {
		name string
		args []string
		dir  string
	}
	result, err := LandPR(context.Background(), Options{
		GHCommand:  "gh",
		BaseBranch: "main",
		Draft:      true,
		Title:      "Fix build",
		Body:       "Automated fix",
		Workdir:    "/worktree",
	}, func(_ context.Context, name string, args []string, dir string) ([]byte, error) {
		calls = append(calls, struct {
			name string
			args []string
			dir  string
		}{name: name, args: append([]string{}, args...), dir: dir})
		switch name {
		case "git":
			if reflect.DeepEqual(args, []string{"rev-parse", "--abbrev-ref", "HEAD"}) {
				return []byte("reeve/m31labs-reeve/task-1\n"), nil
			}
			if reflect.DeepEqual(args, []string{"push", "-u", "origin", "HEAD"}) {
				return []byte("pushed\n"), nil
			}
		case "gh":
			return []byte("https://github.com/M31-Labs/reeve/pull/1\n"), nil
		}
		t.Fatalf("unexpected call %s %#v", name, args)
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Branch != "reeve/m31labs-reeve/task-1" || result.PRURL == "" {
		t.Fatalf("result=%#v", result)
	}
	if len(calls) != 3 {
		t.Fatalf("calls=%#v", calls)
	}
	wantPR := []string{"pr", "create", "--base", "main", "--head", "reeve/m31labs-reeve/task-1", "--title", "Fix build", "--body", "Automated fix", "--draft"}
	if !reflect.DeepEqual(calls[2].args, wantPR) {
		t.Fatalf("pr args=%#v want %#v", calls[2].args, wantPR)
	}
}

func TestLandPRCapturesPushFailure(t *testing.T) {
	result, err := LandPR(context.Background(), Options{
		Title:   "Fix build",
		Workdir: "/worktree",
	}, func(_ context.Context, name string, args []string, dir string) ([]byte, error) {
		if reflect.DeepEqual(args, []string{"rev-parse", "--abbrev-ref", "HEAD"}) {
			return []byte("reeve/task\n"), nil
		}
		return []byte("push rejected"), errors.New("exit 1")
	})
	if err == nil || err.Error() != "push rejected" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if result.Branch != "reeve/task" {
		t.Fatalf("result=%#v", result)
	}
}

func TestLandPRRejectsDetachedHead(t *testing.T) {
	_, err := LandPR(context.Background(), Options{
		Title:   "Fix build",
		Workdir: "/worktree",
	}, func(context.Context, string, []string, string) ([]byte, error) {
		return []byte("HEAD\n"), nil
	})
	if err == nil {
		t.Fatal("expected detached head error")
	}
}
