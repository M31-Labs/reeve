# Reeve

Reeve is a fleet-maintenance conductor for a portfolio of local project spaces.
It discovers maintenance-mode projects from Hyphae, derives mechanical upkeep
signals, records work in Graft, executes safe fixes through Buckley, and projects
the durable trail back into Hyphae traces and spores.

The core idea is simple: keep many quiet projects at their maintenance done-bar
without asking a human to live inside every repo. Reeve is the one long-running
conductor that coordinates the loop.

## Why It Exists

Teams with many active tools tend to accumulate low-grade drag: stale analyses,
unreviewed proposals, red tests, dependency drift, and repo-health warnings. Any
single item is small. Across dozens of repos, the surface becomes a daily tax.

Reeve turns that tax into an autonomous maintenance lane:

- discover projects that explicitly opted into maintenance mode
- rank routine upkeep by severity, priority, staleness, and blast radius
- create idempotent Graft coord tasks with durable Reeve metadata
- execute one scoped task in a fresh Buckley worktree
- land only safe green changes, otherwise propose a signed Hyphae spore
- preserve every decision in traces, receipts, and coord status

## The Stack In Tandem

Reeve is intentionally a conductor, not a replacement for the tools beneath it.

| Tool | Role in the loop |
| --- | --- |
| Hyphae | Canonical spaces, frontmatter config, traces, spores, assessments, receipts |
| Graft | Operational task queue and workspace registry |
| Buckley | Fresh-context task execution, isolated worktrees, commit discipline |
| Arbiter | Governed prioritization and policy oracle for the full v1 path |
| Reeve | Fleet policy, refill loop, safety rails, lifecycle orchestration |

The interesting part is the composition: Reeve gives existing agent tools a
single maintenance axis, an assignment discipline, and a durable audit trail.

## Current Status

Reeve is under active construction. The repository already contains the
foundation needed for a local v1 conductor, with the remaining work focused on
the full unattended loop.

Implemented now:

- `reeve init` writes starter config and ensures the canonical Reeve Hyphae space.
- `reeve status` reads Hyphae, Graft, budget counters, active traces, and spores.
- `reeve plan --scan` discovers eligible maintenance spaces and emits deterministic
  maintenance tasks.
- `reeve plan --scan --apply` creates or reopens Graft coord tasks.
- `reeve run --execute --once` selects one managed task, creates an isolated
  Buckley worktree, wraps execution in Hyphae rituals, and updates Graft status.
- Retry, backoff, dead-letter, quarantine, and propose-only paths have focused
  unit coverage.
- The local test suite covers config, discovery, ranking, producers, coord
  trailers, Graft command adapters, Buckley execution wrappers, and worktree
  safety.

Still in progress for full v1:

- Arbiter-backed ranking/governance as the final selection oracle.
- Pool-wide multi-slot polling and refill cadence.
- Branch push and PR creation for landed tasks.
- Spend accounting from real Buckley token usage.
- 2-3 live maintenance opt-in spaces and a full working-day unattended run.

## How It Works

```text
Hyphae SPACE.md metadata
        |
        v
Reeve fleet discovery  ---- Graft workspace registry
        |
        v
Deterministic producers
        |
        v
Graft coord queue  <---- dedup and reopen logic
        |
        v
Reeve assigner
        |
        v
Hypha assess -> trace start -> Buckley isolated worktree -> green check
        |
        +--> safe, green, allowed: integration branch + PR
        |
        +--> ambiguous, ask, or not green: signed Hyphae spore
        |
        v
trace done + coord status + durable trailer metadata
```

The conductor only acts on tasks that carry a valid fenced `reeve-task` trailer.
Manual or malformed tasks are visible in status output but are never executed.

## Maintenance Signals

`reeve plan --scan` is deterministic-first. It does not ask a model to invent
work when a mechanical signal exists.

Current producer signals:

- red Go build: `go build ./...`
- red Go tests: `go test ./...`
- outdated Go modules: `go list -m -u -json all`
- aging unreviewed Hyphae spores
- stale Hyphae analyses
- Hyphae doctor findings
- dead-code analysis results when `hypha analyze dead` is available

Every emitted task receives a stable dedup key:

```text
sha1(space_uri | axis | signal_kind | target)
```

Open tasks with the same key are skipped. Completed tasks with a still-firing
signal are reopened as regressions.

## Safety Model

Reeve is conservative by default.

- No project is touched unless its Hyphae `SPACE.md` opts into `mode: maintenance`.
- Missing or invalid `priority` makes a maintenance space ineligible.
- `mode: active` and `mode: frozen` are never auto-maintained.
- Execution happens in an isolated Buckley worktree under `worktree_root`.
- The conductor refuses unmanaged tasks.
- Dirty or partial worktrees are quarantined.
- Failed tasks requeue with `retry_backoff`; exhausted tasks become dead-letter.
- Ambiguous Buckley approval, failed green checks, or risky outcomes become
  signed spores instead of landed changes.
- v1 never auto-merges to `main`.

## Per-Space Opt-In

Each project opts in through Hyphae frontmatter:

```yaml
mode: maintenance
priority: 0.7
green_check: "go build ./... && go test ./..."
reeve:
  workspace: "example-project"
```

`reeve.workspace` maps the Hyphae space to a Graft workspace name when the
defaults do not line up.

## Quickstart

Install or build the required local tools first:

- `hypha`
- `graft`
- `buckley`
- Go 1.26 or newer

Then:

```sh
reeve init --write
reeve doctor
reeve status --json
reeve plan --scan
```

To create or reopen planned tasks:

```sh
reeve plan --scan --apply
```

To execute one managed task:

```sh
reeve run --execute --once
```

For a read-only execution preview:

```sh
reeve run --execute --dry-run --once --json
```

## Configuration

Starter config lives at `~/.reeve/config.toml`:

```toml
agent_uri = "agent://reeve/conductor"
signing_identity = "identity://m31labs/reeve-conductor"
pool_size = 3
refill_threshold = 2
max_retries = 3
retry_backoff = "15m"
assess_threshold = "aligned"
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
go = "go"
```

## Development

Run the full suite:

```sh
go test ./...
go build ./...
```

The tests use fake `hypha`, `graft`, `buckley`, and Go command shims where the
real toolchain would otherwise mutate state.

## Repository Visibility

Reeve is a good public showcase once the repo is scrubbed for private operational
state. The source is designed to demonstrate how Hyphae, Graft, Buckley, Arbiter,
and a small Go conductor can work together in an autonomous maintenance loop.

The repo should stay private only while it contains unfinished internal notes,
secrets, or owner-specific state. The code itself is written to be publishable:
configuration defaults use local paths, runtime state lives outside the repo,
and Hyphae/Graft/Buckley integrations are explicit CLI boundaries.

## Roadmap To V1

- Replace weighted in-process ranking with Arbiter strategy evaluation.
- Run a pool of assignments with one in-flight task per space.
- Add branch push and PR opening for `landed` outcomes.
- Record token spend from Buckley runs into the Reeve spend counter.
- Seed 2-3 quiet maintenance spaces and run a full working-day unattended smoke.
- Document the resulting traces, spores, receipts, and PR queue as the v1 proof.
