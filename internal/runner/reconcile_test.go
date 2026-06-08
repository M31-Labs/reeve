package runner

import "testing"

func TestReconcileSpecTable(t *testing.T) {
	tests := []struct {
		name string
		in   ReconcileInput
		want ReconcileAction
	}{
		{"pending", ReconcileInput{CoordStatus: CoordPending}, ActionLeaveQueued},
		{"queued", ReconcileInput{CoordStatus: CoordQueued}, ActionLeaveQueued},
		{"in progress active trace", ReconcileInput{CoordStatus: CoordInProgress, TraceActive: true}, ActionAdopt},
		{"in progress no trace clean", ReconcileInput{CoordStatus: CoordInProgress, Worktree: WorktreeClean}, ActionRequeue},
		{"in progress no trace dirty", ReconcileInput{CoordStatus: CoordInProgress, Worktree: WorktreeDirty}, ActionQuarantine},
		{"in progress no trace partial", ReconcileInput{CoordStatus: CoordInProgress, Worktree: WorktreePartial}, ActionQuarantine},
		{"in progress no trace clean commit", ReconcileInput{CoordStatus: CoordInProgress, Worktree: WorktreeCleanCommit}, ActionResume},
		{"completed", ReconcileInput{CoordStatus: CoordCompleted}, ActionLeaveDone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Reconcile(tt.in); got.Action != tt.want {
				t.Fatalf("action=%s want=%s reason=%s", got.Action, tt.want, got.Reason)
			}
		})
	}
}
