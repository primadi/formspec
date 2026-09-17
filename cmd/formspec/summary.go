// Command `formspec summary` — inspect and rebuild summary projections
// (docs/spec/backend/02-core-extended.md §6, todo 3.6.4).
//
//	formspec summary list    [--spec <path>] [--dsn <dsn>] [--json]
//	formspec summary rebuild <entity> [--spec <path>] [--dsn <dsn>]
//	                         [--workspace <slug>] [--subscriber <module/name>]
//	                         [--reset] [--dry-run] [--json]
//
// A summary Entity is a projection: it holds no data of its own, is written
// exclusively by durable events, and is excluded from backups because it can
// always be recomputed from source data (which is exactly why a rebuild is
// safe and never license-gated). `rebuild` replays that event history — from
// the Tier 2 durable stream, through the same filter → transform → handler path
// live delivery uses — so a rebuilt projection is identical to one produced by
// normal operation.
//
// Two properties worth knowing before running it:
//
//   - Replay re-runs every durable subscriber of the source events, not only
//     the one that feeds the projection. That is safe by contract — durable
//     delivery is at-least-once, so its handlers must be idempotent — but
//     `--subscriber` narrows it when you want to be surgical.
//   - Replay needs a *shared* durable stream backend (Redis/Valkey). With the
//     dev default (in-memory) the stream lives inside the server process, so a
//     separate CLI process has no history to replay. The command says so
//     instead of reporting a silent, empty success.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/primadi/formspec/internal/subscription"
	"github.com/primadi/formspec/internal/summary"
	formspec "github.com/primadi/formspec/resource"
)

// summaryFlags holds the parsed CLI arguments shared by list/rebuild.
type summaryFlags struct {
	specPath    string
	dsn         string
	workspace   string
	subscribers []string
	entityRef   string
	dryRun      bool
	jsonOut     bool
	reset       bool
}

// parseSummaryFlags parses `formspec summary <sub>` arguments. Manual parsing
// (like `seed`/`repl`) so flags may precede or follow the positional entity.
func parseSummaryFlags(sub string, args []string, wantEntity bool) (*summaryFlags, error) {
	f := &summaryFlags{specPath: "spec", dsn: "sqlite:.formspec/data.db"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--spec", "-spec":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--spec needs a value")
			}
			f.specPath = args[i+1]
			i++
		case "--dsn", "-dsn":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--dsn needs a value")
			}
			f.dsn = args[i+1]
			i++
		case "--workspace", "-workspace":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--workspace needs a value")
			}
			f.workspace = args[i+1]
			i++
		case "--subscriber", "-subscriber":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--subscriber needs a value")
			}
			f.subscribers = append(f.subscribers, args[i+1])
			i++
		case "--reset":
			f.reset = true
		case "--dry-run":
			f.dryRun = true
		case "--json":
			f.jsonOut = true
		case "--help", "-h":
			summaryUsage()
			os.Exit(0)
		default:
			if strings.HasPrefix(args[i], "-") {
				return nil, fmt.Errorf("unknown flag %q", args[i])
			}
			if !wantEntity {
				return nil, fmt.Errorf("unexpected argument %q", args[i])
			}
			if f.entityRef != "" {
				return nil, fmt.Errorf("unexpected argument %q (entity already set to %q)", args[i], f.entityRef)
			}
			f.entityRef = args[i]
		}
	}
	if wantEntity && f.entityRef == "" {
		return nil, fmt.Errorf("summary %s needs an entity (module/entity, module.entity, or a summary entity name)", sub)
	}
	return f, nil
}

func runSummary(args []string) {
	if len(args) == 0 {
		summaryUsage()
		os.Exit(1)
	}
	switch args[0] {
	case "rebuild":
		runSummaryRebuild(args[1:])
	case "list":
		runSummaryList(args[1:])
	case "help", "--help", "-h":
		summaryUsage()
	default:
		_, _ = fmt.Fprintf(os.Stderr, "formspec summary: unknown subcommand %q\n", args[0])
		summaryUsage()
		os.Exit(2)
	}
}

func summaryUsage() {
	_, _ = fmt.Fprintf(os.Stderr, "Usage: formspec summary <list|rebuild> [flags]\n\n")
	_, _ = fmt.Fprintf(os.Stderr, "  list                   list summary entities and their rebuild contract\n")
	_, _ = fmt.Fprintf(os.Stderr, "  rebuild <entity>       replay the durable event stream into the projection\n")
	_, _ = fmt.Fprintf(os.Stderr, "\nFlags:\n")
	_, _ = fmt.Fprintf(os.Stderr, "  --spec <path>          spec directory (default: spec)\n")
	_, _ = fmt.Fprintf(os.Stderr, "  --dsn <dsn>            database DSN (default: sqlite:.formspec/data.db)\n")
	_, _ = fmt.Fprintf(os.Stderr, "  --workspace <slug>     limit replay to one workspace (default: all)\n")
	_, _ = fmt.Fprintf(os.Stderr, "  --subscriber <m/n>     replay only this subscription (repeatable)\n")
	_, _ = fmt.Fprintf(os.Stderr, "  --reset                delete existing projection rows before replay\n")
	_, _ = fmt.Fprintf(os.Stderr, "  --dry-run              print the plan without touching the stream\n")
	_, _ = fmt.Fprintf(os.Stderr, "  --json                 machine-readable output\n")
}

// openSummaryApp boots the engine so the CLI sees the same registry, stream
// backend, and subscription wiring the server uses — a rebuild must not
// reconstruct any of that by hand, or it could diverge from live delivery.
func openSummaryApp(f *summaryFlags) (*formspec.App, error) {
	app, err := formspec.New(formspec.Config{
		SpecPath: f.specPath,
		DSN:      resolveDSN(f.dsn, f.specPath),
	})
	if err != nil {
		return nil, err
	}
	return app, nil
}

func runSummaryList(args []string) {
	f, err := parseSummaryFlags("list", args, false)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "formspec summary list: %v\n", err)
		os.Exit(2)
	}

	ctx := context.Background()
	app, err := openSummaryApp(f)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "formspec summary list: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = app.Close(ctx) }()

	reg := app.Registry()
	subReg := app.Subscriptions()

	// `list` is an inventory, not a gate: a projection that cannot be rebuilt
	// is exactly what an operator needs to see here, so the reason is reported
	// alongside the ones that can.
	type entry struct {
		Ref    string        `json:"ref"`
		Plan   *summary.Plan `json:"plan,omitempty"`
		Reason string        `json:"reason,omitempty"`
	}
	var entries []entry
	for _, e := range reg.GetEntitiesByCharacteristic("summary") {
		ref := e.Module + "/" + e.Name
		plan, err := summary.PlanRebuild(reg, subReg, ref)
		if err != nil {
			entries = append(entries, entry{Ref: ref, Reason: err.Error()})
			continue
		}
		entries = append(entries, entry{Ref: ref, Plan: plan})
	}

	if f.jsonOut {
		writeJSON(map[string]any{"summaries": entries})
		return
	}

	if len(entries) == 0 {
		fmt.Println("no summary entities in this spec tree")
		return
	}
	for _, e := range entries {
		if e.Plan == nil {
			fmt.Printf("%s  (not rebuildable)\n    %s\n", e.Ref, e.Reason)
			continue
		}
		p := e.Plan
		fmt.Printf("%s\n", e.Ref)
		fmt.Printf("  strategy: %s   sources: %s\n", p.Strategy, strings.Join(p.Sources, ", "))
		if len(p.Streams) == 0 {
			fmt.Printf("  streams:  none — no durable subscription feeds this projection\n")
		}
		for _, s := range p.Streams {
			fmt.Printf("  stream:   %s (via %s)\n", s.EventName, s.Subscription)
		}
		if len(p.Orphaned) > 0 {
			fmt.Printf("  orphaned: %s (declared source, no durable subscriber)\n", strings.Join(p.Orphaned, ", "))
		}
	}
}

func runSummaryRebuild(args []string) {
	f, err := parseSummaryFlags("rebuild", args, true)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "formspec summary rebuild: %v\n", err)
		os.Exit(2)
	}

	ctx := context.Background()
	app, err := openSummaryApp(f)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "formspec summary rebuild: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = app.Close(ctx) }()

	plan, err := summary.PlanRebuild(app.Registry(), app.Subscriptions(), f.entityRef)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "formspec summary rebuild: %v\n", err)
		os.Exit(1)
	}

	if f.dryRun {
		if f.jsonOut {
			writeJSON(map[string]any{"plan": plan, "dry_run": true})
			return
		}
		printRebuildPlan(plan)
		fmt.Println("\ndry run — nothing replayed")
		return
	}

	if len(plan.Streams) == 0 {
		_, _ = fmt.Fprintf(os.Stderr, "formspec summary rebuild: no durable subscription feeds %s/%s — nothing to replay\n", plan.Module, plan.Entity)
		if len(plan.Orphaned) > 0 {
			_, _ = fmt.Fprintf(os.Stderr, "  sources without a durable subscriber: %s\n", strings.Join(plan.Orphaned, ", "))
		}
		os.Exit(1)
	}

	resetRows := 0
	if f.reset {
		resetRows, err = resetSummaryProjection(ctx, app, plan, f.workspace)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "formspec summary rebuild: reset: %v\n", err)
			os.Exit(1)
		}
	}

	worker := app.StreamingWorker()
	if worker == nil {
		_, _ = fmt.Fprintf(os.Stderr, "formspec summary rebuild: engine has no streaming worker\n")
		os.Exit(1)
	}
	if app.Stream() == nil || isMemoryStream(app) {
		_, _ = fmt.Fprintf(os.Stderr, "formspec summary rebuild: warning: stream backend is in-memory — a separate process has no durable history to replay. Configure a Redis/Valkey datastore (kind: Datastore) for the event stream.\n")
	}

	runID := time.Now().UTC().Format("20060102T150405Z")
	res := worker.ReplaySummaryProjection(ctx, plan.ReplayStreams(), subscription.ReplayOptions{
		WorkspaceID: f.workspace,
		RunID:       runID,
		Subscribers: f.subscribers,
	})

	if f.jsonOut {
		writeJSON(map[string]any{
			"plan":       plan,
			"run_id":     runID,
			"reset_rows": resetRows,
			"replay":     res,
			"incomplete": res.Incomplete(),
		})
	} else {
		printRebuildPlan(plan)
		if f.reset {
			fmt.Printf("reset: deleted %d existing projection row(s)\n", resetRows)
		}
		fmt.Printf("replay run %s\n", runID)
		for _, s := range res.Streams {
			fmt.Printf("  [%s] %s  read=%d dispatched=%d skipped=%d failed=%d", s.EventName, s.Subscription, s.Read, s.Dispatched, s.Skipped, s.Failed)
			if s.Err != "" {
				fmt.Printf("  error=%s", s.Err)
			}
			fmt.Println()
		}
		fmt.Printf("total: read=%d dispatched=%d skipped=%d failed=%d\n", res.Read, res.Dispatched, res.Skipped, res.Failed)
		if res.Read == 0 {
			fmt.Println("note: the durable stream held no entries for these events — a summary fed by events emitted before the stream existed cannot be rebuilt from it.")
		}
	}

	if res.Incomplete() {
		_, _ = fmt.Fprintln(os.Stderr, "\nformspec summary rebuild: incomplete — fix the failures above and re-run (the live worker owns retry; this run acked failures in its own group).")
		os.Exit(1)
	}
}

// printRebuildPlan prints the resolved plan.
func printRebuildPlan(plan *summary.Plan) {
	fmt.Printf("summary %s/%s\n", plan.Module, plan.Entity)
	fmt.Printf("  strategy: %s   join_key: %s   table: %s\n", plan.Strategy, plan.JoinKey, qualifiedTable(*plan))
	if plan.Window != "" || plan.Since != "" {
		fmt.Printf("  window: %s   since: %s\n", plan.Window, plan.Since)
	}
	fmt.Printf("  sources: %s\n", strings.Join(plan.Sources, ", "))
	if len(plan.Orphaned) > 0 {
		fmt.Printf("  orphaned: %s (declared source, no durable subscriber — cannot be replayed)\n", strings.Join(plan.Orphaned, ", "))
	}
}

// tableNamePattern guards the identifiers interpolated into the reset DELETE.
// The name comes from our own DDL generation, but a rebuild should never build
// SQL from an unvalidated string.
var tableNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// qualifiedTable renders the projection's table name, schema-qualified when the
// driver uses schemas (PostgreSQL).
func qualifiedTable(plan summary.Plan) string {
	if plan.Schema == "" {
		return plan.Table
	}
	return plan.Schema + "." + plan.Table
}

// resetSummaryProjection deletes the projection's existing rows so replay
// starts from an empty projection. Only meaningful for an idempotent
// upsert-style handler, which is what `rebuild.strategy: full` implies;
// `partial` rebuilds leave the rest of the projection in place.
func resetSummaryProjection(ctx context.Context, app *formspec.App, plan *summary.Plan, workspace string) (int, error) {
	if plan.Table == "" {
		return 0, fmt.Errorf("table name for %s/%s is unknown — is the schema synced?", plan.Module, plan.Entity)
	}
	if !tableNamePattern.MatchString(plan.Table) {
		return 0, fmt.Errorf("refusing to build SQL from table name %q", plan.Table)
	}
	if plan.Schema != "" && !tableNamePattern.MatchString(plan.Schema) {
		return 0, fmt.Errorf("refusing to build SQL from schema name %q", plan.Schema)
	}
	if plan.Strategy == "partial" {
		return 0, fmt.Errorf("--reset requires rebuild.strategy: full — this entity declares %q, whose projection must keep rows outside the rebuilt window", plan.Strategy)
	}

	stmt := "DELETE FROM " + qualifiedTable(*plan)
	args := []any{}
	if workspace != "" {
		if app.Database().DriverName() == "postgres" {
			stmt += " WHERE tenant_id = $1"
		} else {
			stmt += " WHERE tenant_id = ?"
		}
		args = append(args, workspace)
	}

	if _, err := app.Database().ExecContext(ctx, stmt, args...); err != nil {
		return 0, err
	}
	n, err := rowsAffected(ctx, app, qualifiedTable(*plan), workspace)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// rowsAffected counts the projection's remaining rows after a reset — the
// delete itself cannot report a portable count across drivers.
func rowsAffected(ctx context.Context, app *formspec.App, table, workspace string) (int, error) {
	stmt := "SELECT COUNT(*) FROM " + table
	args := []any{}
	if workspace != "" {
		if app.Database().DriverName() == "postgres" {
			stmt += " WHERE tenant_id = $1"
		} else {
			stmt += " WHERE tenant_id = ?"
		}
		args = append(args, workspace)
	}
	var n int
	if err := app.Database().QueryRowContext(ctx, stmt, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// writeJSON prints v as indented JSON.
func writeJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "formspec: encode json: %v\n", err)
		os.Exit(1)
	}
}

// isMemoryStream reports whether the engine's stream backend is the in-memory
// dev default, which cannot be shared with a separate CLI process.
func isMemoryStream(app *formspec.App) bool {
	return fmt.Sprintf("%T", app.Stream()) == "*stream.Memory"
}
