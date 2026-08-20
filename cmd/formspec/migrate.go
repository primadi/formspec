// Command `formspec migrate plan|apply` — trigger/inspect the automatic
// structural migration from Entity manifests (docs/cli-tools/02-formspec-cli.md §3).
// Migration itself is fully automatic from the Entity diff (not hand-written);
// this verb just drives the existing MigrationRunner
// (renderers/jsonb-persist/migrate.go).
//
//	formspec migrate plan     # show the DDL that would run, without executing
//	formspec migrate apply    # execute (normally automatic via formspec apply)
//
// Usage:
//
//	formspec migrate <plan|apply> [--spec <path>] [--dsn <dsn>]
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.starlark.net/starlark"
	"gopkg.in/yaml.v3"

	"github.com/primadi/formspec/internal/manifest"
	fsstarlark "github.com/primadi/formspec/internal/starlark"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
	formspec "github.com/primadi/formspec/resource"
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
			fmt.Fprintf(os.Stderr, "Usage: formspec migrate <plan|apply> [--spec <path>] [--dsn <dsn>]\n")
			os.Exit(0)
		default:
			positional = append(positional, args[i])
		}
	}
	if len(positional) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: formspec migrate <plan|apply|data> [--spec <path>] [--dsn <dsn>]\n")
		os.Exit(2)
	}
	action := positional[0]
	if action == "data" {
		runMigrateData(specPath, dsn, positional[1:])
		return
	}
	if action != "plan" && action != "apply" {
		fmt.Fprintf(os.Stderr, "formspec migrate: unknown action %q (want plan|apply)\n", action)
		os.Exit(2)
	}

	entities := loadEntityMigrations(specPath)
	customMigrations := loadCustomMigrations(specPath)

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
	runner := db.NewMigrationRunner(database, driver)

	ctx := context.Background()

	// PlanMigrations reads formspec_schema_migrations, so system tables must
	// exist first (they are created idempotently).
	if err := runner.EnsureSystemTables(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: ensure system tables: %v\n", err)
		os.Exit(1)
	}

	if action == "plan" {
		results, err := runner.PlanMigrations(ctx, entities)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: plan migrations: %v\n", err)
			os.Exit(1)
		}
		if len(results) == 0 && len(customMigrations) == 0 {
			fmt.Println("No pending migrations.")
			return
		}
		fmt.Printf("%d pending migration(s):\n\n", len(results))
		for _, r := range results {
			fmt.Printf("── %s ──\n%s\n\n", r.Description, r.DDL)
		}
		for _, cm := range customMigrations {
			fmt.Printf("── custom: %s ──\n%s\n\n", cm.Name, cm.DDL)
		}
		return
	}

	// apply structural migrations
	applied, err := runner.ApplyMigrations(ctx, entities)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: apply migrations: %v\n", err)
		os.Exit(1)
	}

	// apply custom DDL migrations (kind: Migration)
	appliedCustom, err := applyCustomMigrations(ctx, database, customMigrations)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: apply custom migrations: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Applied %d structural + %d custom migration(s).\n", applied, appliedCustom)
}

// customMigration is a loaded kind: Migration manifest.
type customMigration struct {
	Name string
	DDL  string
}

// loadCustomMigrations loads all kind: Migration manifests from the spec tree.
func loadCustomMigrations(specPath string) []customMigration {
	loader := manifest.NewLoader(specPath)
	res, err := loader.LoadAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: load manifests: %v\n", err)
		os.Exit(1)
	}
	var out []customMigration
	for _, m := range res.Manifests {
		if m.Kind != "Migration" {
			continue
		}
		sm, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		var ms spec.MigrationSpec
		if err := reparseAny(sm, &ms); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skip %s: %v\n", m.Source, err)
			continue
		}
		if ms.DDL == "" {
			fmt.Fprintf(os.Stderr, "Warning: skip %s: empty ddl\n", m.Source)
			continue
		}
		out = append(out, customMigration{Name: m.Metadata.Name, DDL: ms.DDL})
	}
	return out
}

// applyCustomMigrations validates each DDL is DDL-only (rejects DML) and
// executes it. Returns the number applied.
func applyCustomMigrations(ctx context.Context, database db.DB, migrations []customMigration) (int, error) {
	applied := 0
	for _, cm := range migrations {
		if err := validateDDLOnly(cm.DDL); err != nil {
			return applied, fmt.Errorf("%s: %w", cm.Name, err)
		}
		if _, err := database.ExecContext(ctx, cm.DDL); err != nil {
			return applied, fmt.Errorf("%s: %w", cm.Name, err)
		}
		applied++
	}
	return applied, nil
}

// validateDDLOnly rejects DML statements (INSERT/UPDATE/DELETE/SELECT) in a
// kind: Migration DDL (01-core-basic.md §4: DML rejected at runtime).
func validateDDLOnly(ddl string) error {
	upper := strings.ToUpper(strings.TrimSpace(ddl))
	for _, prefix := range []string{"INSERT", "UPDATE", "DELETE", "SELECT", "DROP DATABASE", "DROP TABLE"} {
		if strings.HasPrefix(upper, prefix) {
			return fmt.Errorf("DML rejected in kind: Migration (got %q) — only DDL allowed", prefix)
		}
	}
	return nil
}

// reparseAny re-marshals a raw spec (map[string]any) and unmarshals it into
// the given typed struct, ignoring unknown fields gracefully.
func reparseAny(raw any, out any) error {
	b, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(b, out)
}

// dataMigration is a loaded kind: DataMigration manifest (4.2.5).
type dataMigration struct {
	Name     string
	Module   string
	Version  int
	Run      string
	Rollback string
}

// runMigrateData runs or rolls back a versioned data migration (4.2.5).
//
//	formspec migrate data <name> run|rollback [--spec <path>] [--dsn <dsn>]
func runMigrateData(specPath, dsn string, args []string) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: formspec migrate data <name> run|rollback [--spec <path>] [--dsn <dsn>]\n")
		os.Exit(2)
	}
	name := args[0]
	op := args[1]
	if op != "run" && op != "rollback" {
		fmt.Fprintf(os.Stderr, "formspec migrate data: unknown op %q (want run|rollback)\n", op)
		os.Exit(2)
	}

	// Load DataMigration manifests.
	loader := manifest.NewLoader(specPath)
	res, err := loader.LoadAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: load manifests: %v\n", err)
		os.Exit(1)
	}
	var target *dataMigration
	for _, m := range res.Manifests {
		if m.Kind != "DataMigration" || m.Metadata.Name != name {
			continue
		}
		sm, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		var dms spec.DataMigrationSpec
		if err := reparseAny(sm, &dms); err != nil {
			fmt.Fprintf(os.Stderr, "Error: parse %s: %v\n", m.Source, err)
			os.Exit(1)
		}
		target = &dataMigration{Name: name, Module: dms.Module, Version: dms.Version, Run: dms.Run, Rollback: dms.Rollback}
		break
	}
	if target == nil {
		fmt.Fprintf(os.Stderr, "Error: data migration %q not found\n", name)
		os.Exit(1)
	}

	scriptRef := target.Run
	if op == "rollback" {
		if target.Rollback == "" {
			fmt.Fprintf(os.Stderr, "Error: data migration %q has no rollback script\n", name)
			os.Exit(1)
		}
		scriptRef = target.Rollback
	}

	// Resolve the script path relative to the spec tree.
	scriptPath := resolveDataMigrationScript(specPath, target.Module, scriptRef)
	if scriptPath == "" {
		fmt.Fprintf(os.Stderr, "Error: script %q not found for data migration %q\n", scriptRef, name)
		os.Exit(1)
	}

	// Execute the script via the Starlark engine.
	app, err := formspec.New(formspec.Config{SpecPath: specPath, DSN: dsn})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer app.Close(context.Background())

	ctxObj := fsstarlark.NewCtxAPI("demo", "", "migrate", "", nil)
	ctxObj.SetDatastoreResolver(formspec.NewCtxPrimitiveResolver(app.Database(), formspec.StateDirFromDSN(dsn)))
	ctxObj.Config = fsstarlark.NewConfigAPI(map[string]any{})

	predeclared := starlark.StringDict{
		"ctx":      ctxObj,
		"resource": fsstarlark.NewResourceAPI("", "", "", 0, map[string]any{}),
		"ok":       starlark.NewBuiltin("ok", okBuiltin),
		"fail":     starlark.NewBuiltin("fail", failBuiltin),
	}
	thread := &starlark.Thread{Name: "migrate-data"}
	thread.SetLocal("context", context.Background())

	if err := replEval(thread, predeclared, scriptPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s %s: %v\n", name, op, err)
		os.Exit(1)
	}
	fmt.Printf("Data migration %s %s complete (v%d).\n", name, op, target.Version)
}

// resolveDataMigrationScript resolves a script ref to a path under the spec
// tree, trying common locations.
func resolveDataMigrationScript(specPath, module, ref string) string {
	candidates := []string{
		filepath.Join(specPath, "migrations", ref+".star"),
		filepath.Join(specPath, "modules", module, "migrations", ref+".star"),
		filepath.Join(specPath, "modules", module, "scripts", ref+".star"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

// loadEntityMigrations loads all Entity/Document manifests and converts them
// to the EntityMigration list the MigrationRunner consumes.
func loadEntityMigrations(specPath string) []db.EntityMigration {
	loader := manifest.NewLoader(specPath)
	res, err := loader.LoadAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: load manifests: %v\n", err)
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
		es, err := manifest.RawSpecToEntitySpec(sm)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skip %s: %v\n", m.Source, err)
			continue
		}
		entities = append(entities, db.EntityMigration{
			Metadata: spec.Metadata{
				Name:        m.Metadata.Name,
				Module:      m.Metadata.Module,
				Description: m.Metadata.Description,
			},
			EntitySpec: *es,
		})
	}
	return entities
}
