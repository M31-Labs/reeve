package executor

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"m31labs.dev/reeve/internal/coord"
	"m31labs.dev/reeve/internal/taskplan"
)

type Assessment struct {
	Category  string `json:"category"`
	Rationale string `json:"rationale,omitempty"`
}

type TraceStart struct {
	AgentURI string `json:"agent_uri"`
	TaskID   string `json:"task_id"`
	Phase    string `json:"phase"`
	SpaceURI string `json:"space_uri"`
}

type TraceDone struct {
	TraceID  string `json:"trace_id"`
	Status   string `json:"status"`
	SpaceURI string `json:"space_uri"`
	SporeID  string `json:"spore_id,omitempty"`
}

type SporeSubmission struct {
	Task     coord.Task `json:"task"`
	Decision Decision   `json:"decision"`
	Buckley  BuckleyRun `json:"buckley"`
	Green    GreenRun   `json:"green"`
}

type HyphaRituals interface {
	AssessTask(context.Context, coord.Task) (Assessment, error)
	StartTrace(context.Context, TraceStart) (string, error)
	TickTrace(context.Context, string, string, string) error
	DoneTrace(context.Context, TraceDone) error
	SubmitSpore(context.Context, SporeSubmission) (string, error)
}

type WorktreeOps interface {
	Head(context.Context, string) (string, error)
	Dirty(context.Context, string) (bool, error)
	GreenCheck(context.Context, string, string) GreenRun
	Quarantine(context.Context, string, string) (string, error)
}

type GreenRun struct {
	Command  string `json:"command"`
	ExitCode int    `json:"exit_code"`
	Output   string `json:"output,omitempty"`
	Error    string `json:"error,omitempty"`
}

type LifecycleOptions struct {
	AgentURI        string
	AssessThreshold string
	HardAssessGate  bool
	WorkspaceName   string
	WorkspacePath   string
	GreenCheck      string
	BuckleyCommand  string
	BuckleyModel    string
	Timeout         time.Duration
}

type LifecycleResult struct {
	TaskID       string     `json:"task_id"`
	TraceID      string     `json:"trace_id,omitempty"`
	SporeID      string     `json:"spore_id,omitempty"`
	Assessment   Assessment `json:"assessment"`
	Buckley      BuckleyRun `json:"buckley"`
	Green        GreenRun   `json:"green"`
	Decision     Decision   `json:"decision"`
	BeforeHead   string     `json:"before_head,omitempty"`
	AfterHead    string     `json:"after_head,omitempty"`
	Quarantine   string     `json:"quarantine,omitempty"`
	TraceDone    bool       `json:"trace_done"`
	TraceDoneErr string     `json:"trace_done_err,omitempty"`
}

type BuckleySpecRunner func(context.Context, taskplan.CreateSpec, TaskRunOptions) BuckleyRun

func ExecuteLifecycle(ctx context.Context, task coord.Task, opts LifecycleOptions, hypha HyphaRituals, wt WorktreeOps, runBuckley BuckleySpecRunner) (result LifecycleResult, err error) {
	result = LifecycleResult{TaskID: task.ID}
	if !task.Managed {
		return result, errors.New("refusing to execute unmanaged task")
	}
	if hypha == nil {
		return result, errors.New("hypha rituals are required")
	}
	if wt == nil {
		return result, errors.New("worktree ops are required")
	}
	if runBuckley == nil {
		runBuckley = func(ctx context.Context, spec taskplan.CreateSpec, opts TaskRunOptions) BuckleyRun {
			return RunSpec(ctx, spec, opts, nil)
		}
	}
	if strings.TrimSpace(opts.WorkspacePath) == "" {
		return result, errors.New("workspace path is required")
	}
	if strings.TrimSpace(opts.GreenCheck) == "" {
		opts.GreenCheck = "go build ./... && go test ./..."
	}
	if strings.TrimSpace(opts.AgentURI) == "" {
		opts.AgentURI = "agent://reeve/conductor"
	}

	assessment, err := hypha.AssessTask(ctx, task)
	if err != nil {
		return result, fmt.Errorf("assess task: %w", err)
	}
	result.Assessment = assessment
	if opts.HardAssessGate && belowAssessThreshold(assessment.Category, opts.AssessThreshold) {
		result.Decision = Decision{State: StateSkipped, Reason: "assessment below threshold: " + assessment.Category}
		return result, nil
	}

	traceID, err := hypha.StartTrace(ctx, TraceStart{
		AgentURI: opts.AgentURI,
		TaskID:   task.ID,
		Phase:    "maintenance",
		SpaceURI: task.Trailer.SpaceURI,
	})
	if err != nil {
		return result, fmt.Errorf("start trace: %w", err)
	}
	result.TraceID = traceID
	defer func() {
		status := "succeeded"
		if result.Decision.State == StateFailed {
			status = "failed"
		}
		if err := hypha.DoneTrace(ctx, TraceDone{TraceID: traceID, Status: status, SpaceURI: task.Trailer.SpaceURI, SporeID: result.SporeID}); err != nil {
			result.TraceDoneErr = err.Error()
			return
		}
		result.TraceDone = true
	}()

	_ = hypha.TickTrace(ctx, traceID, task.Trailer.SpaceURI, "capturing pre-run worktree state")
	before, err := wt.Head(ctx, opts.WorkspacePath)
	if err != nil {
		return result, fmt.Errorf("head before buckley: %w", err)
	}
	result.BeforeHead = before

	_ = hypha.TickTrace(ctx, traceID, task.Trailer.SpaceURI, "running buckley")
	spec := taskSpecFromCoord(task, opts.WorkspaceName)
	result.Buckley = runBuckley(ctx, spec, TaskRunOptions{
		Command:    opts.BuckleyCommand,
		Workspace:  opts.WorkspacePath,
		GreenCheck: opts.GreenCheck,
		Model:      opts.BuckleyModel,
		Timeout:    opts.Timeout,
	})

	after, err := wt.Head(ctx, opts.WorkspacePath)
	if err != nil {
		return result, fmt.Errorf("head after buckley: %w", err)
	}
	result.AfterHead = after
	dirty, err := wt.Dirty(ctx, opts.WorkspacePath)
	if err != nil {
		return result, fmt.Errorf("dirty worktree check: %w", err)
	}
	newCommit := before != "" && after != "" && before != after
	if result.Buckley.ExitCode == 0 && newCommit && !dirty {
		_ = hypha.TickTrace(ctx, traceID, task.Trailer.SpaceURI, "running green check")
		result.Green = wt.GreenCheck(ctx, opts.WorkspacePath, opts.GreenCheck)
	} else {
		result.Green = GreenRun{Command: opts.GreenCheck, ExitCode: 0}
	}
	green := GreenPass
	if result.Green.ExitCode != 0 {
		green = GreenFail
	}
	result.Decision = Classify(Result{
		BuckleyExitCode: result.Buckley.ExitCode,
		NewCommit:       newCommit,
		DirtyWorktree:   dirty,
		Approval:        result.Buckley.Approval,
		Green:           green,
	})
	if result.Decision.Quarantine {
		path, err := wt.Quarantine(ctx, opts.WorkspacePath, task.ID)
		if err != nil {
			return result, fmt.Errorf("quarantine worktree: %w", err)
		}
		result.Quarantine = path
	}
	if result.Decision.State == StateProposed {
		_ = hypha.TickTrace(ctx, traceID, task.Trailer.SpaceURI, "submitting propose-only spore")
		sporeID, err := hypha.SubmitSpore(ctx, SporeSubmission{
			Task:     task,
			Decision: result.Decision,
			Buckley:  result.Buckley,
			Green:    result.Green,
		})
		if err != nil {
			return result, fmt.Errorf("submit spore: %w", err)
		}
		result.SporeID = sporeID
	}
	return result, nil
}

func taskSpecFromCoord(task coord.Task, workspace string) taskplan.CreateSpec {
	return taskplan.CreateSpec{
		Title:       task.Title,
		Description: task.Description,
		Workspace:   workspace,
		DedupKey:    task.Trailer.DedupKey,
		Action:      "execute",
	}
}

func belowAssessThreshold(category, threshold string) bool {
	if strings.TrimSpace(threshold) == "" {
		threshold = "aligned"
	}
	return assessRank(category) < assessRank(threshold)
}

func assessRank(category string) int {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "blocked", "misaligned", "reject":
		return 0
	case "unclear", "ask":
		return 1
	case "aligned", "allow", "ok", "":
		return 2
	case "strong", "excellent":
		return 3
	default:
		return 1
	}
}
