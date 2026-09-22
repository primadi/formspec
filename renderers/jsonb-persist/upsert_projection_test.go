package db

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// setupSummaryStore creates a `characteristic: summary` entity with a unique
// index on (branch_id, ingredient_id) — the shape of kafe's stock-level
// projection — and returns the store.
func setupSummaryStore(t *testing.T) *EntityStore {
	t.Helper()
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "summary.db"), nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	meta := spec.Metadata{Name: "stock-level", Module: "cafe-stock"}
	entity := &spec.EntitySpec{
		Version:        "v1",
		Characteristic: spec.CharSummary,
		Fields: []spec.Field{
			{Name: "branch_id", Type: spec.FieldString},
			{Name: "ingredient_id", Type: spec.FieldString},
			{Name: "quantity_on_hand", Type: spec.FieldDecimal},
			{Name: "moving_avg_cost", Type: spec.FieldMoney},
		},
		Indexes: []spec.IndexDecl{{Fields: []string{"branch_id", "ingredient_id"}, Unique: true}},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	return NewEntityStore(d, DriverSQLite, meta, entity)
}

// TestUpsertProjection_InsertThenUpdate pins item 4.1: UpsertProjection is the
// one supported write path for a summary projection. The first call inserts,
// the second (same match) updates in place — no duplicate row, and fields not
// mentioned in the second call are preserved (merge, not replace).
func TestUpsertProjection_InsertThenUpdate(t *testing.T) {
	store := setupSummaryStore(t)
	ctx := context.Background()

	id1, created1, err := store.UpsertProjection(ctx, "kafe",
		map[string]any{"branch_id": "B1", "ingredient_id": "kopi"},
		map[string]any{"quantity_on_hand": 10.0, "moving_avg_cost": spec.Money{Amount: "5000", Currency: "IDR"}})
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if !created1 {
		t.Error("first upsert should report created=true")
	}

	// Same key, partial data: must update the same row and keep the fields it
	// did not mention.
	id2, created2, err := store.UpsertProjection(ctx, "kafe",
		map[string]any{"branch_id": "B1", "ingredient_id": "kopi"},
		map[string]any{"quantity_on_hand": 25.0})
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if created2 {
		t.Error("second upsert should report created=false")
	}
	if id1 != id2 {
		t.Errorf("upsert should reuse the row: id1=%s id2=%s", id1, id2)
	}

	rec, err := store.FindByFields(ctx, "kafe", map[string]any{"branch_id": "B1", "ingredient_id": "kopi"})
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if rec.Data["quantity_on_hand"] != 25.0 {
		t.Errorf("quantity_on_hand = %v, want 25", rec.Data["quantity_on_hand"])
	}
	// moving_avg_cost was not in the second call — merge must preserve it.
	if rec.Data["moving_avg_cost"] == nil {
		t.Error("moving_avg_cost was wiped by a partial upsert (merge failed)")
	}

	// A different key is a different row.
	_, created3, err := store.UpsertProjection(ctx, "kafe",
		map[string]any{"branch_id": "B2", "ingredient_id": "kopi"},
		map[string]any{"quantity_on_hand": 5.0})
	if err != nil {
		t.Fatalf("third upsert: %v", err)
	}
	if !created3 {
		t.Error("a different key should insert a new row")
	}
}

// TestFindByFields_NoMatchReturnsNil pins that a find with no matching row
// returns (nil, nil), not ErrNotFound — resource.find() from Starlark treats a
// miss as None, and a guard that checks "does this row exist" must not blow up
// when the answer is "no". (Regression: the maintainer script failed with
// "resource.find(...): not found" on the first movement, before any row existed.)
func TestFindByFields_NoMatchReturnsNil(t *testing.T) {
	store := setupSummaryStore(t)
	ctx := context.Background()

	rec, err := store.FindByFields(ctx, "kafe", map[string]any{"branch_id": "NOPE", "ingredient_id": "NOPE"})
	if err != nil {
		t.Fatalf("find with no match: expected nil error, got %v", err)
	}
	if rec != nil {
		t.Fatalf("find with no match: expected nil record, got %+v", rec)
	}
}

// TestUpsertProjection_RejectsNonSummary pins that the write path is summary-only:
// a normal entity must go through the action pipeline, not this method.
func TestUpsertProjection_RejectsNonSummary(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "master.db"), nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	meta := spec.Metadata{Name: "branch", Module: "cafe-master"}
	entity := &spec.EntitySpec{
		Version:        "v1",
		Characteristic: spec.CharMaster,
		Fields:         []spec.Field{{Name: "code", Type: spec.FieldString}},
	}
	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	store := NewEntityStore(d, DriverSQLite, meta, entity)

	if _, _, err := store.UpsertProjection(ctx, "kafe",
		map[string]any{"code": "B1"}, map[string]any{"code": "B1"}); err == nil {
		t.Error("UpsertProjection on a master entity: expected an error, got none")
	}
}

// TestUpsertProjection_RejectsEmptyMatch pins that a match is required — an
// empty match would upsert "the first row", which is never what a projection
// wants.
func TestUpsertProjection_RejectsEmptyMatch(t *testing.T) {
	store := setupSummaryStore(t)
	if _, _, err := store.UpsertProjection(context.Background(), "kafe", nil, map[string]any{"quantity_on_hand": 1.0}); err == nil {
		t.Error("UpsertProjection with an empty match: expected an error, got none")
	}
}
