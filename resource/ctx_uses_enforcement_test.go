package formspec

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// buildCtxUsesSpecDir writes a minimal spec whose order entity exposes a
// custom action `cachehit` implemented by cachehit.star, which calls
// ctx.cache(). Whether that is allowed depends on the action's
// uses.primitives declaration (toggled via declareUses) AND the app's
// strict mode (todo 2.6.4).
func buildCtxUsesSpecDir(t *testing.T, dir string, declareUses bool) {
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

	usesBlock := ""
	if declareUses {
		usesBlock = `
      uses:
        primitives: [cache]`
	}
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
    - name: cachehit
      required_permission: alpha.orders.cachehit`+usesBlock+`
      impl: { type: script_ref, ref: cachehit }
  expose:
    - type: rest
      actions: [list, find, create, update, delete]
`)

	write("modules/alpha/transaction/order/scripts/cachehit.star", `def execute(resource, params, ctx):
    ctx.cache().set("k", "v", ttl=60)
    return ok({"v": ctx.cache().get("k")})
`)
}

// TestCtxUses_StrictModeBlocked verifies that in strict mode, a script using
// ctx.cache() without declaring uses.primitives is BLOCKED with a
// USES_VIOLATION error (todo 2.6.4).
func TestCtxUses_StrictModeBlocked(t *testing.T) {
	dir := t.TempDir()
	buildCtxUsesSpecDir(t, dir, false) // no uses.primitives

	app, err := New(Config{
		SpecPath:   dir,
		DSN:        "sqlite:" + filepath.Join(t.TempDir(), "ctxuses.db"),
		StrictMode: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	status, out := doJSON(t, app, "POST", "/demo/_ui/entity/alpha/order", map[string]any{
		"transaction_date": recentDate(),
		"number":           "ORD-1",
	})
	if status != http.StatusCreated {
		t.Fatalf("create: status %d, body %v", status, out)
	}
	id, _ := out["data"].(map[string]any)["id"].(string)

	status, out = doJSON(t, app, "POST", "/demo/_ui/entity/alpha/order/"+id+"/cachehit", nil)
	if status == http.StatusOK {
		t.Fatalf("expected USES_VIOLATION in strict mode, got 200: %v", out)
	}
	if s, ok := out["error"].(map[string]any)["message"].(string); !ok || !contains(s, "USES_VIOLATION") {
		t.Fatalf("expected USES_VIOLATION message, got %v", out)
	}
}

// TestCtxUses_StrictModeAllowed verifies that in strict mode, a script using
// ctx.cache() WITH uses.primitives declared is allowed.
func TestCtxUses_StrictModeAllowed(t *testing.T) {
	dir := t.TempDir()
	buildCtxUsesSpecDir(t, dir, true) // declares uses.primitives: [cache]

	app, err := New(Config{
		SpecPath:   dir,
		DSN:        "sqlite:" + filepath.Join(t.TempDir(), "ctxuses.db"),
		StrictMode: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	status, out := doJSON(t, app, "POST", "/demo/_ui/entity/alpha/order", map[string]any{
		"transaction_date": recentDate(),
		"number":           "ORD-1",
	})
	if status != http.StatusCreated {
		t.Fatalf("create: status %d, body %v", status, out)
	}
	id, _ := out["data"].(map[string]any)["id"].(string)

	status, out = doJSON(t, app, "POST", "/demo/_ui/entity/alpha/order/"+id+"/cachehit", nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200 with uses.primitives declared, got %d: %v", status, out)
	}
	data, _ := out["data"].(map[string]any)
	if data["v"] != "v" {
		t.Fatalf("v = %v, want v", data["v"])
	}
}

// TestCtxUses_DevModeRelaxed verifies that in dev mode (non-strict), ctx.*
// works without a uses.primitives declaration.
func TestCtxUses_DevModeRelaxed(t *testing.T) {
	dir := t.TempDir()
	buildCtxUsesSpecDir(t, dir, false) // no uses.primitives

	app, err := New(Config{
		SpecPath: dir,
		DSN:      "sqlite:" + filepath.Join(t.TempDir(), "ctxuses.db"),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	status, out := doJSON(t, app, "POST", "/demo/_ui/entity/alpha/order", map[string]any{
		"transaction_date": recentDate(),
		"number":           "ORD-1",
	})
	if status != http.StatusCreated {
		t.Fatalf("create: status %d, body %v", status, out)
	}
	id, _ := out["data"].(map[string]any)["id"].(string)

	status, out = doJSON(t, app, "POST", "/demo/_ui/entity/alpha/order/"+id+"/cachehit", nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200 in dev mode, got %d: %v", status, out)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
