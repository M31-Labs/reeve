package executor

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestCommandForBuckleyOneShot(t *testing.T) {
	got, err := CommandForBuckley(BuckleyInvocation{
		Command: "/home/draco/go/bin/buckley",
		Prompt:  "Fix the failing tests",
		Model:   "codex/gpt-5",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/home/draco/go/bin/buckley", "--plain", "--no-color", "-m", "codex/gpt-5", "-p", "Fix the failing tests"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestRunBuckleyParsesApprovalAndExit(t *testing.T) {
	run := RunBuckley(context.Background(), BuckleyInvocation{
		Command: "buckley",
		Workdir: "/repo",
		Prompt:  "Do work",
		Timeout: time.Second,
	}, func(_ context.Context, name string, args []string, dir string) ([]byte, error) {
		if name != "buckley" || dir != "/repo" {
			t.Fatalf("name=%q dir=%q", name, dir)
		}
		return []byte(`{"approval_gate":"allow"}`), nil
	})
	if run.ExitCode != 0 || run.Approval != ApprovalAllow || run.Error != "" {
		t.Fatalf("run=%#v", run)
	}
}

func TestRunBuckleyCapturesFailure(t *testing.T) {
	run := RunBuckley(context.Background(), BuckleyInvocation{
		Command: "buckley",
		Prompt:  "Do work",
	}, func(context.Context, string, []string, string) ([]byte, error) {
		return []byte("nope"), errors.New("failed")
	})
	if run.ExitCode != -1 || run.Error != "nope" || run.Approval != ApprovalUnknown {
		t.Fatalf("run=%#v", run)
	}
}
