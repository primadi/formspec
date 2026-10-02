package formspec

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/internal/manifest"
)

// A Service action that declares `public: true` must be callable by an
// ANONYMOUS caller on the UI surface (kafe 10.39).
//
// Why this is the crux of the feature rather than a detail: the guest check-in
// page needs server-side operations ("does this table have an open session",
// "verify the join code"), and neither may go through entity CRUD — an
// anonymous `list table-session` cannot be row-scoped (the guest has no token
// yet), and returning session rows would leak the join code. A Service has no
// App-level `public_entities` allowlist, so `public: true` on its action IS the
// allowlist.
//
// The route machinery already existed (`registerRouteWithPattern` pairs
// `rd.Public` with `RequirePermissionOrAnonymous`); the missing half was that
// `GenerateUIServiceRoutes` never set the flag, so every service route was
// session-only.
func TestServiceAction_PublicActionIsAnonymousCallable(t *testing.T) {
	app := bootPublicServiceApp(t)

	// ── The public action: anonymous must get THROUGH the gate ──
	status, out := doJSON(t, app, http.MethodPost,
		"/default/_ui/service/demo/gateway/open",
		map[string]any{"code": "A-01"})
	if status != http.StatusOK {
		t.Fatalf("a `public: true` service action must be anonymous-callable, got %d (%v)", status, out)
	}
	// The handler ran — proving the request reached the action, not merely that
	// auth let it through.
	data, _ := out["data"].(map[string]any)
	if got, _ := data["echo"].(string); got != "A-01" {
		t.Fatalf("the handler must have run with the posted params; got data=%v", out)
	}

	// ── The non-public action on the SAME service: anonymous must be refused ──
	//
	// This is the half that makes the flag meaningful. Without it, an
	// implementation that merely skipped the permission check for every service
	// route would pass the assertion above while quietly making every service
	// anonymously reachable.
	status, out = doJSON(t, app, http.MethodPost,
		"/default/_ui/service/demo/gateway/internal-only",
		map[string]any{"code": "A-01"})
	if status != http.StatusUnauthorized && status != http.StatusForbidden {
		t.Fatalf("a service action WITHOUT `public` must refuse anonymous callers, got %d (%v)", status, out)
	}
}

// The public grant is a FLOOR, not an anonymous-only lane.
//
// A signed-in caller who does NOT hold the permission falls back to the grant and
// is ALLOWED — the same access a guest gets, no more. That is deliberate and
// measured: requiring the permission made signed-in callers strictly WORSE OFF
// than guests on the App's own surface (kafe: `menu-category` answered 200 for
// anonymous and 404 for the same request with a session), so signing in broke a
// flow that had just worked.
//
// This test exists to pin that direction, because the conspicuous "safer" fix is
// the wrong one: making this a 403 would silently restore the inversion. The
// route comment used to claim the opposite ("a signed-in caller still needs the
// permission"), which is what made me write the wrong assertion first.
func TestServiceAction_PublicRouteDoesNotInvertSignedInCallers(t *testing.T) {
	app := bootPublicServiceApp(t)

	// A real user holding NO grant on this service. They do hold one harmless
	// permission elsewhere in the App: login is App-scoped and an App with zero
	// permissions is refused at the door (plan app-scoped-login.md D6), so
	// "no grant on this service" is the strongest case that can still hold a
	// session.
	seedUser(t, app, "nobody", "pw123456", []string{"demo.invoice.list"})
	token := login(t, app, "nobody", "pw123456")

	status, out := doAuthed(t, app, http.MethodPost,
		"/default/_ui/service/demo/gateway/open", token, map[string]any{"code": "A-01"})
	if status == http.StatusForbidden || status == http.StatusUnauthorized {
		t.Fatalf("a signed-in caller must not be WORSE OFF than a guest on a public route (inversion), got %d (%v)", status, out)
	}
	if status != http.StatusOK {
		t.Fatalf("expected the signed-in caller to reach the action via the public grant, got %d (%v)", status, out)
	}
}

// The declaration rules are enforced at deploy time rather than discovered in
// production. Both refusals are hard errors: each one, if allowed through, is an
// open door rather than an inconvenience.
func TestServiceAction_PublicDeclarationIsValidated(t *testing.T) {
	loader := manifest.NewLoader(t.TempDir())

	refuse := func(t *testing.T, yamlText, wantSubstring string) {
		t.Helper()
		raw := loadRawManifest(t, loader, yamlText)
		err := loader.Validate(raw)
		if err == nil {
			t.Fatalf("expected validation to refuse this declaration")
		}
		if !contains(err.Error(), wantSubstring) {
			t.Errorf("the error must mention %q so the author knows what to change, got: %v", wantSubstring, err)
		}
	}

	t.Run("public without rate_limit is refused", func(t *testing.T) {
		refuse(t, `apiVersion: formspec.dev/v1
kind: Service
metadata: { name: gateway, module: demo }
spec:
  version: v1
  actions:
    - name: open
      public: true
      impl: { type: script_ref, ref: demo/echo }
`, "rate_limit")
	})

	t.Run("public on an entity action is refused", func(t *testing.T) {
		refuse(t, `apiVersion: formspec.dev/v1
kind: Entity
metadata: { name: invoice, module: demo }
spec:
  version: v1
  characteristic: master
  fields:
    - { name: code, type: string, natural_key: true }
  actions:
    - name: list
      public: true
      rate_limit: { max: 10, per: 1m, scope: ip }
      impl: { type: script_ref, ref: demo/echo }
`, "public_entities")
	})

	t.Run("well-formed public action is accepted", func(t *testing.T) {
		// The rules must not blanket-refuse the feature they exist to protect.
		raw := loadRawManifest(t, loader, `apiVersion: formspec.dev/v1
kind: Service
metadata: { name: gateway, module: demo }
spec:
  version: v1
  actions:
    - name: open
      public: true
      rate_limit: { max: 10, per: 1m, scope: ip }
      impl: { type: script_ref, ref: demo/echo }
`)
		if err := loader.Validate(raw); err != nil {
			t.Fatalf("a public action WITH a rate limit must be accepted, got: %v", err)
		}
	})
}

// ── helpers ──

// bootPublicServiceApp boots a minimal app whose only Service declares one
// public action and one internal action, so the pair can be compared.
func bootPublicServiceApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	writePublicServiceSpec(t, dir)

	app, err := New(Config{
		SpecPath:    dir,
		DSN:         "sqlite:" + filepath.Join(t.TempDir(), "public-svc.db"),
		WorkspaceID: "default",
	})
	if err != nil {
		t.Fatalf("boot app: %v", err)
	}
	t.Cleanup(func() { _ = app.Close(context.Background()) })
	app.StartBackgroundWorkers()
	return app
}

func writePublicServiceSpec(t *testing.T, dir string) {
	t.Helper()
	write := func(rel, content string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Login is per-App (plan app-scoped-login.md D1) — the fixture needs an App
	// to scope test logins to.
	write("apps/demo.yaml", `apiVersion: formspec.dev/v1
kind: App
metadata:
  name: demo-app
spec:
  version: 1.0.0
  root_url: /app/demo
  modules: [demo]
`)

	write("modules/demo/module.yaml", `apiVersion: formspec.dev/v1
kind: Module
metadata: { name: demo }
spec: {}
`)

	// `open` is the anonymous entry point; `internal-only` is the control that
	// must stay gated. Same service, same handler — only the flag differs.
	write("modules/demo/services/gateway.yaml", `apiVersion: formspec.dev/v1
kind: Service
metadata: { name: gateway, module: demo }
spec:
  version: v1
  actions:
    - name: open
      public: true
      rate_limit: { max: 20, per: 1m, scope: ip }
      impl: { type: script_ref, ref: demo/echo }
    - name: internal-only
      impl: { type: script_ref, ref: demo/echo }
`)

	write("modules/demo/scripts/echo.star", `def execute(resource, params, ctx):
    return ok({"echo": params.get("code")})
`)
}

// loadRawManifest parses one YAML document into a RawManifest, the same way the
// loader does when it walks a spec directory.
func loadRawManifest(t *testing.T, l *manifest.Loader, yamlText string) manifest.RawManifest {
	t.Helper()
	raws, errs := l.ParseBytes([]byte(yamlText), "inline.yaml")
	if len(errs) > 0 {
		t.Fatalf("parse manifest: %v", errs[0])
	}
	if len(raws) != 1 {
		t.Fatalf("expected exactly 1 manifest, got %d", len(raws))
	}
	return raws[0]
}
