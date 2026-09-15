// ─── Child field picker — "fill this child field by choosing from an entity" ───
// (S1 generalized; replaces the order-specific `order_builder` Page block)
//
// The pattern is not about food orders: purchase-order lines, stock-opname
// counts, stock movements, journal entries, prescriptions, treatments and
// checklist items all need the same thing — pick rows from a source entity,
// carry a quantity, snapshot what was picked.
//
// It lives on the *child field* on purpose:
//
//   - it works in any Form (and Wizard step) that edits that entity, so the
//     writing path stays the Form's — permissions, rules validation,
//     idempotency, lifecycle `action:`, redirect and events all keep working;
//   - submitting is just submitting the form: picker rows are ordinary child
//     rows in form state, not a second write path.
//
// Snapshotting is the same idea as the existing field-level `auto_fill`: when a
// source record is picked, its values are copied into the row so an old order
// stays readable after a price change (05-field-types.md §2, D2).
//
// Everything the block version did *around* the write is gone: `defaults` →
// `FormField.default_from`, `submit_label`/`success_message` →
// the Form's own `submit`, `reset_after_submit` → the Form's lifecycle.

package spec

import (
	"fmt"
	"sort"
	"strings"
)

// PickerDecl declares that a child field is filled by picking records from
// another entity.
type PickerDecl struct {
	// Entity is the source entity rows are picked from (`module.entity` or a
	// bare name resolved against the owning module).
	// @schema {example: "cafe-master.menu-item"}
	Entity string `yaml:"entity" json:"entity"`
	// Filter is an equality pre-filter applied to the source list request.
	// Values interpolate `{dotted.path}` / `{now}` / `{today}` against the
	// render context.
	// @schema {example: "{\"is_available\": \"true\"}"}
	Filter map[string]string `yaml:"filter,omitempty" json:"filter,omitempty"`
	// Display describes how a source record is presented as a tile.
	Display PickerDisplay `yaml:"display" json:"display"`
	// Map describes what gets written into the row.
	Map PickerMap `yaml:"map" json:"map"`
}

// PickerDisplay is the presentation half of a picker: what the user sees.
type PickerDisplay struct {
	// @schema {description: "Source field used as the tile title (default \"name\").", example: "name"}
	NameField string `yaml:"name_field,omitempty" json:"name_field,omitempty"`
	// @schema {description: "Source field holding an image: a file/attachment field (served from the entity file route) or a string URL.", example: "photo"}
	ImageField string `yaml:"image_field,omitempty" json:"image_field,omitempty"`
	// @schema {example: "description"}
	DescriptionField string `yaml:"description_field,omitempty" json:"description_field,omitempty"`
	// @schema {description: "Source relation/enum field rendered as filter chips.", example: "menu_category_id"}
	CategoryField string `yaml:"category_field,omitempty" json:"category_field,omitempty"`
	// PriceEntity reads the price from a separate entity — the per-branch price
	// list case. The client joins it onto the source rows; a row with no price
	// is shown but cannot be picked (no price = nothing to sell).
	// @schema {example: "cafe-master.menu-item-price"}
	PriceEntity string `yaml:"price_entity,omitempty" json:"price_entity,omitempty"`
	// @schema {description: "Field on `price_entity` holding the source id (required with price_entity).", example: "menu_item_id"}
	PriceMatchField string `yaml:"price_match_field,omitempty" json:"price_match_field,omitempty"`
	// @schema {description: "Money field on `price_entity` (required with price_entity).", example: "price"}
	PriceField string `yaml:"price_field,omitempty" json:"price_field,omitempty"`
	// @schema {description: "Narrows the price rows (typically the branch). Values interpolate like `filter`.", example: "{\"branch_id\": \"{session.branch_id}\"}"}
	PriceFilter map[string]string `yaml:"price_filter,omitempty" json:"price_filter,omitempty"`
	// @schema {description: "Tile grid columns, 2–4 (default 3).", example: "3"}
	Columns int `yaml:"columns,omitempty" json:"columns,omitempty"`
	// @schema {description: "Client-side search over the tile title."}
	Search bool `yaml:"search,omitempty" json:"search,omitempty"`
	// @schema {example: "Belum ada data"}
	EmptyText string `yaml:"empty_text,omitempty" json:"empty_text,omitempty"`
}

// PickerMap is the write half of a picker: which row field receives what.
type PickerMap struct {
	// RefField is the row's relation field pointing back at the source record.
	// @schema {example: "menu_item_id"}
	RefField string `yaml:"ref_field" json:"ref_field"`
	// NameField receives the source record's display name (snapshot).
	// @schema {example: "name_snapshot"}
	NameField string `yaml:"name_field,omitempty" json:"name_field,omitempty"`
	// PriceField receives the source record's price (snapshot).
	// @schema {example: "unit_price_snapshot"}
	PriceField string `yaml:"price_field,omitempty" json:"price_field,omitempty"`
	// QuantityField accumulates the picked amount. When omitted, a picked row
	// is quantity-less (one row per pick — e.g. a checklist item).
	// @schema {example: "quantity"}
	QuantityField string `yaml:"quantity_field,omitempty" json:"quantity_field,omitempty"`
	// NoteField is a free-text per-row note (typed by the user, not copied).
	// @schema {example: "note"}
	NoteField string `yaml:"note_field,omitempty" json:"note_field,omitempty"`
	// @schema {description: "Upper bound per row (default 99).", example: "20"}
	MaxQuantity int `yaml:"max_quantity,omitempty" json:"max_quantity,omitempty"`
	// SourceNameField / SourcePriceField override which *source* field the name
	// and price snapshots read from (defaults "name" / the display price field).
	// @schema {example: "title"}
	SourceNameField string `yaml:"source_name_field,omitempty" json:"source_name_field,omitempty"`
}

// childFieldNames lists a child's field names for an error message.
func childFieldNames(fields []Field) string {
	names := make([]string, 0, len(fields))
	for _, f := range fields {
		names = append(names, f.Name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// ValidatePickerDecl validates a child field's picker.
func ValidatePickerDecl(p *PickerDecl, fieldName, where string) error {
	if p == nil {
		return nil
	}
	if p.Entity == "" {
		return fmt.Errorf("%s: child field %q picker.entity is required (the entity rows are picked from)", where, fieldName)
	}
	if p.Map.RefField == "" {
		return fmt.Errorf("%s: child field %q picker.map.ref_field is required (the row's relation field pointing at %s)", where, fieldName, p.Entity)
	}
	if p.Display.Columns != 0 && (p.Display.Columns < 2 || p.Display.Columns > 4) {
		return fmt.Errorf("%s: child field %q picker.display.columns must be 2–4, got %d", where, fieldName, p.Display.Columns)
	}
	if p.Display.PriceEntity != "" {
		if p.Display.PriceMatchField == "" {
			return fmt.Errorf("%s: child field %q picker.display.price_match_field is required with price_entity", where, fieldName)
		}
		if p.Display.PriceField == "" {
			return fmt.Errorf("%s: child field %q picker.display.price_field is required with price_entity", where, fieldName)
		}
	}
	if p.Map.MaxQuantity < 0 {
		return fmt.Errorf("%s: child field %q picker.map.max_quantity must be positive, got %d", where, fieldName, p.Map.MaxQuantity)
	}
	// A quantity field with no ceiling is a footgun on a shared device: a stuck
	// key orders a hundred. Require the bound to be considered, not defaulted.
	if p.Map.QuantityField != "" && p.Map.MaxQuantity == 0 {
		return fmt.Errorf("%s: child field %q picker.map.max_quantity is required with quantity_field (a bounded amount, never unbounded)", where, fieldName)
	}
	return nil
}
