package formspec

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// buildModuleScopedSpecDir writes a minimal spec with two modules bound to
// two different kind: Datastore manifests (todo 2.9.4):
//
//	alpha → ds-alpha (sqlite file datastores/ds-alpha.db)
//	beta  → unbound (resolves to 'default' — the app's primary database)
//
// Each module's order entity exposes a custom action `probe` implemented by
// probe.star, which creates a table via ctx.db() and counts its rows.
func buildModuleScopedSpecDir(t *testing.T, dir string) {
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

	write("datastores/ds-alpha.yaml", `apiVersion: formspec.dev/v1
kind: Datastore
metadata:
  name: ds-alpha
  module: alpha
spec:
  serves: [db]
  driver: sqlite
`)

	write("modules/alpha/module.yaml", `apiVersion: formspec.dev/v1
kind: Module
metadata:
  name: alpha
spec:
  version: 1.0.0
  datastore: ds-alpha
`)

	write("modules/beta/module.yaml", `apiVersion: formspec.dev/v1
kind: Module
metadata:
  name: beta
spec:
  version: 1.0.0
`)

	entityTmpl := `apiVersion: formspec.dev/v1
kind: Entity
metadata:
  name: order
  module: %s
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
    - name: probe
      required_permission: %s.orders.probe
      impl: { type: script_ref, ref: probe }
  expose:
    - type: rest
      actions: [list, find, create, update, delete]
`

	scriptFor := func(mod string) string {
		return "def execute(resource, params, ctx):\n" +
			"    ctx.db().query(\"CREATE TABLE IF NOT EXISTS probe_" + mod + " (id INTEGER)\")\n" +
			"    rows = ctx.db().query(\"SELECT 1 AS one\")\n" +
			"    return ok({\"n\": len(rows)})\n"
	}

	for _, mod := range []string{"alpha", "beta"} {
		write("modules/"+mod+"/transaction/order/entity.yaml",
			strings.Replace(strings.Replace(entityTmpl, "%s", mod, 2), "%s", mod, 1))
		write("modules/"+mod+"/transaction/order/scripts/probe.star", scriptFor(mod))
	}
}

// TestCtxDB_ModuleScoped_EndToEnd proves module-scoped ctx.db() resolution
// end-to-end through the HTTP action path (todo 2.9.4):
//
//   - a script in a bound module writes to its own datastore file
//   - a script in an unbound module writes to the primary database
//   - the two never share storage
func TestCtxDB_ModuleScoped_EndToEnd(t *testing.T) {
	dir := t.TempDir()
	buildModuleScopedSpecDir(t, dir)

	dsn := "sqlite:" + filepath.Join(t.TempDir(), "main.db")
	app, err := New(Config{
		SpecPath: dir,
		DSN:      dsn,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Create one record per module so each custom action has a resource.
	create := func(mod string) string {
		status, out := doJSON(t, app, "POST", "/demo/_ui/entity/"+mod+"/order", map[string]any{
			"transaction_date": recentDate(),
			"number":           "ORD-" + mod,
		})
		if status != http.StatusCreated {
			t.Fatalf("create %s order: status %d, body %v", mod, status, out)
		}
		id, _ := out["data"].(map[string]any)["id"].(string)
		if id == "" {
			t.Fatalf("expected %s order id, got %v", mod, out)
		}
		return id
	}
	alphaID := create("alpha")
	betaID := create("beta")

	// Invoke both probe actions.
	for _, tc := range []struct{ mod, id string }{{"alpha", alphaID}, {"beta", betaID}} {
		status, out := doJSON(t, app, "POST", "/demo/_ui/entity/"+tc.mod+"/order/"+tc.id+"/probe", nil)
		if status != http.StatusOK {
			t.Fatalf("probe %s: status %d, body %v", tc.mod, status, out)
		}
		data, _ := out["data"].(map[string]any)
		if data["n"] != float64(1) {
			t.Fatalf("probe %s: n = %v, want 1", tc.mod, data["n"])
		}
	}

	// Isolation proof: the probe table created by alpha's script must exist
	// in ds-alpha's SQLite file and NOT in the main database.
	stateDir := StateDirFromDSN(dsn)
	alphaDB, err := db.OpenSQLite(filepath.Join(stateDir, "datastores", "ds-alpha.db"), nil)
	if err != nil {
		t.Fatalf("open ds-alpha db: %v", err)
	}
	defer alphaDB.Close()
	ctx := context.Background()
	if ok, _ := alphaDB.HasTable(ctx, "", "probe_alpha"); !ok {
		t.Fatalf("probe_alpha not found in ds-alpha db — alpha's ctx.db() did not resolve to its bound datastore")
	}

	mainDB, err := db.OpenSQLite(strings.TrimPrefix(dsn, "sqlite:"), nil)
	if err != nil {
		t.Fatalf("open main db: %v", err)
	}
	defer mainDB.Close()
	if ok, _ := mainDB.HasTable(ctx, "", "probe_alpha"); ok {
		t.Fatalf("probe_alpha leaked into the main database — cross-datastore isolation broken")
	}
	// beta (unbound) wrote to the primary database.
	if ok, _ := mainDB.HasTable(ctx, "", "probe_beta"); !ok {
		t.Fatalf("probe_beta not found in main database — unbound module did not resolve to 'default'")
	}
	if ok, _ := alphaDB.HasTable(ctx, "", "probe_beta"); ok {
		t.Fatalf("probe_beta leaked into ds-alpha — isolation broken")
	}
}

// TestCtxDB_CrossDatastoreNamedBlocked_EndToEnd proves .named(x) from a
// bound module cannot reach another datastore at runtime — the script fails
// with the §1.1 error instead of silently writing elsewhere.
func TestCtxDB_CrossDatastoreNamedBlocked_EndToEnd(t *testing.T) {
	dir := t.TempDir()
	buildModuleScopedSpecDir(t, dir)

	// Override alpha's probe script to attempt an escape hatch. The chain
	// form is ctx.db.named("x") — .named() resolves immediately and returns
	// the connection runner.
	scriptPath := filepath.Join(dir, "modules/alpha/transaction/order/scripts/probe.star")
	if err := os.WriteFile(scriptPath, []byte(
		"def execute(resource, params, ctx):\n    ctx.db.named(\"default\").query(\"SELECT 1\")\n    return ok({})\n",
	), 0o644); err != nil {
		t.Fatalf("write escape script: %v", err)
	}

	app, err := New(Config{
		SpecPath: dir,
		DSN:      "sqlite:" + filepath.Join(t.TempDir(), "main.db"),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	status, out := doJSON(t, app, "POST", "/demo/_ui/entity/alpha/order", map[string]any{
		"transaction_date": recentDate(),
		"number":           "ORD-1",
	})
	if status != http.StatusCreated {
		t.Fatalf("create order: status %d, body %v", status, out)
	}
	id, _ := out["data"].(map[string]any)["id"].(string)

	status, out = doJSON(t, app, "POST", "/demo/_ui/entity/alpha/order/"+id+"/probe", nil)
	if status == http.StatusOK {
		t.Fatalf("escape hatch succeeded — want rejection, got %v", out)
	}
	body, _ := out["error"].(map[string]any)
	msg, _ := body["message"].(string)
	if !strings.Contains(msg, "not accessible") || !strings.Contains(msg, "06-datastore.md") {
		t.Fatalf("want §1.1 'not accessible' error, got status %d body %v", status, out)
	}
}
