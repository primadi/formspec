// Command formspec schema — manage the locally cached JSON Schema versions.
//
// Schemas are fetched from the registry (default https://schemas.formspec.dev,
// overridable via FORMSPEC_SCHEMA_REGISTRY or formspec-app.yaml schema-registry:)
// and cached under os.UserCacheDir()/formspec/schemas/<version>. `formspec
// validate` and `formspec init` reuse the same cache — a new spec version never
// requires a CLI reinstall.
//
// Usage:
//
//	formspec schema fetch [version] [--out <dir>]   fetch/cache a version
//	formspec schema update [version] [--out <dir>]  force re-fetch
//	formspec schema list                            list cached versions
//	formspec schema clear                           remove the whole cache
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/primadi/formspec/internal/schemaregistry"
)

func runSchema(args []string) {
	if len(args) == 0 {
		schemaUsage()
		os.Exit(1)
	}
	sub := args[0]
	rest := args[1:]
	reg := schemaregistry.New(schemaRegistryBaseURL())

	switch sub {
	case "fetch", "update":
		force := sub == "update"
		fs := flag.NewFlagSet("formspec schema "+sub, flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		version := fs.String("version", "v1", "schema version to fetch (e.g. v1)")
		out := fs.String("out", "", "also copy the schemas into this dir (e.g. ./schemas)")
		if err := fs.Parse(rest); err != nil {
			os.Exit(2)
		}
		if fs.NArg() > 0 {
			*version = fs.Arg(0)
		}
		if err := reg.EnsureFull(*version, force); err != nil {
			fmt.Fprintf(os.Stderr, "formspec schema %s: %v\n", sub, err)
			os.Exit(1)
		}
		dir, err := reg.VersionDir(*version)
		if err != nil {
			fmt.Fprintf(os.Stderr, "formspec schema %s: %v\n", sub, err)
			os.Exit(1)
		}
		fmt.Printf("schema %s %s → %s\n", sub, *version, dir)
		if *out != "" {
			if err := copySchemas(dir, *out); err != nil {
				fmt.Fprintf(os.Stderr, "formspec schema %s: %v\n", sub, err)
				os.Exit(1)
			}
		}

	case "list":
		versions, err := reg.List()
		if err != nil {
			fmt.Fprintf(os.Stderr, "formspec schema list: %v\n", err)
			os.Exit(1)
		}
		if len(versions) == 0 {
			fmt.Println("no schema versions cached")
			return
		}
		for _, v := range versions {
			fmt.Println(v)
		}

	case "clear":
		if err := reg.Clear(); err != nil {
			fmt.Fprintf(os.Stderr, "formspec schema clear: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("schema cache cleared")

	default:
		schemaUsage()
		os.Exit(1)
	}
}

func schemaUsage() {
	fmt.Fprintf(os.Stderr, "Usage: formspec schema <fetch|update|list|clear> [flags]\n\n")
	fmt.Fprintf(os.Stderr, "Manage the locally cached JSON Schema versions.\n")
	fmt.Fprintf(os.Stderr, "  fetch [version]   fetch/cache a schema version (default v1)\n")
	fmt.Fprintf(os.Stderr, "    --out <dir>     also copy schemas into a dir (e.g. ./schemas)\n")
	fmt.Fprintf(os.Stderr, "  update [version]  force re-fetch a version from the registry\n")
	fmt.Fprintf(os.Stderr, "  list              list cached versions\n")
	fmt.Fprintf(os.Stderr, "  clear             remove the whole schema cache\n")
	fmt.Fprintf(os.Stderr, "\nRegistry: %s (override: FORMSPEC_SCHEMA_REGISTRY or schema-registry: in formspec-app.yaml)\n", schemaregistry.DefaultBaseURL)
}
