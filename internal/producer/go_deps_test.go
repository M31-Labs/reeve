package producer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestScanGoDependenciesEmitsOutdatedDeps(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	signals := ScanGoDependencies(context.Background(), "hypha://m31labs/reeve", dir, GoDepScanOptions{
		AgentURI: "agent://reeve/conductor",
		GoBin:    "go",
	}, fakeRunner{
		"go list -m -u -json all": {
			Command: "go list -m -u -json all",
			Output: `{"Path":"example.com/x","Version":"v0.0.0"}
{"Path":"golang.org/x/tools","Version":"v0.1.0","Update":{"Path":"golang.org/x/tools","Version":"v0.2.0"}}
{"Path":"golang.org/x/sync","Version":"v0.1.0"}
`,
		},
	})
	if len(signals) != 1 {
		t.Fatalf("signals=%#v", signals)
	}
	if signals[0].Kind != "outdated-dep" || signals[0].Target != "golang.org/x/tools@v0.2.0" {
		t.Fatalf("signal=%#v", signals[0])
	}
}

func TestScanGoDependenciesSkipsNonGoWorkspace(t *testing.T) {
	signals := ScanGoDependencies(context.Background(), "hypha://m31labs/reeve", t.TempDir(), GoDepScanOptions{}, fakeRunner{})
	if len(signals) != 0 {
		t.Fatalf("signals=%#v", signals)
	}
}
