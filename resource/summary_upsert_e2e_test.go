package formspec

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// buildSummaryUpsertSpecDir writes a minimal spec with a summary projection
// (`stock-level`) maintained by a script, and a transaction entity
// (`stock-movement`) whose `after create` hook calls that maintainer. It also
// declares a second script that is NOT the maintainer, to prove the caller
// check refuses it (item 4.1, Opsi A).
func buildSummaryUpsertSpecDir(t *testing.T, dir string) {
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

	// The summary projection, maintained by alpha/stock_apply.
	write("modules/alpha/summary/stock-level/entity.yaml", `apiVersion: formspec.dev/v1
kind: Entity
metadata:
  name: stock-level
  module: alpha
spec:
  version: v1
  characteristic: summary
  plural: stock-levels
  fields:
    - name: ingredient_id
      type: string
      required: true
    - name: quantity_on_hand
      type: decimal
      scale: 3
      default: 0
  indexes:
    - fields: [ingredient_id]
      unique: true
  maintained_by: alpha/stock_apply
  invariants:
    - unique: [ingredient_id]
      message: "satu saldo per bahan"
`)

	// The transaction entity whose create triggers the maintainer.
	write("modules/alpha/transaction/stock-movement/entity.yaml", `apiVersion: formspec.dev/v1
kind: Entity
metadata:
  name: stock-movement
  module: alpha
spec:
  version: v1
  characteristic: transaction
  fields:
    - name: transaction_date
      type: date
      required: true
      index: true
    - name: ingredient_id
      type: string
      required: true
    - name: quantity
      type: decimal
      scale: 3
      required: true
  hooks:
    - on: after
      action: create
      impl: { type: script_ref, ref: alpha/stock_apply }
  expose:
    - type: rest
      actions: [list, find, create, update, delete]
`)

	// The maintainer: upserts the projection.
	write("modules/alpha/scripts/stock_apply.star", `def execute(resource, params, ctx):
    ingredient_id = resource.field.ingredient_id
    qty = float(resource.field.quantity or 0)
    current = resource.find("alpha.stock-level", {"ingredient_id": ingredient_id})
    if current == None:
        resource.upsert("alpha.stock-level", {"ingredient_id": ingredient_id}, {"quantity_on_hand": qty})
    else:
        resource.upsert("alpha.stock-level", {"ingredient_id": ingredient_id}, {"quantity_on_hand": float(current.field.quantity_on_hand or 0) + qty})
    return ok({})
`)

	// A NON-maintainer script that tries to write the same projection — must be
	// refused by the caller check.
	write("modules/alpha/scripts/rogue.star", `def execute(resource, params, ctx):
    resource.upsert("alpha.stock-level", {"ingredient_id": "X"}, {"quantity_on_hand": 1})
    return ok({})
`)
}

// TestSummaryUpsert_MaintainerWritesProjection proves item 4.1 (Opsi A): a
// stock-movement create triggers its maintainer script, which upserts the
// summary projection through resource.upsert — the ONE supported write path for
// summary entities. Two movements for the same ingredient accumulate.
func TestSummaryUpsert_MaintainerWritesProjection(t *testing.T) {
	dir := t.TempDir()
	buildSummaryUpsertSpecDir(t, dir)

	app, err := New(Config{
		SpecPath: dir,
		DSN:      "sqlite:" + filepath.Join(t.TempDir(), "summary.db"),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tok := seedAdminToken(t, app)

	create := func(qty float64) {
		status, out := doAuthed(t, app, "POST", "/default/_ui/entity/alpha/stock-movement", tok, map[string]any{
			"transaction_date": recentDate(),
			"ingredient_id":    "kopi",
			"quantity":         qty,
		})
		if status != http.StatusCreated {
			t.Fatalf("create stock-movement: status %d, body %v", status, out)
		}
	}

	create(5)
	create(3)

	// The projection must hold the accumulated quantity (5 + 3 = 8).
	status, out := doAuthed(t, app, "GET", "/default/_ui/entity/alpha/stock-level", tok, nil)
	if status != http.StatusOK {
		t.Fatalf("list stock-level: status %d, body %v", status, out)
	}
	rows, _ := out["data"].([]any)
	if len(rows) != 1 {
		t.Fatalf("expected exactly 1 projection row, got %d: %v", len(rows), out)
	}
	row, _ := rows[0].(map[string]any)
	if row["quantity_on_hand"] != float64(8) {
		t.Fatalf("quantity_on_hand = %v, want 8 (maintainer did not accumulate)", row["quantity_on_hand"])
	}
}

// TestSummaryUpsert_NonMaintainerRefused proves the caller check: a script that
// is NOT the entity's `maintained_by` cannot write the projection. This is what
// keeps summary read-only for everyone except its maintainer.
func TestSummaryUpsert_NonMaintainerRefused(t *testing.T) {
	dir := t.TempDir()
	buildSummaryUpsertSpecDir(t, dir)

	// Add a custom action on stock-movement that runs the rogue script.
	entityPath := filepath.Join(dir, "modules/alpha/transaction/stock-movement/entity.yaml")
	content, err := os.ReadFile(entityPath)
	if err != nil {
		t.Fatalf("read entity: %v", err)
	}
	patched := string(content) + `  actions:
    - name: rogue
      required_permission: alpha.stock-movements.rogue
      impl: { type: script_ref, ref: alpha/rogue }
`
	if err := os.WriteFile(entityPath, []byte(patched), 0o644); err != nil {
		t.Fatalf("write entity: %v", err)
	}

	app, err := New(Config{
		SpecPath: dir,
		DSN:      "sqlite:" + filepath.Join(t.TempDir(), "summary.db"),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tok := seedAdminToken(t, app)

	status, out := doAuthed(t, app, "POST", "/default/_ui/entity/alpha/stock-movement", tok, map[string]any{
		"transaction_date": recentDate(),
		"ingredient_id":    "kopi",
		"quantity":         1,
	})
	if status != http.StatusCreated {
		t.Fatalf("create stock-movement: status %d, body %v", status, out)
	}
	id, _ := out["data"].(map[string]any)["id"].(string)

	// The rogue action must fail — it is not the maintainer.
	status, out = doAuthed(t, app, "POST", "/default/_ui/entity/alpha/stock-movement/"+id+"/rogue", tok, nil)
	if status == http.StatusOK {
		t.Fatalf("rogue upsert: expected failure, got 200: %v", out)
	}
}
