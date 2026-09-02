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
	"github.com/primadi/formspec/internal/manifest"
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

func TestHandleMetaUI_SettingsInBundle(t *testing.T) {
	b := setupMetaTestRouter(t)
	// Declare global settings (spec §10) — resolved with defaults by the
	// caller (resource/formspec.go) and shipped on the bundle.
	b.SetSettings(spec.ResolveSettings(&spec.Settings{
		Currency: &spec.CurrencySettings{Code: "IDR", DecimalPlaces: intPtr(0), Symbol: "Rp"},
		Locale:   "id-ID",
	}))
	handler := b.HandleMetaUI()

	identity := &auth.Identity{UserID: "user-1", WorkspaceID: "demo", Permissions: []string{}}
	req := httptest.NewRequest("GET", "/demo/_ui/_meta/ui?app=storefront", nil)
	req = req.WithContext(WithIdentity(context.Background(), identity))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
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
	if bundle.Settings == nil {
		t.Fatal("expected settings in bundle, got nil")
	}
	if bundle.Settings.Currency == nil || bundle.Settings.Currency.Code != "IDR" {
		t.Errorf("expected currency IDR, got %+v", bundle.Settings.Currency)
	}
	if bundle.Settings.Locale != "id-ID" {
		t.Errorf("expected locale id-ID, got %q", bundle.Settings.Locale)
	}
	// Unset fields fall back to standard defaults.
	if bundle.Settings.DateFormat != "YYYY-MM-DD" {
		t.Errorf("expected default date_format YYYY-MM-DD, got %q", bundle.Settings.DateFormat)
	}
}

func TestResolveSettings_Defaults(t *testing.T) {
	d := spec.ResolveSettings(nil)
	if d.Currency == nil || d.Currency.Code != "USD" {
		t.Errorf("expected default currency USD, got %+v", d.Currency)
	}
	if d.Locale != "en-US" {
		t.Errorf("expected default locale en-US, got %q", d.Locale)
	}
	if d.DateFormat != "YYYY-MM-DD" {
		t.Errorf("expected default date_format YYYY-MM-DD, got %q", d.DateFormat)
	}
	if d.DecimalScale != 2 {
		t.Errorf("expected default decimal_scale 2, got %d", d.DecimalScale)
	}
	if d.Rounding != "half_even" {
		t.Errorf("expected default rounding half_even, got %q", d.Rounding)
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

// TestHandleMetaUI_PrivateApp_PublicPageShipsToAnonymous verifies that a page
// explicitly marked `public: true` inside a PRIVATE App is served to
// anonymous callers (auth redesign Fase 3 — App access is the default, pages
// override per-page). Pages without permissions already ship to anonymous;
// this pins the behavior so the frontend can render them without redirecting
// to login.
func TestHandleMetaUI_PrivateApp_PublicPageShipsToAnonymous(t *testing.T) {
	b := setupMetaTestRouter(t)
	b.SetApps(map[string]*formspec_app.ResolvedApp{
		"backoffice": {
			Name:    "backoffice",
			Spec:    &spec.AppSpec{RootURL: "/app", AppRenderer: "topnav", Access: spec.AppAccessPrivate, Modules: []string{"sales"}},
			Modules: map[string]bool{"sales": true},
		},
	})
	// Register a public page (no permissions) + a private page (permissions).
	uiReg := ui.NewRegistry()
	uiReg.Load([]manifest.RawManifest{
		{
			Kind:     "Page",
			Metadata: manifest.RawMetadata{Name: "landing", Module: "sales"},
			Spec: map[string]any{
				"route":  "/landing",
				"public": true,
				"blocks": []any{},
			},
		},
		{
			Kind:     "Page",
			Metadata: manifest.RawMetadata{Name: "orders", Module: "sales"},
			Spec: map[string]any{
				"route":       "/orders",
				"permissions": []any{"sales.orders.list"},
				"blocks":      []any{},
			},
		},
	})
	b.SetUIRegistry(uiReg)

	handler := b.HandleMetaUI()
	// Anonymous caller (no identity in context).
	req := httptest.NewRequest("GET", "/demo/_ui/_meta/ui?app=backoffice", nil)
	req = req.WithContext(context.Background())
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200 for anonymous private-App bundle, got %d: %s", rec.Code, rec.Body.String())
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

	// The public page ships to anonymous; the permission-gated page does not.
	foundLanding := false
	for _, p := range bundle.Pages {
		if p.Name == "landing" {
			foundLanding = true
		}
		if p.Name == "orders" {
			t.Error("expected permission-gated page 'orders' to be filtered for anonymous caller")
		}
	}
	if !foundLanding {
		t.Error("expected public page 'landing' to ship to anonymous caller in a private App")
	}
}

// intPtr returns a pointer to the given int (for *int fields).
func intPtr(v int) *int { return &v }

func TestHandleMetaUI_GrantsMode_RequiresRoleManagePermission(t *testing.T) {
	b := setupMetaTestRouter(t)
	handler := b.HandleMetaUI()

	// Caller with no role-management permission → 403.
	identity := &auth.Identity{UserID: "user-1", WorkspaceID: "demo", Permissions: []string{"sales.orders.list"}}
	req := httptest.NewRequest("GET", "/demo/_ui/_meta/ui?app=storefront&grants=true", nil)
	req = req.WithContext(WithIdentity(context.Background(), identity))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 403 {
		t.Fatalf("expected 403 without role-manage permission, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleMetaUI_GrantsMode_AllowedWithRoleManagePermission(t *testing.T) {
	b := setupMetaTestRouter(t)
	handler := b.HandleMetaUI()

	// Caller with formspec.core.roles.update → 200, app-scoped bundle.
	identity := &auth.Identity{UserID: "user-1", WorkspaceID: "demo", Permissions: []string{"formspec.core.roles.update"}}
	req := httptest.NewRequest("GET", "/demo/_ui/_meta/ui?app=storefront&grants=true", nil)
	req = req.WithContext(WithIdentity(context.Background(), identity))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200 with role-manage permission, got %d: %s", rec.Code, rec.Body.String())
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
	// Grants mode stays app-scoped (role management is per-App).
	if bundle.App.Name != "storefront" {
		t.Errorf("expected bundle scoped to storefront, got %q", bundle.App.Name)
	}
}
