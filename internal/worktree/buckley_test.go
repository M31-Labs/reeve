package worktree

import (
	"context"
	"reflect"
	"testing"
)

func TestBranchForTaskSanitizesSpaceAndTask(t *testing.T) {
	got := BranchForTask("m31labs/reeve", "task:Fix Build")
	want := "reeve/m31labs-reeve/task-fix-build"
	if got != want {
		t.Fatalf("branch=%q want %q", got, want)
	}
}

func TestParseCreatedPath(t *testing.T) {
	got, err := ParseCreatedPath([]byte("✓ Worktree created at /tmp/wt/reeve/branch/source (branch reeve/task)\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "/tmp/wt/reeve/branch/source" {
		t.Fatalf("path=%q", got)
	}
}

func TestCreateWithBuckleyBuildsCommandAndParsesPath(t *testing.T) {
	var gotName, gotDir string
	var gotArgs []string
	wt, err := CreateWithBuckley(context.Background(), "buckley", "/repo/reeve", "/tmp/reeve-worktrees", "reeve/task-1", func(_ context.Context, name string, args []string, dir string) ([]byte, error) {
		gotName = name
		gotArgs = args
		gotDir = dir
		return []byte("✓ Worktree created at /tmp/reeve-worktrees/reeve/reeve/task-1/source (branch reeve/task-1)\n"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	wantArgs := []string{"worktree", "create", "--root", "/tmp/reeve-worktrees", "reeve/task-1"}
	if gotName != "buckley" || gotDir != "/repo/reeve" || !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("name=%q dir=%q args=%#v", gotName, gotDir, gotArgs)
	}
	if wt.Path != "/tmp/reeve-worktrees/reeve/reeve/task-1/source" {
		t.Fatalf("wt=%#v", wt)
	}
}

func TestCreateWithBuckleyRejectsEscapedPath(t *testing.T) {
	_, err := CreateWithBuckley(context.Background(), "buckley", "/repo/reeve", "/tmp/root", "reeve/task-1", func(context.Context, string, []string, string) ([]byte, error) {
		return []byte("✓ Worktree created at /tmp/elsewhere/source (branch reeve/task-1)\n"), nil
	})
	if err == nil {
		t.Fatal("expected escaped path error")
	}
}
