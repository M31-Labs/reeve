package producer

import (
	"crypto/sha1"
	"fmt"
	"strings"

	"m31labs.dev/reeve/internal/coord"
)

type Signal struct {
	SpaceURI  string  `json:"space_uri"`
	Axis      string  `json:"axis"`
	Kind      string  `json:"kind"`
	Target    string  `json:"target"`
	Severity  float64 `json:"severity"`
	Title     string  `json:"title"`
	Snapshot  string  `json:"snapshot,omitempty"`
	CreatedBy string  `json:"created_by"`
}

type ActionKind string

const (
	ActionCreate ActionKind = "create"
	ActionSkip   ActionKind = "skip"
	ActionReopen ActionKind = "reopen"
)

type Action struct {
	Kind       ActionKind    `json:"kind"`
	Signal     Signal        `json:"signal"`
	DedupKey   string        `json:"dedup_key"`
	PriorTask  string        `json:"prior_task,omitempty"`
	Trailer    coord.Trailer `json:"trailer"`
	SkipReason string        `json:"skip_reason,omitempty"`
}

func Plan(signals []Signal, existing []coord.Task) []Action {
	openByKey := map[string]coord.Task{}
	completedByKey := map[string]coord.Task{}
	for _, task := range existing {
		if !task.Managed {
			continue
		}
		key := task.Trailer.DedupKey
		if isOpenStatus(task.Status) {
			openByKey[key] = task
		}
		if task.Status == "completed" {
			completedByKey[key] = task
		}
	}
	seenInBatch := map[string]bool{}
	var actions []Action
	for _, signal := range signals {
		key := DedupKey(signal)
		trailer := coord.Trailer{
			DedupKey:       key,
			SpaceURI:       signal.SpaceURI,
			SignalKind:     signal.Kind,
			Target:         signal.Target,
			Severity:       signal.Severity,
			CreatedBy:      signal.CreatedBy,
			SourceSnapshot: signal.Snapshot,
		}
		action := Action{Kind: ActionCreate, Signal: signal, DedupKey: key, Trailer: trailer}
		if prior, ok := openByKey[key]; ok {
			action.Kind = ActionSkip
			action.PriorTask = prior.ID
			action.SkipReason = "open task already exists"
		} else if seenInBatch[key] {
			action.Kind = ActionSkip
			action.SkipReason = "duplicate signal in batch"
		} else if prior, ok := completedByKey[key]; ok {
			action.Kind = ActionReopen
			action.PriorTask = prior.ID
			action.Trailer.PreviousTaskID = prior.ID
		}
		seenInBatch[key] = true
		actions = append(actions, action)
	}
	return actions
}

func DedupKey(signal Signal) string {
	axis := signal.Axis
	if strings.TrimSpace(axis) == "" {
		axis = "maintenance"
	}
	sum := sha1.Sum([]byte(signal.SpaceURI + "|" + axis + "|" + signal.Kind + "|" + signal.Target))
	return fmt.Sprintf("%x", sum[:])
}

func ShortDedup(key string) string {
	if len(key) <= 8 {
		return key
	}
	return key[:8]
}

func isOpenStatus(status string) bool {
	switch status {
	case "pending", "queued", "in_progress", "blocked":
		return true
	default:
		return false
	}
}
