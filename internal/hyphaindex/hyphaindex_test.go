package hyphaindex

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestReadSpacesReadsMetadata(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "hyphae.db")
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Exec(`CREATE TABLE objects (
		id TEXT, type TEXT, space_id TEXT, title TEXT, metadata_json TEXT
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`INSERT INTO objects VALUES (?, ?, ?, ?, ?)`,
		"space.m31labs-reeve", "space", "m31labs/reeve", "Reeve", `{"uri":"hypha://m31labs/reeve","mode":"maintenance","priority":0.7}`); err != nil {
		t.Fatal(err)
	}
	spaces, err := ReadSpaces(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(spaces) != 1 {
		t.Fatalf("len(spaces)=%d", len(spaces))
	}
	if spaces[0].URI != "hypha://m31labs/reeve" {
		t.Fatalf("URI=%q", spaces[0].URI)
	}
	if mode, _ := StringField(spaces[0].Metadata, "mode"); mode != "maintenance" {
		t.Fatalf("mode=%q", mode)
	}
	if priority, ok := FloatField(spaces[0].Metadata, "priority"); !ok || priority != 0.7 {
		t.Fatalf("priority=%v ok=%v", priority, ok)
	}
}

func TestReadSpacesFallsBackToHyphaFrontmatter(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "hyphae.db")
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Exec(`CREATE TABLE objects (
		id TEXT, type TEXT, space_id TEXT, title TEXT, metadata_json TEXT
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`INSERT INTO objects VALUES (?, ?, ?, ?, ?)`,
		"space.m31labs-reeve", "space", "m31labs/reeve", "Reeve", `{}`); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "hypha")
	if err := os.WriteFile(bin, []byte(`#!/bin/sh
cat <<'EOF'
---
id: space.m31labs-reeve
type: space
uri: hypha://m31labs/reeve
mode: maintenance
priority: 0.8
reeve:
  workspace: reeve-local
---
EOF
`), 0o755); err != nil {
		t.Fatal(err)
	}
	spaces, err := ReadSpacesWithFallback(context.Background(), dbPath, bin)
	if err != nil {
		t.Fatal(err)
	}
	if len(spaces) != 1 {
		t.Fatalf("len(spaces)=%d", len(spaces))
	}
	if !spaces[0].Fallback {
		t.Fatalf("expected fallback marker")
	}
	if mode, _ := StringField(spaces[0].Metadata, "mode"); mode != "maintenance" {
		t.Fatalf("mode=%q", mode)
	}
	if ws, _ := NestedStringField(spaces[0].Metadata, "reeve", "workspace"); ws != "reeve-local" {
		t.Fatalf("workspace=%q", ws)
	}
}

func TestReadSpacesKeepsStaleRowsWhenFallbackFails(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "hyphae.db")
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Exec(`CREATE TABLE objects (
		id TEXT, type TEXT, space_id TEXT, title TEXT, metadata_json TEXT
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`INSERT INTO objects VALUES (?, ?, ?, ?, ?)`,
		"space.m31labs-missing", "space", "m31labs/missing", "Missing", `{}`); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "hypha")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho missing >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	spaces, err := ReadSpacesWithFallback(context.Background(), dbPath, bin)
	if err != nil {
		t.Fatal(err)
	}
	if len(spaces) != 1 {
		t.Fatalf("len(spaces)=%d", len(spaces))
	}
	if spaces[0].MetadataWarning == "" {
		t.Fatalf("expected metadata warning")
	}
	if spaces[0].URI != "hypha://m31labs/missing" {
		t.Fatalf("uri=%q", spaces[0].URI)
	}
}
