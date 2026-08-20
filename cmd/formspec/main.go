// Command formspec is the developer CLI — one binary, subcommand per verb.
// See docs/cli-tools/01-formspec-cli.md for the full verb reference.
//
// SPA frontend (renderers/react-shadcn/dist/) is embedded into the binary so `formspec dev` serves
// both API and UI from a single process — no separate frontend server needed.
package main

import (
	"embed"
	"fmt"
	"os"
)

//go:embed dist/favicon.svg dist/icons.svg dist/index.html dist/assets/*
var spaFS embed.FS

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "apply":
		runApply(os.Args[2:])
	case "generate":
		if len(os.Args) > 2 {
			switch os.Args[2] {
			case "node-app":
				runGenerateNodeApp(os.Args[3:])
			case "go-app":
				runGenerateGoApp(os.Args[3:])
			case "rust-app":
				runGenerateRustApp(os.Args[3:])
			case "php-app":
				runGeneratePHPApp(os.Args[3:])
			case "python-app":
				runGeneratePythonApp(os.Args[3:])
			case "ruby-app":
				runGenerateRubyApp(os.Args[3:])
			case "java-app":
				runGenerateJavaApp(os.Args[3:])
			case "dotnet-app":
				runGenerateDotNetApp(os.Args[3:])
			case "auth":
				runGenerateAuth(os.Args[3:])
			default:
				runGenerate(os.Args[2:])
			}
		} else {
			runGenerate(os.Args[2:])
		}
	case "new":
		runNew(os.Args[2:])
	case "dev":
		runDev(os.Args[2:])
	case "validate":
		runValidate(os.Args[2:])
	case "check":
		runCheck(os.Args[2:])
	case "get":
		runGet(os.Args[2:])
	case "describe":
		runDescribe(os.Args[2:])
	case "delete":
		runDelete(os.Args[2:])
	case "migrate":
		runMigrate(os.Args[2:])
	case "repl":
		runRepl(os.Args[2:])
	case "seed":
		runSeed(os.Args[2:])
	case "diff":
		runDiff(os.Args[2:])
	case "backup":
		runBackup(os.Args[2:])
	case "restore":
		runRestore(os.Args[2:])
	case "logs":
		runLogs(os.Args[2:])
	case "archive":
		runArchive(os.Args[2:])
	case "init":
		runInit(os.Args[2:])
	case "schema":
		runSchema(os.Args[2:])
	case "saga", "module", "sign", "script",
		"freeze", "rollback", "lock", "workspace":
		fmt.Fprintf(os.Stderr, "formspec %s: not implemented yet — see docs/cli-tools/01-formspec-cli.md\n", os.Args[1])
		os.Exit(1)
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: formspec <command> [flags]\n\n")
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  apply               Register YAML manifests to the Control Plane\n")
	fmt.Fprintf(os.Stderr, "  validate            Dry-run validate a spec tree (engine + JSON Schema)\n")
	fmt.Fprintf(os.Stderr, "  check               Cross-file static analysis (field/expr/uses references)\n")
	fmt.Fprintf(os.Stderr, "  get                 Inspect a resource (summary/JSON)\n")
	fmt.Fprintf(os.Stderr, "  describe            Inspect a resource in detail (fields/actions/state machine)\n")
	fmt.Fprintf(os.Stderr, "  delete              Remove a resource from the spec tree (--confirm)\n")
	fmt.Fprintf(os.Stderr, "  migrate             Structural migration: plan (show DDL) or apply\n")
	fmt.Fprintf(os.Stderr, "  repl                Interactive Starlark console with ctx.* access\n")
	fmt.Fprintf(os.Stderr, "  seed                Run YAML seeders for dev/testing data\n")
	fmt.Fprintf(os.Stderr, "  diff                Compare local spec against deployed state (dry-run)\n")
	fmt.Fprintf(os.Stderr, "  backup              Create/inspect a data backup archive\n")
	fmt.Fprintf(os.Stderr, "  restore             Restore data from a backup archive\n")
	fmt.Fprintf(os.Stderr, "  logs                Tail structured logs (event log)\n")
	fmt.Fprintf(os.Stderr, "  archive             Archive transactions (run/view/restore-batch)\n")
	fmt.Fprintf(os.Stderr, "  generate            Derive a typed TypeScript client from entity manifests\n")
	fmt.Fprintf(os.Stderr, "  generate node-app   Scaffold a TypeScript sidecar app\n")
	fmt.Fprintf(os.Stderr, "  generate go-app     Scaffold a Go sidecar app\n")
	fmt.Fprintf(os.Stderr, "  generate rust-app   Scaffold a Rust sidecar app\n")
	fmt.Fprintf(os.Stderr, "  generate php-app    Scaffold a PHP sidecar app\n")
	fmt.Fprintf(os.Stderr, "  generate python-app Scaffold a Python sidecar app\n")
	fmt.Fprintf(os.Stderr, "  generate ruby-app   Scaffold a Ruby sidecar app\n")
	fmt.Fprintf(os.Stderr, "  generate java-app   Scaffold a Java sidecar app\n")
	fmt.Fprintf(os.Stderr, "  generate dotnet-app Scaffold a .NET sidecar app\n")
	fmt.Fprintf(os.Stderr, "  generate auth      Scaffold a customizable auth module (external/auth)\n")
	fmt.Fprintf(os.Stderr, "  new app             Alias for `generate node-app`\n")
	fmt.Fprintf(os.Stderr, "  new module          Scaffold a Module manifest\n")
	fmt.Fprintf(os.Stderr, "  new entity          Scaffold an Entity manifest + basic fields\n")
	fmt.Fprintf(os.Stderr, "  dev                 Development server (API + SPA built-in)\n")
	fmt.Fprintf(os.Stderr, "  init                Scaffold a new FormSpec project with standard layout\n")
	fmt.Fprintf(os.Stderr, "  schema              Manage cached JSON Schema versions (fetch, update, list, clear)\n")
	fmt.Fprintf(os.Stderr, "\nNot yet implemented (see docs/cli-tools/01-formspec-cli.md):\n")
	fmt.Fprintf(os.Stderr, "  saga, module, sign, script,\n")
	fmt.Fprintf(os.Stderr, "  freeze, rollback, lock, workspace\n")
}
