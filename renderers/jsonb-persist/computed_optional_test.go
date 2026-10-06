package db

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// A computed formula must be able to read an OPTIONAL field without failing.
//
// Before `resource` was exposed to computed formulas, the only way to name a
// field was as a bare identifier, and an env map contains a key only when the
// field is SET. So a formula over an optional operand was an "undefined:"
// COMPILE error, which `evaluateComputed` swallows (`continue`) — the target
// field silently ended up absent, with no error at any layer. That is the
// mechanism behind kafe 10.65 (`order.total_amount` empty for every order whose
// optional discount fields were not supplied).
//
// This pins the fixed behaviour end to end through the real store: the formula
// reads optional money through `resource.<field>`, falls back to `money_zero`,
// and survives BOTH cases (operand present and absent). It also asserts the
// reverse — that the bare-identifier form still fails silently — so the guard
// cannot be satisfied by accident if someone re-writes the formula that way.
func TestEntityStore_ComputedOverOptionalField(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "computed_optional.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer func() { _ = d.Close() }()

	const totalFormula = `resource.subtotal + resource.service_charge - ` +
		`sum([money_zero(resource.subtotal), resource.discount_amount])`

	meta := spec.Metadata{Name: "order", Module: "cafe"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "subtotal", Type: spec.FieldMoney},
			{Name: "service_charge", Type: spec.FieldMoney},
			// Optional: absent unless a discount is applied.
			{Name: "discount_amount", Type: spec.FieldMoney},
			{
				Name:     "total_amount",
				Type:     spec.FieldMoney,
				Computed: &spec.ComputedDecl{Formula: totalFormula},
			},
			// The same intent written the WRONG way (bare identifier). Kept in
			// the manifest so the assertion below can show it yields nothing
			// rather than an error.
			{
				Name:     "total_bare",
				Type:     spec.FieldMoney,
				Computed: &spec.ComputedDecl{Formula: `subtotal - discount_amount`},
			},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}
	store := NewEntityStore(d, DriverSQLite, meta, entity)

	moneyOf := func(amount string) map[string]any {
		return map[string]any{"amount": amount, "currency": "IDR"}
	}
	amountOf := func(t *testing.T, rec *EntityRecord, field string) string {
		t.Helper()
		v, ok := rec.Data[field]
		if !ok || v == nil {
			t.Fatalf("%s is absent from the record (computed evaluation failed silently)", field)
		}
		switch m := v.(type) {
		case spec.Money:
			return m.Amount
		case map[string]any:
			s, _ := m["amount"].(string)
			return s
		}
		t.Fatalf("%s has unexpected type %T", field, v)
		return ""
	}

	t.Run("optional operand absent", func(t *testing.T) {
		id, err := store.Insert(ctx, InsertParams{
			WorkspaceID: "t1", CreatedBy: "u1",
			Data: map[string]any{
				"subtotal":       moneyOf("125000"),
				"service_charge": moneyOf("6250"),
				// discount_amount deliberately not supplied.
			},
		})
		if err != nil {
			t.Fatalf("Insert failed: %v", err)
		}
		rec, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: "t1", ID: id})
		if err != nil {
			t.Fatalf("GetByID failed: %v", err)
		}

		// 125000 + 6250 - 0
		if got := amountOf(t, rec, "total_amount"); got != "131250" {
			t.Errorf("total_amount = %q, want 131250", got)
		}
		// The bare-identifier variant shows the old failure mode: absent, not
		// an error. Pinned so the fix's necessity stays documented.
		if v, ok := rec.Data["total_bare"]; ok && v != nil {
			t.Errorf("total_bare should stay absent (bare absent field = compile error swallowed), got %v", v)
		}
	})

	t.Run("optional operand present", func(t *testing.T) {
		id, err := store.Insert(ctx, InsertParams{
			WorkspaceID: "t1", CreatedBy: "u1",
			Data: map[string]any{
				"subtotal":        moneyOf("125000"),
				"service_charge":  moneyOf("6250"),
				"discount_amount": moneyOf("25000"),
			},
		})
		if err != nil {
			t.Fatalf("Insert failed: %v", err)
		}
		rec, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: "t1", ID: id})
		if err != nil {
			t.Fatalf("GetByID failed: %v", err)
		}

		// 125000 + 6250 - 25000
		if got := amountOf(t, rec, "total_amount"); got != "106250" {
			t.Errorf("total_amount = %q, want 106250", got)
		}
	})
}
