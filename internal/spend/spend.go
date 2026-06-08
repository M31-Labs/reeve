package spend

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"m31labs.dev/reeve/internal/config"
)

type Counter struct {
	Date        string           `json:"date"`
	TotalTokens int64            `json:"total_tokens"`
	SpaceTokens map[string]int64 `json:"space_tokens"`
}

type Snapshot struct {
	GlobalLimit         int64 `json:"global_limit"`
	GlobalUsed          int64 `json:"global_used"`
	GlobalRemaining     int64 `json:"global_remaining"`
	PerSpaceLimit       int64 `json:"per_space_limit,omitempty"`
	SpaceUsed           int64 `json:"space_used"`
	SpaceRemaining      int64 `json:"space_remaining,omitempty"`
	WithinBudget        bool  `json:"within_budget"`
	GlobalBudgetEnabled bool  `json:"global_budget_enabled"`
	SpaceBudgetEnabled  bool  `json:"space_budget_enabled"`
}

func Load(stateDir string, day time.Time) (Counter, error) {
	date := day.Format("2006-01-02")
	counter := Counter{Date: date, SpaceTokens: map[string]int64{}}
	path := filepath.Join(stateDir, "spend-"+date+".json")
	body, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return counter, nil
		}
		return Counter{}, err
	}
	if err := json.Unmarshal(body, &counter); err != nil {
		return Counter{}, fmt.Errorf("parse spend counter %s: %w", path, err)
	}
	if counter.SpaceTokens == nil {
		counter.SpaceTokens = map[string]int64{}
	}
	if counter.Date == "" {
		counter.Date = date
	}
	return counter, nil
}

func Add(stateDir string, day time.Time, spaceID string, tokens int64) (Counter, error) {
	if tokens < 0 {
		return Counter{}, fmt.Errorf("tokens must be non-negative")
	}
	counter, err := Load(stateDir, day)
	if err != nil {
		return Counter{}, err
	}
	counter.TotalTokens += tokens
	if counter.SpaceTokens == nil {
		counter.SpaceTokens = map[string]int64{}
	}
	counter.SpaceTokens[spaceID] += tokens
	if err := Save(stateDir, counter); err != nil {
		return Counter{}, err
	}
	return counter, nil
}

func Save(stateDir string, counter Counter) error {
	if counter.SpaceTokens == nil {
		counter.SpaceTokens = map[string]int64{}
	}
	if counter.Date == "" {
		return fmt.Errorf("missing spend counter date")
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(counter, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	path := filepath.Join(stateDir, "spend-"+counter.Date+".json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func Check(cfg config.Budget, counter Counter, spaceID string) Snapshot {
	spaceUsed := counter.SpaceTokens[spaceID]
	snap := Snapshot{
		GlobalLimit:         cfg.DailyTokens,
		GlobalUsed:          counter.TotalTokens,
		PerSpaceLimit:       cfg.PerSpaceTokens,
		SpaceUsed:           spaceUsed,
		WithinBudget:        true,
		GlobalBudgetEnabled: cfg.DailyTokens > 0,
		SpaceBudgetEnabled:  cfg.PerSpaceTokens > 0,
	}
	if cfg.DailyTokens > 0 {
		snap.GlobalRemaining = cfg.DailyTokens - counter.TotalTokens
		if snap.GlobalRemaining <= 0 {
			snap.WithinBudget = false
		}
	}
	if cfg.PerSpaceTokens > 0 {
		snap.SpaceRemaining = cfg.PerSpaceTokens - spaceUsed
		if snap.SpaceRemaining <= 0 {
			snap.WithinBudget = false
		}
	}
	return snap
}
