package executor

import "testing"

func TestClassifySuccessContract(t *testing.T) {
	tests := []struct {
		name string
		in   Result
		want TerminalState
		q    bool
	}{
		{"no op", Result{BuckleyExitCode: 0}, StateNoOp, false},
		{"buckley fail", Result{BuckleyExitCode: 1}, StateFailed, true},
		{"dirty no commit", Result{DirtyWorktree: true}, StateFailed, true},
		{"partial commit", Result{NewCommit: true, DirtyWorktree: true}, StateFailed, true},
		{"green fail proposes", Result{NewCommit: true, Green: GreenFail, Approval: ApprovalAllow}, StateProposed, false},
		{"ask proposes", Result{NewCommit: true, Green: GreenPass, Approval: ApprovalAsk}, StateProposed, false},
		{"unknown proposes", Result{NewCommit: true, Green: GreenPass, Approval: ApprovalUnknown}, StateProposed, false},
		{"already red proposes", Result{NewCommit: true, Green: GreenPass, Approval: ApprovalAllow, BaseWasRed: true}, StateProposed, false},
		{"allowed lands", Result{NewCommit: true, Green: GreenPass, Approval: ApprovalAllow}, StateLanded, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.in)
			if got.State != tt.want || got.Quarantine != tt.q {
				t.Fatalf("got=%#v want=%s quarantine=%v", got, tt.want, tt.q)
			}
		})
	}
}

func TestParseApprovalDefaultsAmbiguousToUnknown(t *testing.T) {
	if got := ParseApproval(`{"approval_gate":"allow"}`); got != ApprovalAllow {
		t.Fatalf("allow parsed as %s", got)
	}
	if got := ParseApproval(`approval_gate: ask`); got != ApprovalAsk {
		t.Fatalf("ask parsed as %s", got)
	}
	if got := ParseApproval(`looks safe maybe`); got != ApprovalUnknown {
		t.Fatalf("ambiguous parsed as %s", got)
	}
}
