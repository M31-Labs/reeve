package executor

import "strings"

type Approval string
type Green string
type TerminalState string

const (
	ApprovalAllow   Approval = "allow"
	ApprovalAsk     Approval = "ask"
	ApprovalUnknown Approval = "unknown"

	GreenPass Green = "pass"
	GreenFail Green = "fail"
	GreenSkip Green = "skip"

	StateNoOp     TerminalState = "no-op"
	StateSkipped  TerminalState = "skipped"
	StateFailed   TerminalState = "failed"
	StateLanded   TerminalState = "landed"
	StateProposed TerminalState = "proposed"
)

type Result struct {
	BuckleyExitCode int
	NewCommit       bool
	DirtyWorktree   bool
	Approval        Approval
	Green           Green
	BaseWasRed      bool
	TaskFixesRed    bool
}

type Decision struct {
	State      TerminalState `json:"state"`
	Quarantine bool          `json:"quarantine"`
	Reason     string        `json:"reason"`
}

func Classify(r Result) Decision {
	if r.BuckleyExitCode != 0 {
		return Decision{State: StateFailed, Quarantine: true, Reason: "buckley failed"}
	}
	if r.DirtyWorktree && !r.NewCommit {
		return Decision{State: StateFailed, Quarantine: true, Reason: "dirty worktree without clean commit"}
	}
	if !r.NewCommit {
		return Decision{State: StateNoOp, Reason: "buckley succeeded without a new commit"}
	}
	if r.DirtyWorktree {
		return Decision{State: StateFailed, Quarantine: true, Reason: "partial commit or dirty worktree"}
	}
	if r.BaseWasRed && !r.TaskFixesRed {
		return Decision{State: StateProposed, Reason: "base branch was already red"}
	}
	if r.Green != GreenPass {
		return Decision{State: StateProposed, Reason: "green check did not pass"}
	}
	if r.Approval == ApprovalAllow {
		return Decision{State: StateLanded, Reason: "approval allowed and green check passed"}
	}
	return Decision{State: StateProposed, Reason: "approval requires review or is ambiguous"}
}

func ParseApproval(output string) Approval {
	lower := strings.ToLower(output)
	switch {
	case strings.Contains(lower, "approval_gate") && strings.Contains(lower, "allow"):
		return ApprovalAllow
	case strings.Contains(lower, "approval_gate") && strings.Contains(lower, "ask"):
		return ApprovalAsk
	case strings.Contains(lower, `"approval"`) && strings.Contains(lower, `"allow"`):
		return ApprovalAllow
	case strings.Contains(lower, `"approval"`) && strings.Contains(lower, `"ask"`):
		return ApprovalAsk
	default:
		return ApprovalUnknown
	}
}
