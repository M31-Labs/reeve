package fleet

import (
	"testing"

	"m31labs.dev/reeve/internal/config"
	"m31labs.dev/reeve/internal/hyphaindex"
	"m31labs.dev/reeve/internal/spend"
	"m31labs.dev/reeve/internal/workspaces"
)

func TestBuildSpacesDistinguishesDiscoveredAndEligible(t *testing.T) {
	cfg, err := config.Normalize(config.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	spaces := []hyphaindex.Space{
		{ObjectID: "space.a", SpaceID: "m31labs/a", URI: "hypha://m31labs/a", Metadata: map[string]any{}},
		{ObjectID: "space.b", SpaceID: "m31labs/b", URI: "hypha://m31labs/b", Metadata: map[string]any{"mode": "maintenance", "priority": 0.7}},
	}
	reg := workspaces.Registry{"b": {Name: "b", Path: "/repo/b"}}
	got := BuildSpaces(spaces, reg, cfg, spend.Counter{SpaceTokens: map[string]int64{}})
	if got[0].Eligible {
		t.Fatalf("missing mode should default to active and be ineligible")
	}
	if got[0].Mode != ModeActive {
		t.Fatalf("mode=%q", got[0].Mode)
	}
	if !got[1].Eligible {
		t.Fatalf("maintenance space should be eligible: %#v", got[1].Reasons)
	}
}

func TestBuildSpacesMarksBudgetExhaustedIneligible(t *testing.T) {
	cfg, err := config.Normalize(config.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	cfg.Budget = config.Budget{DailyTokens: 100, PerSpaceTokens: 100}
	spaces := []hyphaindex.Space{
		{ObjectID: "space.b", SpaceID: "m31labs/b", URI: "hypha://m31labs/b", Metadata: map[string]any{"mode": "maintenance", "priority": 0.7}},
	}
	reg := workspaces.Registry{"b": {Name: "b", Path: "/repo/b"}}
	got := BuildSpaces(spaces, reg, cfg, spend.Counter{TotalTokens: 100, SpaceTokens: map[string]int64{"m31labs/b": 0}})
	if got[0].Eligible {
		t.Fatalf("budget exhausted space should be ineligible")
	}
	if got[0].Reasons[len(got[0].Reasons)-1] != "budget exhausted" {
		t.Fatalf("reasons=%#v", got[0].Reasons)
	}
}
