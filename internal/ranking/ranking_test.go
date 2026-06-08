package ranking

import (
	"math"
	"testing"

	"m31labs.dev/reeve/internal/config"
)

func TestScoreUsesNormalizedWeightedSum(t *testing.T) {
	score := Score(config.PriorityWeights{Gap: 4, Project: 3, Staleness: 2, Blast: 1}, Factors{
		GapSeverity:     1,
		ProjectPriority: 0.5,
		Staleness:       0,
		BlastRadius:     0.5,
	})
	want := 0.4*1 + 0.3*0.5 + 0.2*0 + 0.1*0.5
	if math.Abs(score-want) > 0.000001 {
		t.Fatalf("score=%f want=%f", score, want)
	}
}
