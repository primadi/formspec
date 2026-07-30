// Command forma-gen-schema reads pkg/spec Go types and generates
// JSON Schema (Draft-07) files for every Forma resource kind.
//
// Usage:
//
//	forma-gen-schema [--out schemas/]
//
// The tool produces:
//   - schemas/forma.schema.json       — root discriminator schema
//   - schemas/kinds/{Kind}.schema.json — per-kind spec schemas
//
// Register the root schema in VS Code settings:
//
//	"yaml.schemas": {
//	  "schemas/forma.schema.json": ["spec/**/*.yaml"]
//	}
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/primadi/forma/internal/genjsonschema"
)

func main() {
	outDir := flag.String("out", "schemas", "output directory for schema files")
	pkgPath := flag.String("pkg", "github.com/primadi/forma/pkg/spec", "Go package path to read types from")
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

	// Generate
	fmt.Println("⚡ Generating JSON Schema...")
	result := converter.Generate(collect)

	// Ensure output directories exist
	kindsDir := filepath.Join(outDir, "kinds")
	if err := os.MkdirAll(kindsDir, 0755); err != nil {
		return fmt.Errorf("create output dir %s: %w", kindsDir, err)
	}

	// Write root schema
	rootSchema := result.RootSchema
	rootData, err := json.MarshalIndent(rootSchema, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal root schema: %w", err)
	}

	rootPath := filepath.Join(outDir, "forma.schema.json")
	if err := os.WriteFile(rootPath, rootData, 0644); err != nil {
		return fmt.Errorf("write root schema: %w", err)
	}
	fmt.Printf("   ✅ Root schema: %s\n", rootPath)

	// Write per-kind schemas
	var kindNames []string
	for name := range result.KindSchemas {
		kindNames = append(kindNames, name)
	}
	sort.Strings(kindNames)

	for _, name := range kindNames {
		schema := result.KindSchemas[name]
		data, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "   ⚠️  Error marshaling %s schema: %v\n", name, err)
			continue
		}
		kindPath := filepath.Join(kindsDir, name+".schema.json")
		if err := os.WriteFile(kindPath, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "   ⚠️  Error writing %s schema: %v\n", name, err)
			continue
		}
		fmt.Printf("   ✅ Kind schema: %s\n", kindPath)
	}

	// Generate VS Code settings snippet
	fmt.Println("\n📋 VS Code settings untuk yaml.schemas:")
	fmt.Println("   (Tambahkan ke .vscode/settings.json)")
	fmt.Println()
	fmt.Println(`{
  "yaml.schemas": {
    "` + filepath.ToSlash(filepath.Join(outDir, "forma.schema.json")) + `": ["spec/**/*.yaml", "spec/**/*.yml"]
  }
}`)

	// Summary
	fmt.Println()
	fmt.Printf("📊 Summary:\n")
	fmt.Printf("   - %d kinds with schemas\n", len(result.KindSchemas))
	fmt.Printf("   - %d shared type definitions\n", len(result.SharedDefs))

	// List which kinds have schemas vs which are missing
	fmt.Println()
	var missingKinds []string
	for _, entry := range genjsonschema.KindMapping() {
		if _, ok := result.KindSchemas[entry.Kind]; !ok {
			missingKinds = append(missingKinds, entry.Kind)
		}
	}
	if len(missingKinds) > 0 {
		fmt.Printf("   ⚠️  Missing schemas for: %s\n", strings.Join(missingKinds, ", "))
	}

	return nil
}
