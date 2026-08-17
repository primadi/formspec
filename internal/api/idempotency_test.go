package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/action"
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// setupIdempotencyEnv builds a registry with an entity whose create action is
// declared idempotent with a server-sourced key, plus a custom idempotent
// action, and wires an IdempotencyStore into the factory.
func setupIdempotencyEnv(t *testing.T) (*entity.Registry, db.DB, *HandlerFactory, *db.IdempotencyStore) {
	t.Helper()

	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "idem.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	reg := entity.NewRegistry(d, db.DriverSQLite, dir)

	entitySpec := spec.EntitySpec{
		Version: "v1",
		Plural:  "orders",
		Fields: []spec.Field{
			{Name: "number", Type: spec.FieldString},
			{Name: "status", Type: spec.FieldString},
		},
		Actions: []spec.Action{
			{
				Name:       "create",
				Idempotent: true,
				IdempotencyKey: &spec.IdempotencyDecl{
					From: "server",
				},
			},
			{
				Name:       "confirm",
				Idempotent: true,
				IdempotencyKey: &spec.IdempotencyDecl{
					From: "server",
				},
				Impl: &spec.ImplDecl{Type: spec.ImplNative, Ref: "Test.Confirm"},
			},
		},
	}
	registerTestEntity(t, d, reg, "billing", "order", entitySpec)

	// Native handler for the confirm custom action.
	nativeEx := action.NewNativeExecutor()
	nativeEx.Register("Test.Confirm", func(ctx context.Context, p action.ExecuteParams) (any, error) {
		return map[string]any{"confirmed": true, "id": p.ResourceID}, nil
	})
	disp := action.NewDispatcher()
	disp.RegisterExecutor(spec.ImplNative, nativeEx)
	disp.SetNativeExecutor(nativeEx)

	factory := NewHandlerFactory(reg)
	factory.SetDispatcher(disp)
	factory.SetSpecLookup(func(module, name string) (*spec.EntitySpec, bool) {
		info, ok := reg.GetEntity(module, name)
		if !ok || info.EntitySpec == nil {
			return nil, false
		}
		return info.EntitySpec, true
	})

	store := db.NewIdempotencyStore(d, db.DriverSQLite)
	factory.SetIdempotencyStore(store)

	return reg, d, factory, store
}

// TestHandlePrepare_IssuesKey verifies the two-step prepare endpoint returns
// a fresh idempotency key for a server-sourced idempotent action (2.7.1).
func TestHandlePrepare_IssuesKey(t *testing.T) {
	_, _, factory, _ := setupIdempotencyEnv(t)

	actionSpec := spec.Action{
		Name:       "create",
		Idempotent: true,
		IdempotencyKey: &spec.IdempotencyDecl{
			From: "server",
		},
	}
	handler := factory.HandlePrepare("billing", "order", "create", actionSpec)

	req := httptest.NewRequest("POST", "/billing/orders/create/prepare", nil)
	req = req.WithContext(WithWorkspace(req.Context(), "t1"))
	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp SingleResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %T", resp.Data)
	}
	key, _ := data["idempotency_key"].(string)
	if key == "" {
		t.Fatal("expected non-empty idempotency_key")
	}
}

// TestHandlePrepare_RejectsNonServerSource verifies prepare 404s for actions
// that are not server-sourced idempotent (header/param or non-idempotent).
func TestHandlePrepare_RejectsNonServerSource(t *testing.T) {
	_, _, factory, _ := setupIdempotencyEnv(t)

	// Non-idempotent action → prepare must 404.
	handler := factory.HandlePrepare("billing", "order", "create", spec.Action{Name: "create"})
	req := httptest.NewRequest("POST", "/billing/orders/create/prepare", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-idempotent prepare, got %d", rr.Code)
	}

	// Header-sourced idempotent action → prepare must 404 (client supplies key).
	handler = factory.HandlePrepare("billing", "order", "create", spec.Action{
		Name:       "create",
		Idempotent: true,
		IdempotencyKey: &spec.IdempotencyDecl{
			From: "header",
		},
	})
	req = httptest.NewRequest("POST", "/billing/orders/create/prepare", nil)
	rr = httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for header-sourced prepare, got %d", rr.Code)
	}
}

// TestHandleCreate_IdempotentReplay verifies the core 2.7.2 contract: a
// create declared idempotent with a server-sourced key replays the original
// response on a duplicate call instead of inserting a second row.
func TestHandleCreate_IdempotentReplay(t *testing.T) {
	reg, _, factory, _ := setupIdempotencyEnv(t)
	ctx := context.Background()

	handler := factory.HandleCreate("billing", "order")

	// First call with a key → creates the record.
	key := "key-1"
	body := strings.NewReader(`{"number":"ORD-1"}`)
	req := httptest.NewRequest("POST", "/billing/orders", body)
	req.Header.Set("Idempotency-Key", key)
	req = req.WithContext(WithWorkspace(req.Context(), "t1"))
	req = req.WithContext(WithUser(req.Context(), "tester"))
	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 on first create, got %d: %s", rr.Code, rr.Body.String())
	}
	var first SingleResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &first); err != nil {
		t.Fatalf("unmarshal first: %v", err)
	}

	// Duplicate call with the same key → replay, no second row.
	body2 := strings.NewReader(`{"number":"ORD-1"}`)
	req2 := httptest.NewRequest("POST", "/billing/orders", body2)
	req2.Header.Set("Idempotency-Key", key)
	req2 = req2.WithContext(WithWorkspace(req2.Context(), "t1"))
	req2 = req2.WithContext(WithUser(req2.Context(), "tester"))
	rr2 := httptest.NewRecorder()
	handler(rr2, req2)

	if rr2.Code != http.StatusCreated {
		t.Fatalf("expected 201 on replay, got %d: %s", rr2.Code, rr2.Body.String())
	}
	var second SingleResponse
	if err := json.Unmarshal(rr2.Body.Bytes(), &second); err != nil {
		t.Fatalf("unmarshal second: %v", err)
	}

	// Same record id in both responses → replay, not a new insert.
	firstData, _ := first.Data.(map[string]any)
	secondData, _ := second.Data.(map[string]any)
	if firstData["id"] != secondData["id"] {
		t.Fatalf("expected replay (same id), got first=%v second=%v", firstData["id"], secondData["id"])
	}

	// Exactly one row exists.
	store, err := reg.GetEntityStore("billing", "order")
	if err != nil {
		t.Fatalf("GetEntityStore: %v", err)
	}
	list, err := store.List(ctx, db.ListParams{WorkspaceID: "t1", PerPage: 100})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if list.Total != 1 {
		t.Fatalf("expected exactly 1 row after replay, got %d", list.Total)
	}
}

// TestHandleCreate_IdempotentRequiresKey verifies an idempotent create
// without a key is rejected with 422 (spec §5: idempotent requires a key).
func TestHandleCreate_IdempotentRequiresKey(t *testing.T) {
	_, _, factory, _ := setupIdempotencyEnv(t)

	handler := factory.HandleCreate("billing", "order")
	body := strings.NewReader(`{"number":"ORD-2"}`)
	req := httptest.NewRequest("POST", "/billing/orders", body)
	req = req.WithContext(WithWorkspace(req.Context(), "t1"))
	req = req.WithContext(WithUser(req.Context(), "tester"))
	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for missing key, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestHandleCreate_IdempotentInFlight409 verifies a duplicate call while the
// key is still pending (in-flight) returns 409 CONFLICT (2.7.2).
func TestHandleCreate_IdempotentInFlight409(t *testing.T) {
	_, _, factory, store := setupIdempotencyEnv(t)
	ctx := context.Background()

	// Pre-claim the key as pending (simulating an in-flight request).
	claimed, existing, err := store.TryClaim(ctx, "t1", "create", "key-inflight")
	if err != nil {
		t.Fatalf("TryClaim: %v", err)
	}
	if !claimed || existing != nil {
		t.Fatalf("expected fresh claim, got claimed=%v existing=%v", claimed, existing)
	}

	handler := factory.HandleCreate("billing", "order")
	body := strings.NewReader(`{"number":"ORD-3"}`)
	req := httptest.NewRequest("POST", "/billing/orders", body)
	req.Header.Set("Idempotency-Key", "key-inflight")
	req = req.WithContext(WithWorkspace(req.Context(), "t1"))
	req = req.WithContext(WithUser(req.Context(), "tester"))
	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 for in-flight key, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestHandleCustomAction_IdempotentReplay verifies idempotency enforcement
// on custom actions: duplicate call with the same key replays the stored
// response instead of re-running the handler.
func TestHandleCustomAction_IdempotentReplay(t *testing.T) {
	reg, _, factory, _ := setupIdempotencyEnv(t)
	ctx := context.Background()

	store, err := reg.GetEntityStore("billing", "order")
	if err != nil {
		t.Fatalf("GetEntityStore: %v", err)
	}
	id, err := store.Insert(ctx, db.InsertParams{
		WorkspaceID: "t1",
		CreatedBy:   "tester",
		Data:        map[string]any{"number": "ORD-9", "status": "draft"},
	})
	if err != nil {
		t.Fatalf("seed insert: %v", err)
	}

	actionSpec := spec.Action{
		Name:       "confirm",
		Idempotent: true,
		IdempotencyKey: &spec.IdempotencyDecl{
			From: "server",
		},
		Impl: &spec.ImplDecl{Type: spec.ImplNative, Ref: "Test.Confirm"},
	}
	handler := factory.HandleCustomAction("billing", "order", "confirm", actionSpec, "")

	key := "confirm-key-1"
	doCall := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/billing/orders/"+id+"/confirm", nil)
		req.SetPathValue("id", id)
		req.Header.Set("Idempotency-Key", key)
		req = req.WithContext(WithWorkspace(req.Context(), "t1"))
		req = req.WithContext(WithUser(req.Context(), "tester"))
		rr := httptest.NewRecorder()
		handler(rr, req)
		return rr
	}

	rr1 := doCall()
	if rr1.Code != http.StatusOK {
		t.Fatalf("expected 200 on first confirm, got %d: %s", rr1.Code, rr1.Body.String())
	}
	rr2 := doCall()
	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200 on replay, got %d: %s", rr2.Code, rr2.Body.String())
	}

	var first, second SingleResponse
	if err := json.Unmarshal(rr1.Body.Bytes(), &first); err != nil {
		t.Fatalf("unmarshal first: %v", err)
	}
	if err := json.Unmarshal(rr2.Body.Bytes(), &second); err != nil {
		t.Fatalf("unmarshal second: %v", err)
	}
	firstJSON, _ := json.Marshal(first.Data)
	secondJSON, _ := json.Marshal(second.Data)
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("expected identical replayed data, got first=%s second=%s", firstJSON, secondJSON)
	}
}

// TestGeneratePrepareRoutes verifies prepare routes are generated for
// server-sourced idempotent actions on both surfaces, and NOT for
// header-sourced or non-idempotent actions.
func TestGeneratePrepareRoutes(t *testing.T) {
	es := &spec.EntitySpec{
		Version: "v1",
		Plural:  "orders",
		Actions: []spec.Action{
			{
				Name:       "create",
				Idempotent: true,
				IdempotencyKey: &spec.IdempotencyDecl{
					From: "server",
				},
			},
			{
				Name:       "confirm",
				Idempotent: true,
				IdempotencyKey: &spec.IdempotencyDecl{
					From: "server",
				},
			},
			{
				Name:       "ship",
				Idempotent: true,
				IdempotencyKey: &spec.IdempotencyDecl{
					From: "header",
				},
			},
			{
				Name: "plain",
			},
		},
	}

	routes := generatePrepareRoutes("billing", "order", "orders", es, false, nil)

	got := map[string]bool{}
	for _, rd := range routes {
		if rd.Handler != "prepare" {
			t.Errorf("expected handler=prepare, got %q", rd.Handler)
		}
		got[rd.Path] = true
	}

	if !got["/api/v1/billing/orders/create/prepare"] {
		t.Errorf("missing create prepare route; got %v", got)
	}
	if !got["/api/v1/billing/orders/confirm/prepare"] {
		t.Errorf("missing confirm prepare route; got %v", got)
	}
	if got["/api/v1/billing/orders/ship/prepare"] {
		t.Errorf("header-sourced action must NOT get a prepare route; got %v", got)
	}
	if got["/api/v1/billing/orders/plain/prepare"] {
		t.Errorf("non-idempotent action must NOT get a prepare route; got %v", got)
	}
}
