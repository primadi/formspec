// Command forma is the developer CLI — one binary, subcommand per verb.
// See docs/cli-tools/01-forma-cli.md for the full verb reference.
//
// SPA frontend (web/dist/) is embedded into the binary so `forma dev` serves
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
		runGenerate(os.Args[2:])
	case "dev":
		runDev(os.Args[2:])
	case "diff", "delete", "get", "describe", "validate", "new", "repl",
		"migrate", "seed", "backup", "restore", "archive", "saga", "module", "sign", "script",
		"freeze", "rollback", "lock", "workspace":
		fmt.Fprintf(os.Stderr, "forma %s: not implemented yet — see docs/cli-tools/01-forma-cli.md\n", os.Args[1])
		os.Exit(1)
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: forma <command> [flags]\n\n")
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  apply       Register YAML manifests to the Control Plane\n")
	fmt.Fprintf(os.Stderr, "  generate    Derive a typed TypeScript client from entity manifests\n")
	fmt.Fprintf(os.Stderr, "  dev         Development server (API + SPA built-in)\n")
	fmt.Fprintf(os.Stderr, "\nNot yet implemented (see docs/cli-tools/01-forma-cli.md):\n")
	fmt.Fprintf(os.Stderr, "  diff, delete, get, describe, validate, new, repl,\n")
	fmt.Fprintf(os.Stderr, "  migrate, seed, backup, restore, archive, saga, module, sign, script,\n")
	fmt.Fprintf(os.Stderr, "  freeze, rollback, lock, workspace\n")
}
