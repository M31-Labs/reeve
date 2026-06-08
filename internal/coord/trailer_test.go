package coord

import "testing"

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
