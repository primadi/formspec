// Command `formspec repl` — interactive Starlark console with full ctx.*
// access (docs/cli-tools/02-formspec-cli.md §4). A first-class feature, not a
// one-off debug tool — also the surface for AI Agent Skill debugging.
//
//	formspec repl [--spec <path>] [--dsn <dsn>] [--environment <env>]
//	formspec repl -e 'ctx.db().query("SELECT 1")'   # one-shot (scriptable)
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
	specPath := "spec"
	dsn := "sqlite:.formspec/data.db"
	environment := ""
	expr := ""
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
		case "--environment", "-environment":
			if i+1 < len(args) {
				environment = args[i+1]
				i++
			}
		case "-e", "--eval":
			if i+1 < len(args) {
				expr = args[i+1]
				i++
			}
		case "--help", "-h":
			fmt.Fprintf(os.Stderr, "Usage: formspec repl [--spec <path>] [--dsn <dsn>] [--environment <env>] [-e <expr>]\n")
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "formspec repl: unknown flag %q\n", args[i])
			os.Exit(2)
		}
	}

	app, err := formspec.New(formspec.Config{SpecPath: specPath, DSN: dsn})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer app.Close(context.Background())

	// Build the ctx object with the same live datastore resolver the action
	// dispatcher wires into scripts, so ctx.db()/ctx.cache()/... resolve to
	// real backends in the console.
	ctxObj := fsstarlark.NewCtxAPI("demo", "", "repl", "", nil)
	ctxObj.SetDatastoreResolver(formspec.NewCtxPrimitiveResolver(app.Database(), formspec.StateDirFromDSN(dsn)))
	ctxObj.Config = fsstarlark.NewConfigAPI(map[string]any{})
	if environment != "" {
		// Control Plane environment policy (platform/04 §7) is deferred; the
		// flag is accepted for forward-compat but has no effect yet.
		fmt.Fprintf(os.Stderr, "formspec repl: note: --environment %q accepted; environment policy is deferred (Control Plane)\n", environment)
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
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	repl.REPLOptions(syntax.LegacyFileOptions(), thread, predeclared)
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
