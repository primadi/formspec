package db

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// `docs/spec/backend/05-field-types.md` §5.1 is NORMATIVE about computed fields:
//
//	"Never client-writable" — nilai `computed` di payload klien DIABAIKAN
//	"Recomputed on save"   — formula dievaluasi ulang pada tiap create/update
//	                          SEBELUM persist, sehingga nilai tersimpan selalu
//	                          konsisten dengan input terkini
//
// Measured before this was implemented (kafe dev server, `order.total_amount`):
//
//	CREATE with total_amount=999999 → STORED 999999 (the response showed 1155)
//	PATCH  with total_amount=777777 → STORED 777777
//
// Two defects in one: the forge was accepted, and — worse — storage disagreed
// with the API. The numeric derived column (`_total_amount`) that sort, range
// filters, and SQL reports read carried the forged number while the response
// carried the real one, which is the "wrong answer with no symptom" class §2.2
// warns about.
//
// A third defect is pinned here too: compute ran only as a side effect of the
// read-modify-write in UPDATE, so a freshly CREATEd row had no computed value in
// storage at all (kafe 10.70/10.68).
func TestComputedField_WriteContract(t *testing.T) {
	newStore := func(t *testing.T) (*EntityStore, DB) {
		t.Helper()
		dir := t.TempDir()
		d, err := OpenSQLite(filepath.Join(dir, "computed_write.db"), nil)
		if err != nil {
			t.Fatalf("OpenSQLite failed: %v", err)
		}
		t.Cleanup(func() { _ = d.Close() })

		meta := spec.Metadata{Name: "order", Module: "cafe"}
		entity := &spec.EntitySpec{
			Version: "v1",
			Fields: []spec.Field{
				{Name: "note", Type: spec.FieldString},
				{Name: "subtotal", Type: spec.FieldMoney},
				{
					Name: "total",
					Type: spec.FieldMoney,
					Computed: &spec.ComputedDecl{
						Formula: `resource.subtotal * 2`,
					},
				},
				{
					Name: "lines",
					Type: spec.FieldChild,
					Child: &spec.ChildDecl{
						Storage: "jsonb",
						Fields: []spec.Field{
							{Name: "qty", Type: spec.FieldInteger},
							{Name: "price", Type: spec.FieldMoney},
							{
								Name: "line_total",
								Type: spec.FieldMoney,
								Computed: &spec.ComputedDecl{
									Formula: `resource.qty * resource.price`,
								},
							},
						},
					},
				},
			},
		}
		r := NewMigrationRunner(d, DriverSQLite)
		if _, err := r.ApplyMigrations(context.Background(),
			[]EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
			t.Fatalf("ApplyMigrations failed: %v", err)
		}
		return NewEntityStore(d, DriverSQLite, meta, entity), d
	}

	money := func(amount string) map[string]any {
		return map[string]any{"amount": amount, "currency": "IDR"}
	}
	// amount reads the value straight out of storage — never the response and
	// never a recomputed read — so a fix that only corrects the returned copy
	// cannot pass.
	storedAmount := func(t *testing.T, d DB, store *EntityStore, id, field string) any {
		t.Helper()
		rows := rawRowByID(t, d, store, id)
		if rows == nil {
			t.Fatalf("row %s not found in storage", id)
		}
		return rows[field]
	}

	t.Run("create ignores a forged parent value and stores the formula's", func(t *testing.T) {
		store, d := newStore(t)
		id, err := store.Insert(context.Background(), InsertParams{
			WorkspaceID: "t1", CreatedBy: "u1",
			Data: map[string]any{
				"note":     "forged",
				"subtotal": money("1000"),
				// The forge: a client must not be able to set a computed field.
				"total": money("999999"),
			},
		})
		if err != nil {
			t.Fatalf("Insert failed: %v", err)
		}

		got := amountOfAny(storedAmount(t, d, store, id, "total"))
		if got == "999999" {
			t.Errorf("a client-supplied value for a computed field was STORED (%s); "+
				"spec §5.1 requires it to be ignored", got)
		}
		if got != "2000" {
			t.Errorf("stored total = %q, want the formula's 2000 (subtotal × 2)", got)
		}
	})

	t.Run("create persists the computed value even when the client omits it", func(t *testing.T) {
		store, d := newStore(t)
		id, err := store.Insert(context.Background(), InsertParams{
			WorkspaceID: "t1", CreatedBy: "u1",
			Data: map[string]any{"subtotal": money("1000")},
		})
		if err != nil {
			t.Fatalf("Insert failed: %v", err)
		}
		// "Recomputed on save" means the value is written, not merely derived on
		// read — otherwise the numeric derived column stays NULL until some later
		// update happens to touch the row (kafe 10.70/10.68).
		if v := storedAmount(t, d, store, id, "total"); v == nil {
			t.Error("create did not persist the computed value; " +
				"spec §5.1 requires it to be recomputed and stored on save")
		}
	})

	t.Run("update ignores a forged value and re-derives from the new input", func(t *testing.T) {
		store, d := newStore(t)
		ctx := context.Background()
		id, err := store.Insert(ctx, InsertParams{
			WorkspaceID: "t1", CreatedBy: "u1",
			Data: map[string]any{"subtotal": money("1000")},
		})
		if err != nil {
			t.Fatalf("Insert failed: %v", err)
		}
		rec, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: "t1", ID: id})
		if err != nil {
			t.Fatalf("GetByID failed: %v", err)
		}

		// Caller changes the input AND tries to pin the output.
		if _, err := store.Update(ctx, UpdateParams{
			WorkspaceID: "t1", ID: id, Version: rec.Version, UpdatedBy: "u1",
			Data: map[string]any{
				"subtotal": money("500"),
				"total":    money("777777"),
			},
		}); err != nil {
			t.Fatalf("Update failed: %v", err)
		}

		got := amountOfAny(storedAmount(t, d, store, id, "total"))
		if got == "777777" {
			t.Errorf("a client-supplied computed value survived an update; spec §5.1 "+
				"requires it to be ignored (stored %s)", got)
		}
		if got != "1000" {
			t.Errorf("stored total = %q, want the re-derived 1000 (500 × 2) — "+
				"the formula must run against the NEW input on update", got)
		}
	})

	t.Run("child computed values follow the same contract", func(t *testing.T) {
		store, d := newStore(t)
		id, err := store.Insert(context.Background(), InsertParams{
			WorkspaceID: "t1", CreatedBy: "u1",
			Data: map[string]any{
				"subtotal": money("1000"),
				"lines": []any{
					map[string]any{
						"qty": float64(3), "price": money("100"),
						"line_total": money("123456"), // forge
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("Insert failed: %v", err)
		}
		rows := rawChildRows(t, d, store, id)
		if len(rows) != 1 {
			t.Fatalf("expected 1 child row, got %d", len(rows))
		}
		if got := amountOfAny(rows[0]["line_total"]); got != "300" {
			t.Errorf("stored child line_total = %q, want the formula's 300 (3 × 100); "+
				"a forge must not survive and the value must be persisted", got)
		}
	})
}

// amountOfAny renders a money value (map or spec.Money) as its amount string.
func amountOfAny(v any) string {
	switch m := v.(type) {
	case spec.Money:
		return m.Amount
	case *spec.Money:
		if m == nil {
			return ""
		}
		return m.Amount
	case map[string]any:
		s, _ := m["amount"].(string)
		if s == "" {
			return amountOfAny(m["amount"])
		}
		return s
	case string:
		return m
	}
	return ""
}

// rawRowByID reads one stored payload straight from the table, bypassing every
// read-path derivation.
func rawRowByID(t *testing.T, d DB, store *EntityStore, id string) map[string]any {
	t.Helper()
	rows, err := d.QueryContext(context.Background(),
		`SELECT data FROM `+store.tableName+` WHERE id = ?`, id)
	if err != nil {
		t.Fatalf("query raw row: %v", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil
	}
	var raw string
	if err := rows.Scan(&raw); err != nil {
		t.Fatalf("scan raw row: %v", err)
	}
	out := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("decode raw row: %v", err)
	}
	return out
}
