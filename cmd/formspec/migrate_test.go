package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

func newManifestLoader(path string) *manifest.Loader { return manifest.NewLoader(path) }

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
	if err := spec.ValidateDDLOnly("CREATE INDEX idx_x ON t(c)", "test"); err != nil {
		t.Fatalf("expected DDL allowed, got %v", err)
	}
	// Data statements are rejected: a manifest declares schema, and a repair
	// that touches rows is run once, outside the spec.
	for _, dml := range []string{"INSERT INTO t VALUES (1)", "UPDATE t SET c=1", "DELETE FROM t", "SELECT * FROM t", "TRUNCATE t"} {
		if err := spec.ValidateDDLOnly(dml, "test"); err == nil {
			t.Fatalf("expected %q rejected as a data statement", dml)
		}
	}
	// Dropping storage is rejected too — the one operation whose blast radius a
	// diff cannot show.
	for _, drop := range []string{"DROP TABLE t", "DROP DATABASE d", "DROP INDEX idx_x"} {
		if err := spec.ValidateDDLOnly(drop, "test"); err == nil {
			t.Fatalf("expected %q rejected as a drop", drop)
		}
	}
}

// writeMigrateSpecWithFields rewrites the entity manifest with the given field
// block, so a test can walk a manifest through a schema change.
func writeMigrateSpecWithFields(t *testing.T, dir, fields string) {
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
` + fields
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestMigrateRefusesUndeclaredRemoval is the end-to-end contract for a
// destructive change, driven entirely through YAML:
//
//  1. a field disappears from the manifest — refused, because the diff cannot
//     tell an intended removal from a typo;
//  2. the manifest declares it (`removed: true` + `reason`) — applied, and the
//     stored values go with it;
//  3. the tombstone is deleted — the next apply is clean, because there is
//     nothing left to remove.
func TestMigrateRefusesUndeclaredRemoval(t *testing.T) {
	dir := t.TempDir()
	writeMigrateSpecWithFields(t, dir, "    - { name: code, type: string, unique: true }\n    - { name: legacy_code, type: string }\n")

	database, err := db.Open("sqlite::memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	runner := db.NewMigrationRunner(database, db.DriverSQLite)
	ctx := context.Background()

	if _, err := runner.ApplySpecSet(ctx, loadEntityMigrations(dir)); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	// A row that still holds the field about to be removed.
	if _, err := database.ExecContext(ctx,
		`INSERT INTO alpha_items (id, tenant_id, version, created_at, updated_at, doc_status, data) `+
			`VALUES ('11111111-1111-7111-8111-111111111111', 'demo', 1, '2026-09-16', '2026-09-16', NULL, `+
			`'{"code": "A-1", "legacy_code": "OLD"}')`); err != nil {
		t.Fatalf("seed row: %v", err)
	}

	// 1. Removed without a declaration.
	writeMigrateSpecWithFields(t, dir, "    - { name: code, type: string, unique: true }\n")
	_, err = runner.ApplySpecSet(ctx, loadEntityMigrations(dir))
	if err == nil {
		t.Fatal("expected an undeclared field removal to be refused")
	}
	msg := err.Error()
	for _, want := range []string{"field_removed", "legacy_code", "removed: true"} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal %q does not mention %q", msg, want)
		}
	}

	// 2. Declared.
	writeMigrateSpecWithFields(t, dir,
		"    - { name: code, type: string, unique: true }\n"+
			"    - { name: legacy_code, type: string, removed: true, reason: \"digantikan code\" }\n")
	if _, err := runner.ApplySpecSet(ctx, loadEntityMigrations(dir)); err != nil {
		t.Fatalf("declared removal must apply: %v", err)
	}

	var leftover int
	if err := database.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM alpha_items WHERE json_extract(data, '$.legacy_code') IS NOT NULL").Scan(&leftover); err != nil {
		t.Fatalf("count leftovers: %v", err)
	}
	if leftover != 0 {
		t.Errorf("expected the stored values to be stripped, %d row(s) still hold them", leftover)
	}

	// 3. The tombstone is no longer needed.
	writeMigrateSpecWithFields(t, dir, "    - { name: code, type: string, unique: true }\n")
	if _, err := runner.ApplySpecSet(ctx, loadEntityMigrations(dir)); err != nil {
		t.Fatalf("apply after dropping the tombstone: %v", err)
	}
}

// TestMigrateRefusesDroppedEntity: a manifest that no longer declares an entity
// whose table still exists is refused, and no declaration exists that could make
// it acceptable.
func TestMigrateRefusesDroppedEntity(t *testing.T) {
	dir := t.TempDir()
	writeMigrateSpecWithFields(t, dir, "    - { name: code, type: string }\n")

	database, err := db.Open("sqlite::memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	runner := db.NewMigrationRunner(database, db.DriverSQLite)
	ctx := context.Background()

	if _, err := runner.ApplySpecSet(ctx, loadEntityMigrations(dir)); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// The manifest is gone; the table is not. Only the Entity is removed here, so
	// the dropped table is the only pending change.
	if err := os.Remove(filepath.Join(dir, "modules", "alpha", "master", "item.yaml")); err != nil {
		t.Fatal(err)
	}

	_, err = runner.ApplySpecSet(ctx, loadEntityMigrations(dir))
	if err == nil {
		t.Fatal("expected a dropped entity to be refused")
	}
	for _, want := range []string{"alpha_items", "drop it by hand"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not mention %q", err.Error(), want)
		}
	}

	// The table was removed by hand: now the stale snapshot is cleared instead of
	// refusing the same deployment forever.
	if _, err := database.ExecContext(ctx, "DROP TABLE alpha_items"); err != nil {
		t.Fatalf("drop table by hand: %v", err)
	}
	if _, err := runner.ApplySpecSet(ctx, loadEntityMigrations(dir)); err != nil {
		t.Fatalf("apply after the manual drop: %v", err)
	}
}
