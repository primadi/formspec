package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// A picked row's quantity is bounded by the PICKER construct, not by one
// application: at least 1, and at most the declared `map.max_quantity`.
//
// Why this is not merely a UI concern (measured before it existed): the browser
// clamps the field (`clampQuantity`), but the clamp is in the browser. A direct
// API caller, or any client that skips the picker, could store any number — and
// because a row's amount is `quantity × value`, that number feeds the record's
// totals, its tax and the journal. `map.max_quantity` was validated as a manifest
// requirement and then never enforced on the write path.
func pickerQuantitySpec(quantityField string, max int) *spec.EntitySpec {
	return &spec.EntitySpec{
		Fields: []spec.Field{{
			Name: "lines", Type: spec.FieldChild,
			Child: &spec.ChildDecl{
				Storage: "jsonb",
				Fields:  []spec.Field{{Name: "quantity", Type: spec.FieldInteger}},
				Picker: &spec.PickerDecl{
					Entity: "cafe-master.menu-item",
					Map: spec.PickerMap{
						RefField:      "menu_item_id",
						QuantityField: quantityField,
						MaxQuantity:   max,
					},
				},
			},
		}},
	}
}

func runQuantityGuard(t *testing.T, es *spec.EntitySpec, rows ...any) error {
	t.Helper()
	body := map[string]any{"lines": rows}
	return (&HandlerFactory{}).preparePickerRows(
		WithWorkspace(context.Background(), "kafe"), "cafe-order", "order", es, body, nil)
}

func TestPickerQuantity_FloorIsOne(t *testing.T) {
	es := pickerQuantitySpec("quantity", 20)

	// 0 and negatives are refused: a picked row exists because something was
	// picked, and "none of it" is expressed by removing the row.
	for _, bad := range []any{0, -1, float64(0), json.Number("0")} {
		row := map[string]any{"menu_item_id": "m1", "quantity": bad}
		err := runQuantityGuard(t, es, row)
		if err == nil {
			t.Fatalf("quantity %v must be refused (min 1)", bad)
		}
		if !strings.Contains(err.Error(), "at least 1") {
			t.Errorf("quantity %v: want the floor named, got: %v", bad, err)
		}
	}

	// 1 is the floor, so it is accepted.
	if err := runQuantityGuard(t, es, map[string]any{"menu_item_id": "m1", "quantity": 1}); err != nil {
		t.Fatalf("quantity 1 must be accepted: %v", err)
	}
}

func TestPickerQuantity_CeilingComesFromTheDeclaration(t *testing.T) {
	es := pickerQuantitySpec("quantity", 20)

	if err := runQuantityGuard(t, es, map[string]any{"menu_item_id": "m1", "quantity": 20}); err != nil {
		t.Fatalf("the declared maximum itself must be accepted: %v", err)
	}
	err := runQuantityGuard(t, es, map[string]any{"menu_item_id": "m1", "quantity": 21})
	if err == nil {
		t.Fatal("above map.max_quantity must be refused — otherwise the declaration is decoration")
	}
	if !strings.Contains(err.Error(), "at most 20") {
		t.Errorf("want the ceiling named, got: %v", err)
	}

	// With no declared ceiling, NO ceiling is invented: that would be a rule out
	// of nowhere, and the manifest author's choice is the only one that counts.
	unbounded := pickerQuantitySpec("quantity", 0)
	if err := runQuantityGuard(t, unbounded, map[string]any{"menu_item_id": "m1", "quantity": 100000}); err != nil {
		t.Fatalf("an undeclared ceiling must not be invented: %v", err)
	}
}

func TestPickerQuantity_WholeNumbersOnly(t *testing.T) {
	es := pickerQuantitySpec("quantity", 20)

	// Truncating 2.5 to 2 would change what the caller asked for, so it is
	// refused instead of silently rounded.
	for _, bad := range []any{2.5, "dua", true, []any{1}} {
		err := runQuantityGuard(t, es, map[string]any{"menu_item_id": "m1", "quantity": bad})
		if err == nil {
			t.Fatalf("quantity %#v must be refused as a non-whole number", bad)
		}
	}

	// Values arrive as float64 over JSON and as int in-process; both are counts.
	for _, ok := range []any{float64(3), 3, int64(3), json.Number("3"), "3"} {
		if err := runQuantityGuard(t, es, map[string]any{"menu_item_id": "m1", "quantity": ok}); err != nil {
			t.Errorf("quantity %#v must be accepted: %v", ok, err)
		}
	}
}

// An absent quantity is left to the field's own `default` (and `required`), so
// the guard does not become a second, invisible rule about presence.
func TestPickerQuantity_AbsenceIsLeftToTheField(t *testing.T) {
	es := pickerQuantitySpec("quantity", 20)
	if err := runQuantityGuard(t, es, map[string]any{"menu_item_id": "m1"}); err != nil {
		t.Fatalf("an omitted quantity is governed by `default`/`required`, not by this guard: %v", err)
	}
}

// A picker with no quantity field takes one row per pick (a checklist), so there
// is nothing to bound — and the guard must not demand a quantity that the
// construct does not have.
func TestPickerQuantity_NoQuantityFieldIsUnaffected(t *testing.T) {
	es := pickerQuantitySpec("", 0)
	if err := runQuantityGuard(t, es, map[string]any{"menu_item_id": "m1"}); err != nil {
		t.Fatalf("a quantity-less picker must be unaffected: %v", err)
	}
	// Even a stray value is not this construct's business.
	if err := runQuantityGuard(t, es, map[string]any{"menu_item_id": "m1", "quantity": 0}); err != nil {
		t.Fatalf("without a declared quantity_field there is nothing to bound: %v", err)
	}
}

// The guard runs on the SAME pass as the lookup derivation, so a bad quantity is
// reported before any lookup is attempted — and, more importantly, an entity
// that declares only a quantity (no lookup) is still covered. That was the gap
// this pins: the previous single-purpose function returned early for such a
// picker.
func TestPickerQuantity_AppliesWithoutALookup(t *testing.T) {
	es := pickerQuantitySpec("quantity", 20)
	if es.Fields[0].Child.Picker.Lookup != nil {
		t.Fatal("fixture must declare no lookup — that is the case under test")
	}
	err := runQuantityGuard(t, es, map[string]any{"menu_item_id": "m1", "quantity": 0})
	if err == nil {
		t.Fatal("a picker with a quantity but no lookup must still bound the quantity")
	}
}
