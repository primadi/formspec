package db

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// RowPredicates is the storage-side half of "which rows may this caller
// touch?". It exists so an entity's `row_scope` AND a role grant's `row_scope`
// reach the SQL together (ANDed) instead of one silently replacing the other —
// that difference is "narrower than declared" versus "wider than declared", and
// only the first is safe.
//
// Kafe 10.67 is the case that motivated it: the rule "hanya pesanan lunas yang
// masuk dapur" belongs to the ROLE, so no per-entity declaration can express it
// without blinding the cashier to the drafts they are composing.
//
// These tests read the RAW stored payload / call the store directly, so they pin
// enforcement at the storage boundary rather than in one HTTP handler.

func rowPredicateStore(t *testing.T) (*EntityStore, context.Context) {
	t.Helper()
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "row_pred.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	meta := spec.Metadata{Name: "order", Module: "cafe"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "number", Type: spec.FieldString},
			{Name: "branch_id", Type: spec.FieldString},
			{Name: "status", Type: spec.FieldString},
		},
	}
	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}
	return NewEntityStore(d, DriverSQLite, meta, entity), ctx
}

func seedOrder(t *testing.T, store *EntityStore, ctx context.Context, ws, number, branch, status string) string {
	t.Helper()
	id, err := store.Insert(ctx, InsertParams{
		WorkspaceID: ws, CreatedBy: "u1", SystemCaller: true,
		Data: map[string]any{"number": number, "branch_id": branch, "status": status},
	})
	if err != nil {
		t.Fatalf("seed %s: %v", number, err)
	}
	return id
}

func TestEntityStore_ListRowPredicatesNarrows(t *testing.T) {
	store, ctx := rowPredicateStore(t)
	const ws = "t1"
	seedOrder(t, store, ctx, ws, "D1", "B1", "draft")
	seedOrder(t, store, ctx, ws, "P1", "B1", "paid")
	seedOrder(t, store, ctx, ws, "K1", "B1", "in_kitchen")

	// The predicate a kitchen role would carry: only paid-and-onward.
	res, err := store.List(ctx, ListParams{
		WorkspaceID: ws, PerPage: 100,
		RowPredicates: []RowPredicate{{Field: "status", Op: "in", Value: []string{"paid", "in_kitchen"}}},
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if res.Total != 2 {
		t.Fatalf("total = %d, want 2 — the draft order must not be listed", res.Total)
	}
	for _, rec := range res.Data {
		if rec.Data["status"] == "draft" {
			t.Fatalf("a draft order leaked through the row predicate: %#v", rec.Data)
		}
	}

	// Without the predicate the same query sees everything, which is what makes
	// the assertion above about the predicate and not about the data.
	all, err := store.List(ctx, ListParams{WorkspaceID: ws, PerPage: 100})
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if all.Total != 3 {
		t.Fatalf("unscoped total = %d, want 3", all.Total)
	}
}

func TestEntityStore_ListRowPredicatesCombineWithFilters(t *testing.T) {
	store, ctx := rowPredicateStore(t)
	const ws = "t1"
	seedOrder(t, store, ctx, ws, "P-B1", "B1", "paid")
	seedOrder(t, store, ctx, ws, "P-B2", "B2", "paid")

	// A client filter AND a server predicate on DIFFERENT fields: both must
	// apply. If the predicate were merged into the filter map instead of being
	// ANDed, one of the two would win and the caller could widen their view.
	res, err := store.List(ctx, ListParams{
		WorkspaceID: ws, PerPage: 100,
		Filters:       map[string]FilterOp{"branch_id": {Op: "eq", Value: "B2"}},
		RowPredicates: []RowPredicate{{Field: "status", Op: "eq", Value: "paid"}},
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if res.Total != 1 {
		t.Fatalf("total = %d, want 1 (B2 AND paid)", res.Total)
	}
	if res.Data[0].Data["number"] != "P-B2" {
		t.Fatalf("wrong row: %#v", res.Data[0].Data)
	}
}

func TestEntityStore_RowPredicatesBlockingSecondConstraintOnSameField(t *testing.T) {
	store, ctx := rowPredicateStore(t)
	const ws = "t1"
	seedOrder(t, store, ctx, ws, "A", "B1", "paid")
	seedOrder(t, store, ctx, ws, "B", "B1", "draft")

	// Two predicates on the SAME field: entity scope says "B1", the grant says
	// "paid only". ANDed, they narrow; merged into a map, one would erase the
	// other and the draft would be visible.
	res, err := store.List(ctx, ListParams{
		WorkspaceID: ws, PerPage: 100,
		RowPredicates: []RowPredicate{
			{Field: "branch_id", Op: "eq", Value: "B1"},
			{Field: "status", Op: "eq", Value: "paid"},
		},
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if res.Total != 1 {
		t.Fatalf("total = %d, want 1 — same-field predicates must AND, not replace", res.Total)
	}
}

func TestEntityStore_GetByIDHidesRowsOutsideScope(t *testing.T) {
	store, ctx := rowPredicateStore(t)
	const ws = "t1"
	draftID := seedOrder(t, store, ctx, ws, "D1", "B1", "draft")

	// Visible without a predicate...
	if _, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: ws, ID: draftID}); err != nil {
		t.Fatalf("unscoped read should succeed: %v", err)
	}

	// ...and ABSENT with one. Absent, not forbidden: the caller must not learn
	// that a row exists just beyond their boundary (the same answer `list`
	// gives when it hides those rows).
	_, err := store.GetByID(ctx, GetByIDParams{
		WorkspaceID: ws, ID: draftID,
		RowPredicates: []RowPredicate{{Field: "status", Op: "in", Value: []string{"paid", "in_kitchen"}}},
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound so the record reads as absent", err)
	}
}

func TestEntityStore_UpdateRespectsRowPredicates(t *testing.T) {
	store, ctx := rowPredicateStore(t)
	const ws = "t1"
	draftID := seedOrder(t, store, ctx, ws, "D1", "B1", "draft")
	paidID := seedOrder(t, store, ctx, ws, "P1", "B1", "paid")

	kitchenScope := []RowPredicate{{Field: "status", Op: "eq", Value: "paid"}}

	// Outside the scope: the update fails as not-found, BEFORE any write.
	if _, err := store.Update(ctx, UpdateParams{
		WorkspaceID: ws, ID: draftID, Version: 1, UpdatedBy: "u1",
		Data:          map[string]any{"status": "in_kitchen"},
		RowPredicates: kitchenScope,
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update outside scope: err = %v, want ErrNotFound", err)
	}
	// The proof that nothing was written: the row is still a draft.
	after, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: ws, ID: draftID})
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if after.Data["status"] != "draft" {
		t.Fatalf("status = %v, want draft — a refused update must not have written", after.Data["status"])
	}

	// Inside the scope: the same update succeeds.
	if _, err := store.Update(ctx, UpdateParams{
		WorkspaceID: ws, ID: paidID, Version: 1, UpdatedBy: "u1",
		Data:          map[string]any{"status": "in_kitchen"},
		RowPredicates: kitchenScope,
	}); err != nil {
		t.Fatalf("update inside scope: %v", err)
	}
}

func TestEntityStore_DeleteRespectsRowPredicates(t *testing.T) {
	store, ctx := rowPredicateStore(t)
	const ws = "t1"
	draftID := seedOrder(t, store, ctx, ws, "D1", "B1", "draft")
	paidID := seedOrder(t, store, ctx, ws, "P1", "B1", "paid")

	kitchenScope := []RowPredicate{{Field: "status", Op: "eq", Value: "paid"}}

	if err := store.SoftDelete(ctx, DeleteParams{
		WorkspaceID: ws, ID: draftID, DeletedBy: "u1",
		RowPredicates: kitchenScope,
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete outside scope: err = %v, want ErrNotFound", err)
	}
	if _, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: ws, ID: draftID}); err != nil {
		t.Fatalf("the refused delete must not have removed the row: %v", err)
	}

	if err := store.SoftDelete(ctx, DeleteParams{
		WorkspaceID: ws, ID: paidID, DeletedBy: "u1",
		RowPredicates: kitchenScope,
	}); err != nil {
		t.Fatalf("delete inside scope: %v", err)
	}
}

// An inexpressible predicate must FAIL the query, not vanish. A row restriction
// that silently disappears is worse than a rejected request: it turns a boundary
// into a decoration, and nothing in the response says so.
func TestEntityStore_InexpressiblePredicateFailsClosed(t *testing.T) {
	store, ctx := rowPredicateStore(t)
	const ws = "t1"
	seedOrder(t, store, ctx, ws, "P1", "B1", "paid")

	for _, tc := range []struct {
		name string
		pred RowPredicate
	}{
		{"between with one bound", RowPredicate{Field: "status", Op: "between", Value: "paid"}},
		{"in with an empty list", RowPredicate{Field: "status", Op: "in", Value: []any{}}},
		{"no field", RowPredicate{Op: "eq", Value: "paid"}},
		{"unknown operator", RowPredicate{Field: "status", Op: "sounds_like", Value: "paid"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := store.List(ctx, ListParams{WorkspaceID: ws, PerPage: 10, RowPredicates: []RowPredicate{tc.pred}}); err == nil {
				t.Fatal("expected the list to fail rather than run unfiltered")
			}
			if _, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: ws, ID: "x", RowPredicates: []RowPredicate{tc.pred}}); err == nil {
				t.Fatal("expected the read to fail rather than return an unfiltered row")
			}
		})
	}
}
