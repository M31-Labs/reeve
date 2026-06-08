package taskplan

import (
	"strings"
	"testing"

	"m31labs.dev/reeve/internal/coord"
	"m31labs.dev/reeve/internal/producer"
)

func TestFromActionIncludesTagsAndTrailer(t *testing.T) {
	signal := producer.Signal{
		SpaceURI:  "hypha://m31labs/reeve",
		Axis:      "maintenance",
		Kind:      "red-test",
		Target:    "go test ./...",
		Severity:  0.9,
		Title:     "Fix failing Go tests",
		CreatedBy: "agent://reeve/conductor",
	}
	action := producer.Plan([]producer.Signal{signal}, nil)[0]
	spec, err := FromAction(action, "reeve")
	if err != nil {
		t.Fatal(err)
	}
	if spec.Priority != "p0" {
		t.Fatalf("priority=%q", spec.Priority)
	}
	joined := strings.Join(spec.Tags, ",")
	for _, want := range []string{"reeve", "axis:maintenance", "signal:red-test", "space:reeve", "dedup:"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("tags missing %q: %#v", want, spec.Tags)
		}
	}
	if _, err := coord.ParseTrailer(spec.Description); err != nil {
		t.Fatalf("description missing valid trailer: %v\n%s", err, spec.Description)
	}
}
