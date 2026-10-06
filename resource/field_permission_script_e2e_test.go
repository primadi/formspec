package formspec

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/internal/api"
)

// kafe 10.71: the field-level guard of 05-field-types.md §5.3 must hold on the
// SCRIPT path too, not only over HTTP.
//
// A guard placed on one write path leaves the other open — the shape of bug
// behind 10.46 (transition gates) and 10.71 itself. The store is the one choke
// point both paths share, so the guard lives there; this test proves the script
// path actually reaches it, end to end, with a real Starlark action.
//
// It also pins the fix to `resource.create`: that handler was the single script
// write path that passed NO caller identity (unlike `resource.save`), so a
// gated field would have been refused even for a caller who holds the
// permission — a false denial rather than a hole, but the same missing wiring.
func buildFieldPermScriptSpecDir(t *testing.T, dir string) {
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

	// `secret` is the gated field: readable AND writable only with the
	// permission (§5.3 guards both directions).
	write("modules/acme/master/widget.yaml", `apiVersion: formspec.dev/v1
kind: Entity
metadata: { name: widget, module: acme }
spec:
  version: v1
  characteristic: master
  fields:
    - { name: code, type: string, natural_key: true, rules: [required] }
    - { name: secret, type: string, required_permission: acme.widgets.secret.write }
`)

	// The runner carries the actions whose SCRIPTS do the writing.
	write("modules/acme/master/runner.yaml", `apiVersion: formspec.dev/v1
kind: Entity
metadata: { name: runner, module: acme }
spec:
  version: v1
  characteristic: master
  fields:
    - { name: code, type: string, natural_key: true, rules: [required] }
  actions:
    - name: set-secret-via-save
      impl: { type: script_ref, ref: acme/set_secret_via_save }
    - name: create-widget-with-secret
      impl: { type: script_ref, ref: acme/create_widget_with_secret }
`)

	// Loads an existing widget and re-saves it with a new secret → Update.
	write("modules/acme/scripts/set_secret_via_save.star", `def execute(resource, params, ctx):
    w = resource.find("acme.widget", {"code": params["code"]})
    w.set("secret", params["value"])
    w.save()
    return {"ok": True}
`)

	// Creates a widget carrying the gated field → Insert, through the handler
	// that used to pass no permissions at all.
	write("modules/acme/scripts/create_widget_with_secret.star", `def execute(resource, params, ctx):
    resource.create("acme.widget", {"code": params["code"], "secret": params["value"]})
    return {"ok": True}
`)
}

func bootFieldPermScriptApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	buildFieldPermScriptSpecDir(t, dir)
	api.ResetAuthRateLimiters()

	app, err := New(Config{
		SpecPath:  dir,
		DSN:       "sqlite:" + filepath.Join(t.TempDir(), "fieldperm_script.db"),
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

func TestScriptWriteEnforcesFieldPermission(t *testing.T) {
	app := bootFieldPermScriptApp(t)

	// `limited` may run the runner actions and read widgets, but does NOT hold
	// acme.widgets.secret.write.
	seedUser(t, app, "admin", "admin", []string{"*"})
	seedUser(t, app, "limited", "limited", []string{
		"acme.widgets.list", "acme.widgets.view",
		"acme.runners.list", "acme.runners.view",
		"acme.runners.set-secret-via-save", "acme.runners.create-widget-with-secret",
	})

	adminTok := login(t, app, "admin", "admin")
	limitedTok := login(t, app, "limited", "limited")

	// Admin seeds a widget the action can load.
	const code = "W-1"
	status, out := doAuthed(t, app, http.MethodPost, "/default/_ui/entity/acme/widget", adminTok, map[string]any{
		"code": code, "secret": "original",
	})
	if status != http.StatusCreated {
		t.Fatalf("admin seed widget: %d (%v)", status, out)
	}
	// The runner row the action is applied to.
	status, out = doAuthed(t, app, http.MethodPost, "/default/_ui/entity/acme/runner", adminTok, map[string]any{
		"code": "R-1",
	})
	if status != http.StatusCreated {
		t.Fatalf("admin seed runner: %d (%v)", status, out)
	}
	runnerID, _ := out["data"].(map[string]any)["id"].(string)

	call := func(t *testing.T, token, action string, body map[string]any) (int, map[string]any) {
		t.Helper()
		return doAuthed(t, app, http.MethodPost,
			"/default/_ui/entity/acme/runner/"+runnerID+"/"+action, token, body)
	}

	t.Run("save: a script without the field permission is refused", func(t *testing.T) {
		status, out := call(t, limitedTok, "set-secret-via-save",
			map[string]any{"code": code, "value": "hacked"})
		if status != http.StatusForbidden {
			t.Fatalf("script write of a gated field must be refused with 403, got %d (%v)", status, out)
		}
		// And the stored value must be untouched.
		if got := readWidgetSecret(t, app, adminTok, code); got != "original" {
			t.Errorf("secret = %q, want the original untouched", got)
		}
	})

	t.Run("save: a script WITH the field permission succeeds", func(t *testing.T) {
		status, out := call(t, adminTok, "set-secret-via-save",
			map[string]any{"code": code, "value": "changed-by-admin"})
		if status != http.StatusOK {
			t.Fatalf("a caller holding the permission must be allowed, got %d (%v)", status, out)
		}
		if got := readWidgetSecret(t, app, adminTok, code); got != "changed-by-admin" {
			t.Errorf("secret = %q, want the written value", got)
		}
	})

	t.Run("create: a script without the field permission is refused", func(t *testing.T) {
		status, out := call(t, limitedTok, "create-widget-with-secret",
			map[string]any{"code": "W-2", "value": "hacked"})
		if status != http.StatusForbidden {
			t.Fatalf("script create carrying a gated field must be refused with 403, got %d (%v)", status, out)
		}
	})

	t.Run("create: a script WITH the field permission succeeds", func(t *testing.T) {
		status, out := call(t, adminTok, "create-widget-with-secret",
			map[string]any{"code": "W-3", "value": "ok"})
		if status != http.StatusOK && status != http.StatusCreated {
			t.Fatalf("admin script create must succeed, got %d (%v)", status, out)
		}
		if got := readWidgetSecret(t, app, adminTok, "W-3"); got != "ok" {
			t.Errorf("secret = %q, want the written value", got)
		}
	})
}

// readWidgetSecret reads the gated field as an admin (who may see it). Fails when
// the field is absent, since "absent" cannot distinguish a refused write from a
// stripped response.
func readWidgetSecret(t *testing.T, app *App, adminTok, code string) string {
	t.Helper()
	status, out := doAuthed(t, app, http.MethodGet,
		"/default/_ui/entity/acme/widget/"+code, adminTok, nil)
	if status != http.StatusOK {
		t.Fatalf("admin read widget %s: %d (%v)", code, status, out)
	}
	data, _ := out["data"].(map[string]any)
	v, ok := data["secret"]
	if !ok {
		t.Fatalf("admin must see `secret` to judge the write; keys: %v", keysOf(data))
	}
	s, _ := v.(string)
	return s
}
