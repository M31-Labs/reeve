package coord

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const FenceName = "reeve-task"

type Trailer struct {
	DedupKey        string  `yaml:"dedup_key" json:"dedup_key"`
	SpaceURI        string  `yaml:"space_uri" json:"space_uri"`
	SignalKind      string  `yaml:"signal_kind" json:"signal_kind"`
	Target          string  `yaml:"target" json:"target"`
	Severity        float64 `yaml:"severity" json:"severity"`
	CreatedBy       string  `yaml:"created_by" json:"created_by"`
	SourceSnapshot  string  `yaml:"source_snapshot,omitempty" json:"source_snapshot,omitempty"`
	RetryCount      int     `yaml:"retry_count" json:"retry_count"`
	PreviousTaskID  string  `yaml:"previous_task_id,omitempty" json:"previous_task_id,omitempty"`
	LastDisposition string  `yaml:"last_disposition,omitempty" json:"last_disposition,omitempty"`
	LastError       string  `yaml:"last_error,omitempty" json:"last_error,omitempty"`
	NextAttemptAt   string  `yaml:"next_attempt_at,omitempty" json:"next_attempt_at,omitempty"`
	LandingBranch   string  `yaml:"landing_branch,omitempty" json:"landing_branch,omitempty"`
	LandingPR       string  `yaml:"landing_pr,omitempty" json:"landing_pr,omitempty"`
}

type Task struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Status      string  `json:"status"`
	Trailer     Trailer `json:"trailer,omitempty"`
	Managed     bool    `json:"managed"`
	Blocked     bool    `json:"blocked"`
	BlockReason string  `json:"block_reason,omitempty"`
}

func RenderTrailer(t Trailer) (string, error) {
	if err := validateTrailer(t); err != nil {
		return "", err
	}
	body, err := yaml.Marshal(t)
	if err != nil {
		return "", err
	}
	return "```" + FenceName + "\n" + string(body) + "```\n", nil
}

func AppendTrailer(description string, t Trailer) (string, error) {
	fence, err := RenderTrailer(t)
	if err != nil {
		return "", err
	}
	description = strings.TrimRight(description, "\n")
	if description != "" {
		description += "\n\n"
	}
	return description + fence, nil
}

func ReplaceTrailer(description string, t Trailer) (string, error) {
	fence, err := RenderTrailer(t)
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.ReplaceAll(description, "\r\n", "\n"), "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != "```"+FenceName {
			continue
		}
		for j := i + 1; j < len(lines); j++ {
			if strings.TrimSpace(lines[j]) == "```" {
				out := append([]string{}, lines[:i]...)
				out = append(out, strings.TrimRight(fence, "\n"))
				out = append(out, lines[j+1:]...)
				return strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n", nil
			}
		}
		return "", errors.New("unterminated reeve-task trailer")
	}
	return AppendTrailer(description, t)
}

func ParseTrailer(description string) (Trailer, error) {
	body, ok := extractFence(description)
	if !ok {
		return Trailer{}, errors.New("missing reeve-task trailer")
	}
	var out Trailer
	if err := yaml.Unmarshal([]byte(body), &out); err != nil {
		return Trailer{}, fmt.Errorf("parse reeve-task trailer: %w", err)
	}
	if err := validateTrailer(out); err != nil {
		return Trailer{}, err
	}
	return out, nil
}

func ClassifyTask(id, title, description, status string) Task {
	t := Task{ID: id, Title: title, Description: description, Status: status}
	trailer, err := ParseTrailer(description)
	if err != nil {
		t.Blocked = true
		t.BlockReason = err.Error()
		return t
	}
	t.Managed = true
	t.Trailer = trailer
	return t
}

func validateTrailer(t Trailer) error {
	switch {
	case strings.TrimSpace(t.DedupKey) == "":
		return errors.New("missing dedup_key")
	case strings.TrimSpace(t.SpaceURI) == "":
		return errors.New("missing space_uri")
	case strings.TrimSpace(t.SignalKind) == "":
		return errors.New("missing signal_kind")
	case strings.TrimSpace(t.Target) == "":
		return errors.New("missing target")
	case strings.TrimSpace(t.CreatedBy) == "":
		return errors.New("missing created_by")
	case t.Severity < 0 || t.Severity > 1:
		return errors.New("severity must be between 0 and 1")
	case t.RetryCount < 0:
		return errors.New("retry_count must be non-negative")
	}
	return nil
}

func extractFence(description string) (string, bool) {
	lines := strings.Split(strings.ReplaceAll(description, "\r\n", "\n"), "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != "```"+FenceName {
			continue
		}
		var body []string
		for _, inner := range lines[i+1:] {
			if strings.TrimSpace(inner) == "```" {
				return strings.Join(body, "\n"), true
			}
			body = append(body, inner)
		}
		return "", false
	}
	return "", false
}
