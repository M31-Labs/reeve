package producer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

type fakeRunner map[string]Result

func (f fakeRunner) Run(_ context.Context, _ string, name string, args ...string) Result {
	key := name
	for _, arg := range args {
		key += " " + arg
	}
	if res, ok := f[key]; ok {
		return res
	}
	return Result{Command: key}
}

func TestScanGoMaintenanceEmitsRedBuildAndTestSignals(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	signals := ScanGoMaintenance(context.Background(), "hypha://m31labs/reeve", dir, ScanOptions{
		AgentURI: "agent://reeve/conductor",
		GoBin:    "go",
	}, fakeRunner{
		"go build ./...": {Command: "go build ./...", ExitCode: 1, Output: "build failed"},
		"go test ./...":  {Command: "go test ./...", ExitCode: 1, Output: "test failed"},
	})
	if len(signals) != 2 {
		t.Fatalf("len(signals)=%d", len(signals))
	}
	if signals[0].Kind != "red-build" || signals[1].Kind != "red-test" {
		t.Fatalf("signals=%#v", signals)
	}
}

func TestScanGoMaintenanceSkipsNonGoWorkspace(t *testing.T) {
	signals := ScanGoMaintenance(context.Background(), "hypha://m31labs/reeve", t.TempDir(), ScanOptions{}, fakeRunner{})
	if len(signals) != 0 {
		t.Fatalf("signals=%#v", signals)
	}
}
