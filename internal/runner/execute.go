package runner

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"m31labs.dev/reeve/internal/config"
	"m31labs.dev/reeve/internal/coord"
	"m31labs.dev/reeve/internal/executor"
	"m31labs.dev/reeve/internal/fleet"
	"m31labs.dev/reeve/internal/graftcoord"
	"m31labs.dev/reeve/internal/ranking"
)

type ExecutionOptions struct {
	DryRun                   bool
	AllowRegisteredWorkspace bool
	Hypha                    executor.HyphaRituals
	Worktree                 executor.WorktreeOps
	RunBuckley               executor.BuckleySpecRunner
	UpdateExec               graftcoord.ExecFunc
}

type ExecutionReport struct {
	GeneratedAt string                    `json:"generated_at"`
	DryRun      bool                      `json:"dry_run"`
	Selected    *ExecutableCandidate      `json:"selected,omitempty"`
	Updates     []graftcoord.UpdateResult `json:"updates,omitempty"`
	Result      *executor.LifecycleResult `json:"result,omitempty"`
	Warnings    []string                  `json:"warnings,omitempty"`
}

type ExecutableCandidate struct {
	TaskID        string          `json:"task_id"`
	Title         string          `json:"title"`
	Status        string          `json:"status"`
	SpaceID       string          `json:"space_id"`
	SpaceURI      string          `json:"space_uri"`
	WorkspaceName string          `json:"workspace_name"`
	WorkspacePath string          `json:"workspace_path"`
	SignalKind    string          `json:"signal_kind"`
	Target        string          `json:"target"`
	Factors       ranking.Factors `json:"factors"`
	Score         float64         `json:"score"`
}

func ExecuteOnce(ctx context.Context, cfg config.Config, opts ExecutionOptions) (ExecutionReport, error) {
	report, err := BuildReportWithOptions(ctx, cfg, Options{DryRun: true})
	if err != nil {
		return ExecutionReport{}, err
	}
	out := ExecutionReport{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		DryRun:      opts.DryRun,
		Warnings:    report.Warnings,
	}
	candidates := executableCandidates(report.Spaces, report.Coord.Tasks, cfg)
	if len(candidates) == 0 {
		return out, nil
	}
	selected := candidates[0]
	out.Selected = &selected
	if opts.DryRun {
		return out, nil
	}
	task, space, ok := selectedTaskAndSpace(selected, report.Coord.Tasks, report.Spaces)
	if !ok {
		return out, errors.New("selected task or space disappeared")
	}
	worktreeRoot := filepath.Join(cfg.StateDir, "worktrees")
	if !opts.AllowRegisteredWorkspace && !pathWithin(worktreeRoot, space.WorkspacePath) {
		return out, fmt.Errorf("selected workspace %s is not below isolated worktree root %s", space.WorkspacePath, worktreeRoot)
	}
	if opts.Hypha == nil {
		opts.Hypha = executor.ShellHypha{Command: cfg.Commands.Hypha, SigningIdentity: cfg.SigningIdentity}
	}
	if opts.Worktree == nil {
		opts.Worktree = executor.ShellWorktree{QuarantineDir: cfg.QuarantineDir, WorktreeRoot: worktreeRoot}
	}
	updateOpts := graftcoord.UpdateOptions{Exec: opts.UpdateExec}
	start := graftcoord.UpdateTask(ctx, cfg.Commands.Graft, graftcoord.TaskUpdate{
		TaskID:    task.ID,
		Status:    graftcoord.StatusInProgress,
		Assign:    cfg.AgentURI,
		Workspace: space.WorkspaceName,
	}, updateOpts)
	out.Updates = append(out.Updates, start)
	if start.Error != "" {
		return out, fmt.Errorf("mark task in progress: %s", start.Error)
	}
	result, lifecycleErr := executor.ExecuteLifecycle(ctx, task, executor.LifecycleOptions{
		AgentURI:        cfg.AgentURI,
		AssessThreshold: cfg.AssessThreshold,
		WorkspaceName:   space.WorkspaceName,
		WorkspacePath:   space.WorkspacePath,
		GreenCheck:      space.GreenCheck,
		BuckleyCommand:  cfg.Commands.Buckley,
	}, opts.Hypha, opts.Worktree, opts.RunBuckley)
	out.Result = &result
	status, description, reason := terminalUpdate(task, result.Decision, lifecycleErr, cfg.MaxRetries)
	done := graftcoord.UpdateTask(ctx, cfg.Commands.Graft, graftcoord.TaskUpdate{
		TaskID:      task.ID,
		Status:      status,
		Description: description,
	}, updateOpts)
	out.Updates = append(out.Updates, done)
	if done.Error != "" {
		return out, fmt.Errorf("write terminal task status after %s: %s", reason, done.Error)
	}
	if lifecycleErr != nil {
		return out, lifecycleErr
	}
	return out, nil
}

func executableCandidates(spaces []fleet.SpaceStatus, tasks []coord.Task, cfg config.Config) []ExecutableCandidate {
	byURI := map[string]fleet.SpaceStatus{}
	for _, space := range spaces {
		if space.Eligible {
			byURI[space.URI] = space
		}
	}
	var out []ExecutableCandidate
	for _, task := range tasks {
		if !task.Managed || !openForExecution(task.Status) {
			continue
		}
		space, ok := byURI[task.Trailer.SpaceURI]
		if !ok {
			continue
		}
		factors := ranking.Factors{
			GapSeverity:     task.Trailer.Severity,
			ProjectPriority: space.Priority,
			Staleness:       0.5,
			BlastRadius:     0.5,
		}
		out = append(out, ExecutableCandidate{
			TaskID:        task.ID,
			Title:         task.Title,
			Status:        task.Status,
			SpaceID:       space.SpaceID,
			SpaceURI:      space.URI,
			WorkspaceName: space.WorkspaceName,
			WorkspacePath: space.WorkspacePath,
			SignalKind:    task.Trailer.SignalKind,
			Target:        task.Trailer.Target,
			Factors:       factors,
			Score:         ranking.Score(cfg.PriorityWeights, factors),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].TaskID < out[j].TaskID
		}
		return out[i].Score > out[j].Score
	})
	return out
}

func openForExecution(status string) bool {
	switch status {
	case graftcoord.StatusPending, "queued":
		return true
	default:
		return false
	}
}

func selectedTaskAndSpace(selected ExecutableCandidate, tasks []coord.Task, spaces []fleet.SpaceStatus) (coord.Task, fleet.SpaceStatus, bool) {
	var task coord.Task
	var space fleet.SpaceStatus
	taskOK := false
	spaceOK := false
	for _, item := range tasks {
		if item.ID == selected.TaskID {
			task = item
			taskOK = true
			break
		}
	}
	for _, item := range spaces {
		if item.URI == selected.SpaceURI {
			space = item
			spaceOK = true
			break
		}
	}
	return task, space, taskOK && spaceOK
}

func terminalUpdate(task coord.Task, decision executor.Decision, lifecycleErr error, maxRetries int) (status, description, reason string) {
	if lifecycleErr != nil {
		decision = executor.Decision{State: executor.StateFailed, Reason: lifecycleErr.Error()}
	}
	switch decision.State {
	case executor.StateLanded, executor.StateNoOp:
		return graftcoord.StatusCompleted, "", string(decision.State)
	case executor.StateFailed:
		next := task.Trailer
		next.RetryCount++
		updated, err := coord.ReplaceTrailer(task.Description, next)
		if err != nil {
			return graftcoord.StatusBlocked, "", "retry trailer update failed"
		}
		if maxRetries <= 0 || next.RetryCount <= maxRetries {
			return graftcoord.StatusPending, updated, "retry"
		}
		return graftcoord.StatusBlocked, updated, "retry budget exhausted"
	case executor.StateProposed, executor.StateSkipped:
		return graftcoord.StatusBlocked, "", string(decision.State)
	default:
		return graftcoord.StatusBlocked, "", "unknown terminal state"
	}
}

func pathWithin(root, path string) bool {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
