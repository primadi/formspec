package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"

	formspec_app "github.com/primadi/formspec/internal/app"
	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/ui"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// setupMetaTestRouter builds a RouterBuilder with an empty entity registry,
// an empty UI registry, and one resolved App ("storefront") — enough to
// exercise HandleMetaUI's admin vs App-scoped branching without needing real
// entity manifests on disk (entity content itself is covered by
// TestBuildBundlePermissionFiltering in internal/ui).
func setupMetaTestRouter(t *testing.T) *RouterBuilder {
	t.Helper()
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "meta_test.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	reg := entity.NewRegistry(d, db.DriverSQLite, dir)
	b := NewRouterBuilder(reg)
	b.SetUIRegistry(ui.NewRegistry())
	b.SetApps(map[string]*formspec_app.ResolvedApp{
		"storefront": {
			Name:    "storefront",
			Spec:    &spec.AppSpec{RootURL: "/app/storefront", Modules: []string{"sales"}},
			Modules: map[string]bool{"sales": true},
		},
	})
	return b
}

func TestHandleMetaUI_AdminMode_RequiresPermission(t *testing.T) {
	b := setupMetaTestRouter(t)
	handler := b.HandleMetaUI()

	identity := &auth.Identity{UserID: "user-1", WorkspaceID: "demo", Permissions: []string{}}
	req := httptest.NewRequest("GET", "/demo/_ui/_meta/ui?admin=true", nil)
	req = req.WithContext(WithIdentity(context.Background(), identity))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 403 {
		t.Fatalf("expected 403 without _admin.access, got %d: %s", rec.Code, rec.Body.String())
	}

	var errResp ErrorResponse
	json.NewDecoder(rec.Body).Decode(&errResp)
	if errResp.Error.Code != "FORBIDDEN" {
		t.Errorf("expected FORBIDDEN, got %s", errResp.Error.Code)
	}
}

func TestHandleMetaUI_AdminMode_AllowedWithPermission(t *testing.T) {
	b := setupMetaTestRouter(t)
	handler := b.HandleMetaUI()

	identity := &auth.Identity{UserID: "user-1", WorkspaceID: "demo", Permissions: []string{adminAccessPermission}}
	req := httptest.NewRequest("GET", "/demo/_ui/_meta/ui?admin=true", nil)
	req = req.WithContext(WithIdentity(context.Background(), identity))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200 with _admin.access, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp SingleResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, _ := json.Marshal(resp.Data)
	var bundle ui.Bundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		t.Fatalf("decode bundle: %v", err)
	}
	// Admin mode is unscoped by any App (Core §4.4 — _admin isn't App-scoped).
	if bundle.App.Name != "" {
		t.Errorf("expected unscoped bundle (no App name), got %q", bundle.App.Name)
	}
}

func TestHandleMetaUI_AppScoped_UnaffectedByAdminChange(t *testing.T) {
	b := setupMetaTestRouter(t)
	handler := b.HandleMetaUI()

	identity := &auth.Identity{UserID: "user-1", WorkspaceID: "demo", Permissions: []string{}}
	req := httptest.NewRequest("GET", "/demo/_ui/_meta/ui?app=storefront", nil)
	req = req.WithContext(WithIdentity(context.Background(), identity))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200 for regular App-scoped request, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp SingleResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, _ := json.Marshal(resp.Data)
	var bundle ui.Bundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		t.Fatalf("decode bundle: %v", err)
	}
	if bundle.App.Name != "storefront" {
		t.Errorf("expected bundle scoped to storefront, got %q", bundle.App.Name)
	}
}

func TestHandleMetaUI_PublicApp_AnonymousAllowed(t *testing.T) {
	b := setupMetaTestRouter(t)
	// Reconfigure the app as `access: public` (entirely public, no-nav).
	b.SetApps(map[string]*formspec_app.ResolvedApp{
		"storefront": {
			Name:    "storefront",
			Spec:    &spec.AppSpec{RootURL: "/", AppRenderer: "no-nav", Access: spec.AppAccessPublic, Modules: []string{"sales"}},
			Modules: map[string]bool{"sales": true},
		},
	})
	handler := b.HandleMetaUI()

	// No identity in context — anonymous caller.
	req := httptest.NewRequest("GET", "/demo/_ui/_meta/ui?app=storefront", nil)
	req = req.WithContext(context.Background())
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200 for anonymous public App bundle, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp SingleResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, _ := json.Marshal(resp.Data)
	var bundle ui.Bundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		t.Fatalf("decode bundle: %v", err)
	}
	if bundle.App.AppRenderer != "no-nav" {
		t.Errorf("expected app_renderer no-nav, got %q", bundle.App.AppRenderer)
	}
	if bundle.App.Access != "public" {
		t.Errorf("expected access public, got %q", bundle.App.Access)
	}
}

func TestPublicEntities_PublicApp(t *testing.T) {
	b := setupMetaTestRouter(t)
	b.SetApps(map[string]*formspec_app.ResolvedApp{
		"storefront": {
			Name:    "storefront",
			Spec:    &spec.AppSpec{RootURL: "/", AppRenderer: "no-nav", Access: spec.AppAccessPublic, Modules: []string{"sales"}},
			Modules: map[string]bool{"sales": true},
		},
		"backoffice": {
			Name:    "backoffice",
			Spec:    &spec.AppSpec{RootURL: "/app", AppRenderer: "topnav", Access: spec.AppAccessPrivate, Modules: []string{"sales"}},
			Modules: map[string]bool{"sales": true},
		},
	})

	if !b.isPublicEntity("sales", "product") {
		t.Error("expected sales/product to be public (mounted by public App)")
	}
	if b.isPublicEntity("hr", "employee") {
		t.Error("expected hr/employee to NOT be public (not in public App modules)")
	}
}

func TestPublicEntities_NoPublicApp(t *testing.T) {
	b := setupMetaTestRouter(t)
	// Only private Apps (any renderer) — nothing is public.
	b.SetApps(map[string]*formspec_app.ResolvedApp{
		"backoffice": {
			Name:    "backoffice",
			Spec:    &spec.AppSpec{RootURL: "/app", AppRenderer: "no-nav", Access: spec.AppAccessPrivate, Modules: []string{"sales"}},
			Modules: map[string]bool{"sales": true},
		},
	})
	if b.isPublicEntity("sales", "product") {
		t.Error("expected sales/product to NOT be public with only private Apps")
	}
}
