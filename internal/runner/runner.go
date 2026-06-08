package runner

import (
	"context"
	"crypto/sha1"
	"fmt"
	"sort"
	"time"

	"m31labs.dev/reeve/internal/config"
	"m31labs.dev/reeve/internal/coord"
	"m31labs.dev/reeve/internal/fleet"
	"m31labs.dev/reeve/internal/graftcoord"
	"m31labs.dev/reeve/internal/hypha"
	"m31labs.dev/reeve/internal/hyphaindex"
	"m31labs.dev/reeve/internal/producer"
	"m31labs.dev/reeve/internal/ranking"
	"m31labs.dev/reeve/internal/spend"
	"m31labs.dev/reeve/internal/taskplan"
	"m31labs.dev/reeve/internal/workspaces"
)

type Report struct {
	GeneratedAt string                   `json:"generated_at"`
	DryRun      bool                     `json:"dry_run"`
	Spaces      []fleet.SpaceStatus      `json:"spaces"`
	Candidates  []Candidate              `json:"candidates"`
	Plan        []PlanItem               `json:"plan,omitempty"`
	Apply       []graftcoord.ApplyResult `json:"apply,omitempty"`
	Selected    *Candidate               `json:"selected,omitempty"`
	Telemetry   hypha.Telemetry          `json:"telemetry"`
	Coord       graftcoord.Summary       `json:"coord"`
	Warnings    []string                 `json:"warnings,omitempty"`
	Summary     Summary                  `json:"summary"`
}

type Options struct {
	DryRun bool
	Scan   bool
}

type Summary struct {
	DiscoveredSpaces int `json:"discovered_spaces"`
	EligibleSpaces   int `json:"eligible_spaces"`
	OptedInSpaces    int `json:"opted_in_spaces"`
	Candidates       int `json:"candidates"`
	ActiveTraces     int `json:"active_traces"`
	UnreviewedSpores int `json:"unreviewed_spores"`
	CoordOpenTasks   int `json:"coord_open_tasks"`
	CoordFailures    int `json:"coord_failures"`
}

type Candidate struct {
	ID         string          `json:"id"`
	SpaceID    string          `json:"space_id"`
	SpaceURI   string          `json:"space_uri"`
	Title      string          `json:"title"`
	SignalKind string          `json:"signal_kind"`
	Target     string          `json:"target"`
	Factors    ranking.Factors `json:"factors"`
	Score      float64         `json:"score"`
	Decision   string          `json:"decision"`
}

type PlanItem struct {
	SpaceID string              `json:"space_id"`
	Spec    taskplan.CreateSpec `json:"spec"`
}

func BuildReport(ctx context.Context, cfg config.Config, dryRun bool) (Report, error) {
	return BuildReportWithOptions(ctx, cfg, Options{DryRun: dryRun})
}

func BuildReportWithOptions(ctx context.Context, cfg config.Config, opts Options) (Report, error) {
	indexSpaces, err := hyphaindex.ReadSpacesWithFallback(ctx, cfg.HyphaIndexPath, cfg.Commands.Hypha)
	if err != nil {
		return Report{}, err
	}
	registry, warnings, err := workspaces.Load(ctx, cfg.Commands.Graft)
	if err != nil {
		return Report{}, err
	}
	counter, err := spend.Load(cfg.StateDir, time.Now())
	if err != nil {
		return Report{}, err
	}
	spaces := fleet.BuildSpaces(indexSpaces, registry, cfg, counter)
	telemetry := hypha.LoadTelemetry(ctx, cfg.Commands.Hypha, "hypha://m31labs/reeve")
	coordSummary := graftcoord.LoadSummary(ctx, cfg.Commands.Graft)
	warnings = append(warnings, telemetry.Warnings...)
	warnings = append(warnings, coordSummary.Warnings...)
	candidates, plan := BuildCandidatesWithOptions(ctx, spaces, cfg, coordSummary.Tasks, opts)
	var selected *Candidate
	if len(candidates) > 0 {
		selected = &candidates[0]
		selected.Decision = "would_assign"
	}
	return Report{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		DryRun:      opts.DryRun,
		Spaces:      spaces,
		Candidates:  candidates,
		Plan:        plan,
		Selected:    selected,
		Telemetry:   telemetry,
		Coord:       coordSummary,
		Warnings:    warnings,
		Summary: Summary{
			DiscoveredSpaces: len(spaces),
			EligibleSpaces:   fleet.CountEligible(spaces),
			OptedInSpaces:    fleet.CountOptedIn(spaces),
			Candidates:       len(candidates),
			ActiveTraces:     len(telemetry.ActiveTraces),
			UnreviewedSpores: len(telemetry.UnreviewedSpores),
			CoordOpenTasks:   openTaskCount(coordSummary.Counts),
			CoordFailures:    coordSummary.Counts["failed"] + coordSummary.Counts["blocked"] + coordSummary.Counts["unmanaged_blocked"],
		},
	}, nil
}

func BuildCandidates(spaces []fleet.SpaceStatus, cfg config.Config) []Candidate {
	candidates, _ := BuildCandidatesWithOptions(context.Background(), spaces, cfg, nil, Options{})
	return candidates
}

func BuildCandidatesWithOptions(ctx context.Context, spaces []fleet.SpaceStatus, cfg config.Config, existing []coord.Task, opts Options) ([]Candidate, []PlanItem) {
	out := make([]Candidate, 0)
	plan := make([]PlanItem, 0)
	for _, space := range spaces {
		if !space.Eligible {
			continue
		}
		if opts.Scan {
			signals := producer.ScanGoMaintenance(ctx, space.URI, space.WorkspacePath, producer.ScanOptions{
				AgentURI: cfg.AgentURI,
				GoBin:    cfg.Commands.Go,
			}, nil)
			actions := producer.Plan(signals, existing)
			for _, action := range actions {
				if action.Kind == producer.ActionSkip {
					continue
				}
				spec, err := taskplan.FromAction(action, space.WorkspaceName)
				if err == nil {
					plan = append(plan, PlanItem{SpaceID: space.SpaceID, Spec: spec})
				}
				factors := ranking.Factors{
					GapSeverity:     action.Signal.Severity,
					ProjectPriority: space.Priority,
					Staleness:       0.5,
					BlastRadius:     0.5,
				}
				out = append(out, Candidate{
					ID:         candidateID(space.SpaceID, action.Signal.Kind+"|"+action.Signal.Target),
					SpaceID:    space.SpaceID,
					SpaceURI:   space.URI,
					Title:      action.Signal.Title,
					SignalKind: action.Signal.Kind,
					Target:     action.Signal.Target,
					Factors:    factors,
					Score:      ranking.Score(cfg.PriorityWeights, factors),
					Decision:   string(action.Kind),
				})
			}
			continue
		}
		factors := ranking.Factors{
			GapSeverity:     0.5,
			ProjectPriority: space.Priority,
			Staleness:       0.5,
			BlastRadius:     0.5,
		}
		candidate := Candidate{
			ID:         candidateID(space.SpaceID, "maintenance-scan"),
			SpaceID:    space.SpaceID,
			SpaceURI:   space.URI,
			Title:      fmt.Sprintf("Run maintenance scan for %s", space.SpaceID),
			SignalKind: "maintenance-scan",
			Target:     space.WorkspaceName,
			Factors:    factors,
			Score:      ranking.Score(cfg.PriorityWeights, factors),
			Decision:   "ranked",
		}
		out = append(out, candidate)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].SpaceID < out[j].SpaceID
		}
		return out[i].Score > out[j].Score
	})
	return out, plan
}

func candidateID(spaceID, signal string) string {
	sum := sha1.Sum([]byte(spaceID + "|" + signal))
	return fmt.Sprintf("candidate.%x", sum[:6])
}

func openTaskCount(counts map[string]int) int {
	return counts["pending"] + counts["queued"] + counts["in_progress"]
}
