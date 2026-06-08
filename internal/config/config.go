package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

type Duration struct {
	time.Duration
}

func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

func (d *Duration) UnmarshalText(text []byte) error {
	v, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}
	d.Duration = v
	return nil
}

type Config struct {
	AgentURI        string          `toml:"agent_uri" json:"agent_uri"`
	SigningIdentity string          `toml:"signing_identity" json:"signing_identity"`
	PoolSize        int             `toml:"pool_size" json:"pool_size"`
	RefillThreshold int             `toml:"refill_threshold" json:"refill_threshold"`
	MaxRetries      int             `toml:"max_retries" json:"max_retries"`
	RetryBackoff    Duration        `toml:"retry_backoff" json:"retry_backoff"`
	AssessThreshold string          `toml:"assess_threshold" json:"assess_threshold"`
	BaseBranch      string          `toml:"base_branch" json:"base_branch"`
	DraftPR         bool            `toml:"draft_pr" json:"draft_pr"`
	PollInterval    Duration        `toml:"poll_interval" json:"poll_interval"`
	StalenessCap    Duration        `toml:"staleness_cap" json:"staleness_cap"`
	BlastCap        int             `toml:"blast_cap" json:"blast_cap"`
	StateDir        string          `toml:"state_dir" json:"state_dir"`
	WorktreeRoot    string          `toml:"worktree_root" json:"worktree_root"`
	QuarantineDir   string          `toml:"quarantine_dir" json:"quarantine_dir"`
	HyphaIndexPath  string          `toml:"hypha_index_path" json:"hypha_index_path"`
	Paused          bool            `toml:"paused" json:"paused"`
	Budget          Budget          `toml:"budget" json:"budget"`
	PriorityWeights PriorityWeights `toml:"priority_weights" json:"priority_weights"`
	Commands        Commands        `toml:"commands" json:"commands"`
}

type Budget struct {
	DailyTokens    int64 `toml:"daily_tokens" json:"daily_tokens"`
	PerSpaceTokens int64 `toml:"per_space_tokens" json:"per_space_tokens"`
}

type PriorityWeights struct {
	Gap       float64 `toml:"gap" json:"gap"`
	Project   float64 `toml:"project" json:"project"`
	Staleness float64 `toml:"staleness" json:"staleness"`
	Blast     float64 `toml:"blast" json:"blast"`
}

type Commands struct {
	Hypha   string `toml:"hypha" json:"hypha"`
	Graft   string `toml:"graft" json:"graft"`
	Buckley string `toml:"buckley" json:"buckley"`
	GH      string `toml:"gh" json:"gh"`
	Go      string `toml:"go" json:"go"`
}

func DefaultPath() string {
	if v := os.Getenv("REEVE_CONFIG"); v != "" {
		return v
	}
	return "~/.reeve/config.toml"
}

func DefaultConfig() Config {
	return Config{
		AgentURI:        "agent://reeve/conductor",
		SigningIdentity: "identity://m31labs/reeve-conductor",
		PoolSize:        3,
		RefillThreshold: 2,
		MaxRetries:      3,
		RetryBackoff:    Duration{Duration: 15 * time.Minute},
		AssessThreshold: "aligned",
		BaseBranch:      "main",
		DraftPR:         true,
		PollInterval:    Duration{Duration: 60 * time.Second},
		StalenessCap:    Duration{Duration: 168 * time.Hour},
		BlastCap:        50,
		StateDir:        "~/.reeve/state",
		WorktreeRoot:    "~/.reeve/worktrees",
		QuarantineDir:   "~/.reeve/quarantine",
		HyphaIndexPath:  "~/.hyphae/.index/hyphae.db",
		Budget: Budget{
			DailyTokens:    2_000_000,
			PerSpaceTokens: 400_000,
		},
		PriorityWeights: PriorityWeights{
			Gap:       0.4,
			Project:   0.3,
			Staleness: 0.2,
			Blast:     0.1,
		},
		Commands: Commands{
			Hypha:   "hypha",
			Graft:   "graft",
			Buckley: "buckley",
			GH:      "gh",
			Go:      "go",
		},
	}
}

func Load(path string) (Config, error) {
	if path == "" {
		path = DefaultPath()
	}
	path, err := ExpandPath(path)
	if err != nil {
		return Config{}, err
	}
	cfg := DefaultConfig()
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return Normalize(cfg)
		}
		return Config{}, err
	}
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	return Normalize(cfg)
}

func Normalize(cfg Config) (Config, error) {
	def := DefaultConfig()
	if cfg.AgentURI == "" {
		cfg.AgentURI = def.AgentURI
	}
	if cfg.SigningIdentity == "" {
		cfg.SigningIdentity = def.SigningIdentity
	}
	if cfg.PoolSize == 0 {
		cfg.PoolSize = def.PoolSize
	}
	if cfg.RefillThreshold == 0 {
		cfg.RefillThreshold = def.RefillThreshold
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = def.MaxRetries
	}
	if cfg.RetryBackoff.Duration == 0 {
		cfg.RetryBackoff = def.RetryBackoff
	}
	if cfg.AssessThreshold == "" {
		cfg.AssessThreshold = def.AssessThreshold
	}
	if cfg.BaseBranch == "" {
		cfg.BaseBranch = def.BaseBranch
	}
	if cfg.PollInterval.Duration == 0 {
		cfg.PollInterval = def.PollInterval
	}
	if cfg.StalenessCap.Duration == 0 {
		cfg.StalenessCap = def.StalenessCap
	}
	if cfg.BlastCap == 0 {
		cfg.BlastCap = def.BlastCap
	}
	if cfg.StateDir == "" {
		cfg.StateDir = def.StateDir
	}
	if cfg.WorktreeRoot == "" {
		cfg.WorktreeRoot = def.WorktreeRoot
	}
	if cfg.QuarantineDir == "" {
		cfg.QuarantineDir = def.QuarantineDir
	}
	if cfg.HyphaIndexPath == "" {
		cfg.HyphaIndexPath = def.HyphaIndexPath
	}
	if cfg.Budget.DailyTokens == 0 {
		cfg.Budget.DailyTokens = def.Budget.DailyTokens
	}
	if cfg.PriorityWeights == (PriorityWeights{}) {
		cfg.PriorityWeights = def.PriorityWeights
	}
	if cfg.Commands.Hypha == "" {
		cfg.Commands.Hypha = def.Commands.Hypha
	}
	if cfg.Commands.Hypha == def.Commands.Hypha {
		cfg.Commands.Hypha = resolveDefaultCommand(cfg.Commands.Hypha)
	}
	if cfg.Commands.Graft == "" {
		cfg.Commands.Graft = def.Commands.Graft
	}
	if cfg.Commands.Graft == def.Commands.Graft {
		cfg.Commands.Graft = resolveDefaultGraft(cfg.Commands.Graft)
	}
	if cfg.Commands.Buckley == "" {
		cfg.Commands.Buckley = def.Commands.Buckley
	}
	if cfg.Commands.Buckley == def.Commands.Buckley {
		cfg.Commands.Buckley = resolveDefaultBuckley(cfg.Commands.Buckley)
	}
	if cfg.Commands.GH == "" {
		cfg.Commands.GH = def.Commands.GH
	}
	if cfg.Commands.GH == def.Commands.GH {
		cfg.Commands.GH = resolveDefaultCommand(cfg.Commands.GH)
	}
	if cfg.Commands.Go == "" {
		cfg.Commands.Go = def.Commands.Go
	}
	if cfg.Commands.Go == def.Commands.Go {
		cfg.Commands.Go = resolveDefaultGo(cfg.Commands.Go)
	}
	var err error
	if cfg.StateDir, err = ExpandPath(cfg.StateDir); err != nil {
		return Config{}, err
	}
	if cfg.WorktreeRoot, err = ExpandPath(cfg.WorktreeRoot); err != nil {
		return Config{}, err
	}
	if cfg.QuarantineDir, err = ExpandPath(cfg.QuarantineDir); err != nil {
		return Config{}, err
	}
	if cfg.HyphaIndexPath, err = ExpandPath(cfg.HyphaIndexPath); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func resolveDefaultCommand(command string) string {
	if _, err := exec.LookPath(command); err == nil {
		return command
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return command
	}
	candidate := filepath.Join(home, ".local", "bin", command)
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
		return candidate
	}
	return command
}

func resolveDefaultGraft(command string) string {
	if _, err := exec.LookPath(command); err == nil {
		return command
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return command
	}
	candidates := []string{
		filepath.Join(home, ".local", "bin", command),
		filepath.Join(home, "go", "bin", command),
		filepath.Join(home, "work", "graft", command),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return candidate
		}
	}
	return command
}

func resolveDefaultBuckley(command string) string {
	if _, err := exec.LookPath(command); err == nil {
		return command
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return command
	}
	candidates := []string{
		filepath.Join(home, ".local", "bin", command),
		filepath.Join(home, "go", "bin", command),
		filepath.Join(home, "work", "buckley", command),
		filepath.Join(home, "work", "buckley-internal", command),
		filepath.Join(home, "work", "buckley-internal", "dist", "buckley_linux_amd64_v1", command),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return candidate
		}
	}
	return command
}

func resolveDefaultGo(command string) string {
	if _, err := exec.LookPath(command); err == nil {
		return command
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return command
	}
	matches, err := filepath.Glob(filepath.Join(home, "sdk", "go*", "bin", "go"))
	if err != nil || len(matches) == 0 {
		return command
	}
	for i := len(matches) - 1; i >= 0; i-- {
		if info, err := os.Stat(matches[i]); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return matches[i]
		}
	}
	return command
}

func ExpandPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	path = os.ExpandEnv(path)
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

func StarterTOML() string {
	return `agent_uri = "agent://reeve/conductor"
signing_identity = "identity://m31labs/reeve-conductor"
pool_size = 3
refill_threshold = 2
max_retries = 3
retry_backoff = "15m"
assess_threshold = "aligned"
base_branch = "main"
draft_pr = true
poll_interval = "60s"
staleness_cap = "168h"
blast_cap = 50
state_dir = "~/.reeve/state"
worktree_root = "~/.reeve/worktrees"
quarantine_dir = "~/.reeve/quarantine"
hypha_index_path = "~/.hyphae/.index/hyphae.db"
paused = false

[budget]
daily_tokens = 2000000
per_space_tokens = 400000

[priority_weights]
gap = 0.4
project = 0.3
staleness = 0.2
blast = 0.1

[commands]
hypha = "hypha"
graft = "graft"
buckley = "buckley"
gh = "gh"
go = "go"
`
}

func WriteStarter(path string) (string, error) {
	if path == "" {
		path = DefaultPath()
	}
	path, err := ExpandPath(path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return path, nil
		}
		return "", err
	}
	defer f.Close()
	if _, err := f.WriteString(StarterTOML()); err != nil {
		return "", err
	}
	cfg, err := Load(path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		return "", err
	}
	if err := os.MkdirAll(cfg.QuarantineDir, 0o755); err != nil {
		return "", err
	}
	return path, nil
}
