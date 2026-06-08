package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"m31labs.dev/reeve/internal/config"
	"m31labs.dev/reeve/internal/fleet"
	"m31labs.dev/reeve/internal/graftcoord"
	"m31labs.dev/reeve/internal/hyphaindex"
	"m31labs.dev/reeve/internal/runner"
	"m31labs.dev/reeve/internal/taskplan"
)

const usage = `reeve - fleet-maintenance conductor

Usage:
  reeve init --print
  reeve init --write [--config <path>]
  reeve run --dry-run --once [--json] [--config <path>]
  reeve run --execute --once [--allow-registered-workspace] [--json] [--config <path>]
  reeve plan [--scan] [--apply] [--json] [--config <path>]
  reeve status --json [--config <path>]
  reeve doctor [--json] [--config <path>]
`

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "reeve:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprint(stdout, usage)
		return nil
	}
	switch args[0] {
	case "init":
		return cmdInit(args[1:], stdout)
	case "run":
		return cmdRun(args[1:], stdout)
	case "plan":
		return cmdPlan(args[1:], stdout)
	case "status":
		return cmdStatus(args[1:], stdout)
	case "doctor":
		return cmdDoctor(args[1:], stdout)
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return nil
	default:
		return fmt.Errorf("unknown command %q\n%s", args[0], usage)
	}
}

func cmdPlan(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", config.DefaultPath(), "config path")
	scan := fs.Bool("scan", false, "run deterministic read-only workspace checks")
	apply := fs.Bool("apply", false, "create or reopen coord tasks for planned actions")
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *apply && !*scan {
		return errors.New("plan --apply requires --scan")
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	report, err := runner.BuildReportWithOptions(context.Background(), cfg, runner.Options{DryRun: !*apply, Scan: *scan})
	if err != nil {
		return err
	}
	if *apply {
		specs := make([]taskplan.CreateSpec, 0, len(report.Plan))
		for _, item := range report.Plan {
			specs = append(specs, item.Spec)
		}
		report.Apply = graftcoord.ApplyPlan(context.Background(), cfg.Commands.Graft, specs, graftcoord.ApplyOptions{})
	}
	if *jsonOut {
		return writeJSON(stdout, report)
	}
	renderReport(stdout, report)
	if len(report.Plan) > 0 {
		fmt.Fprintln(stdout, "plan:")
		for _, item := range report.Plan {
			fmt.Fprintf(stdout, "  would-%s %s [%s] workspace=%s priority=%s\n",
				item.Spec.Action, item.Spec.Title, item.SpaceID, item.Spec.Workspace, item.Spec.Priority)
		}
	}
	if len(report.Apply) > 0 {
		fmt.Fprintln(stdout, "apply:")
		for _, result := range report.Apply {
			status := "ok"
			if result.Error != "" {
				status = "error: " + result.Error
			} else if result.DryRun {
				status = "dry-run"
			} else if result.TaskID != "" {
				status = "task=" + result.TaskID
			}
			fmt.Fprintf(stdout, "  %s %s [%s]\n", result.Action, result.Title, status)
		}
	}
	return nil
}

func cmdInit(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", config.DefaultPath(), "config path")
	printOnly := fs.Bool("print", false, "print starter config")
	write := fs.Bool("write", false, "write starter config if absent")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *printOnly || !*write {
		fmt.Fprint(stdout, config.StarterTOML())
		return nil
	}
	path, err := config.WriteStarter(*configPath)
	if err != nil {
		return err
	}
	spacePath, created, err := ensureReeveSpace()
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "config: %s\n", path)
	if created {
		fmt.Fprintf(stdout, "space:  %s (created)\n", spacePath)
	} else {
		fmt.Fprintf(stdout, "space:  %s (exists)\n", spacePath)
	}
	return nil
}

func cmdRun(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", config.DefaultPath(), "config path")
	dryRun := fs.Bool("dry-run", false, "print planned actions without mutation")
	once := fs.Bool("once", false, "run one assignment pass")
	execute := fs.Bool("execute", false, "execute one managed coord task")
	allowRegistered := fs.Bool("allow-registered-workspace", false, "allow execution directly in a registered workspace")
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	if *execute {
		if !*once {
			return errors.New("run --execute requires --once")
		}
		report, err := runner.ExecuteOnce(context.Background(), cfg, runner.ExecutionOptions{
			DryRun:                   *dryRun,
			AllowRegisteredWorkspace: *allowRegistered,
		})
		if err != nil {
			return err
		}
		if *jsonOut {
			return writeJSON(stdout, report)
		}
		renderExecutionReport(stdout, report)
		return nil
	}
	if !*dryRun || !*once {
		return errors.New("run requires either --dry-run --once or --execute --once")
	}
	report, err := runner.BuildReport(context.Background(), cfg, true)
	if err != nil {
		return err
	}
	if *jsonOut {
		return writeJSON(stdout, report)
	}
	renderReport(stdout, report)
	return nil
}

func renderExecutionReport(w io.Writer, report runner.ExecutionReport) {
	mode := "execute"
	if report.DryRun {
		mode = "execute dry-run"
	}
	fmt.Fprintf(w, "Reeve %s: ", mode)
	if report.Selected == nil {
		fmt.Fprintln(w, "no managed task selected")
	} else {
		fmt.Fprintf(w, "selected %s score=%.3f %s\n", report.Selected.TaskID, report.Selected.Score, report.Selected.Title)
		fmt.Fprintf(w, "  space=%s workspace=%s signal=%s target=%s\n",
			report.Selected.SpaceID, report.Selected.WorkspaceName, report.Selected.SignalKind, report.Selected.Target)
	}
	for _, warning := range report.Warnings {
		fmt.Fprintf(w, "warn: %s\n", warning)
	}
	if report.Worktree != nil {
		fmt.Fprintf(w, "worktree: %s branch=%s\n", report.Worktree.Path, report.Worktree.Branch)
	}
	if report.Landing != nil {
		fmt.Fprintf(w, "landing: branch=%s pr=%s\n", report.Landing.Branch, report.Landing.PRURL)
	}
	for _, update := range report.Updates {
		status := update.Status
		if update.Error != "" {
			status = "error: " + update.Error
		}
		fmt.Fprintf(w, "coord: %s %s\n", update.TaskID, status)
	}
	if report.Result != nil {
		fmt.Fprintf(w, "result: %s %s\n", report.Result.Decision.State, report.Result.Decision.Reason)
		if report.Result.SporeID != "" {
			fmt.Fprintf(w, "spore: %s\n", report.Result.SporeID)
		}
	}
}

func cmdStatus(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", config.DefaultPath(), "config path")
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	report, err := runner.BuildReport(context.Background(), cfg, false)
	if err != nil {
		return err
	}
	if *jsonOut {
		return writeJSON(stdout, report)
	}
	renderReport(stdout, report)
	return nil
}

type doctorReport struct {
	OK     bool          `json:"ok"`
	Checks []doctorCheck `json:"checks"`
}

type doctorCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

func cmdDoctor(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", config.DefaultPath(), "config path")
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	report := doctorReport{OK: true}
	check := func(name, status, message string) {
		if status == "error" {
			report.OK = false
		}
		report.Checks = append(report.Checks, doctorCheck{Name: name, Status: status, Message: message})
	}

	configOK := false
	cfg, err := config.Load(*configPath)
	if err != nil {
		check("config", "error", err.Error())
	} else {
		configOK = true
		check("config", "ok", "config parsed")
	}
	spacePath, created, err := ensureReeveSpacePathOnly()
	if err != nil {
		check("reeve_space", "error", err.Error())
	} else if created {
		check("reeve_space", "warn", "canonical space missing: "+spacePath)
	} else {
		check("reeve_space", "ok", spacePath)
	}
	if configOK && cfg.HyphaIndexPath != "" {
		spaces, rerr := hyphaindex.ReadSpacesWithFallback(context.Background(), cfg.HyphaIndexPath, cfg.Commands.Hypha)
		if rerr != nil {
			check("hypha_index", "error", rerr.Error())
		} else {
			opted := 0
			for _, s := range spaces {
				if mode, ok := hyphaindex.StringField(s.Metadata, "mode"); ok && mode == fleet.ModeMaintenance {
					opted++
				}
			}
			check("hypha_index", "ok", fmt.Sprintf("%d spaces discovered, %d maintenance opt-in", len(spaces), opted))
		}
	}
	if cfg.StateDir != "" {
		checkDir("state_dir", cfg.StateDir, check)
	}
	if cfg.WorktreeRoot != "" {
		checkDir("worktree_root", cfg.WorktreeRoot, check)
	}
	if cfg.QuarantineDir != "" {
		checkDir("quarantine_dir", cfg.QuarantineDir, check)
	}
	checkCommand("hypha", cfg.Commands.Hypha, check)
	checkCommand("graft", cfg.Commands.Graft, check)
	checkCommand("buckley", cfg.Commands.Buckley, check)
	checkCommand("gh", cfg.Commands.GH, check)
	checkCommand("go", cfg.Commands.Go, check)
	checkCommandRun("graft_health", cfg.Commands.Graft, []string{"version"}, check)
	checkCommandRun("buckley_health", cfg.Commands.Buckley, []string{"doctor"}, check)
	checkCommandRun("gh_health", cfg.Commands.GH, []string{"auth", "status"}, check)
	checkCommandRun("go_health", cfg.Commands.Go, []string{"version"}, check)

	if *jsonOut {
		return writeJSON(stdout, report)
	}
	for _, c := range report.Checks {
		fmt.Fprintf(stdout, "%-16s %-5s %s\n", c.Name, c.Status, c.Message)
	}
	if !report.OK {
		return errors.New("doctor found errors")
	}
	return nil
}

func renderReport(w io.Writer, report runner.Report) {
	mode := "status"
	if report.DryRun {
		mode = "dry-run"
	}
	fmt.Fprintf(w, "Reeve %s: %d discovered, %d opted-in, %d eligible, %d candidate(s)\n",
		mode, report.Summary.DiscoveredSpaces, report.Summary.OptedInSpaces, report.Summary.EligibleSpaces, report.Summary.Candidates)
	fmt.Fprintf(w, "  traces=%d unreviewed_spores=%d coord_open=%d coord_failures=%d\n",
		report.Summary.ActiveTraces, report.Summary.UnreviewedSpores, report.Summary.CoordOpenTasks, report.Summary.CoordFailures)
	for _, warning := range report.Warnings {
		fmt.Fprintf(w, "warn: %s\n", warning)
	}
	for _, s := range report.Spaces {
		state := "ineligible"
		if s.Eligible {
			state = "eligible"
		}
		reasons := strings.Join(s.Reasons, "; ")
		if reasons == "" {
			reasons = "ready"
		}
		fmt.Fprintf(w, "  %-10s %-24s mode=%s priority=%.2f workspace=%s %s\n",
			state, s.SpaceID, s.Mode, s.Priority, s.WorkspaceName, reasons)
		if s.Warning != "" {
			fmt.Fprintf(w, "    warn: %s\n", s.Warning)
		}
	}
	if report.Selected != nil {
		fmt.Fprintf(w, "selected: %s score=%.3f %s\n", report.Selected.ID, report.Selected.Score, report.Selected.Title)
	} else {
		fmt.Fprintln(w, "selected: none")
	}
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func checkDir(name, path string, check func(string, string, string)) {
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		check(name, "ok", path)
		return
	}
	if os.IsNotExist(err) {
		check(name, "warn", "directory does not exist yet: "+path)
		return
	}
	if err != nil {
		check(name, "error", err.Error())
		return
	}
	check(name, "error", "not a directory: "+path)
}

func checkCommand(name, command string, check func(string, string, string)) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		check("command_"+name, "error", "empty command")
		return
	}
	if _, err := exec.LookPath(parts[0]); err != nil {
		check("command_"+name, "warn", parts[0]+" not found on PATH")
		return
	}
	check("command_"+name, "ok", parts[0])
}

func checkCommandRun(name, command string, extra []string, check func(string, string, string)) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		check(name, "warn", "command is empty")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, parts[0], append(parts[1:], extra...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		check(name, "warn", msg)
		return
	}
	msg := firstLine(strings.TrimSpace(string(out)))
	if msg == "" {
		msg = "ok"
	}
	check(name, "ok", msg)
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func ensureReeveSpacePathOnly() (string, bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false, err
	}
	path := filepath.Join(home, ".hyphae", "spaces", "m31labs-reeve", "SPACE.md")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return path, true, nil
		}
		return "", false, err
	}
	return path, false, nil
}

func ensureReeveSpace() (string, bool, error) {
	path, missing, err := ensureReeveSpacePathOnly()
	if err != nil || !missing {
		return path, false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", false, err
	}
	body := `---
mdpp: "0.1"
id: space.m31labs-reeve
type: space
uri: hypha://m31labs/reeve
scope: team
visibility: public
authority: m31labs
status: active
created: 2026-06-07
owners:
  - identity://odvcencio
trust_default: team
tags: [orchestration, autonomous-agents, fleet, maintenance, governance]
---

# Space: m31labs/reeve

Reeve is the fleet-maintenance conductor space.
`
	return path, true, os.WriteFile(path, []byte(body), 0o644)
}
