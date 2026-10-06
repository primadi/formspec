package formspec

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/internal/api"
	"github.com/primadi/formspec/internal/auth"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// kafe 10.74: a script READ must obey the caller's row boundary.
//
// The HTTP layer was taught to enforce a role grant's `row_scope` (10.67), and
// the store is where that enforcement lives. But a Starlark action reaches the
// same rows through `resource.find()` / `resource.fetch()`, and those handlers
// called the store WITHOUT the caller's predicates. The result was two different
// databases for the same caller: `GET .../widget/W-1` answered 404 while a guard
// script in the same request matched that row and read its fields.
//
// That is not a smaller version of the boundary — it is the boundary gone, and it
// is invisible because the script legitimately returns 200. So the assertion is
// the CONTENT the script saw: a script asked to find the hidden row must report
// "not found", exactly as the HTTP read does.

func buildScriptRowScopeSpecDir(t *testing.T, dir string) {
	t.Helper()
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("apps/acme.yaml", `apiVersion: formspec.dev/v1
kind: App
metadata:
  name: acme-app
spec:
  version: 1.0.0
  root_url: /app/acme
  modules: [acme]
`)

	write("modules/acme/module.yaml", `apiVersion: formspec.dev/v1
kind: Module
metadata: { name: acme }
spec: {}
`)

	// `status` is the field the grant restricts on, and it exists on the entity —
	// a row scope names a COLUMN of the row it limits, so a restriction on an
	// undeclared field is refused rather than silently ignored.
	write("modules/acme/master/widget.yaml", `apiVersion: formspec.dev/v1
kind: Entity
metadata: { name: widget, module: acme }
spec:
  version: v1
  characteristic: master
  fields:
    - { name: code, type: string, natural_key: true, rules: [required] }
    - { name: status, type: string, rules: [required] }
    - { name: secret, type: string }
`)

	// The runner carries the SCRIPT actions; the script is what reads the widget.
	write("modules/acme/master/runner.yaml", `apiVersion: formspec.dev/v1
kind: Entity
metadata: { name: runner, module: acme }
spec:
  version: v1
  characteristic: master
  fields:
    - { name: code, type: string, natural_key: true, rules: [required] }
  actions:
    - name: peek-widget
      impl: { type: script_ref, ref: acme/peek_widget }
    - name: fetch-widget
      impl: { type: script_ref, ref: acme/fetch_widget }
`)

	// `resource.find()` — the read the ledger measured.
	write("modules/acme/scripts/peek_widget.star", `def execute(resource, params, ctx):
    w = resource.find("acme.widget", {"code": params["code"]})
    if w == None:
        return ok({"found": False})
    return ok({"found": True, "secret": w.field.secret})
`)

	// `resource.fetch()` — the id-addressed read, which must take the same
	// decision so a script cannot pick the helper that leaks. It reports a row it
	// cannot reach as an ERROR (its long-standing "not found" answer), and the
	// message is identical for a row that never existed and one outside the
	// boundary — so it is not an existence oracle either.
	write("modules/acme/scripts/fetch_widget.star", `def execute(resource, params, ctx):
    w = resource.fetch("acme.widget", params["id"])
    if w == None:
        return ok({"found": False})
    return ok({"found": True, "secret": w.field.secret})
`)
}

func bootScriptRowScopeApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	buildScriptRowScopeSpecDir(t, dir)
	api.ResetAuthRateLimiters()

	app, err := New(Config{
		SpecPath:  dir,
		DSN:       "sqlite:" + filepath.Join(t.TempDir(), "script_rowscope.db"),
		ProdMode:  true,
		JWTSecret: "test-secret",
	})
	if err != nil {
		t.Fatalf("boot app: %v", err)
	}
	t.Cleanup(func() { _ = app.Close(context.Background()) })
	app.StartBackgroundWorkers()
	return app
}

// seedScopedRole creates a role whose `view` grant carries a row scope, exactly
// the shape kafe's seed uses (a literal status constant).
func seedScopedRole(t *testing.T, app *App, name, status string) {
	t.Helper()
	store, err := app.Registry().GetEntityStore("formspec.core", "role")
	if err != nil {
		t.Fatalf("role store: %v", err)
	}
	if _, err := store.Insert(context.Background(), db.InsertParams{
		WorkspaceID: "default", CreatedBy: "test", SystemCaller: true,
		Data: map[string]any{
			"name": name, "app": "", "description": "script row-scope test role",
			"grants": []map[string]any{{
				"page": "widget-page",
				"actions": []map[string]any{
					{"name": "list"},
					{"name": "view", "row_scope": []map[string]any{
						{"field": "status", "op": "eq", "value": status},
					}},
				},
			}},
		},
	}); err != nil {
		t.Fatalf("insert role %s: %v", name, err)
	}
}

// seedRoleUser creates an account holding one role, with the runner permissions
// granted directly (they are not what this test is about).
func seedRoleUser(t *testing.T, app *App, username, role, password string) {
	t.Helper()
	store, err := app.Registry().GetEntityStore("formspec.core", "user")
	if err != nil {
		t.Fatalf("user store: %v", err)
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if _, err := store.Insert(context.Background(), db.InsertParams{
		WorkspaceID: "default", CreatedBy: "test", SystemCaller: true,
		Data: map[string]any{
			"username": username, "password_hash": hash, "active": true,
			"roles": []string{role},
			"permissions": []string{
				"acme.runners.list", "acme.runners.view",
				"acme.runners.peek-widget", "acme.runners.fetch-widget",
			},
		},
	}); err != nil {
		t.Fatalf("insert user %s: %v", username, err)
	}
}

func TestScriptReadObeysGrantRowScope(t *testing.T) {
	app := bootScriptRowScopeApp(t)

	seedUser(t, app, "admin", "admin", []string{"*"})
	seedScopedRole(t, app, "scoped", "published")
	seedRoleUser(t, app, "scoped", "scoped", "scoped")

	adminTok := login(t, app, "admin", "admin")
	scopedTok := login(t, app, "scoped", "scoped")

	// Two widgets: one inside the caller's boundary, one outside it.
	seed := func(code, status, secret string) string {
		t.Helper()
		status2, out := doAuthed(t, app, http.MethodPost, "/default/_ui/entity/acme/widget", adminTok, map[string]any{
			"code": code, "status": status, "secret": secret,
		})
		if status2 != http.StatusCreated {
			t.Fatalf("seed widget %s: %d (%v)", code, status2, out)
		}
		id, _ := out["data"].(map[string]any)["id"].(string)
		return id
	}
	publishedID := seed("W-PUB", "published", "public-secret")
	draftID := seed("W-DRAFT", "draft", "draft-secret")

	// The runner row the actions are applied to.
	status, out := doAuthed(t, app, http.MethodPost, "/default/_ui/entity/acme/runner", adminTok, map[string]any{
		"code": "R-1",
	})
	if status != http.StatusCreated {
		t.Fatalf("seed runner: %d (%v)", status, out)
	}
	runnerID, _ := out["data"].(map[string]any)["id"].(string)

	call := func(t *testing.T, token, action string, body map[string]any) map[string]any {
		t.Helper()
		st, resp := doAuthed(t, app, http.MethodPost,
			"/default/_ui/entity/acme/runner/"+runnerID+"/"+action, token, body)
		if st != http.StatusOK {
			t.Fatalf("%s: status %d (%v)", action, st, resp)
		}
		data, _ := resp["data"].(map[string]any)
		return data
	}

	// Control: the HTTP read refuses the draft. This is what the script must
	// agree with — if this ever returns 200, the script assertion below would be
	// comparing against a broken baseline.
	if st, _ := doAuthed(t, app, http.MethodGet,
		"/default/_ui/entity/acme/widget/"+draftID, scopedTok, nil); st != http.StatusNotFound {
		t.Fatalf("baseline: GET of an out-of-scope widget = %d, want 404", st)
	}

	t.Run("find: the row outside the boundary reads as absent", func(t *testing.T) {
		got := call(t, scopedTok, "peek-widget", map[string]any{"code": "W-DRAFT"})
		if got["found"] != false {
			t.Fatalf("resource.find() matched a row the same caller cannot GET: %v", got)
		}
		if _, leaked := got["secret"]; leaked {
			t.Fatalf("the script read a field of a row outside the boundary: %v", got)
		}
	})

	t.Run("find: the row inside the boundary is still readable", func(t *testing.T) {
		got := call(t, scopedTok, "peek-widget", map[string]any{"code": "W-PUB"})
		if got["found"] != true {
			t.Fatalf("the script lost its own row: %v", got)
		}
		if got["secret"] != "public-secret" {
			t.Fatalf("script read the wrong row: %v", got)
		}
	})

	t.Run("fetch: the id-addressed read takes the same decision", func(t *testing.T) {
		// A row outside the boundary is reported as not found — the same answer
		// this helper already gave for an id that never existed, so the script
		// learns nothing about which rows exist beyond its scope.
		st, resp := doAuthed(t, app, http.MethodPost,
			"/default/_ui/entity/acme/runner/"+runnerID+"/fetch-widget", scopedTok,
			map[string]any{"id": draftID})
		if st == http.StatusOK {
			t.Fatalf("resource.fetch() reached a row the same caller cannot GET: %v", resp)
		}

		got := call(t, scopedTok, "fetch-widget", map[string]any{"id": publishedID})
		if got["found"] != true || got["secret"] != "public-secret" {
			t.Fatalf("resource.fetch() lost the in-scope row: %v", got)
		}
	})

	t.Run("an unscoped caller sees both", func(t *testing.T) {
		got := call(t, adminTok, "peek-widget", map[string]any{"code": "W-DRAFT"})
		if got["found"] != true || got["secret"] != "draft-secret" {
			t.Fatalf("the wildcard admin must still read every row: %v", got)
		}
	})
}
