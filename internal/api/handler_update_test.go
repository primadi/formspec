package api

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/forma/internal/entity"
	"github.com/primadi/forma/pkg/spec"
	db "github.com/primadi/forma/renderers/jsonbpersist"
)

// setupUpdateTestEntity registers a simple "billing/order" entity and
// inserts one record, returning the handler factory and the record's id +
// initial version — shared fixture for the If-Match (2.6.5) tests below.
func setupUpdateTestEntity(t *testing.T) (factory *HandlerFactory, id string, version int) {
	t.Helper()
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "handler_update.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	reg := entity.NewRegistry(d, db.DriverSQLite, dir)
	orderSpec := spec.EntitySpec{
		Version: "v1",
		Plural:  "orders",
		Fields:  []spec.Field{{Name: "status", Type: spec.FieldString}},
	}
	registerTestEntity(t, d, reg, "billing", "order", orderSpec)

	store, err := reg.GetEntityStore("billing", "order")
	if err != nil {
		t.Fatalf("GetEntityStore: %v", err)
	}
	id, err = store.Insert(context.Background(), db.InsertParams{
		WorkspaceID: "t1",
		CreatedBy:   "tester",
		Data:        map[string]any{"status": "draft"},
	})
	if err != nil {
		t.Fatalf("seed insert: %v", err)
	}

	return NewHandlerFactory(reg), id, 1
}

func doUpdate(t *testing.T, factory *HandlerFactory, id, ifMatch string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("PATCH", "/billing/orders/"+id, strings.NewReader(`{"status":"submitted"}`))
	req.SetPathValue("id", id)
	req = req.WithContext(WithWorkspace(req.Context(), "t1"))
	req = req.WithContext(WithUser(req.Context(), "tester"))
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
	rr := httptest.NewRecorder()
	factory.HandleUpdate("billing", "order")(rr, req)
	return rr
}

// TestHandleUpdate_IfMatch_CorrectVersionSucceeds verifies the server now
// actually honors the If-Match header renderers/web's apiPatch already
// sends (lib/api/client.ts) — a matching version updates normally.
func TestHandleUpdate_IfMatch_CorrectVersionSucceeds(t *testing.T) {
	factory, id, version := setupUpdateTestEntity(t)
	rr := doUpdate(t, factory, id, "version=1")
	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	_ = version
}

// TestHandleUpdate_IfMatch_StaleVersionConflicts verifies a client-supplied
// version that doesn't match the current row is a real CAS conflict (409),
// not silently overwritten with whatever the server just fetched.
func TestHandleUpdate_IfMatch_StaleVersionConflicts(t *testing.T) {
	factory, id, _ := setupUpdateTestEntity(t)
	rr := doUpdate(t, factory, id, "version=99")
	if rr.Code != 409 {
		t.Fatalf("expected 409 CONFLICT for stale If-Match, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestHandleUpdate_IfMatch_Malformed verifies a garbled If-Match header is
// rejected as a validation error rather than silently ignored.
func TestHandleUpdate_IfMatch_Malformed(t *testing.T) {
	factory, id, _ := setupUpdateTestEntity(t)
	rr := doUpdate(t, factory, id, "bogus")
	if rr.Code != 422 {
		t.Fatalf("expected 422 for malformed If-Match, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestHandleUpdate_RelaxedMode_MissingIfMatchSucceeds verifies today's
// behavior is preserved by default (relaxed/dev mode, matching the clinic
// e2e example and any other caller that doesn't send the header yet).
func TestHandleUpdate_RelaxedMode_MissingIfMatchSucceeds(t *testing.T) {
	SetStrictMode(false)
	factory, id, _ := setupUpdateTestEntity(t)
	rr := doUpdate(t, factory, id, "")
	if rr.Code != 200 {
		t.Fatalf("expected 200 in relaxed mode without If-Match, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestHandleUpdate_StrictMode_MissingIfMatchConflicts verifies production
// posture: strict mode requires the client to prove it read the current
// version before writing (todo.md 2.6.5 — "update without version → 409").
func TestHandleUpdate_StrictMode_MissingIfMatchConflicts(t *testing.T) {
	SetStrictMode(true)
	t.Cleanup(func() { SetStrictMode(false) })

	factory, id, _ := setupUpdateTestEntity(t)
	rr := doUpdate(t, factory, id, "")
	if rr.Code != 409 {
		t.Fatalf("expected 409 in strict mode without If-Match, got %d: %s", rr.Code, rr.Body.String())
	}
}
