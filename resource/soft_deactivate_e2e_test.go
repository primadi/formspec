package formspec

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// buildSoftDeactivateSpecDir writes a minimal spec whose customer entity
// declares soft_deactivate: {enabled: true} (1.4.10 / 4.10.2).
func buildSoftDeactivateSpecDir(t *testing.T, dir string) {
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

	write("modules/alpha/master/customer/entity.yaml", `apiVersion: formspec.dev/v1
kind: Entity
metadata:
  name: customer
  module: alpha
spec:
  version: v1
  characteristic: master
  soft_deactivate: { enabled: true }
  fields:
    - name: code
      type: string
      natural_key: true
      required: true
    - name: name
      type: string
  expose:
    - type: rest
      actions: [list, find, create, update, delete, deactivate, reactivate]
`)
}

// TestSoftDeactivate_EndToEnd verifies the soft-deactivation pattern
// (4.10.2): is_active defaults to true, deactivate sets it false, reactivate
// sets it true again.
func TestSoftDeactivate_EndToEnd(t *testing.T) {
	dir := t.TempDir()
	buildSoftDeactivateSpecDir(t, dir)

	app, err := New(Config{
		SpecPath: dir,
		DSN:      "sqlite:" + filepath.Join(t.TempDir(), "softdeact.db"),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Create a customer.
	status, out := doJSON(t, app, "POST", "/demo/_ui/entity/alpha/customer", map[string]any{
		"code": "C-001",
		"name": "PT Maju",
	})
	if status != http.StatusCreated {
		t.Fatalf("create: status %d, body %v", status, out)
	}
	id, _ := out["data"].(map[string]any)["id"].(string)
	if id == "" {
		t.Fatalf("expected id, got %v", out)
	}

	// is_active should default to true.
	status, out = doJSON(t, app, "GET", "/demo/_ui/entity/alpha/customer/"+id, nil)
	if status != http.StatusOK {
		t.Fatalf("find: status %d, body %v", status, out)
	}
	data, _ := out["data"].(map[string]any)
	if data["is_active"] != true {
		t.Fatalf("is_active = %v, want true", data["is_active"])
	}

	// Deactivate → is_active=false.
	status, out = doJSON(t, app, "POST", "/demo/_ui/entity/alpha/customer/"+id+"/deactivate", nil)
	if status != http.StatusOK {
		t.Fatalf("deactivate: status %d, body %v", status, out)
	}
	data, _ = out["data"].(map[string]any)
	if data["is_active"] != false {
		t.Fatalf("is_active after deactivate = %v, want false", data["is_active"])
	}

	// Reactivate → is_active=true.
	status, out = doJSON(t, app, "POST", "/demo/_ui/entity/alpha/customer/"+id+"/reactivate", nil)
	if status != http.StatusOK {
		t.Fatalf("reactivate: status %d, body %v", status, out)
	}
	data, _ = out["data"].(map[string]any)
	if data["is_active"] != true {
		t.Fatalf("is_active after reactivate = %v, want true", data["is_active"])
	}
}
