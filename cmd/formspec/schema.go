// Command formspec schema — manage the locally cached JSON Schema versions.
//
// Schemas are fetched from the registry (default https://schemas.formspec.dev,
// overridable via FORMSPEC_SCHEMA_REGISTRY or formspec-app.yaml schema-registry:)
// and cached under os.UserCacheDir()/formspec/schemas/<version>. `formspec
// validate` reuses the same cache — a new spec version never requires a CLI
// reinstall. (`formspec init` no longer fetches schemas; it points
// .vscode/settings.json straight at the registry URL.)
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
	"path/filepath"
	"strings"

	"github.com/primadi/formspec/internal/schemaregistry"
)

// copySchemas copies formspec.schema.json + kinds/*.schema.json from srcDir
// into destDir, preserving the registry layout (used by `formspec schema
// fetch --out ./schemas`). `formspec init` does NOT use this anymore — it
// wires yaml.schemas straight to the registry URL instead.
func copySchemas(srcDir, destDir string) error {
	files := []string{"formspec.schema.json"}
	kindEntries, err := os.ReadDir(filepath.Join(srcDir, "kinds"))
	if err != nil {
		return fmt.Errorf("read cached kinds: %w", err)
	}
	for _, e := range kindEntries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".schema.json") {
			files = append(files, "kinds/"+e.Name())
		}
	}
	for _, rel := range files {
		src := filepath.Join(srcDir, rel)
		dst := filepath.Join(destDir, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(dst), err)
		}
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("read %s: %w", rel, err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", rel, err)
		}
		fmt.Fprintf(os.Stderr, "  ✓ schemas/%s\n", rel)
	}
	return nil
}

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
