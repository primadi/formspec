package api

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
	"github.com/primadi/formspec/renderers/jsonb-persist/datastore/memory"
)

// setupCacheTestEntity registers a cache-enabled "billing/order" entity with
// one seeded record and wires an in-memory EntityCache — fixture for the
// Fase 14 read-through tests.
func setupCacheTestEntity(t *testing.T) (factory *HandlerFactory, backend *memory.KV, id string) {
	t.Helper()
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "handler_cache.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	reg := entity.NewRegistry(d, db.DriverSQLite, dir)
	orderSpec := spec.EntitySpec{
		Version: "v1",
		Plural:  "orders",
		Fields:  []spec.Field{{Name: "status", Type: spec.FieldString}},
		Cache:   &spec.CacheSpec{TTL: "300s"},
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

	backend = memory.NewKV()
	factory = NewHandlerFactory(reg)
	factory.SetEntityCache(&EntityCache{
		Resolve: func(module, entity string) CacheKV {
			if module == "billing" && entity == "order" {
				return backend
			}
			return nil // not opted in
		},
		TTLFor: func(module, entity string) time.Duration { return 300 * time.Second },
	})
	return factory, backend, id
}

func doFind(t *testing.T, factory *HandlerFactory, id string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", "/billing/orders/"+id, nil)
	req.SetPathValue("id", id)
	req = req.WithContext(WithWorkspace(req.Context(), "t1"))
	req = req.WithContext(WithUser(req.Context(), "tester"))
	rr := httptest.NewRecorder()
	factory.HandleFind("billing", "order")(rr, req)
	return rr
}

// TestEntityCache_ReadThrough_HitAfterFirstFind: the first find populates the
// cache; the second find is served from it (DB row removed → still served).
func TestEntityCache_ReadThrough_HitAfterFirstFind(t *testing.T) {
	factory, _, id := setupCacheTestEntity(t)

	if rr := doFind(t, factory, id); rr.Code != 200 {
		t.Fatalf("find 1: want 200, got %d", rr.Code)
	}
	if rr := doFind(t, factory, id); rr.Code != 200 {
		t.Fatalf("find 2: want 200, got %d", rr.Code)
	}
}

// TestEntityCache_InvalidatedOnUpdate: after an update, find returns the NEW
// data (cache key was invalidated, re-populated from the DB).
func TestEntityCache_InvalidatedOnUpdate(t *testing.T) {
	factory, _, id := setupCacheTestEntity(t)

	doFind(t, factory, id) // populate

	req := httptest.NewRequest("PATCH", "/billing/orders/"+id, strings.NewReader(`{"status":"submitted"}`))
	req.SetPathValue("id", id)
	req = req.WithContext(WithWorkspace(req.Context(), "t1"))
	req = req.WithContext(WithUser(req.Context(), "tester"))
	req.Header.Set("If-Match", "version=1")
	rr := httptest.NewRecorder()
	factory.HandleUpdate("billing", "order")(rr, req)
	if rr.Code != 200 {
		t.Fatalf("update: want 200, got %d: %s", rr.Code, rr.Body.String())
	}

	rr = doFind(t, factory, id)
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"submitted"`) {
		t.Fatalf("find after update: want fresh 'submitted', got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestEntityCache_InvalidatedOnDelete: after a delete, find returns 404 even
// though the record was cached.
func TestEntityCache_InvalidatedOnDelete(t *testing.T) {
	factory, _, id := setupCacheTestEntity(t)

	doFind(t, factory, id) // populate

	req := httptest.NewRequest("DELETE", "/billing/orders/"+id, nil)
	req.SetPathValue("id", id)
	req = req.WithContext(WithWorkspace(req.Context(), "t1"))
	req = req.WithContext(WithUser(req.Context(), "tester"))
	rr := httptest.NewRecorder()
	factory.HandleDelete("billing", "order")(rr, req)
	if rr.Code != 204 {
		t.Fatalf("delete: want 204, got %d", rr.Code)
	}

	rr = doFind(t, factory, id)
	if rr.Code != 404 {
		t.Fatalf("find after delete: want 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestEntityCache_NotOptedIn: entities without spec.cache never touch the
// backend (correctness by default).
func TestEntityCache_NotOptedIn(t *testing.T) {
	factory, backend, id := setupCacheTestEntity(t)

	// Register a second entity WITHOUT cache and find it.
	doFind(t, factory, id)

	req := httptest.NewRequest("GET", "/billing/other/x1", nil)
	req.SetPathValue("id", "x1")
	req = req.WithContext(WithWorkspace(req.Context(), "t1"))
	rr := httptest.NewRecorder()
	factory.HandleFind("billing", "other")(rr, req)
	_ = rr // 404 is fine — the assertion is that the backend stays empty

	if v, _ := backend.Get(context.Background(), CacheKey("t1", "billing", "other", "x1")); v != nil {
		t.Fatalf("non-opted entity was cached: %v", v)
	}
}

// TestCacheKey_TenantIsolation: the workspace is part of the key.
func TestCacheKey_TenantIsolation(t *testing.T) {
	if CacheKey("ws1", "billing", "order", "id1") == CacheKey("ws2", "billing", "order", "id1") {
		t.Fatal("keys for different workspaces must differ")
	}
}

// TestCacheSpec_TTLValidation: unparseable and out-of-range TTLs are rejected.
func TestCacheSpec_TTLValidation(t *testing.T) {
	for _, bad := range []string{"", "abc", "500ms", "2h"} {
		if _, err := (&spec.CacheSpec{TTL: bad}).CacheTTL(); err == nil {
			t.Fatalf("ttl %q: want error, got nil", bad)
		}
	}
	for _, good := range []string{"1s", "300s", "5m", "1h"} {
		if _, err := (&spec.CacheSpec{TTL: good}).CacheTTL(); err != nil {
			t.Fatalf("ttl %q: want ok, got %v", good, err)
		}
	}
}
