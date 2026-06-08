package executor

import (
	"context"
	"strings"
	"testing"

	"m31labs.dev/reeve/internal/taskplan"
)

func TestPromptForSpecCarriesTaskContract(t *testing.T) {
	spec := taskplan.CreateSpec{
		Title:       "Fix failing Go tests",
		Description: "The tests are red.\n\n```reeve-task\nkind: red-test\n```",
		Workspace:   "reeve",
		DedupKey:    "abc123",
	}
	prompt := PromptForSpec(spec, "go test ./...")
	for _, want := range []string{
		"Fix failing Go tests",
		"Workspace: reeve",
		"Green check: go test ./...",
		"Dedup key: abc123",
		"approval_gate: allow or approval_gate: ask",
		"```reeve-task",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestRunSpecUsesBuckleyOneShotPrompt(t *testing.T) {
	spec := taskplan.CreateSpec{Title: "Fix build", Description: "body", Workspace: "reeve"}
	run := RunSpec(context.Background(), spec, TaskRunOptions{
		Command:    "buckley",
		Workspace:  "/repo/reeve",
		GreenCheck: "go test ./...",
	}, func(_ context.Context, name string, args []string, dir string) ([]byte, error) {
		if name != "buckley" || dir != "/repo/reeve" {
			t.Fatalf("name=%q dir=%q", name, dir)
		}
		joined := strings.Join(args, "\x00")
		if !strings.Contains(joined, "Fix build") || !strings.Contains(joined, "approval_gate") {
			t.Fatalf("args=%#v", args)
		}
		return []byte("approval_gate: ask"), nil
	})
	if run.Approval != ApprovalAsk || run.ExitCode != 0 {
		t.Fatalf("run=%#v", run)
	}
}
