package runner

type CoordStatus string
type WorktreeState string
type ReconcileAction string

const (
	CoordPending    CoordStatus = "pending"
	CoordQueued     CoordStatus = "queued"
	CoordInProgress CoordStatus = "in_progress"
	CoordCompleted  CoordStatus = "completed"

	WorktreeUnknown     WorktreeState = "unknown"
	WorktreeClean       WorktreeState = "clean"
	WorktreeDirty       WorktreeState = "dirty"
	WorktreePartial     WorktreeState = "partial_commit"
	WorktreeCleanCommit WorktreeState = "clean_commit"

	ActionLeaveQueued ReconcileAction = "leave_queued"
	ActionAdopt       ReconcileAction = "adopt_executing"
	ActionRequeue     ReconcileAction = "requeue"
	ActionQuarantine  ReconcileAction = "quarantine_and_requeue"
	ActionResume      ReconcileAction = "resume_green_check"
	ActionLeaveDone   ReconcileAction = "leave_done"
	ActionBlock       ReconcileAction = "block_unmanaged"
)

type ReconcileInput struct {
	CoordStatus CoordStatus   `json:"coord_status"`
	TraceActive bool          `json:"trace_active"`
	Worktree    WorktreeState `json:"worktree"`
}

type ReconcileDecision struct {
	Action ReconcileAction `json:"action"`
	Reason string          `json:"reason"`
}

func Reconcile(input ReconcileInput) ReconcileDecision {
	switch input.CoordStatus {
	case CoordPending, CoordQueued:
		return ReconcileDecision{Action: ActionLeaveQueued, Reason: "task is not assigned"}
	case CoordInProgress:
		if input.TraceActive {
			return ReconcileDecision{Action: ActionAdopt, Reason: "active trace still owns task"}
		}
		switch input.Worktree {
		case WorktreeClean, WorktreeUnknown:
			return ReconcileDecision{Action: ActionRequeue, Reason: "no active trace and no committed work"}
		case WorktreeDirty, WorktreePartial:
			return ReconcileDecision{Action: ActionQuarantine, Reason: "partial work must not be reused"}
		case WorktreeCleanCommit:
			return ReconcileDecision{Action: ActionResume, Reason: "clean commit can resume at green check"}
		default:
			return ReconcileDecision{Action: ActionBlock, Reason: "unknown worktree state"}
		}
	case CoordCompleted:
		return ReconcileDecision{Action: ActionLeaveDone, Reason: "terminal coord state"}
	default:
		return ReconcileDecision{Action: ActionBlock, Reason: "unknown coord status"}
	}
}
