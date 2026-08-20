package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// setupEnforcementEnv builds a router with an exposed entity (billing.order)
// and a JWT validator, so permission enforcement can be tested with a real
// token carrying specific permissions.
func setupEnforcementEnv(t *testing.T) http.Handler {
	t.Helper()
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "enforce_test.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	reg := entity.NewRegistry(d, db.DriverSQLite, "")
	// Register an exposed entity (non-internal) so routes are generated.
	raw := manifest.RawManifest{
		APIVersion: "formspec.dev/v1",
		Kind:       "Entity",
		Metadata:   manifest.RawMetadata{Name: "order", Module: "billing"},
		Source:     "test",
	}
	if err := reg.RegisterArtifactManifest(raw, &spec.EntitySpec{
		Version:        "v1",
		Plural:         "orders",
		Characteristic: spec.CharMaster,
		Fields:         []spec.Field{{Name: "number", Type: spec.FieldString}},
		Expose: []spec.ExposeConfig{
			{Type: spec.ProtocolREST, Actions: []string{"list", "find", "create"}},
		},
	}); err != nil {
		t.Fatalf("RegisterArtifactManifest: %v", err)
	}
	if _, err := reg.SyncSchema(context.Background()); err != nil {
		t.Fatalf("SyncSchema: %v", err)
	}

	// Prod-mode JWT validator so identity carries real permissions.
	// Restore the previous validator on cleanup so other tests (e.g. WS tests
	// relying on the dev fallback) are unaffected.
	prev := GetAuthValidator()
	SetAuthValidator(auth.NewJWTValidator("test-secret", "formspec", ""))
	t.Cleanup(func() { SetAuthValidator(prev) })

	rb := NewRouterBuilder(reg)
	rb.BuildRoutes()
	return rb.BuildHTTP()
}

// issueToken signs an access token with the given permissions for user "u1".
func issueToken(t *testing.T, perms []string) string {
	t.Helper()
	issuer := auth.NewTokenIssuer("test-secret", "formspec", "", 0, 0)
	tok, err := issuer.IssueAccessToken(&auth.User{
		ID:          "u1",
		Username:    "u1",
		WorkspaceID: "demo",
		Permissions: perms,
	})
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}
	return tok
}

func TestEnforcement_UISurface_EntityList_NoPerm_404(t *testing.T) {
	handler := setupEnforcementEnv(t)
	token := issueToken(t, []string{"billing.orders.create"}) // no list/view

	req := httptest.NewRequest(http.MethodGet, "/demo/_ui/entity/billing/order", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("UI entity list without list/view: expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestEnforcement_External_EntityList_NoPerm_403(t *testing.T) {
	handler := setupEnforcementEnv(t)
	token := issueToken(t, []string{"billing.orders.create"}) // no list/view

	req := httptest.NewRequest(http.MethodGet, "/demo/api/v1/billing/orders", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("external entity list without list/view: expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestEnforcement_UISurface_EntityList_WithPerm_200(t *testing.T) {
	handler := setupEnforcementEnv(t)
	token := issueToken(t, []string{"billing.orders.list"})

	req := httptest.NewRequest(http.MethodGet, "/demo/_ui/entity/billing/order", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("UI entity list with list perm: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestEnforcement_UISurface_EntityCreate_NoPerm_403(t *testing.T) {
	handler := setupEnforcementEnv(t)
	token := issueToken(t, []string{"billing.orders.list"}) // no create

	req := httptest.NewRequest(http.MethodPost, "/demo/_ui/entity/billing/order", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// create is NOT a list/view permission → 403 (not 404).
	if rec.Code != http.StatusForbidden {
		t.Fatalf("UI entity create without create perm: expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestEnforcement_NoToken_401(t *testing.T) {
	handler := setupEnforcementEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/demo/_ui/entity/billing/order", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token: expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}
