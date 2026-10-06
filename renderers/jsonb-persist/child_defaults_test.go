package db

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// A `default` declared on a CHILD field must be applied when the row is written.
//
// kafe 10.66 found the symptom on `order.lines[].line_status` (`default:
// queued`): a stored line simply had no `line_status` at all — not null, the
// key was absent. `EntityStore.applyDefaults` iterates `s.fields`, the PARENT
// fields, and there is no child counterpart, so every child field that relies
// on a declared default was silently left unset.
//
// The asymmetry is with `computed` (which the engine does evaluate for child
// rows, in `evaluateComputed`), and it is not deliberate: both are declared on
// the same child field list.
//
// Verified against the RAW stored payload, not a read result, so a fix cannot
// pass by applying the default only in the returned copy.
func TestEntityStore_ChildFieldDefaults(t *testing.T) {
	for _, storage := range []string{"jsonb", "table"} {
		t.Run(storage, func(t *testing.T) {
			dir := t.TempDir()
			d, err := OpenSQLite(filepath.Join(dir, "child_defaults.db"), nil)
			if err != nil {
				t.Fatalf("OpenSQLite failed: %v", err)
			}
			defer func() { _ = d.Close() }()

			meta := spec.Metadata{Name: "order", Module: "cafe"}
			entity := &spec.EntitySpec{
				Version: "v1",
				Fields: []spec.Field{
					{Name: "number", Type: spec.FieldString},
					{
						Name: "lines",
						Type: spec.FieldChild,
						Child: &spec.ChildDecl{
							Storage: storage,
							Fields: []spec.Field{
								{Name: "line_no", Type: spec.FieldInteger},
								{Name: "quantity", Type: spec.FieldInteger, Default: float64(1)},
								{
									Name:    "line_status",
									Type:    spec.FieldEnum,
									Default: "queued",
									EnumValues: []string{
										"queued", "preparing", "ready", "served",
									},
								},
							},
						},
					},
				},
			}

			r := NewMigrationRunner(d, DriverSQLite)
			ctx := context.Background()
			if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
				t.Fatalf("ApplyMigrations failed: %v", err)
			}
			store := NewEntityStore(d, DriverSQLite, meta, entity)

			// The row deliberately omits both defaulted fields — the shape the
			// real customer form produces (it never sends `line_status`).
			id, err := store.Insert(ctx, InsertParams{
				WorkspaceID: "t1", CreatedBy: "u1",
				Data: map[string]any{
					"number": "ORD-1",
					"lines": []any{
						map[string]any{"line_no": float64(1)},
						map[string]any{"line_no": float64(2), "quantity": float64(3)},
					},
				},
			})
			if err != nil {
				t.Fatalf("Insert failed: %v", err)
			}

			// Read the RAW stored row: a read-time default would be a different
			// (and insufficient) fix, so it must be visible in storage.
			rows := rawChildRows(t, d, store, id)
			if len(rows) != 2 {
				t.Fatalf("expected 2 stored child rows, got %d (%v)", len(rows), rows)
			}
			if got := rows[0]["line_status"]; got != "queued" {
				t.Errorf("row 1 line_status = %v, want the declared default %q", got, "queued")
			}
			// A default must not OVERWRITE a value the caller supplied.
			if got := rows[1]["quantity"]; numberOfAny(got) != 3 {
				t.Errorf("row 2 quantity = %v, want the caller's 3 (default must not win)", got)
			}
			// And it must fill in what the caller omitted.
			if got := numberOfAny(rows[0]["quantity"]); got != 1 {
				t.Errorf("row 1 quantity = %v, want the declared default 1", got)
			}
			if got := rows[1]["line_status"]; got != "queued" {
				t.Errorf("row 2 line_status = %v, want %q", got, "queued")
			}
		})
	}
}

// rawChildRows reads the child rows from storage, bypassing reads/derivation —
// for `jsonb` storage they live in the parent payload, for `table` storage in
// the child table.
func rawChildRows(t *testing.T, d DB, store *EntityStore, parentID string) []map[string]any {
	t.Helper()
	ctx := context.Background()

	if cs, ok := store.children["lines"]; ok && cs.Storage() == "jsonb" {
		rec, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: "t1", ID: parentID})
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		raw, ok := rec.Data["lines"].([]any)
		if !ok {
			t.Fatalf("expected lines []any, got %T", rec.Data["lines"])
		}
		out := make([]map[string]any, 0, len(raw))
		for _, item := range raw {
			m, ok := item.(map[string]any)
			if !ok {
				t.Fatalf("expected a child map, got %T", item)
			}
			out = append(out, m)
		}
		return out
	}

	// table storage: read the child table directly (the raw persisted rows, not
	// the hydrated-and-computed view). Ordered by insertion (`rowid`) because
	// this fixture declares no `sequence_field`, so no such column exists.
	table := store.tableName + "__lines"
	rows, err := d.QueryContext(ctx, `SELECT data FROM `+table+` ORDER BY rowid`)
	if err != nil {
		t.Fatalf("query child table: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var out []map[string]any
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			t.Fatalf("scan child row: %v", err)
		}
		m := map[string]any{}
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			t.Fatalf("decode child row: %v", err)
		}
		out = append(out, m)
	}
	return out
}

// numberOfAny coerces a numeric JSON value for assertions.
func numberOfAny(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	}
	return -1
}
