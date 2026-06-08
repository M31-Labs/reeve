package fleet

import (
	"fmt"

	"m31labs.dev/reeve/internal/config"
	"m31labs.dev/reeve/internal/hyphaindex"
	"m31labs.dev/reeve/internal/spend"
	"m31labs.dev/reeve/internal/workspaces"
)

const (
	ModeActive      = "active"
	ModeMaintenance = "maintenance"
	ModeFrozen      = "frozen"
)

type SpaceStatus struct {
	ObjectID      string         `json:"object_id"`
	SpaceID       string         `json:"space_id"`
	URI           string         `json:"uri"`
	Title         string         `json:"title"`
	Mode          string         `json:"mode"`
	Priority      float64        `json:"priority,omitempty"`
	PriorityValid bool           `json:"priority_valid"`
	GreenCheck    string         `json:"green_check"`
	WorkspaceName string         `json:"workspace_name"`
	WorkspacePath string         `json:"workspace_path,omitempty"`
	Warning       string         `json:"warning,omitempty"`
	Discovered    bool           `json:"discovered"`
	Eligible      bool           `json:"eligible"`
	Reasons       []string       `json:"reasons,omitempty"`
	Budget        spend.Snapshot `json:"budget"`
}

func BuildSpaces(indexSpaces []hyphaindex.Space, registry workspaces.Registry, cfg config.Config, counter spend.Counter) []SpaceStatus {
	out := make([]SpaceStatus, 0, len(indexSpaces))
	for _, in := range indexSpaces {
		status := SpaceStatus{
			ObjectID:   in.ObjectID,
			SpaceID:    in.SpaceID,
			URI:        in.URI,
			Title:      in.Title,
			Mode:       ModeActive,
			GreenCheck: "go build ./... && go test ./...",
			Warning:    in.MetadataWarning,
			Discovered: true,
		}
		if mode, ok := hyphaindex.StringField(in.Metadata, "mode"); ok {
			status.Mode = mode
		}
		if green, ok := hyphaindex.StringField(in.Metadata, "green_check"); ok {
			status.GreenCheck = green
		}
		if p, ok := hyphaindex.FloatField(in.Metadata, "priority"); ok && p >= 0 && p <= 1 {
			status.Priority = p
			status.PriorityValid = true
		}
		if override, ok := hyphaindex.NestedStringField(in.Metadata, "reeve", "workspace"); ok {
			status.WorkspaceName = override
		} else {
			status.WorkspaceName = workspaces.ShortName(in.SpaceID)
		}
		if ws, ok := registry.Find(status.WorkspaceName); ok {
			status.WorkspacePath = ws.Path
		}
		status.Budget = spend.Check(cfg.Budget, counter, status.SpaceID)
		status.Reasons = eligibilityReasons(status, cfg)
		status.Eligible = len(status.Reasons) == 0
		out = append(out, status)
	}
	return out
}

func CountEligible(spaces []SpaceStatus) int {
	n := 0
	for _, s := range spaces {
		if s.Eligible {
			n++
		}
	}
	return n
}

func CountOptedIn(spaces []SpaceStatus) int {
	n := 0
	for _, s := range spaces {
		if s.Mode == ModeMaintenance {
			n++
		}
	}
	return n
}

func eligibilityReasons(status SpaceStatus, cfg config.Config) []string {
	var reasons []string
	switch status.Mode {
	case ModeMaintenance:
	case ModeFrozen:
		return []string{"mode frozen"}
	default:
		return []string{fmt.Sprintf("mode %s", status.Mode)}
	}
	if !status.PriorityValid {
		reasons = append(reasons, "missing or invalid priority")
	}
	if status.WorkspacePath == "" {
		reasons = append(reasons, "graft workspace not found")
	}
	if cfg.Paused {
		reasons = append(reasons, "global pause enabled")
	}
	if !status.Budget.WithinBudget {
		reasons = append(reasons, "budget exhausted")
	}
	return reasons
}
