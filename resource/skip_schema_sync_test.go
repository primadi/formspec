package formspec

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSkipSchemaSync_RepairSurfaceOpensRefusedDatabase locks the deadlock the
// migration gate used to create for data repairs (2026-09-28).
//
// The gate is right to refuse: a unique index cannot be added while the data
// still violates it, and docs/spec/backend/01-core-basic.md §4.4 says the
// operator repairs the data once and applies again. What was broken is that the
// documented repair surface could not open the database at all — `formspec repl`
// goes through New, New syncs the schema, and the sync is the refused thing. So
// the operator was told to run a command that fails with the very message it was
// supposed to resolve.
//
// Every step of the loop is asserted: the ordinary boot refuses, the repair boot
// opens, the repair through ctx.db() lands, and the next ordinary boot passes
// with the constraint actually enforced (a boot reporting success while the
// index was never created would be worse than the refusal).
func TestSkipSchemaSync_RepairSurfaceOpensRefusedDatabase(t *testing.T) {
	dir := t.TempDir()
	dsn := "sqlite:" + filepath.Join(t.TempDir(), "repair.db")

	// v1: no index at all — duplicates accumulate freely, which is exactly the
	// state the gate later refuses.
	writeRepairSpec(t, dir, false)
	app, err := New(Config{SpecPath: dir, DSN: dsn})
	if err != nil {
		t.Fatalf("v1 boot: %v", err)
	}
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if _, err := app.Database().ExecContext(ctx,
			`INSERT INTO acme_items (id, tenant_id, version, data) VALUES (?, 'default', 1, ?)`,
			dupRowID(i), `{"sku":"DUP","label":"x"}`,
		); err != nil {
			t.Fatalf("seed duplicate %d: %v", i, err)
		}
	}
	if err := app.Close(ctx); err != nil {
		t.Fatalf("close v1: %v", err)
	}

	// v2 declares a unique index over `sku`. Three identical rows block it, so
	// the refusal must stop a normal boot.
	writeRepairSpec(t, dir, true)
	if _, err := New(Config{SpecPath: dir, DSN: dsn}); err == nil {
		t.Fatal("v2 boot without SkipSchemaSync: expected the destructive-change refusal, got nil")
	} else {
		msg := err.Error()
		if !strings.Contains(msg, "destructive change(s) refused") {
			t.Fatalf("v2 boot refused for the wrong reason: %v", err)
		}
		if !strings.Contains(msg, "idx_acme_items_sku") {
			t.Fatalf("refusal does not name the blocking index: %v", err)
		}
		// The remedy has to be runnable — that is the whole point of the item.
		if !strings.Contains(msg, "--no-sync") {
			t.Fatalf("refusal does not name the runnable repair command: %v", err)
		}
	}

	// The repair surface: same spec, same database, schema sync skipped.
	repair, err := New(Config{SpecPath: dir, DSN: dsn, SkipSchemaSync: true})
	if err != nil {
		t.Fatalf("repair boot (SkipSchemaSync): %v", err)
	}
	if _, err := repair.Database().ExecContext(ctx,
		`DELETE FROM acme_items WHERE id IN (
		     SELECT id FROM acme_items WHERE json_extract(data, '$.sku') = 'DUP'
		     ORDER BY id LIMIT 2)`); err != nil {
		t.Fatalf("repair delete: %v", err)
	}
	if err := repair.Close(ctx); err != nil {
		t.Fatalf("close repair: %v", err)
	}

	// Re-apply: the data now satisfies the constraint, so the ordinary boot must
	// pass.
	final, err := New(Config{SpecPath: dir, DSN: dsn})
	if err != nil {
		t.Fatalf("boot after repair: %v", err)
	}
	defer func() { _ = final.Close(ctx) }()

	var n int
	if err := final.Database().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_acme_items_sku'").Scan(&n); err != nil {
		t.Fatalf("look up index: %v", err)
	}
	if n != 1 {
		t.Fatalf("idx_acme_items_sku missing after repair (rows=%d)", n)
	}
	// And it enforces: one DUP row survives, so a second one must be refused
	// rather than silently accepted.
	if _, err := final.Database().ExecContext(ctx,
		`INSERT INTO acme_items (id, tenant_id, version, data) VALUES (?, 'default', 1, ?)`,
		dupRowID(9), `{"sku":"DUP","label":"y"}`); err == nil {
		t.Fatal("inserting a duplicate sku succeeded — the unique index is not enforced")
	}
}

// dupRowID builds a distinct UUIDv7-shaped id so the seeded rows differ by
// primary key and collide only on the indexed field.
func dupRowID(i int) string {
	return "01a10000-0000-7000-8000-00000000000" + string(rune('0'+i))
}

// writeRepairSpec writes a one-entity spec. `unique` toggles the declaration the
// gate refuses to add while duplicates exist; with it off the spec declares no
// index at all, so the change really is an index *addition* (a changed
// definition would be `derived` and would not be refused).
func writeRepairSpec(t *testing.T, dir string, unique bool) {
	t.Helper()
	indexes := ""
	if unique {
		indexes = `
  indexes:
    - fields: [sku]
      unique: true`
	}
	files := map[string]string{
		"apps/acme.yaml": `apiVersion: formspec.dev/v1
kind: App
metadata:
  name: acme-app
spec:
  version: 1.0.0
  root_url: /app/acme
  modules: [acme]
`,
		"modules/acme/module.yaml": `apiVersion: formspec.dev/v1
kind: Module
metadata:
  name: acme
spec:
  version: 1.0.0
`,
		"modules/acme/master/item/entity.yaml": `apiVersion: formspec.dev/v1
kind: Entity
metadata:
  name: item
  module: acme
spec:
  version: v1
  plural: items
  characteristic: master
  expose:
    - type: rest
  fields:
    - name: sku
      type: string
      required: true
    - name: label
      type: string` + indexes + `
`,
	}
	for rel, content := range files {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
}
