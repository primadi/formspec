package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/internal/entity"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// setupApiKeyStoreEnv builds a registry with core entities synced and returns
// an ApiKeyStore backed by formspec.core.api-key, with the global api key
// store wired into the middleware.
func setupApiKeyStoreEnv(t *testing.T) *auth.ApiKeyStore {
	t.Helper()
	ResetAuthRateLimiters()
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "apikey_mw_test.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	reg := entity.NewRegistry(d, db.DriverSQLite, "")
	if err := auth.RegisterCoreEntities(reg); err != nil {
		t.Fatalf("RegisterCoreEntities: %v", err)
	}
	if _, err := reg.SyncSchema(context.Background()); err != nil {
		t.Fatalf("SyncSchema: %v", err)
	}

	es, err := auth.NewRoleResolver(reg).Resolve(auth.RoleApiKey)
	if err != nil {
		t.Fatalf("resolve api-key: %v", err)
	}
	store := auth.NewApiKeyStore(es)
	prev := GetApiKeyStore()
	SetApiKeyStore(store)
	t.Cleanup(func() { SetApiKeyStore(prev) })
	return store
}

func TestApiKeyMiddleware_ExternalSurface(t *testing.T) {
	store := setupApiKeyStoreEnv(t)
	ctx := context.Background()

	plaintext, err := store.Create(ctx, "demo", &auth.ApiKey{
		Name:        "svc",
		Scope:       "workspace",
		Permissions: []string{"billing.customers.list"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	handler := AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := IdentityFromContext(r.Context())
		if id == nil || !id.IsAuthenticated() {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if !id.HasPermission("billing.customers.list") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	// External surface (/api/v1/) accepts X-FormSpec-Key.
	req := httptest.NewRequest(http.MethodGet, "/demo/api/v1/billing/customers", nil)
	req.Header.Set("X-FormSpec-Key", plaintext)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 on external surface, got %d", rr.Code)
	}
}

func TestApiKeyMiddleware_RejectedOnUISurface(t *testing.T) {
	store := setupApiKeyStoreEnv(t)
	ctx := context.Background()

	plaintext, err := store.Create(ctx, "demo", &auth.ApiKey{
		Name:        "svc",
		Scope:       "workspace",
		Permissions: []string{"*"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	handler := AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := IdentityFromContext(r.Context())
		if id != nil && id.IsAuthenticated() {
			w.WriteHeader(http.StatusOK)
			return
		}
		// API key must NOT authenticate on the UI surface → anonymous.
		w.WriteHeader(http.StatusUnauthorized)
	}))

	// UI surface (/_ui/) must NOT accept X-FormSpec-Key.
	req := httptest.NewRequest(http.MethodGet, "/demo/_ui/entity/billing/customers", nil)
	req.Header.Set("X-FormSpec-Key", plaintext)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on UI surface (api key rejected), got %d", rr.Code)
	}
}

func TestApiKeyMiddleware_InvalidKey(t *testing.T) {
	setupApiKeyStoreEnv(t)

	handler := AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/demo/api/v1/billing/customers", nil)
	req.Header.Set("X-FormSpec-Key", "fs_bogus_key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid key, got %d", rr.Code)
	}
}
