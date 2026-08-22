package formspec

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// recentDate returns yesterday's date (YYYY-MM-DD) — a transaction_date that
// is always within the default backdate limit, so tests are not
// time-dependent.
func recentDate() string {
	return time.Now().AddDate(0, 0, -1).Format("2006-01-02")
}

// buildUsesSpecDir writes a minimal two-module spec (alpha + beta) to dir.
// alpha's order entity exposes a custom action `peek` implemented by
// peek.star, which does a CROSS-MODULE resource.fetch("beta.item", id).
// Whether that fetch is allowed depends on the order entity's
// uses.resources declaration — toggled via declareUses.
func buildUsesSpecDir(t *testing.T, dir string, declareUses bool) {
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
    - beta
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
        resources: [beta.item]`
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
    - name: peek
      required_permission: alpha.orders.peek`+usesBlock+`
      impl: { type: script_ref, ref: peek }
  expose:
    - type: rest
      actions: [list, find, create, update, delete]
`)

	write("modules/alpha/transaction/order/scripts/peek.star", `def execute(resource, params, ctx):
    item = resource.fetch("beta.item", params["item_id"])
    return ok({"name": item.field.name})
`)

	write("modules/beta/module.yaml", `apiVersion: formspec.dev/v1
kind: Module
metadata:
  name: beta
spec:
  version: 1.0.0
`)

	write("modules/beta/master/item/entity.yaml", `apiVersion: formspec.dev/v1
kind: Entity
metadata:
  name: item
  module: beta
spec:
  version: v1
  characteristic: master
  fields:
    - name: name
      type: string
  expose:
    - type: rest
      actions: [list, find, create, update, delete]
`)
}

// doJSON performs an HTTP request against app.Handler() and returns status
// plus decoded envelope.
func doJSON(t *testing.T, app *App, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)
	var out map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	return rr.Code, out
}

// TestUsesEnforcement_CrossModuleFetchBlocked verifies end-to-end that a
// Starlark script's cross-module resource.fetch is BLOCKED with a
// USES_VIOLATION error when the caller action does not declare the target in
// uses.resources, and ALLOWED once the declaration is present (todo 2.6.4).
func TestUsesEnforcement_CrossModuleFetchBlocked(t *testing.T) {
	dir := t.TempDir()
	buildUsesSpecDir(t, dir, false) // no uses declaration → blocked

	app, err := New(Config{
		SpecPath: dir,
		DSN:      "sqlite:" + filepath.Join(t.TempDir(), "uses.db"),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Seed a beta item so the cross-module fetch has a target.
	status, out := doJSON(t, app, "POST", "/demo/_ui/entity/beta/item", map[string]any{"name": "Paracetamol"})
	if status != http.StatusCreated {
		t.Fatalf("create item: status %d, body %v", status, out)
	}
	itemID, _ := out["data"].(map[string]any)["id"].(string)
	if itemID == "" {
		t.Fatalf("expected item id, got %v", out)
	}

	// Now call alpha's peek action, which fetches beta.item cross-module.
	status, out = doJSON(t, app, "POST", "/demo/_ui/entity/alpha/order/"+itemID+"/peek", map[string]any{"item_id": itemID})
	if status == http.StatusOK {
		t.Fatalf("expected cross-module fetch to be BLOCKED (no uses declaration), got 200: %v", out)
	}
	errObj, _ := out["error"].(map[string]any)
	code, _ := errObj["code"].(string)
	if code != "ACTION_ERROR" {
		t.Fatalf("expected ACTION_ERROR wrapping USES_VIOLATION, got code=%q body=%v", code, out)
	}
	msg, _ := errObj["message"].(string)
	if !strings.Contains(msg, "USES_VIOLATION") {
		t.Fatalf("expected USES_VIOLATION in message, got %q", msg)
	}
}

// TestUsesEnforcement_CrossModuleFetchAllowed verifies the same script path
// succeeds once the action declares uses.resources with the target module.
func TestUsesEnforcement_CrossModuleFetchAllowed(t *testing.T) {
	dir := t.TempDir()
	buildUsesSpecDir(t, dir, true) // declare uses.resources: [beta.item]

	app, err := New(Config{
		SpecPath: dir,
		DSN:      "sqlite:" + filepath.Join(t.TempDir(), "uses_ok.db"),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	status, out := doJSON(t, app, "POST", "/demo/_ui/entity/beta/item", map[string]any{"name": "Paracetamol"})
	if status != http.StatusCreated {
		t.Fatalf("create item: status %d, body %v", status, out)
	}
	itemID, _ := out["data"].(map[string]any)["id"].(string)

	status, out = doJSON(t, app, "POST", "/demo/_ui/entity/alpha/order/"+itemID+"/peek", map[string]any{"item_id": itemID})
	if status != http.StatusOK {
		t.Fatalf("expected cross-module fetch allowed with uses declaration, got %d: %v", status, out)
	}
	data, _ := out["data"].(map[string]any)
	if data["name"] != "Paracetamol" {
		t.Fatalf("expected peek to return item name, got %v", data)
	}
}

// TestUsesEnforcement_SameModuleAllowed verifies same-module resource access
// is always allowed even with no uses declaration (module owns its resources).
func TestUsesEnforcement_SameModuleAllowed(t *testing.T) {
	dir := t.TempDir()
	buildUsesSpecDir(t, dir, false)

	// Patch alpha's peek script to fetch same-module (alpha.item doesn't
	// exist, so instead fetch alpha.order — a record we can create). We
	// rewrite peek.star to fetch "order" (same module).
	script := `def execute(resource, params, ctx):
    rec = resource.fetch("order", resource.id)
    return ok({"number": rec.field.number})
`
	if err := os.WriteFile(filepath.Join(dir, "modules/alpha/transaction/order/scripts/peek.star"), []byte(script), 0o644); err != nil {
		t.Fatalf("rewrite script: %v", err)
	}

	app, err := New(Config{
		SpecPath: dir,
		DSN:      "sqlite:" + filepath.Join(t.TempDir(), "uses_same.db"),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	status, out := doJSON(t, app, "POST", "/demo/_ui/entity/alpha/order", map[string]any{"number": "ORD-1", "transaction_date": recentDate()})
	if status != http.StatusCreated {
		t.Fatalf("create order: status %d, body %v", status, out)
	}
	orderID, _ := out["data"].(map[string]any)["id"].(string)

	status, out = doJSON(t, app, "POST", "/demo/_ui/entity/alpha/order/"+orderID+"/peek", nil)
	if status != http.StatusOK {
		t.Fatalf("expected same-module fetch allowed without uses, got %d: %v", status, out)
	}
	data, _ := out["data"].(map[string]any)
	if data["number"] != "ORD-1" {
		t.Fatalf("expected peek to return order number, got %v", data)
	}
}

// TestUsesEnforcement_CrossModuleLoadBlocked verifies the enforcement also
// covers the load path (resource.fetch) at the unit level through
// checkCrossModuleUses with the load-handler argument shape.
func TestUsesEnforcement_CrossModuleLoadBlocked(t *testing.T) {
	declared := []string{} // no uses
	if err := checkCrossModuleUses("alpha", "beta", "item", declared); err == nil {
		t.Fatal("expected checkCrossModuleUses to block undeclared alpha→beta load")
	}
	if err := checkCrossModuleUses("alpha", "beta", "item", []string{"beta.item"}); err != nil {
		t.Fatalf("expected declared load allowed, got %v", err)
	}
}
