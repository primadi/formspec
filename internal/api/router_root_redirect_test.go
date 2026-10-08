package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	formspec_app "github.com/primadi/formspec/internal/app"
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/ui"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// TestRootRedirect_DevOnlyShape covers the dev-only `GET /` redirect that keeps
// a bare host name from answering chi's plain-text "404 page not found".
//
// Two properties are load-bearing and both are easy to lose:
//   - the target must be the SLASHED workspace root ("/demo/"), because the
//     redirect lands the visitor on a mount rather than on a path that no
//     route matches;
//   - nothing else about the root may change — a non-GET, an existing route, a
//     disabled redirect, or a missing SPA must all behave as before.
func TestRootRedirect_DevOnlyShape(t *testing.T) {
	b := setupSPARouter(t, "/")
	b.SetRootRedirect("/demo/")
	h := b.BuildHTTP()

	t.Run("GET / redirects to the workspace root", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusFound {
			t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusFound, rec.Body.String())
		}
		if loc := rec.Header().Get("Location"); loc != "/demo/" {
			t.Errorf("Location = %q, want %q", loc, "/demo/")
		}
	})

	t.Run("the redirect target actually serves the SPA", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/demo/", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "spa") {
			t.Errorf("target /demo/ = %d %q, want 200 with the shell", rec.Code, rec.Body.String())
		}
	})

	t.Run("only GET is redirected", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code == http.StatusFound {
			t.Errorf("POST / redirected; the root redirect must not swallow other methods")
		}
	})
}

// TestRootRedirect_DisabledByDefault pins the production-safe default: without
// an explicit SetRootRedirect the root keeps its old behavior (no redirect),
// and with no SPA at all the redirect is never registered — it would only lead
// to a JSON 404 one hop later.
func TestRootRedirect_DisabledByDefault(t *testing.T) {
	b := setupSPARouter(t, "/")
	h := b.BuildHTTP()
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code == http.StatusFound {
		t.Fatalf("GET / redirected without SetRootRedirect (Location: %q)", rec.Header().Get("Location"))
	}

	// Same builder shape, but the redirect is asked for while no SPA is served.
	noSPA := NewRouterBuilder(mustSQLiteRegistry(t))
	noSPA.SetUIRegistry(ui.NewRegistry())
	noSPA.SetRootRedirect("/demo/")
	rec2 := httptest.NewRecorder()
	noSPA.BuildHTTP().ServeHTTP(rec2, httptest.NewRequest("GET", "/", nil))
	if rec2.Code == http.StatusFound {
		t.Errorf("GET / redirected although no SPA is served (Location: %q)", rec2.Header().Get("Location"))
	}
}

func mustSQLiteRegistry(t *testing.T) *entity.Registry {
	t.Helper()
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "router.db"), nil)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return entity.NewRegistry(d, db.DriverSQLite, dir)
}

// TestAppMountPaths covers the list banners build URLs from. Two properties
// matter: the fixed workspace-level mounts (_admin, /app) must be absent — they
// own no App, and /_admin now holds only framework auth routes, so advertising
// it is what made the registry banner point at a "Page not found" (plan
// app-scoped-login.md D4) — and the list must be deterministic (sorted, deduped)
// so banners do not shuffle between builds.
func TestAppMountPaths(t *testing.T) {
	t.Run("single free-form mount", func(t *testing.T) {
		got := setupSPARouter(t, "/barbershop").AppMountPaths()
		if len(got) != 1 || got[0] != "/barbershop" {
			t.Fatalf("AppMountPaths() = %v, want [/barbershop]", got)
		}
	})

	t.Run("root App reports / not empty", func(t *testing.T) {
		got := setupSPARouter(t, "/").AppMountPaths()
		if len(got) != 1 || got[0] != "/" {
			t.Fatalf("AppMountPaths() = %v, want [/]", got)
		}
	})

	t.Run("sorted, deduped, no fixed mounts", func(t *testing.T) {
		b := NewRouterBuilder(mustSQLiteRegistry(t))
		b.SetUIRegistry(ui.NewRegistry())
		b.SetApps(map[string]*formspec_app.ResolvedApp{
			// Two Apps sharing a mount: a URL is built from the MOUNT, so
			// duplicates must collapse rather than be printed twice.
			"zeta":  {Name: "zeta", Spec: &spec.AppSpec{RootURL: "/zeta"}},
			"beta":  {Name: "beta", Spec: &spec.AppSpec{RootURL: "/zeta"}},
			"alpha": {Name: "alpha", Spec: &spec.AppSpec{RootURL: "/alpha"}},
			"stage": {Name: "stage", Spec: &spec.AppSpec{RootURL: "/stage"}},
		})
		got := b.AppMountPaths()
		want := []string{"/alpha", "/stage", "/zeta"}
		if len(got) != len(want) {
			t.Fatalf("AppMountPaths() = %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("AppMountPaths() = %v, want %v", got, want)
			}
		}
		for _, m := range got {
			if m == "/_admin" || m == "/app" {
				t.Errorf("AppMountPaths() leaked the fixed workspace-level mount %q", m)
			}
		}
	})
}
