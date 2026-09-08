package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"testing/fstest"

	formspec_app "github.com/primadi/formspec/internal/app"
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/ui"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// wsPtr builds a *[]string allowlist (test helper for AppSpec.Workspaces).
func wsPtr(slugs ...string) *[]string {
	s := slugs
	return &s
}

// setupWorkspaceScopeRouter builds a RouterBuilder with two Apps:
// "cafe-app" mounted for workspace cafe only, and "open-app" with no
// allowlist (all workspaces). Embedded SPA so mounts are live.
func setupWorkspaceScopeRouter(t *testing.T) *RouterBuilder {
	t.Helper()
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "wsscope_test.db"), nil)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	reg := entity.NewRegistry(d, db.DriverSQLite, dir)
	b := NewRouterBuilder(reg)
	b.SetUIRegistry(ui.NewRegistry())
	cafeOnly := wsPtr("cafe")
	staged := wsPtr()
	b.SetApps(map[string]*formspec_app.ResolvedApp{
		"cafe-app": {
			Name: "cafe-app",
			Spec: &spec.AppSpec{
				RootURL:    "/app/cafe",
				Modules:    []string{"sales"},
				Workspaces: cafeOnly,
				Version:    "1.2.0",
				Vendor:     "acme",
			},
			Modules: map[string]bool{"sales": true},
		},
		"staged-app": {
			Name: "staged-app",
			Spec: &spec.AppSpec{
				RootURL:    "/app/staged",
				Modules:    []string{"sales"},
				Workspaces: staged,
			},
			Modules: map[string]bool{"sales": true},
		},
		"open-app": {
			Name: "open-app",
			Spec: &spec.AppSpec{
				RootURL: "/app/open",
				Modules: []string{"sales"},
				Version: "1.2.0",
				Vendor:  "acme",
			},
			Modules: map[string]bool{"sales": true},
		},
	})
	b.SetWebFS(fstest.MapFS{"index.html": {Data: []byte("<html>spa</html>")}})
	return b
}

// TestHandleMetaApps_WorkspaceScope verifies the allowlist filter: only Apps
// mounted in the request's workspace are listed; the staged App is listed
// nowhere; version/vendor are exposed (AppVersion decision).
func TestHandleMetaApps_WorkspaceScope(t *testing.T) {
	b := setupWorkspaceScopeRouter(t)
	handler := b.HandleMetaApps()

	list := func(ws string) []appMetaSummary {
		req := httptest.NewRequest("GET", "/"+ws+"/_ui/_meta/apps", nil)
		req = req.WithContext(WithWorkspace(context.Background(), ws))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d", ws, rec.Code)
		}
		var resp SingleResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		data, _ := resp.Data.([]any)
		out := make([]appMetaSummary, 0, len(data))
		raw, _ := json.Marshal(resp.Data)
		_ = json.Unmarshal(raw, &out)
		return out
	}

	inCafe := list("cafe")
	if len(inCafe) != 2 {
		t.Fatalf("cafe: expected 2 apps (cafe-app + open-app; staged listed nowhere), got %d: %+v", len(inCafe), inCafe)
	}
	inDefault := list("default")
	if len(inDefault) != 1 || inDefault[0].Name != "open-app" {
		t.Fatalf("default: expected only open-app, got %+v", inDefault)
	}
	if inDefault[0].Version != "1.2.0" || inDefault[0].Vendor != "acme" {
		t.Fatalf("open-app version/vendor not exposed: %+v", inDefault[0])
	}
}

// TestResolveAppContext_WorkspaceScope verifies that an App outside the
// workspace allowlist is rejected with the same message as an unknown app
// (anti-enumeration).
func TestResolveAppContext_WorkspaceScope(t *testing.T) {
	b := setupWorkspaceScopeRouter(t)

	req := httptest.NewRequest("GET", "/default/_ui/_meta/ui?app=cafe-app", nil)
	req = req.WithContext(WithWorkspace(context.Background(), "default"))
	if _, errMsg := b.resolveAppContext(req); errMsg != "unknown app cafe-app" {
		t.Fatalf("expected 'unknown app cafe-app', got %q", errMsg)
	}

	req = httptest.NewRequest("GET", "/cafe/_ui/_meta/ui?app=cafe-app", nil)
	req = req.WithContext(WithWorkspace(context.Background(), "cafe"))
	if _, errMsg := b.resolveAppContext(req); errMsg != "" {
		t.Fatalf("expected allowed in cafe, got %q", errMsg)
	}

	req = httptest.NewRequest("GET", "/cafe/_ui/_meta/ui?app=staged-app", nil)
	req = req.WithContext(WithWorkspace(context.Background(), "cafe"))
	if _, errMsg := b.resolveAppContext(req); errMsg != "unknown app staged-app" {
		t.Fatalf("staged app must resolve nowhere, got %q", errMsg)
	}
}

// TestSPAMounts_WorkspaceScope verifies the SPA mount enforcement: an App
// outside the workspace allowlist serves 404, not the SPA shell.
func TestSPAMounts_WorkspaceScope(t *testing.T) {
	b := setupWorkspaceScopeRouter(t)
	h := b.BuildHTTP()

	cases := []struct {
		path string
		want int
	}{
		{"/cafe/app/cafe", 200},
		{"/cafe/app/cafe/orders/42", 200},
		{"/cafe/app/open", 200},    // open-app: no allowlist
		{"/default/app/cafe", 404}, // cafe-app not mounted here
		{"/default/app/open", 200},
		{"/default/app/staged", 404}, // staged = mounted nowhere
	}
	for _, tc := range cases {
		req := httptest.NewRequest("GET", tc.path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("%s: status %d, want %d", tc.path, rec.Code, tc.want)
		}
	}
}
