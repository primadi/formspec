package sidecar

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/forma/renderers/jsonbpersist"
	"github.com/primadi/forma/pkg/spec"
)

// TestCtxHandler_ScopeIdJoinsSameTransaction proves the cross-process
// correlation described in renderers/jsonbpersist/txscope.go: a request
// carrying X-Forma-Scope-Id resolves back to the SAME live *db.TxScope a
// direct in-process caller registered, and an entity write made through
// the HTTP /ctx/entity/update path joins that transaction rather than
// committing on its own — rolling back together with the in-process write
// when the scope is rolled back.
func TestCtxHandler_ScopeIdJoinsSameTransaction(t *testing.T) {
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "sidecar_scope.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "order", Module: "billing"}
	entitySpec := &spec.EntitySpec{
		Version: "v1",
		Plural:  "orders",
		Fields: []spec.Field{
			{Name: "status", Type: spec.FieldString},
			{Name: "note", Type: spec.FieldString},
		},
	}
	runner := db.NewMigrationRunner(d, db.DriverSQLite)
	ctx := context.Background()
	if _, err := runner.ApplyMigrations(ctx, []db.EntityMigration{{Metadata: meta, EntitySpec: *entitySpec}}); err != nil {
		t.Fatalf("ApplyMigrations: %v", err)
	}
	store := db.NewEntityStore(d, db.DriverSQLite, meta, entitySpec)

	id, err := store.Insert(ctx, db.InsertParams{
		WorkspaceID: "t1", CreatedBy: "tester",
		Data: map[string]any{"status": "draft", "note": "orig"},
	})
	if err != nil {
		t.Fatalf("seed insert: %v", err)
	}

	resolver := func(primitiveType, name string) (any, error) {
		if primitiveType == "entity" {
			return store, nil
		}
		return nil, nil
	}
	h := NewCtxHandler(resolver, "t1")

	// Open a scope in-process, as HandleCustomAction would, and register it
	// — simulating the outbound /invoke/... call's SidecarExecutor having
	// forwarded this scope's id in a header.
	scope := db.NewTxScope()
	scopeID := db.RegisterScope(scope)
	defer db.UnregisterScope(scopeID)

	scopedCtx := db.WithTxScope(ctx, scope, scopeID)
	if err := store.UpdateFields(scopedCtx, "t1", id, map[string]any{"status": "in-progress"}); err != nil {
		t.Fatalf("in-process scoped write: %v", err)
	}

	// The "app process" callback: an HTTP request carrying the scope id.
	req := httptest.NewRequest(http.MethodPost, "/ctx/entity/update",
		strings.NewReader(`{"key":"`+id+`","fields":{"note":"from-sidecar"}}`))
	req.Header.Set("X-Forma-Scope-Id", scopeID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ctx/entity/update status = %d body=%s", rec.Code, rec.Body)
	}

	// Nothing should be visible outside the scope yet — both writes are
	// still uncommitted in the same open transaction.
	if err := scope.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	final, err := store.GetByID(ctx, db.GetByIDParams{WorkspaceID: "t1", ID: id})
	if err != nil {
		t.Fatalf("final GetByID: %v", err)
	}
	if final.Data["status"] != "draft" {
		t.Fatalf("expected status still draft after rollback (in-process write undone), got %v", final.Data["status"])
	}
	if final.Data["note"] != "orig" {
		t.Fatalf("expected note still orig after rollback (sidecar-callback write undone too — proves it joined the same transaction), got %v", final.Data["note"])
	}
}

// TestCtxHandler_NoScopeIdCommitsIndependently confirms the fallback: a
// callback with no (or an unknown) X-Forma-Scope-Id header behaves exactly
// as before this change — a self-contained, independently committed write.
func TestCtxHandler_NoScopeIdCommitsIndependently(t *testing.T) {
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "sidecar_noscope.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "order", Module: "billing"}
	entitySpec := &spec.EntitySpec{
		Version: "v1",
		Plural:  "orders",
		Fields:  []spec.Field{{Name: "note", Type: spec.FieldString}},
	}
	runner := db.NewMigrationRunner(d, db.DriverSQLite)
	ctx := context.Background()
	if _, err := runner.ApplyMigrations(ctx, []db.EntityMigration{{Metadata: meta, EntitySpec: *entitySpec}}); err != nil {
		t.Fatalf("ApplyMigrations: %v", err)
	}
	store := db.NewEntityStore(d, db.DriverSQLite, meta, entitySpec)
	id, err := store.Insert(ctx, db.InsertParams{WorkspaceID: "t1", CreatedBy: "tester", Data: map[string]any{"note": "orig"}})
	if err != nil {
		t.Fatalf("seed insert: %v", err)
	}

	h := NewCtxHandler(func(primitiveType, name string) (any, error) { return store, nil }, "t1")

	req := httptest.NewRequest(http.MethodPost, "/ctx/entity/update",
		strings.NewReader(`{"key":"`+id+`","fields":{"note":"updated"}}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ctx/entity/update status = %d body=%s", rec.Code, rec.Body)
	}

	final, err := store.GetByID(ctx, db.GetByIDParams{WorkspaceID: "t1", ID: id})
	if err != nil {
		t.Fatalf("final GetByID: %v", err)
	}
	if final.Data["note"] != "updated" {
		t.Fatalf("expected note=updated (committed independently, no scope), got %v", final.Data["note"])
	}
}
