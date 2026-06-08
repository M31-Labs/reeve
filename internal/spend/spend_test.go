package spend

import (
	"testing"
	"time"

	"m31labs.dev/reeve/internal/config"
)

func TestAddPersistsDailyAndPerSpaceSpend(t *testing.T) {
	day := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	if _, err := Add(dir, day, "m31labs/reeve", 100); err != nil {
		t.Fatal(err)
	}
	if _, err := Add(dir, day, "m31labs/hyphae", 50); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir, day)
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalTokens != 150 {
		t.Fatalf("TotalTokens=%d", got.TotalTokens)
	}
	if got.SpaceTokens["m31labs/reeve"] != 100 {
		t.Fatalf("reeve tokens=%d", got.SpaceTokens["m31labs/reeve"])
	}
}

func TestCheckPausesOnGlobalOrPerSpaceBudget(t *testing.T) {
	counter := Counter{
		Date:        "2026-06-08",
		TotalTokens: 100,
		SpaceTokens: map[string]int64{
			"m31labs/reeve": 40,
		},
	}
	cfg := config.Budget{DailyTokens: 100, PerSpaceTokens: 100}
	if snap := Check(cfg, counter, "m31labs/reeve"); snap.WithinBudget {
		t.Fatalf("expected exhausted global budget: %#v", snap)
	}
	cfg = config.Budget{DailyTokens: 200, PerSpaceTokens: 40}
	if snap := Check(cfg, counter, "m31labs/reeve"); snap.WithinBudget {
		t.Fatalf("expected exhausted per-space budget: %#v", snap)
	}
	cfg = config.Budget{DailyTokens: 200, PerSpaceTokens: 100}
	if snap := Check(cfg, counter, "m31labs/reeve"); !snap.WithinBudget {
		t.Fatalf("expected budget to allow dispatch: %#v", snap)
	}
}
