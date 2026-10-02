// Command `formspec repl` — interactive Starlark console with full ctx.*
// access (docs/cli-tools/02-formspec-cli.md §4). A first-class feature, not a
// one-off debug tool — also the surface for AI Agent Skill debugging.
//
//	formspec repl [--spec <path>] [--dsn <dsn>] [--environment <env>] [--no-sync]
//	formspec repl -e 'ctx.db().query("SELECT 1")'    # one-shot expression (scriptable)
//	formspec repl -f migrations/dedupe.star         # one-shot script file
//
// `--no-sync` opens the datastore WITHOUT syncing the schema. Its reason to
// exist is the repair the migration gate asks for: a migration refused because
// the data does not satisfy it yet (duplicates blocking a unique index) also
// refuses the normal boot — including this console's, which syncs the schema on
// the way up. The repair therefore has to run against a boot that does not touch
// the schema (docs/spec/backend/01-core-basic.md §4.4).
//
// The console predeclares `ctx` (CtxAPI wired to the app's live datastore
// resolver — todo 2.9.1–2.9.3), `resource` (an empty ResourceAPI), and the
// `ok`/`fail` result helpers, so expressions behave like they would inside an
// action script.
package main

import (
	"context"
	"fmt"
	"os"

	"go.starlark.net/repl"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"

	fsstarlark "github.com/primadi/formspec/internal/starlark"
	formspec "github.com/primadi/formspec/resource"
)

func runRepl(args []string) {
	// Defaults come from formspec-app.yaml, exactly as `formspec dev` resolves
	// them (project_defaults.go). Without this the console opened
	// sqlite:.formspec/data.db while the dev server served the DSN the config
	// file declared — the repair landed in a database nobody was looking at.
	d := loadProjectDefaults()
	specPath := d.SpecPath
	dsn := d.DSN
	workspaceID := d.WorkspaceID
	workspaceExplicit := d.WorkspaceExplicit
	environment := ""
	expr := ""
	scriptFile := ""
	noSync := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--spec", "-spec":
			if i+1 < len(args) {
				specPath = args[i+1]
				i++
			}
		case "--dsn", "-dsn":
			if i+1 < len(args) {
				dsn = args[i+1]
				i++
			}
		case "--workspace", "-workspace", "--workspace-id":
			if i+1 < len(args) {
				workspaceID = args[i+1]
				workspaceExplicit = true
				i++
			}
		case "--environment", "-environment":
			if i+1 < len(args) {
				environment = args[i+1]
				i++
			}
		case "--no-sync", "-no-sync":
			noSync = true
		case "-e", "--eval":
			if i+1 < len(args) {
				expr = args[i+1]
				i++
			}
		case "-f", "--file":
			if i+1 < len(args) {
				scriptFile = args[i+1]
				i++
			}
		case "--help", "-h":
			_, _ = fmt.Fprintf(os.Stderr, "Usage: formspec repl [--spec <path>] [--dsn <dsn>] [--workspace <slug>] [--environment <env>] [--no-sync] [-e <expr> | -f <script.star>]\n")
			os.Exit(0)
		default:
			_, _ = fmt.Fprintf(os.Stderr, "formspec repl: unknown flag %q\n", args[i])
			os.Exit(2)
		}
	}

	// Anchor the relative SQLite DSN to the project root and resolve the active
	// workspace against the declared ones (#48) — the same two steps `dev`
	// performs, in the same order.
	specPath, dsn, workspaceID = finishProjectDefaults(specPath, dsn, workspaceID, workspaceExplicit)

	app, err := formspec.New(formspec.Config{SpecPath: specPath, DSN: dsn, SkipSchemaSync: noSync})
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		if !noSync {
			// The refusal this console can hit is the one it is meant to repair —
			// say so, instead of leaving the operator to discover that the repair
			// surface cannot start under the condition it exists for.
			_, _ = fmt.Fprintf(os.Stderr, "hint: if a migration was refused, re-run with --no-sync to open the datastore without syncing the schema\n")
		}
		os.Exit(1)
	}
	defer func() { _ = app.Close(context.Background()) }()

	// Build the ctx object with the same live datastore resolver the action
	// dispatcher wires into scripts, so ctx.db()/ctx.cache()/... resolve to
	// real backends in the console.
	//
	// The workspace is the RESOLVED one (#48 / config file), not a literal: the
	// console writes under its tenant, so `ctx.db()` reading nothing while the
	// app shows rows is the symptom of getting this wrong (measured 2026-09-29:
	// the console used "demo" while the kafe app ran as "kafe").
	ctxObj := fsstarlark.NewCtxAPI(workspaceID, "", "repl", "", nil)
	ctxObj.SetDatastoreResolver(formspec.NewCtxPrimitiveResolver(app.Database(), formspec.StateDirFromDSN(dsn)))
	ctxObj.Config = fsstarlark.NewConfigAPI(map[string]any{})
	_, _ = fmt.Fprintf(os.Stderr, "[formspec] repl: spec=%s dsn=%s workspace=%s\n", specPath, dsn, workspaceID)
	if environment != "" {
		// Control Plane environment policy (platform/04 §7) is deferred; the
		// flag is accepted for forward-compat but has no effect yet.
		_, _ = fmt.Fprintf(os.Stderr, "formspec repl: note: --environment %q accepted; environment policy is deferred (Control Plane)\n", environment)
	}

	// Predeclare the same helpers action scripts get.
	predeclared := starlark.StringDict{
		"ctx":      ctxObj,
		"resource": fsstarlark.NewResourceAPI("", "", "", 0, map[string]any{}),
		"ok":       starlark.NewBuiltin("ok", okBuiltin),
		"fail":     starlark.NewBuiltin("fail", failBuiltin),
	}

	thread := &starlark.Thread{Name: "repl"}
	thread.SetLocal("context", context.Background())

	if expr != "" {
		// One-shot evaluation (scriptable / testable).
		if err := replEval(thread, predeclared, expr); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if scriptFile != "" {
		// One-shot script file. This is the sanctioned surface for a data repair
		// that a migration refused: repairs are run once, by an operator, and are
		// not part of the manifest — so they need a way to run that is honest
		// about being out-of-band, not a kind that pretends to declare them.
		src, err := os.ReadFile(scriptFile)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: read %s: %v\n", scriptFile, err)
			os.Exit(1)
		}
		if err := replExecFile(thread, predeclared, scriptFile, string(src)); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: %s: %v\n", scriptFile, err)
			os.Exit(1)
		}
		fmt.Printf("Ran %s.\n", scriptFile)
		return
	}

	repl.REPLOptions(syntax.LegacyFileOptions(), thread, predeclared)
}

// replExecFile runs a whole Starlark FILE against the predeclared globals.
//
// It exists because `-f` used to go through replEval — i.e. through
// syntax.ParseCompoundStmt, the REPL's MODAL parser. That parser is right for a
// console prompt (it decides whether the line you typed is a complete
// statement, and a blank line ends the input) and wrong for a file: it returns
// only the first statement, so a repair script executed at most its first
// statement, and a file whose first line was a comment executed NOTHING. Exit
// status was 0 and the console printed `Ran <file>.` — a silent no-op reported
// as success, on the one surface whose whole job is a data repair.
//
// Measured 2026-09-28 with a two-line file: `print("FIRST") / print("SECOND")`
// printed only FIRST; the same file with a leading comment printed nothing.
func replExecFile(thread *starlark.Thread, predeclared starlark.StringDict, filename, src string) error {
	_, err := starlark.ExecFileOptions(syntax.LegacyFileOptions(), thread, filename, src, predeclared)
	return err
}

// replEval evaluates a single Starlark expression/statement against the
// predeclared globals, mutating them in place (REPL semantics — assignments
// persist across calls). Extracted for testability.
func replEval(thread *starlark.Thread, globals starlark.StringDict, expr string) error {
	opts := syntax.LegacyFileOptions()
	f, err := opts.ParseCompoundStmt("<repl>", func() ([]byte, error) {
		return []byte(expr + "\n"), nil
	})
	if err != nil {
		return err
	}
	return starlark.ExecREPLChunk(f, thread, globals)
}

// okBuiltin / failBuiltin mirror the result helpers from the script runtime so
// `return ok(data)` / `return fail(msg)` behave identically in the console.
func okBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var data starlark.Value = starlark.None
	if err := starlark.UnpackArgs("ok", args, kwargs, "data?", &data); err != nil {
		return nil, err
	}
	return starlark.Tuple{starlark.True, data}, nil
}

func failBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var msg string
	if err := starlark.UnpackArgs("fail", args, kwargs, "msg", &msg); err != nil {
		return nil, err
	}
	return starlark.Tuple{starlark.False, starlark.String(msg)}, nil
}
