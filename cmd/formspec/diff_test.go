package main

import (
	"context"
	"testing"

	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// TestDiffReportsDifferences verifies computeDiff reports pending structural
// migrations when the local spec has not been applied to the database.
func TestDiffReportsDifferences(t *testing.T) {
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

	// Fresh DB → 1 pending migration (the item table).
	results, err := computeDiff(ctx, runner, entities)
	if err != nil {
		t.Fatalf("computeDiff: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 difference, got %d", len(results))
	}

	// After applying, no differences remain.
	if _, err := runner.ApplyMigrations(ctx, entities); err != nil {
		t.Fatalf("apply: %v", err)
	}
	results, err = computeDiff(ctx, runner, entities)
	if err != nil {
		t.Fatalf("computeDiff after apply: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 differences after apply, got %d", len(results))
	}
}
