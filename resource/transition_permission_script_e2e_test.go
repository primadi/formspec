package formspec

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/internal/api"
)

// kafe 10.46: a transition gate must hold on the SCRIPT path too.
//
// The gate used to live only in the HTTP handler, so `resource.save()` from a
// Starlark action crossed ANY transition regardless of the caller's
// permissions — the same class of hole as 10.71 (field-level guard), and the
// reason both now live in the store.
//
// This drives a real action script that re-saves a record into a gated state,
// as a caller who does NOT hold the transition's permission.
func buildTransitionPermScriptSpecDir(t *testing.T, dir string) {
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

	// `lock` is reachable only through a GATED transition; `occupy` is
	// deliberately ungated so the same script shape can be shown to work.
	write("modules/acme/master/table.yaml", `apiVersion: formspec.dev/v1
kind: Entity
metadata: { name: table, module: acme }
spec:
  version: v1
  characteristic: master
  fields:
    - { name: code, type: string, natural_key: true, rules: [required] }
    - name: table_status
      type: enum
      enum_values: [available, occupied, locked]
      default: available
  state_machine:
    field: table_status
    initial: available
    states:
      - { name: available }
      - { name: occupied }
      - { name: locked }
    transitions:
      - { from: available, to: occupied, via: occupy }
      - from: available
        to: locked
        via: lock
        require_permission: tables.lock
`)

	write("modules/acme/master/runner.yaml", `apiVersion: formspec.dev/v1
kind: Entity
metadata: { name: runner, module: acme }
spec:
  version: v1
  characteristic: master
  fields:
    - { name: code, type: string, natural_key: true, rules: [required] }
  actions:
    - name: lock-table-via-save
      impl: { type: script_ref, ref: acme/lock_table_via_save }
    - name: occupy-table-via-save
      impl: { type: script_ref, ref: acme/occupy_table_via_save }
`)

	write("modules/acme/scripts/lock_table_via_save.star", `def execute(resource, params, ctx):
    t = resource.find("acme.table", {"code": params["code"]})
    t.set("table_status", "locked")
    t.save()
    return {"ok": True}
`)

	write("modules/acme/scripts/occupy_table_via_save.star", `def execute(resource, params, ctx):
    t = resource.find("acme.table", {"code": params["code"]})
    t.set("table_status", "occupied")
    t.save()
    return {"ok": True}
`)
}

func bootTransitionPermScriptApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	buildTransitionPermScriptSpecDir(t, dir)
	api.ResetAuthRateLimiters()

	app, err := New(Config{
		SpecPath:  dir,
		DSN:       "sqlite:" + filepath.Join(t.TempDir(), "transitionperm_script.db"),
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

func TestScriptWriteEnforcesTransitionPermission(t *testing.T) {
	app := bootTransitionPermScriptApp(t)

	seedUser(t, app, "admin", "admin", []string{"*"})
	// `limited` may update tables and run the runner actions — but does NOT
	// hold acme.tables.lock.
	seedUser(t, app, "limited", "limited", []string{
		"acme.tables.list", "acme.tables.view", "acme.tables.update",
		"acme.runners.list", "acme.runners.view",
		"acme.runners.lock-table-via-save", "acme.runners.occupy-table-via-save",
	})

	adminTok := login(t, app, "admin", "admin")
	limitedTok := login(t, app, "limited", "limited")

	const code = "T-1"
	status, out := doAuthed(t, app, http.MethodPost, "/default/_ui/entity/acme/table", adminTok, map[string]any{
		"code": code,
	})
	if status != http.StatusCreated {
		t.Fatalf("admin seed table: %d (%v)", status, out)
	}
	status, out = doAuthed(t, app, http.MethodPost, "/default/_ui/entity/acme/runner", adminTok, map[string]any{
		"code": "R-1",
	})
	if status != http.StatusCreated {
		t.Fatalf("admin seed runner: %d (%v)", status, out)
	}
	runnerID, _ := out["data"].(map[string]any)["id"].(string)

	call := func(t *testing.T, token, action string) (int, map[string]any) {
		t.Helper()
		return doAuthed(t, app, http.MethodPost,
			"/default/_ui/entity/acme/runner/"+runnerID+"/"+action, token,
			map[string]any{"code": code})
	}

	t.Run("gated transition from a script: refused without the permission", func(t *testing.T) {
		status, out := call(t, limitedTok, "lock-table-via-save")
		if status != http.StatusForbidden {
			t.Fatalf("a script must not cross a gated transition for a caller "+
				"without the permission; got %d (%v)", status, out)
		}
		if got := readTableStatus(t, app, adminTok, code); got == "locked" {
			t.Errorf("table_status = %q — the gated transition happened anyway", got)
		}
	})

	t.Run("gated transition from a script: succeeds with the permission", func(t *testing.T) {
		status, out := call(t, adminTok, "lock-table-via-save")
		if status != http.StatusOK && status != http.StatusCreated {
			t.Fatalf("a caller holding the transition permission must pass, got %d (%v)", status, out)
		}
		if got := readTableStatus(t, app, adminTok, code); got != "locked" {
			t.Errorf("table_status = %q, want locked", got)
		}
	})

	t.Run("ungated transition from a script is unaffected", func(t *testing.T) {
		// Fresh table so the transition starts from `available`.
		status, out := doAuthed(t, app, http.MethodPost, "/default/_ui/entity/acme/table", adminTok, map[string]any{
			"code": "T-2",
		})
		if status != http.StatusCreated {
			t.Fatalf("seed table T-2: %d (%v)", status, out)
		}
		// Re-point the script at T-2 by calling with its code.
		status, out = doAuthed(t, app, http.MethodPost,
			"/default/_ui/entity/acme/runner/"+runnerID+"/occupy-table-via-save", limitedTok,
			map[string]any{"code": "T-2"})
		if status != http.StatusOK && status != http.StatusCreated {
			t.Fatalf("an ungated transition must not need a permission, got %d (%v)", status, out)
		}
		if got := readTableStatus(t, app, adminTok, "T-2"); got != "occupied" {
			t.Errorf("table_status = %q, want occupied", got)
		}
	})
}

// readTableStatus reads the state field as an admin.
func readTableStatus(t *testing.T, app *App, adminTok, code string) string {
	t.Helper()
	status, out := doAuthed(t, app, http.MethodGet,
		"/default/_ui/entity/acme/table/"+code, adminTok, nil)
	if status != http.StatusOK {
		t.Fatalf("admin read table %s: %d (%v)", code, status, out)
	}
	data, _ := out["data"].(map[string]any)
	s, _ := data["table_status"].(string)
	return s
}
