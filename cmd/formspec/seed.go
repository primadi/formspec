// Command `formspec seed` — run YAML seeders for dev/testing data
// (docs/cli-tools/02-formspec-cli.md §6). The `formspec/seed` official module
// does not exist yet, so this verb defines a minimal declarative seed format
// (kind: Seed) and inserts records through the same EntityStore the engine
// uses, so natural-key generation, field defaults, and validation all apply.
//
//	formspec seed [--spec <path>] [--dsn <dsn>] [--module <module>]
//
// Seed manifest format:
//
//	apiVersion: formspec.dev/v1
//	kind: Seed
//	metadata:
//	  name: demo-data
//	  module: billing
//	spec:
//	  entities:
//	    - entity: customer
//	      records:
//	        - { code: C-001, name: "PT Maju" }
//
// Idempotent: a record whose natural key already exists is skipped with a
// warning, not an error.
package main

import (
	"context"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/manifest"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// SeedSpec is the spec body of a kind: Seed manifest.
type SeedSpec struct {
	Entities []SeedEntity `yaml:"entities"`
}

// SeedEntity declares records for one entity.
type SeedEntity struct {
	Entity  string           `yaml:"entity"`
	Records []map[string]any `yaml:"records"`
}

func runSeed(args []string) {
	specPath := "spec"
	dsn := "sqlite:.formspec/data.db"
	moduleFilter := ""
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
		case "--module", "-module":
			if i+1 < len(args) {
				moduleFilter = args[i+1]
				i++
			}
		case "--help", "-h":
			fmt.Fprintf(os.Stderr, "Usage: formspec seed [--spec <path>] [--dsn <dsn>] [--module <module>]\n")
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "formspec seed: unknown flag %q\n", args[i])
			os.Exit(2)
		}
	}

	loader := manifest.NewLoader(specPath)
	res, err := loader.LoadAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: load manifests: %v\n", err)
		os.Exit(1)
	}

	database, err := db.Open(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: open database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	driver := db.DriverSQLite
	if database.DriverName() == "postgres" {
		driver = db.DriverPostgres
	}

	reg := entity.NewRegistry(database, driver, specPath)
	for _, loadErr := range reg.LoadEntities() {
		fmt.Fprintf(os.Stderr, "formspec seed: load warning: %v\n", loadErr)
	}
	if _, err := reg.SyncSchema(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "Error: sync schema: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	inserted, skipped, failed := seedAll(ctx, res, reg, moduleFilter)

	fmt.Printf("Seed complete: %d inserted, %d skipped, %d failed.\n", inserted, skipped, failed)
	if failed > 0 {
		os.Exit(1)
	}
}

// seedAll runs every kind: Seed manifest against the registry, returning
// insert/skip/fail counts. Extracted for testability.
func seedAll(ctx context.Context, res *manifest.LoadResult, reg *entity.Registry, moduleFilter string) (inserted, skipped, failed int) {
	for _, m := range res.Manifests {
		if m.Kind != "Seed" {
			continue
		}
		if moduleFilter != "" && m.Metadata.Module != moduleFilter {
			continue
		}
		var seed SeedSpec
		if err := reparseSpec(m.Spec, &seed); err != nil {
			fmt.Fprintf(os.Stderr, "formspec seed: %s: invalid seed spec: %v\n", m.Source, err)
			failed++
			continue
		}
		for _, se := range seed.Entities {
			store, err := reg.GetEntityStore(m.Metadata.Module, se.Entity)
			if err != nil {
				fmt.Fprintf(os.Stderr, "formspec seed: %s: entity %s.%s: %v\n", m.Source, m.Metadata.Module, se.Entity, err)
				failed++
				continue
			}
			info, ok := reg.GetEntity(m.Metadata.Module, se.Entity)
			if !ok || info.EntitySpec == nil {
				fmt.Fprintf(os.Stderr, "formspec seed: %s: entity %s.%s not found\n", m.Source, m.Metadata.Module, se.Entity)
				failed++
				continue
			}
			nkField := info.EntitySpec.NaturalKeyField
			for _, rec := range se.Records {
				if nkField != "" {
					if exists, err := naturalKeyExists(ctx, store, "demo", nkField, rec[nkField]); err == nil && exists {
						fmt.Fprintf(os.Stderr, "formspec seed: skip %s.%s %s=%v (already exists)\n",
							m.Metadata.Module, se.Entity, nkField, rec[nkField])
						skipped++
						continue
					}
				}
				if _, err := store.Insert(ctx, db.InsertParams{
					WorkspaceID: "demo",
					CreatedBy:   "seed",
					Data:        rec,
				}); err != nil {
					fmt.Fprintf(os.Stderr, "formspec seed: %s.%s insert %v: %v\n", m.Metadata.Module, se.Entity, rec, err)
					failed++
					continue
				}
				inserted++
			}
		}
	}
	return inserted, skipped, failed
}

// reparseSpec re-marshals the raw spec (map[string]any) and unmarshals it into
// the typed SeedSpec, so unknown fields are ignored gracefully.
func reparseSpec(raw any, out *SeedSpec) error {
	b, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(b, out)
}

// naturalKeyExists reports whether a record with the given natural-key value
// already exists for the entity.
func naturalKeyExists(ctx context.Context, store *db.EntityStore, workspaceID, field string, value any) (bool, error) {
	if value == nil {
		return false, nil
	}
	res, err := store.List(ctx, db.ListParams{
		WorkspaceID: workspaceID,
		Page:        1,
		PerPage:     1,
		Filters:     map[string]db.FilterOp{field: {Op: "eq", Value: value}},
	})
	if err != nil {
		return false, err
	}
	return res.Total > 0, nil
}
