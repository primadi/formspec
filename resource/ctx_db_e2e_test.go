package formspec

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// buildCtxDBSpecDir writes a minimal spec whose order entity exposes a custom
// action `ping` implemented by ping.star, which calls ctx.db().query(...)
// against the app's primary database (todo 2.9.1).
func buildCtxDBSpecDir(t *testing.T, dir string) {
	t.Helper()

	write := func(rel, content string) {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	write("apps/test.yaml", `apiVersion: formspec.dev/v1
kind: App
metadata:
  name: test
spec:
  version: 1.0.0
  root_url: /app/test
  modules:
    - alpha
`)

	write("modules/alpha/module.yaml", `apiVersion: formspec.dev/v1
kind: Module
metadata:
  name: alpha
spec:
  version: 1.0.0
`)

	write("modules/alpha/transaction/order/entity.yaml", `apiVersion: formspec.dev/v1
kind: Entity
metadata:
  name: order
  module: alpha
spec:
  version: v1
  characteristic: transaction
  fields:
    - name: transaction_date
      type: date
      required: true
      index: true
    - name: number
      type: string
  actions:
    - name: ping
      required_permission: alpha.orders.ping
      impl: { type: script_ref, ref: ping }
  expose:
    - type: rest
      actions: [list, find, create, update, delete]
`)

	write("modules/alpha/transaction/order/scripts/ping.star", `def execute(resource, params, ctx):
    rows = ctx.db().query("SELECT 1 AS one")
    return ok({"n": len(rows), "one": rows[0]["one"]})
`)
}

// TestCtxDBQuery_EndToEnd proves the ctx.* resolver is wired from the App
// (newDispatcher) through to script execution: a script action calling
// ctx.db().query(...) runs against the app's primary database and returns
// the rows (todo 2.9.1). Before this change every ctx.* primitive failed
// with "datastore resolver not configured".
func TestCtxDBQuery_EndToEnd(t *testing.T) {
	dir := t.TempDir()
	buildCtxDBSpecDir(t, dir)

	app, err := New(Config{
		SpecPath: dir,
		DSN:      "sqlite:" + filepath.Join(t.TempDir(), "ctxdb.db"),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Create a record so the custom action has a resource to act on.
	status, out := doJSON(t, app, "POST", "/demo/_ui/entity/alpha/order", map[string]any{
		"transaction_date": "2026-08-16",
		"number":           "ORD-1",
	})
	if status != http.StatusCreated {
		t.Fatalf("create order: status %d, body %v", status, out)
	}
	id, _ := out["data"].(map[string]any)["id"].(string)
	if id == "" {
		t.Fatalf("expected order id, got %v", out)
	}

	// Invoke the ping action, which runs ctx.db().query("SELECT 1 AS one").
	status, out = doJSON(t, app, "POST", "/demo/_ui/entity/alpha/order/"+id+"/ping", nil)
	if status != http.StatusOK {
		t.Fatalf("ping: status %d, body %v", status, out)
	}
	data, _ := out["data"].(map[string]any)
	if data["n"] != float64(1) {
		t.Fatalf("n = %v, want 1 (ctx.db().query did not return rows)", data["n"])
	}
	if data["one"] != float64(1) {
		t.Fatalf("one = %v, want 1", data["one"])
	}
}
