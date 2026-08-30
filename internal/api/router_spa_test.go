package api

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	formspec_app "github.com/primadi/formspec/internal/app"
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/ui"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// spaTestFS is a minimal SPA bundle: index.html plus one asset.
func spaTestFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":    {Data: []byte("<html>spa</html>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}
}

// setupSPARouter builds a RouterBuilder with an embedded SPA and one App
// mounted at a free-form root_url (docs/plan/flexible-root-url.md).
func setupSPARouter(t *testing.T, rootURL string) *RouterBuilder {
	t.Helper()
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "spa_test.db"), nil)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	reg := entity.NewRegistry(d, db.DriverSQLite, dir)
	b := NewRouterBuilder(reg)
	b.SetUIRegistry(ui.NewRegistry())
	b.SetApps(map[string]*formspec_app.ResolvedApp{
		"barbershop": {
			Name:    "barbershop",
			Spec:    &spec.AppSpec{RootURL: rootURL, Modules: []string{"shop"}},
			Modules: map[string]bool{"shop": true},
		},
	})
	b.SetWebFS(spaTestFS())
	return b
}

func TestSPAMounts_FlexibleRootURL(t *testing.T) {
	b := setupSPARouter(t, "/barbershop")
	h := b.BuildHTTP()

	cases := []struct {
		path       string
		wantStatus int
		wantBody   string
	}{
		// The App's mount serves the SPA shell (index.html fallback).
		{"/demo/barbershop", 200, "spa"},
		{"/demo/barbershop/login", 200, "spa"},
		{"/demo/barbershop/orders/42", 200, "spa"},
		// Legacy mounts still work.
		{"/demo/app", 200, "spa"},
		{"/demo/_admin", 200, "spa"},
		// Unrelated workspace paths stay JSON 404 (no root App).
		{"/demo/unknown", 404, "NOT_FOUND"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest("GET", tc.path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tc.wantStatus {
			t.Errorf("%s: status %d, want %d (body: %s)", tc.path, rec.Code, tc.wantStatus, rec.Body.String())
			continue
		}
		if tc.wantBody != "" && !strings.Contains(rec.Body.String(), tc.wantBody) {
			t.Errorf("%s: body %q, want containing %q", tc.path, rec.Body.String(), tc.wantBody)
		}
	}
}

func TestSPAMounts_WorkspaceRoot(t *testing.T) {
	b := setupSPARouter(t, "/")
	h := b.BuildHTTP()

	cases := []struct {
		path       string
		wantStatus int
		wantBody   string
	}{
		// A root App owns the whole workspace subtree.
		{"/demo/", 200, "spa"},
		{"/demo/barbershop/orders", 200, "spa"},
		// Fixed surfaces still win over the root splat.
		{"/demo/_admin", 200, "spa"},
		{"/demo/app", 200, "spa"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest("GET", tc.path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tc.wantStatus {
			t.Errorf("%s: status %d, want %d (body: %s)", tc.path, rec.Code, tc.wantStatus, rec.Body.String())
			continue
		}
		if tc.wantBody != "" && !strings.Contains(rec.Body.String(), tc.wantBody) {
			t.Errorf("%s: body %q, want containing %q", tc.path, rec.Body.String(), tc.wantBody)
		}
	}
}
