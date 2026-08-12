// Command formspec-gen-kind-docs reads pkg/spec Go types and generates (or
// updates in place) human-readable per-kind reference docs under docs/kind/.
//
// Usage:
//
//	formspec-gen-kind-docs [--out docs/kind]
//
// The tool writes one Markdown file per kind at out/<group>/<Kind>.md:
//
//	docs/kind/curation/{App,Module}.md
//	docs/kind/data/{Entity,Service,...}.md
//	docs/kind/ui/{Page,Form,Table,...}.md
//	docs/kind/infra/{Renderer,PersistBackend,...}.md
//
// Attribute tables are generated from the same pkg/spec source as
// schemas/kinds/*.schema.json (zero drift). Narrative sections between
// <!-- generated:... --> markers are preserved on regenerate; everything
// outside the markers is author-maintained and never overwritten.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/primadi/formspec/internal/genjsonschema"
	"github.com/primadi/formspec/internal/genkinddocs"
)

func main() {
	outDir := flag.String("out", "docs/kind", "output directory for kind reference docs")
	pkgPath := flag.String("pkg", "github.com/primadi/formspec/pkg/spec", "Go package path to read types from")
	flag.Parse()

	if err := run(*outDir, *pkgPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(outDir, pkgPath string) error {
	fmt.Printf("🔍 Loading package: %s\n", pkgPath)

	converter := genjsonschema.New(pkgPath)
	collect, err := converter.Collect()
	if err != nil {
		return fmt.Errorf("collect types: %w", err)
	}

	fmt.Printf("   Found %d structs, %d enums\n", len(collect.Structs), len(collect.Enums))

	fmt.Println("⚡ Generating kind reference docs...")
	written, err := genkinddocs.Generate(collect, outDir)
	if err != nil {
		return err
	}

	fmt.Printf("\n📊 Summary: %d kind docs written/updated under %s\n", written, outDir)
	fmt.Println("   Narrative sections are preserved; only generated regions were refreshed.")
	return nil
}
