package formspec

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// buildHookSpecDir writes a minimal spec whose entity declares a `before
// create` guard that calls fail(), plus a custom action whose script writes that
// entity twice: once with a legal value, once with a value the guard rejects.
//
// The guard is the point of the test. On the HTTP path a `before create` hook
// runs before the row is written and aborts it; on the script path
// `resource.create`/`resource.save` went straight to EntityStore.Insert, so the
// guard was simply absent — the same manifest, enforced on one path and ignored
// on the other.
func buildHookSpecDir(t *testing.T, dir string) {
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

	write("modules/acme/module.yaml", `apiVersion: formspec.dev/v1
kind: Module
metadata: { name: acme }
spec: {}
`)

	// `widget` carries a guard: quantity must be > 0. It exists so a script
	// write that violates it has to be refused.
	write("modules/acme/master/widget.yaml", `apiVersion: formspec.dev/v1
kind: Entity
metadata: { name: widget, module: acme }
spec:
  version: v1
  characteristic: master
  fields:
    - { name: code, type: string, natural_key: true, rules: [required] }
    - { name: quantity, type: integer }
  hooks:
    - on: before
      action: create
      impl: { type: script_ref, ref: acme/guard_quantity }
`)

	write("modules/acme/scripts/guard_quantity.star", `def execute(resource, params, ctx):
    qty = resource.field.quantity
    if qty != None and int(qty) <= 0:
        fail("GUARD: quantity harus lebih dari nol (dapat " + str(qty) + ")")
    return None
`)

	// `probe` is the entity whose action script performs the writes.
	write("modules/acme/master/probe.yaml", `apiVersion: formspec.dev/v1
kind: Entity
metadata: { name: probe, module: acme }
spec:
  version: v1
  characteristic: master
  fields:
    - { name: code, type: string, natural_key: true, rules: [required] }
  actions:
    - name: seed-widget
      impl: { type: script_ref, ref: acme/seed_widget }
      uses:
        resources: [acme.widget]
    - name: seed-widget-via-save
      impl: { type: script_ref, ref: acme/seed_widget_via_save }
      uses:
        resources: [acme.widget]
`)

	write("modules/acme/scripts/seed_widget.star", `def execute(resource, params, ctx):
    resource.create("acme.widget", {"code": params["code"], "quantity": int(params["quantity"])})
    return {"ok": True}
`)

	// Same write through `resource.save` on a fresh handle (ID "" → INSERT), so
	// the second builtin reaching the store is covered independently.
	write("modules/acme/scripts/seed_widget_via_save.star", `def execute(resource, params, ctx):
    w = resource.new("acme.widget")
    w.set("code", params["code"])
    w.set("quantity", int(params["quantity"]))
    w.save()
    return {"ok": True}
`)
}

// bootHookApp boots a fresh app for the hook spec.
func bootHookApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	buildHookSpecDir(t, dir)
	app, err := New(Config{
		SpecPath:    dir,
		DSN:         "sqlite:" + filepath.Join(t.TempDir(), "hooks.db"),
		WorkspaceID: "default",
	})
	if err != nil {
		t.Fatalf("boot app: %v", err)
	}
	t.Cleanup(func() { _ = app.Close(context.Background()) })
	app.StartBackgroundWorkers()
	return app
}

// TestScriptWriteEnforcesBeforeHook is the regression test for the gap found
// while wiring kafe 10.6/7.8.9: `resource.create` (and `resource.save`) skipped
// the target entity's `before` hooks.
//
// Consequence, and why this matters more than the missing `after` hook: guards
// are what stop bad data being written at all. An entity whose `before create`
// script rejects an out-of-range value was protected on every HTTP create and
// wide open on every script create — with no error, no log line, and no
// difference in the manifest to hint at it.
func TestScriptWriteEnforcesBeforeHook(t *testing.T) {
	app := bootHookApp(t)
	token := seedAdminToken(t, app)

	// Seed the probe row the action runs against.
	probeStore, err := app.Registry().GetEntityStore("acme", "probe")
	if err != nil {
		t.Fatalf("probe store: %v", err)
	}
	probeID, err := probeStore.Insert(context.Background(), db.InsertParams{
		WorkspaceID: "default", CreatedBy: "test",
		Data: map[string]any{"code": "P-1"},
	})
	if err != nil {
		t.Fatalf("insert probe: %v", err)
	}

	widgetStore, err := app.Registry().GetEntityStore("acme", "widget")
	if err != nil {
		t.Fatalf("widget store: %v", err)
	}
	countWidgets := func() int {
		res, err := widgetStore.List(context.Background(), db.ListParams{
			WorkspaceID: "default", Page: 1, PerPage: 50,
		})
		if err != nil {
			t.Fatalf("list widgets: %v", err)
		}
		return res.Total
	}

	// 1. A legal write succeeds.
	status, body := doAuthed(t, app, http.MethodPost,
		"/default/_ui/entity/acme/probe/"+probeID+"/seed-widget", token,
		map[string]any{"code": "W-OK", "quantity": 5})
	if status != http.StatusOK {
		t.Fatalf("legal script write: status %d body %v", status, body)
	}
	if got := countWidgets(); got != 1 {
		t.Fatalf("after a legal write: %d widgets, want 1", got)
	}

	// 2. A write the guard rejects must NOT land. Before the fix this succeeded
	//    and the widget row existed with quantity=-3.
	status, body = doAuthed(t, app, http.MethodPost,
		"/default/_ui/entity/acme/probe/"+probeID+"/seed-widget", token,
		map[string]any{"code": "W-BAD", "quantity": -3})

	if got := countWidgets(); got != 1 {
		t.Errorf("guard bypassed on the script path: %d widgets exist after a write the "+
			"`before create` hook rejects, want 1 (the bad row was written anyway)\nbody=%v",
			got, body)
	}
	if status == http.StatusOK {
		t.Errorf("script write violating a `before` guard returned 200; the guard did not abort it")
	}
}

// TestScriptWriteEnforcesBeforeHookOnSave covers the `resource.save` path
// separately: it is a different builtin reaching the same store, and the fix
// had to be applied to both.
func TestScriptWriteEnforcesBeforeHookOnSave(t *testing.T) {
	app := bootHookApp(t)
	token := seedAdminToken(t, app)

	probeStore, err := app.Registry().GetEntityStore("acme", "probe")
	if err != nil {
		t.Fatalf("probe store: %v", err)
	}
	probeID, err := probeStore.Insert(context.Background(), db.InsertParams{
		WorkspaceID: "default", CreatedBy: "test",
		Data: map[string]any{"code": "P-2"},
	})
	if err != nil {
		t.Fatalf("insert probe: %v", err)
	}

	widgetStore, err := app.Registry().GetEntityStore("acme", "widget")
	if err != nil {
		t.Fatalf("widget store: %v", err)
	}
	before, err := widgetStore.List(context.Background(), db.ListParams{
		WorkspaceID: "default", Page: 1, PerPage: 50,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	status, body := doAuthed(t, app, http.MethodPost,
		"/default/_ui/entity/acme/probe/"+probeID+"/seed-widget-via-save", token,
		map[string]any{"code": "W-SAVE-BAD", "quantity": 0})

	after, err := widgetStore.List(context.Background(), db.ListParams{
		WorkspaceID: "default", Page: 1, PerPage: 50,
	})
	if err != nil {
		t.Fatalf("list after: %v", err)
	}
	if after.Total != before.Total {
		t.Errorf("resource.save bypassed the `before` guard: widget count %d → %d\nbody=%v",
			before.Total, after.Total, body)
	}
	if status == http.StatusOK {
		t.Errorf("resource.save violating a `before` guard returned 200")
	}
}
