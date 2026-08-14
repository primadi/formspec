package db

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/primadi/formspec/pkg/spec"
)

// These tests reproduce the deadlock documented in
// examples/arisan/docs/engine-sqlite-deadlock.md: a request-scoped TxScope
// opens a transaction on SQLite's single connection (SetMaxOpenConns(1),
// see sqlite_db.go), and any store read/write that bypasses the scope by
// going through the raw connection pool has no free connection left to
// check out — it hangs forever instead of erroring.
//
// Every scoped context below carries a short deadline so a regression
// fails fast (context.DeadlineExceeded / a read/write error) instead of
// hanging the test suite.

func TestEntityStore_ResolveRelations_NoDeadlockUnderTxScope(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_txscope_relations.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	custMeta, custEntity := customerEntity()
	orderMeta, orderEnt := orderEntity()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{
		{Metadata: custMeta, EntitySpec: *custEntity},
		{Metadata: orderMeta, EntitySpec: *orderEnt},
	}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	custStore := NewEntityStore(d, DriverSQLite, custMeta, custEntity)
	orderStore := NewEntityStore(d, DriverSQLite, orderMeta, orderEnt)

	scope := NewTxScope()
	scopedCtx, cancel := context.WithTimeout(WithTxScope(ctx, scope, ""), 5*time.Second)
	defer cancel()

	custID, err := custStore.Insert(scopedCtx, InsertParams{
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "Jane Doe", "email": "jane@example.com"},
	})
	if err != nil {
		t.Fatalf("customer Insert under open TxScope failed: %v", err)
	}
	// A belongs_to target must be submitted (or lifecycle-free) before it
	// can be referenced — ValidateRelationTargets enforces this.
	if err := custStore.Submit(scopedCtx, "t1", custID, "u1"); err != nil {
		t.Fatalf("customer Submit under open TxScope failed: %v", err)
	}

	orderID, err := orderStore.Insert(scopedCtx, InsertParams{
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{
			"number":      "ORD-2026-000001",
			"customer_id": custID,
			"total":       float64(100),
			"status":      "draft",
		},
	})
	if err != nil {
		t.Fatalf("order Insert under open TxScope failed: %v", err)
	}

	// The scope's transaction is still open here (not yet committed) —
	// exactly the state a custom action's resource.fetch() runs in.
	// Before the fix, resolveRelations queried the raw connection pool and
	// deadlocked against this open transaction.
	rec, err := orderStore.GetByID(scopedCtx, GetByIDParams{WorkspaceID: "t1", ID: orderID})
	if err != nil {
		t.Fatalf("GetByID under open TxScope failed (deadlock?): %v", err)
	}

	if err := scope.Commit(); err != nil {
		t.Fatalf("scope commit failed: %v", err)
	}

	customerData, ok := rec.Data["customer"].(map[string]any)
	if !ok {
		t.Fatalf("expected resolved relation data at rec.Data[%q], got %#v", "customer", rec.Data["customer"])
	}
	if customerData["id"] != custID {
		t.Errorf("expected resolved customer id %q, got %v", custID, customerData["id"])
	}
	if customerData["name"] != "Jane Doe" {
		t.Errorf("expected resolved customer name %q, got %v", "Jane Doe", customerData["name"])
	}
}

func TestEntityStore_ListAndSubmit_NoDeadlockUnderTxScope(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_txscope_list_submit.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "invoice", Module: "billing"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields:  []spec.Field{{Name: "total", Type: spec.FieldNumber}},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	scope := NewTxScope()
	scopedCtx, cancel := context.WithTimeout(WithTxScope(ctx, scope, ""), 5*time.Second)
	defer cancel()

	id, err := store.Insert(scopedCtx, InsertParams{
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{"total": float64(50)},
	})
	if err != nil {
		t.Fatalf("Insert under open TxScope failed: %v", err)
	}

	// List's count + page queries ran on the raw pool before the fix —
	// same class of deadlock as resolveRelations, just not yet
	// script-reachable (query builder is still a stub).
	result, err := store.List(scopedCtx, ListParams{WorkspaceID: "t1", Page: 1, PerPage: 20})
	if err != nil {
		t.Fatalf("List under open TxScope failed (deadlock?): %v", err)
	}
	if result.Total != 1 {
		t.Errorf("expected 1 record, got %d", result.Total)
	}

	// Submit's UPDATE ran on the raw pool before the fix — same deadlock,
	// lifecycle-only so not yet script-reachable either.
	if err := store.Submit(scopedCtx, "t1", id, "u1"); err != nil {
		t.Fatalf("Submit under open TxScope failed (deadlock?): %v", err)
	}

	if err := scope.Commit(); err != nil {
		t.Fatalf("scope commit failed: %v", err)
	}

	rec, err := store.GetByID(context.Background(), GetByIDParams{WorkspaceID: "t1", ID: id})
	if err != nil {
		t.Fatalf("GetByID after commit failed: %v", err)
	}
	if rec.DocStatus != "submitted" {
		t.Errorf("expected doc_status='submitted', got %q", rec.DocStatus)
	}
}
