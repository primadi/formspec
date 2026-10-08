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
	// Lookup enriches each picked row with a value read from a related entity
	// (see PickerLookup). Optional: a picker that only chooses rows needs none.
	Lookup *PickerLookup `yaml:"lookup,omitempty" json:"lookup,omitempty"`
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
	// @schema {description: "Tile grid columns, 2–4 (default 3).", example: "3"}
	Columns int `yaml:"columns,omitempty" json:"columns,omitempty"`
	// @schema {description: "Client-side search over the tile title."}
	Search bool `yaml:"search,omitempty" json:"search,omitempty"`
	// @schema {example: "Belum ada data"}
	EmptyText string `yaml:"empty_text,omitempty" json:"empty_text,omitempty"`
}

// PickerLookup enriches a picked row with a value read from a RELATED entity —
// one record per picked row, matched by a key.
//
// Generic on purpose, and that is the whole point: "read one value per row from
// a table keyed by that row" is one shape, not a food-ordering feature. Change
// the nouns and it is a per-outlet price, a per-region tax rate, a per-warehouse
// stock level, a per-clinic fee, a per-tenant quota or a localized label. The
// picker's own entity supplies the ROWS; the lookup supplies the VALUE that goes
// with each row.
//
// It is declared on the picker (not on the entity) because the picker is what
// causes the fetch, so the declaration and the enforcement cannot come from two
// different places.
type PickerLookup struct {
	// Entity is the related entity the value is read from (`module.entity` or a
	// bare name resolved against the owning module).
	// @schema {example: "cafe-master.menu-item-price"}
	Entity string `yaml:"entity" json:"entity"`
	// Key is the field on `entity` matched against the picked row's source id
	// (the value written into `map.ref_field`).
	// @schema {example: "menu_item_id"}
	Key string `yaml:"key" json:"key"`
	// Field is the field on `entity` whose value is used.
	// @schema {example: "price"}
	Field string `yaml:"field" json:"field"`
	// Filter narrows the lookup list IN THE BROWSER. Values interpolate
	// `{dotted.path}` / `{now}` / `{today}` against the render context, exactly
	// like the picker's `filter`.
	//
	// It is NOT a boundary: any client can omit or widen it. Use `scope` for a
	// narrowing the server must enforce (the same split as a kind's `filters`
	// versus an entity's `row_scope`).
	// @schema {example: "{\"is_active\": \"true\"}"}
	Filter map[string]string `yaml:"filter,omitempty" json:"filter,omitempty"`
	// Scope is the SERVER-ENFORCED row scope for reads of `entity` caused by this
	// lookup. Same shape as `row_scope` (including `from: route` with `via`), so a
	// value can be DERIVED from a record the request references instead of being
	// taken from the request itself.
	//
	// Why it exists: a lookup is a read of another entity, and on a public surface
	// that read is anonymous. Narrowing it with `filter` looks like protection
	// while providing none — the value is computed by the client, so any caller
	// can widen it and read rows outside their dimension. Declaring `scope` puts
	// the same intent where the server can enforce it: the derived public grant
	// for `entity` inherits it (internal/ui, `visitField`).
	Scope []FilterSpec `yaml:"scope,omitempty" json:"scope,omitempty"`
}

// PickerMap is the write half of a picker: which row field receives what.
type PickerMap struct {
	// RefField is the row's relation field pointing back at the source record.
	// @schema {example: "menu_item_id"}
	RefField string `yaml:"ref_field" json:"ref_field"`
	// NameField receives the source record's display name (snapshot).
	// @schema {example: "name_snapshot"}
	NameField string `yaml:"name_field,omitempty" json:"name_field,omitempty"`
	// LookupField receives `lookup.field` for the picked row (snapshot).
	//
	// When declared, the value is DERIVED — the server reads it from the lookup
	// entity itself rather than accepting it from the request, the same rule that
	// applies to `computed` ("a derived value is never the caller's to set").
	// A caller-supplied value is replaced, and a row the lookup cannot price is
	// refused rather than stored with an amount nobody authorized.
	// @schema {example: "unit_price_snapshot"}
	LookupField string `yaml:"lookup_field,omitempty" json:"lookup_field,omitempty"`
	// QuantityField accumulates the picked amount. When omitted, a picked row
	// is quantity-less (one row per pick — e.g. a checklist item).
	// @schema {example: "quantity"}
	QuantityField string `yaml:"quantity_field,omitempty" json:"quantity_field,omitempty"`
	// NoteField is a free-text per-row note (typed by the user, not copied).
	// @schema {example: "note"}
	NoteField string `yaml:"note_field,omitempty" json:"note_field,omitempty"`
	// @schema {description: "Upper bound per row (default 99).", example: "20"}
	MaxQuantity int `yaml:"max_quantity,omitempty" json:"max_quantity,omitempty"`
	// SourceNameField overrides which *source* field the name snapshot reads from
	// (default "name").
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
	// The lookup names where a value comes from, so all three parts are needed
	// for it to resolve: the entity, the key that matches a picked row, and the
	// field read out of it. A half-declared lookup would render tiles with no
	// value and (when `map.lookup_field` is set) refuse every submit.
	if p.Lookup != nil {
		lk := p.Lookup
		if lk.Entity == "" {
			return fmt.Errorf("%s: child field %q picker.lookup.entity is required (the entity the value is read from)", where, fieldName)
		}
		if lk.Key == "" {
			return fmt.Errorf("%s: child field %q picker.lookup.key is required (the field on %s matching the picked row)", where, fieldName, lk.Entity)
		}
		if lk.Field == "" {
			return fmt.Errorf("%s: child field %q picker.lookup.field is required (the field on %s whose value is used)", where, fieldName, lk.Entity)
		}
		// `scope` is a SERVER-enforced narrowing, so it is held to the same shape
		// rule as any other row scope — including the `via` pairing that lets the
		// value be derived from a record the request references.
		if len(lk.Scope) > 0 {
			if err := ValidateRowScopeFilters(
				fmt.Sprintf("%s: child field %q picker.lookup.scope", where, fieldName),
				lk.Scope, nil); err != nil {
				return err
			}
		}
	}
	if p.Map.MaxQuantity < 0 {
		return fmt.Errorf("%s: child field %q picker.map.max_quantity must be positive, got %d", where, fieldName, p.Map.MaxQuantity)
	} // A receiving field with no lookup to fill it is a declaration that can never
	// work: the value would stay empty and (because the server resolves it) the
	// submit would be refused with no way for the manifest author to tell why.
	if p.Map.LookupField != "" && p.Lookup == nil {
		return fmt.Errorf("%s: child field %q picker.map.lookup_field is declared without picker.lookup — nothing would fill it", where, fieldName)
	}
	// A quantity field with no ceiling is a footgun on a shared device: a stuck
	// key orders a hundred. Require the bound to be considered, not defaulted.
	if p.Map.QuantityField != "" && p.Map.MaxQuantity == 0 {
		return fmt.Errorf("%s: child field %q picker.map.max_quantity is required with quantity_field (a bounded amount, never unbounded)", where, fieldName)
	}
	return nil
}
