package spec

import (
	"strings"
	"testing"
)

// The child-field picker is normative (S1): it decides what a Form can write
// into a child field. These tests pin the rules that keep an unusable picker
// from shipping — a tile grid that cannot map its picks onto a row is worse
// than a validation error at apply time.

func pickerEntity() *EntitySpec {
	return &EntitySpec{
		Version:        "v1",
		Characteristic: "transaction",
		Fields: []Field{
			{Name: "code", Type: FieldString},
			// `characteristic: transaction` requires it — unrelated to the picker,
			// but it runs first, so the fixture must satisfy it.
			{Name: "transaction_date", Type: FieldDateTime},
			{Name: "lines", Type: FieldChild, Child: &ChildDecl{
				Storage:       "jsonb",
				SequenceField: "line_no",
				Fields: []Field{
					{Name: "line_no", Type: FieldInteger},
					{Name: "menu_item_id", Type: FieldRelation},
					{Name: "name_snapshot", Type: FieldString},
					{Name: "unit_price_snapshot", Type: FieldMoney},
					{Name: "quantity", Type: FieldInteger},
					{Name: "note", Type: FieldString},
				},
				Picker: &PickerDecl{
					Entity:  "cafe-master.menu-item",
					Filter:  map[string]string{"is_available": "true"},
					Display: PickerDisplay{NameField: "name", Columns: 3, Search: true},
					Map: PickerMap{
						RefField:      "menu_item_id",
						NameField:     "name_snapshot",
						PriceField:    "unit_price_snapshot",
						QuantityField: "quantity",
						NoteField:     "note",
						MaxQuantity:   20,
					},
				},
			}},
		},
	}
}

func TestValidateEntitySpec_PickerValid(t *testing.T) {
	if err := ValidateEntitySpec(pickerEntity()); err != nil {
		t.Fatalf("a complete picker must validate, got: %v", err)
	}
}

// TestValidateEntitySpec_PickerRequired — ref_field is the mapping back to the
// source; without it a pick writes a row nothing can resolve.
func TestValidateEntitySpec_PickerRequired(t *testing.T) {
	e := pickerEntity()
	e.Fields[2].Child.Picker.Map.RefField = ""
	err := ValidateEntitySpec(e)
	if err == nil {
		t.Fatal("want an error for a picker without map.ref_field")
	}
	if !strings.Contains(err.Error(), "map.ref_field is required") {
		t.Fatalf("want ref_field named, got: %v", err)
	}

	e = pickerEntity()
	e.Fields[2].Child.Picker.Entity = ""
	err = ValidateEntitySpec(e)
	if err == nil || !strings.Contains(err.Error(), "picker.entity is required") {
		t.Fatalf("want picker.entity required, got: %v", err)
	}
}

// TestValidateEntitySpec_PickerMapTargetsChildFields — a mapping onto a field
// that does not exist in the child would drop the snapshot silently.
func TestValidateEntitySpec_PickerMapTargetsChildFields(t *testing.T) {
	cases := []struct{ name, want string }{
		{"map.name_field", "map.name_field"},
		{"map.price_field", "map.price_field"},
		{"map.quantity_field", "map.quantity_field"},
		{"map.note_field", "map.note_field"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := pickerEntity()
			// Picker is a pointer, so mutating through it edits the entity
			// (copying `Map` would only edit a copy).
			m := &e.Fields[2].Child.Picker.Map
			switch tc.name {
			case "map.name_field":
				m.NameField = "tidak_ada"
			case "map.price_field":
				m.PriceField = "tidak_ada"
			case "map.quantity_field":
				m.QuantityField = "tidak_ada"
			case "map.note_field":
				m.NoteField = "tidak_ada"
			}
			err := ValidateEntitySpec(e)
			if err == nil {
				t.Fatalf("want an error for %s pointing outside the child", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q in the message, got: %v", tc.want, err)
			}
			// The message must list the child's real fields — the fix has to be
			// obvious without opening the entity.
			if !strings.Contains(err.Error(), "menu_item_id") {
				t.Errorf("want the child's field list in the message, got: %v", err)
			}
		})
	}
}

// TestValidateEntitySpec_PickerQuantityNeedsBound — an unbounded quantity is a
// footgun on a shared device (a stuck key orders a hundred). The bound must be
// chosen, not defaulted.
func TestValidateEntitySpec_PickerQuantityNeedsBound(t *testing.T) {
	e := pickerEntity()
	e.Fields[2].Child.Picker.Map.MaxQuantity = 0
	err := ValidateEntitySpec(e)
	if err == nil {
		t.Fatal("want an error for quantity_field without max_quantity")
	}
	if !strings.Contains(err.Error(), "max_quantity is required with quantity_field") {
		t.Fatalf("want the bound explained, got: %v", err)
	}
}

// TestValidateEntitySpec_PickerPriceEntityContract — a price join is declared,
// never guessed.
func TestValidateEntitySpec_PickerPriceEntityContract(t *testing.T) {
	e := pickerEntity()
	e.Fields[2].Child.Picker.Display.PriceEntity = "cafe-master.menu-item-price"
	e.Fields[2].Child.Picker.Display.PriceMatchField = "menu_item_id"
	e.Fields[2].Child.Picker.Display.PriceField = "price"
	if err := ValidateEntitySpec(e); err != nil {
		t.Fatalf("a declared price join must validate, got: %v", err)
	}

	e = pickerEntity()
	e.Fields[2].Child.Picker.Display.PriceEntity = "cafe-master.menu-item-price"
	e.Fields[2].Child.Picker.Display.PriceField = "price"
	err := ValidateEntitySpec(e)
	if err == nil || !strings.Contains(err.Error(), "price_match_field is required") {
		t.Fatalf("want price_match_field required, got: %v", err)
	}

	e = pickerEntity()
	e.Fields[2].Child.Picker.Display.PriceEntity = "cafe-master.menu-item-price"
	e.Fields[2].Child.Picker.Display.PriceMatchField = "menu_item_id"
	err = ValidateEntitySpec(e)
	if err == nil || !strings.Contains(err.Error(), "price_field is required") {
		t.Fatalf("want price_field required, got: %v", err)
	}
}

// TestValidateEntitySpec_PickerWithoutQuantity — the list case (journal lines
// pick an account, a checklist picks an item): one row per pick, no quantity.
func TestValidateEntitySpec_PickerWithoutQuantity(t *testing.T) {
	e := pickerEntity()
	e.Fields[2].Child.Picker.Map = PickerMap{RefField: "menu_item_id"}
	if err := ValidateEntitySpec(e); err != nil {
		t.Fatalf("a picker without quantity must validate, got: %v", err)
	}
}

func TestValidateEntitySpec_PickerColumnsRange(t *testing.T) {
	e := pickerEntity()
	e.Fields[2].Child.Picker.Display.Columns = 1
	if err := ValidateEntitySpec(e); err == nil ||
		!strings.Contains(err.Error(), "columns must be 2–4") {
		t.Fatalf("want a columns range error, got: %v", err)
	}
}

// TestValidateEntitySpec_PickerOnNonChildIgnored — a picker only means something
// on a child field; the validator must not fail on unrelated shapes.
func TestValidateEntitySpec_PickerAbstractIsPerChild(t *testing.T) {
	e := pickerEntity()
	// Two children, only one with a picker — the other stays an ordinary grid.
	e.Fields = append(e.Fields, Field{Name: "notes", Type: FieldChild, Child: &ChildDecl{
		Storage: "jsonb",
		Fields:  []Field{{Name: "text", Type: FieldText}},
	}})
	if err := ValidateEntitySpec(e); err != nil {
		t.Fatalf("a plain child alongside a picker child must validate, got: %v", err)
	}
}
