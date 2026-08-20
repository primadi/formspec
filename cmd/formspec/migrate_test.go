package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

func newManifestLoader(path string) *manifest.Loader { return manifest.NewLoader(path) }

func TestResolveDataMigrationScript(t *testing.T) {
	dir := t.TempDir()
	// Create a script under modules/alpha/migrations/.
	path := filepath.Join(dir, "modules", "alpha", "migrations", "backfill.star")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("def run():\n  pass\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := resolveDataMigrationScript(dir, "alpha", "backfill")
	if got == "" {
		t.Fatal("expected script to resolve")
	}
	if got != path {
		t.Fatalf("expected %q, got %q", path, got)
	}

	// Unknown script → empty.
	if got := resolveDataMigrationScript(dir, "alpha", "nope"); got != "" {
		t.Fatalf("expected empty for unknown script, got %q", got)
	}
}

func TestLoadDataMigrationManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "migration.yaml")
	content := `apiVersion: formspec.dev/v1
kind: DataMigration
metadata: { name: backfill-2026, module: alpha }
spec:
  version: 1
  run: backfill
  rollback: undo-backfill
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := newManifestLoader(dir)
	res, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	found := false
	for _, m := range res.Manifests {
		if m.Kind == "DataMigration" && m.Metadata.Name == "backfill-2026" {
			found = true
			sm, ok := m.Spec.(map[string]any)
			if !ok {
				t.Fatal("spec not a map")
			}
			var dms spec.DataMigrationSpec
			if err := reparseAny(sm, &dms); err != nil {
				t.Fatalf("parse: %v", err)
			}
			if dms.Version != 1 || dms.Run != "backfill" || dms.Rollback != "undo-backfill" {
				t.Fatalf("unexpected spec: %+v", dms)
			}
		}
	}
	if !found {
		t.Fatal("DataMigration manifest not found")
	}
}

func writeMigrateSpec(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, "modules", "alpha", "master", "item.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := `apiVersion: formspec.dev/v1
kind: Entity
metadata: { name: item, module: alpha }
spec:
  version: v1
  characteristic: master
  fields:
    - { name: code, type: string, rules: [required] }
    - { name: price, type: money }
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadEntityMigrations(t *testing.T) {
	dir := t.TempDir()
	writeMigrateSpec(t, dir)

	entities := loadEntityMigrations(dir)
	if len(entities) != 1 {
		t.Fatalf("expected 1 entity migration, got %d", len(entities))
	}
	if entities[0].Metadata.Name != "item" || entities[0].Metadata.Module != "alpha" {
		t.Fatalf("unexpected entity: %+v", entities[0].Metadata)
	}
	if len(entities[0].EntitySpec.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(entities[0].EntitySpec.Fields))
	}
}

func TestMigratePlanAndApply(t *testing.T) {
	dir := t.TempDir()
	writeMigrateSpec(t, dir)

	database, err := db.Open("sqlite::memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	runner := db.NewMigrationRunner(database, db.DriverSQLite)
	ctx := context.Background()
	if err := runner.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("ensure system tables: %v", err)
	}

	entities := loadEntityMigrations(dir)

	// Plan should show 1 pending migration.
	results, err := runner.PlanMigrations(ctx, entities)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 pending migration, got %d", len(results))
	}

	// Apply should apply 1.
	applied, err := runner.ApplyMigrations(ctx, entities)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if applied != 1 {
		t.Fatalf("expected 1 applied, got %d", applied)
	}

	// Plan again should show 0 pending (idempotent).
	results, err = runner.PlanMigrations(ctx, entities)
	if err != nil {
		t.Fatalf("plan after apply: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 pending after apply, got %d", len(results))
	}
}

func TestValidateDDLOnly(t *testing.T) {
	// DDL is allowed.
	if err := validateDDLOnly("CREATE INDEX idx_x ON t(c)"); err != nil {
		t.Fatalf("expected DDL allowed, got %v", err)
	}
	// DML is rejected.
	for _, dml := range []string{"INSERT INTO t VALUES (1)", "UPDATE t SET c=1", "DELETE FROM t", "SELECT * FROM t"} {
		if err := validateDDLOnly(dml); err == nil {
			t.Fatalf("expected %q rejected as DML", dml)
		}
	}
}

func TestLoadAndApplyCustomMigrations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "migration.yaml")
	content := `apiVersion: formspec.dev/v1
kind: Migration
metadata: { name: add-index }
spec:
  ddl: "CREATE INDEX idx_customer_code ON alpha_customers(code)"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	migrations := loadCustomMigrations(dir)
	if len(migrations) != 1 {
		t.Fatalf("expected 1 custom migration, got %d", len(migrations))
	}
	if migrations[0].Name != "add-index" {
		t.Fatalf("unexpected name: %s", migrations[0].Name)
	}

	database, err := db.Open("sqlite::memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	// Create the target table first so the index DDL succeeds.
	if _, err := database.ExecContext(context.Background(), "CREATE TABLE alpha_customers (id text, code text)"); err != nil {
		t.Fatalf("create table: %v", err)
	}

	applied, err := applyCustomMigrations(context.Background(), database, migrations)
	if err != nil {
		t.Fatalf("apply custom: %v", err)
	}
	if applied != 1 {
		t.Fatalf("expected 1 applied, got %d", applied)
	}
}

func TestApplyCustomMigrations_RejectsDML(t *testing.T) {
	database, err := db.Open("sqlite::memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	_, err = applyCustomMigrations(context.Background(), database, []customMigration{
		{Name: "bad", DDL: "DELETE FROM t"},
	})
	if err == nil {
		t.Fatal("expected DML to be rejected")
	}
}
