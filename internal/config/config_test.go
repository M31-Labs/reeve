package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadOverlaysDefaultsAndExpandsPaths(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	path := filepath.Join(tmp, "config.toml")
	if err := os.WriteFile(path, []byte(`pool_size = 5
poll_interval = "30s"

[commands]
graft = "/tmp/graft"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PoolSize != 5 {
		t.Fatalf("PoolSize=%d, want 5", cfg.PoolSize)
	}
	if cfg.AgentURI != "agent://reeve/conductor" {
		t.Fatalf("AgentURI=%q", cfg.AgentURI)
	}
	if cfg.PollInterval.Duration != 30*time.Second {
		t.Fatalf("PollInterval=%s", cfg.PollInterval)
	}
	if cfg.StateDir != filepath.Join(tmp, ".reeve", "state") {
		t.Fatalf("StateDir=%q", cfg.StateDir)
	}
	if cfg.Commands.Graft != "/tmp/graft" {
		t.Fatalf("Graft command=%q", cfg.Commands.Graft)
	}
}

func TestNormalizeResolvesUserSDKGoWhenGoMissingFromPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PATH", filepath.Join(tmp, "empty"))
	goPath := filepath.Join(tmp, "sdk", "go1.26.0", "bin", "go")
	if err := os.MkdirAll(filepath.Dir(goPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(goPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg, err := Normalize(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Commands.Go != goPath {
		t.Fatalf("Go command=%q, want %q", cfg.Commands.Go, goPath)
	}
}

func TestNormalizeResolvesUserGoBinBuckleyWhenMissingFromPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PATH", filepath.Join(tmp, "empty"))
	buckleyPath := filepath.Join(tmp, "go", "bin", "buckley")
	if err := os.MkdirAll(filepath.Dir(buckleyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(buckleyPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg, err := Normalize(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Commands.Buckley != buckleyPath {
		t.Fatalf("Buckley command=%q, want %q", cfg.Commands.Buckley, buckleyPath)
	}
}

func TestNormalizeResolvesWorkGraftWhenMissingFromPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PATH", filepath.Join(tmp, "empty"))
	graftPath := filepath.Join(tmp, "work", "graft", "graft")
	if err := os.MkdirAll(filepath.Dir(graftPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(graftPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg, err := Normalize(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Commands.Graft != graftPath {
		t.Fatalf("Graft command=%q, want %q", cfg.Commands.Graft, graftPath)
	}
}
