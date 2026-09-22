// Command `formspec migrate plan|apply` — the automatic structural migration.
//
// Migrations are never hand-written: the framework diffs the Entity manifests
// against what the database has applied and rates every difference
// (renderers/jsonb-persist/diff.go). This verb drives that engine and reports
// what it found.
//
//	formspec migrate plan     # show the differences, without executing
//	formspec migrate apply    # execute (normally automatic via formspec dev / apply)
//
// A change that destroys what the spec cannot recompute — a removed field, an
// unverifiable type change, a unique index blocked by duplicates — is refused
// unless the manifest declares it (`removed: true` / `accept_data_loss: true`
// with a `reason`). Dropping a table has no declaration: back up, drop it by
// hand, then remove the manifest.
//
// Usage:
//
//	formspec migrate <plan|apply> [--spec <path>] [--dsn <dsn>]
package main

import (
	"context"
	"fmt"
	"os"

	db "github.com/primadi/formspec/renderers/jsonb-persist"

	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
)

func runMigrate(args []string) {
	specPath := "spec"
	dsn := "sqlite:.formspec/data.db"
	var positional []string
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
		case "--help", "-h":
			_, _ = fmt.Fprintf(os.Stderr, "Usage: formspec migrate <plan|apply> [--spec <path>] [--dsn <dsn>]\n")
			os.Exit(0)
		default:
			positional = append(positional, args[i])
		}
	}

	// Anchor relative SQLite DSN ke lokasi spec (plan dsn-spec-anchored.md).
	dsn = resolveDSN(dsn, specPath)

	if len(positional) < 1 {
		_, _ = fmt.Fprintf(os.Stderr, "Usage: formspec migrate <plan|apply> [--spec <path>] [--dsn <dsn>]\n")
		os.Exit(2)
	}
	action := positional[0]
	if action != "plan" && action != "apply" {
		_, _ = fmt.Fprintf(os.Stderr, "formspec migrate: unknown action %q (want plan|apply)\n", action)
		os.Exit(2)
	}

	entities := loadEntityMigrations(specPath)

	database, err := db.Open(dsn)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: open database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	driver := db.DriverSQLite
	if database.DriverName() == "postgres" {
		driver = db.DriverPostgres
	}
	runner := db.NewMigrationRunner(database, driver)
	// Framework-owned entities (formspec.core) are registered by the server at
	// runtime, never by a user manifest. Without this, every `formspec migrate`
	// against a database the dev server had migrated refused with
	// `table_removed formspec_core_*` — a change no manifest can declare
	// (kafe TODO 3.10).
	runner.IgnoreModules(auth.CoreModule)
	ctx := context.Background()

	// PlanMigrations reads formspec_schema_migrations, so system tables must
	// exist first (they are created idempotently).
	if err := runner.EnsureSystemTables(ctx); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: ensure system tables: %v\n", err)
		os.Exit(1)
	}

	if action == "plan" {
		plans, err := runner.PlanSpecSet(ctx, entities)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: plan migrations: %v\n", err)
			os.Exit(1)
		}
		changes := collectChanges(plans)
		if len(changes) == 0 {
			fmt.Println("No pending migrations.")
			return
		}

		fmt.Printf("%d change(s) pending:\n\n", len(changes))
		for _, c := range changes {
			fmt.Printf("  %s\n", c)
		}
		fmt.Println()

		// A refused change is reported as a failure, not a note: the next
		// `apply` would stop on it, so pretending the plan is fine would only
		// move the surprise to deployment.
		if err := db.RefuseUndeclared(changes); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	res, err := runner.ApplySpecSetDetailed(ctx, entities)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if res.Applied == 0 && res.Forgotten == 0 {
		fmt.Println("No pending migrations.")
		return
	}

	// Say what happened, change by change. An applied change that destroys
	// something must be visible in the output, not just in the migration table.
	for _, c := range res.Changes {
		fmt.Printf("  %s\n", c)
	}
	fmt.Printf("Applied %d migration(s)", res.Applied)
	if res.Forgotten > 0 {
		fmt.Printf(", forgot %d stale snapshot(s)", res.Forgotten)
	}
	fmt.Println(".")
}

// collectChanges flattens the plans' changes in plan order.
func collectChanges(plans []db.MigrationPlan) []db.Change {
	var out []db.Change
	for _, p := range plans {
		out = append(out, p.Changes...)
	}
	return out
}

// loadEntityMigrations loads all Entity/Document manifests and converts them
// to the EntityMigration list the MigrationRunner consumes.
func loadEntityMigrations(specPath string) []db.EntityMigration {
	loader := manifest.NewLoader(specPath)
	res, err := loader.LoadAll()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: load manifests: %v\n", err)
		os.Exit(1)
	}

	var entities []db.EntityMigration
	for _, m := range res.Manifests {
		if m.Kind != "Entity" && m.Kind != "Document" {
			continue
		}
		sm, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		// EntitySpecFromRaw, not RawSpecToEntitySpec: the schema snapshot is
		// written from this spec, and it must describe the entity the way the
		// runtime does — including the fields the engine injects
		// (`is_active` from soft_deactivate). Converting without normalizing made
		// this path disagree with the dev server about the shape of the same
		// entity, so `formspec migrate` reported `field_removed is_active` on
		// every database the dev server had migrated (kafe TODO 3.10).
		es, err := manifest.EntitySpecFromRaw(sm)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: skip %s: %v\n", m.Source, err)
			continue
		}
		entities = append(entities, db.EntityMigration{
			Metadata: spec.Metadata{
				Name:        m.Metadata.Name,
				Module:      m.Metadata.Module,
				Description: m.Metadata.Description,
				Labels:      m.Metadata.Labels,
				Annotations: m.Metadata.Annotations,
			},
			EntitySpec: *es,
		})
	}
	return entities
}
