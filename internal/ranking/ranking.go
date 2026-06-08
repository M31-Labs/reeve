package ranking

import "m31labs.dev/reeve/internal/config"

type Factors struct {
	GapSeverity     float64 `json:"gap_severity"`
	ProjectPriority float64 `json:"project_priority"`
	Staleness       float64 `json:"staleness"`
	BlastRadius     float64 `json:"blast_radius"`
}

func Score(weights config.PriorityWeights, factors Factors) float64 {
	weights = NormalizeWeights(weights)
	return weights.Gap*clamp(factors.GapSeverity) +
		weights.Project*clamp(factors.ProjectPriority) +
		weights.Staleness*clamp(factors.Staleness) +
		weights.Blast*clamp(factors.BlastRadius)
}

func NormalizeWeights(weights config.PriorityWeights) config.PriorityWeights {
	sum := weights.Gap + weights.Project + weights.Staleness + weights.Blast
	if sum <= 0 {
		return config.DefaultConfig().PriorityWeights
	}
	return config.PriorityWeights{
		Gap:       weights.Gap / sum,
		Project:   weights.Project / sum,
		Staleness: weights.Staleness / sum,
		Blast:     weights.Blast / sum,
	}
}

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
