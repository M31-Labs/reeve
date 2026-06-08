package coord

import (
	"strings"
	"testing"
)

func TestRenderParseTrailerRoundTrip(t *testing.T) {
	in := Trailer{
		DedupKey:       "abc123",
		SpaceURI:       "hypha://m31labs/reeve",
		SignalKind:     "red-build",
		Target:         "go test ./...",
		Severity:       0.9,
		CreatedBy:      "agent://reeve/conductor",
		SourceSnapshot: "commit:abc",
		RetryCount:     1,
	}
	description, err := AppendTrailer("Fix the build.", in)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseTrailer(description)
	if err != nil {
		t.Fatal(err)
	}
	if got.DedupKey != in.DedupKey || got.SignalKind != in.SignalKind || got.RetryCount != 1 {
		t.Fatalf("round trip mismatch: %#v", got)
	}
}

func TestClassifyTaskBlocksMalformedTrailer(t *testing.T) {
	missing := ClassifyTask("t1", "manual task", "no trailer", "pending")
	if missing.Managed || !missing.Blocked {
		t.Fatalf("missing trailer should be unmanaged blocked: %#v", missing)
	}
	bad := ClassifyTask("t2", "bad", "```reeve-task\nspace_uri: hypha://m31labs/reeve\n```\n", "pending")
	if bad.Managed || !bad.Blocked {
		t.Fatalf("bad trailer should be unmanaged blocked: %#v", bad)
	}
}

func TestReplaceTrailerUpdatesExistingFence(t *testing.T) {
	description, err := AppendTrailer("Fix the build.", Trailer{
		DedupKey:   "abc123",
		SpaceURI:   "hypha://m31labs/reeve",
		SignalKind: "red-build",
		Target:     "go test ./...",
		Severity:   0.9,
		CreatedBy:  "agent://reeve/conductor",
		RetryCount: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	next, err := ReplaceTrailer(description, Trailer{
		DedupKey:   "abc123",
		SpaceURI:   "hypha://m31labs/reeve",
		SignalKind: "red-build",
		Target:     "go test ./...",
		Severity:   0.9,
		CreatedBy:  "agent://reeve/conductor",
		RetryCount: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	trailer, err := ParseTrailer(next)
	if err != nil {
		t.Fatal(err)
	}
	if trailer.RetryCount != 2 {
		t.Fatalf("retry=%d", trailer.RetryCount)
	}
	if strings.Count(next, "```"+FenceName) != 1 {
		t.Fatalf("expected one trailer fence:\n%s", next)
	}
}
