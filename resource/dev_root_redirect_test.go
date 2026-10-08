package formspec

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/primadi/formspec/internal/api"
)

// testSPAFS is a minimal SPA bundle. The dev root redirect is gated on an SPA
// being served — without one it would only lead to a JSON 404 one hop later —
// so a fixture that wants the redirect must wire one.
func testSPAFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":    {Data: []byte("<html>spa</html>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}
}

// newRootProbeApp boots a minimal app whose single App is mounted at a
// free-form root_url, so the dev root redirect has something unambiguous to
// resolve.
func newRootProbeApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	buildAuthSpecDir(t, dir) // declares App acme-app with root_url /app/acme
	api.ResetAuthRateLimiters()
	app, err := New(Config{
		SpecPath:  dir,
		DSN:       "sqlite:" + filepath.Join(t.TempDir(), "root_redirect.db"),
		JWTSecret: "test-secret",
		WebFS:     testSPAFS(),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = app.Close(context.Background()) })
	return app
}

// rootLocation issues GET / and returns the redirect target ("" when the
// request was not redirected).
func rootLocation(t *testing.T, app *App, method string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(method, "/", nil)
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)
	return rec.Code, rec.Header().Get("Location")
}

// TestDevRootRedirect_ResolvesFromAppMount pins the development affordance that
// keeps a bare host name from answering chi's plain-text "404 page not found".
//
// Everything the engine serves is workspace-prefixed (D50), and the SPA itself
// assumes "/" is a shell URL (App.tsx navigates "/" to "/{workspace}"), so the
// two disagree unless the server bridges them. The target must come from the
// App's root_url — a hardcoded workspace root would land the visitor somewhere
// that is not the App's surface.
func TestDevRootRedirect_ResolvesFromAppMount(t *testing.T) {
	app := newRootProbeApp(t)

	code, loc := rootLocation(t, app, http.MethodGet)
	if code != http.StatusFound {
		t.Fatalf("GET / = %d, want %d (body not redirected)", code, http.StatusFound)
	}
	// buildAuthSpecDir mounts the App at /app/acme; the surface URL keeps the
	// workspace prefix and does NOT grow a trailing slash for a non-root mount.
	if want := "/default/app/acme"; loc != want {
		t.Errorf("Location = %q, want %q", loc, want)
	}

	// The redirect must not invent a route: the target is a real SPA mount.
	req := httptest.NewRequest(http.MethodGet, loc, nil)
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("redirect target %s = %d, want 200", loc, rec.Code)
	}
}

// TestDevRootRedirect_NotInProdMode guards the deployment boundary. In
// production the root segment belongs to the edge (a subdomain per workspace,
// ingress rules); a redirect here would silently rewrite that contract for
// every deployment, and nobody would see it in the manifest.
func TestDevRootRedirect_NotInProdMode(t *testing.T) {
	dir := t.TempDir()
	buildAuthSpecDir(t, dir)
	app, err := New(Config{
		SpecPath:  dir,
		DSN:       "sqlite:" + filepath.Join(t.TempDir(), "root_redirect_prod.db"),
		JWTSecret: "test-secret",
		ProdMode:  true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = app.Close(context.Background()) }()

	if code, loc := rootLocation(t, app, http.MethodGet); code == http.StatusFound {
		t.Errorf("GET / redirected in ProdMode (Location: %q); the gate is dev-only", loc)
	}
}

// TestDevRootRedirect_SurvivesReload is the regression guard for the wiring
// class this codebase keeps re-learning: ReloadSpec builds a FRESH
// RouterBuilder, so every wireAppSurfaces item silently starts empty. A lost
// redirect does not fail loudly — it just puts the 404 back in the address bar.
func TestDevRootRedirect_SurvivesReload(t *testing.T) {
	app := newRootProbeApp(t)

	code, before := rootLocation(t, app, http.MethodGet)
	if code != http.StatusFound {
		t.Fatalf("GET / before reload = %d, want %d", code, http.StatusFound)
	}

	if err := app.ReloadSpec(); err != nil {
		t.Fatalf("ReloadSpec: %v", err)
	}

	code, after := rootLocation(t, app, http.MethodGet)
	if code != http.StatusFound {
		t.Fatalf("GET / after reload = %d, want %d (redirect lost on reload)", code, http.StatusFound)
	}
	if after != before {
		t.Errorf("redirect target changed across reload: %q → %q", before, after)
	}
}

// TestUIAppURLs_Shape pins what the startup banners print. The workspace slug
// is configurable and the surface belongs to the App, so a banner may not
// hardcode either — and it must not advertise /{ws}/_admin, whose entity panel
// was retired (plan app-scoped-login.md D4).
func TestUIAppURLs_Shape(t *testing.T) {
	app := newRootProbeApp(t)

	got := app.UIAppURLs()
	if len(got) != 1 || got[0] != "/default/app/acme" {
		t.Fatalf("UIAppURLs() = %v, want [/default/app/acme]", got)
	}
	for _, u := range got {
		if strings.Contains(u, "_admin") {
			t.Errorf("UIAppURLs() advertised the retired /_admin surface: %q", u)
		}
		if !strings.HasPrefix(u, "/default/") {
			t.Errorf("UIAppURLs() omitted the workspace prefix: %q", u)
		}
	}
}
