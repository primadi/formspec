package api

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// preparePickerRows is the SERVER half of a picker declaration on the write
// path: it bounds what the caller may state, and derives what only the server
// may state.
//
// Two responsibilities, in that order, because the first is cheap and makes the
// second meaningful:
//
//  1. the row's QUANTITY is bounded (see validatePickerQuantities) — part of what
//     "a picked row" means, so it applies whether or not the picker looks anything
//     up;
//  2. the row's LOOKUP value is DERIVED from the related entity, never taken from
//     the request (see below) — only when the picker declares a lookup and a
//     receiving field.
//
// They live in one pass deliberately: both are the picker's write-path contract,
// and splitting them into two functions called from two handler sites is how a
// future change gets wired into create but not update.
//
// On the lookup half, generic rule with no domain knowledge: when a picker
// declares a `lookup` AND a `map.lookup_field` to receive it, the value is
// DERIVED — the server reads it from `lookup.entity` — rather than accepted from
// the payload. That is the same treatment the framework already gives `computed`
// ("a derived value is never the caller's to set"), and for the same reason: the
// manifest says where the value lives, so letting a caller author it makes the
// declaration decorative.
//
// Measured before this existed (kafe 10.79): a picked line's `unit_price_snapshot`
// was an ordinary money field, so an anonymous guest could send any amount and
// the line, its tax and the resulting journal all agreed with it — while the
// authoritative amount sat in the catalog the picker had already pointed at.
//
// Two properties are worth stating because they are choices, not accidents:
//
//   - FROZEN, not re-derived. A row whose reference is already on the stored
//     record keeps ITS value; only rows joining the record are resolved. The
//     stored value is what the caller agreed to, and re-reading the catalog on
//     every edit would silently re-price a record nobody agreed to re-price —
//     and would make "edit the quantity" a way to author the value.
//   - The lookup is narrowed by the RECORD's own dimension, taken from the
//     payload (then the stored row for a PATCH that omits it). Using the record
//     rather than re-reading the request is deliberate: the value that gets
//     stored is the one this row belongs to, and for a write whose dimension is
//     pinned (`create_scope`) that value has already been validated against the
//     record it references. When the field is absent the request is REFUSED
//     rather than resolved loosely — a lookup matched against nothing would pick
//     an arbitrary row of the related entity.
//
// The dimension is read from the lookup's `scope` entries (the same declaration
// the read path enforces), so declaration and enforcement cannot drift.
//
// `current` is the stored record for an update, and nil for a create.
func (f *HandlerFactory) preparePickerRows(ctx context.Context, module, entity string, es *spec.EntitySpec, body, current map[string]any) error {
	if es == nil || len(body) == 0 {
		return nil
	}
	for i := range es.Fields {
		fld := &es.Fields[i]
		if fld.Type != spec.FieldChild || fld.Child == nil || fld.Child.Picker == nil {
			continue
		}
		p := fld.Child.Picker

		rows := childRows(body[fld.Name])
		if len(rows) == 0 {
			continue
		}

		if err := validatePickerQuantities(fld.Name, p, rows); err != nil {
			return err
		}

		if p.Lookup == nil || p.Lookup.Entity == "" {
			continue
		}
		// A lookup used only for display needs no server work: nothing is stored,
		// so there is nothing to derive. The client renders it from the tile.
		if p.Map.LookupField == "" {
			continue
		}

		// Values already frozen on the record, keyed by the row's reference.
		stored := map[string]any{}
		if current != nil {
			for _, raw := range childRows(current[fld.Name]) {
				row, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				if ref := createScopeString(row[p.Map.RefField]); ref != "" {
					if v, ok := row[p.Map.LookupField]; ok {
						stored[ref] = v
					}
				}
			}
		}

		// The narrowing: the lookup's own `scope`, resolved against this record.
		narrow, err := f.lookupNarrowing(body, current, p.Lookup)
		if err != nil {
			return fmt.Errorf("child field %q: %w", fld.Name, err)
		}

		lookupModule, lookupEntity := splitEntityRef(p.Lookup.Entity)
		if lookupModule == "" || lookupEntity == "" {
			return fmt.Errorf("child field %q: lookup.entity %q is not a valid entity reference", fld.Name, p.Lookup.Entity)
		}
		store, err := f.registry.GetEntityStore(lookupModule, lookupEntity)
		if err != nil {
			return fmt.Errorf("child field %q: cannot resolve %s: %w", fld.Name, p.Lookup.Entity, err)
		}

		for j, raw := range rows {
			row, ok := raw.(map[string]any)
			if !ok {
				return fmt.Errorf("child field %q row %d: row is not an object", fld.Name, j+1)
			}
			ref := createScopeString(row[p.Map.RefField])
			if ref == "" {
				// A row the picker cannot address has no source to read from.
				// Refused rather than left as sent: a row whose value came from
				// nowhere is exactly the hole this closes.
				return fmt.Errorf("child field %q row %d: %s is empty — every row must name the record its value comes from",
					fld.Name, j+1, p.Map.RefField)
			}
			if frozen, ok := stored[ref]; ok {
				row[p.Map.LookupField] = frozen
				continue
			}
			if _, clash := narrow[p.Lookup.Key]; clash {
				return fmt.Errorf("child field %q: lookup.key and a lookup.scope entry both name %q — the match and the narrowing would overwrite each other",
					fld.Name, p.Lookup.Key)
			}

			filters := make(map[string]db.FilterOp, len(narrow)+1)
			for k, v := range narrow {
				filters[k] = v
			}
			filters[p.Lookup.Key] = db.FilterOp{Op: "eq", Value: ref}

			res, err := store.List(ctx, db.ListParams{
				WorkspaceID: workspaceFromContext(ctx),
				Page:        1,
				PerPage:     2, // two is enough to detect an ambiguous catalog
				Filters:     filters,
			})
			if err != nil {
				return fmt.Errorf("child field %q row %d: %w", fld.Name, j+1, err)
			}
			switch len(res.Data) {
			case 0:
				return fmt.Errorf("child field %q row %d: no %s row matches %s=%s%s — refusing to take the %s from the request",
					fld.Name, j+1, p.Lookup.Entity, p.Lookup.Key, ref, describeNarrowing(narrow), p.Lookup.Field)
			case 1:
				value, ok := res.Data[0].Data[p.Lookup.Field]
				if !ok || value == nil {
					return fmt.Errorf("child field %q row %d: %s row carries no %q",
						fld.Name, j+1, p.Lookup.Entity, p.Lookup.Field)
				}
				row[p.Map.LookupField] = value
			default:
				return fmt.Errorf("child field %q row %d: %s has more than one row for %s=%s%s — the lookup must be unambiguous",
					fld.Name, j+1, p.Lookup.Entity, p.Lookup.Key, ref, describeNarrowing(narrow))
			}
		}
		body[fld.Name] = rows
	}
	return nil
}

// validatePickerQuantities enforces the bounds of a picked row's quantity.
//
// The bounds belong to the PICKER construct rather than to one application, which
// is why they are enforced here instead of being left to each manifest:
//
//   - the floor is 1, always. A picked row exists because something was picked;
//     zero of it is "not picked", which the client expresses by REMOVING the row.
//     A stored 0 or negative is not a smaller order — it is a row that says
//     nothing, and since the row's amount is quantity × value it is also a way to
//     corrupt every total derived from it.
//   - the ceiling is `map.max_quantity` WHEN DECLARED. No ceiling is invented when
//     it is absent: that would be a rule out of nowhere, and the declared bound is
//     the one the manifest author chose and the UI clamps to.
//
// Measured before this (kafe): `clampQuantity` bounded the quantity in the BROWSER
// only, so a direct API caller — or any client that skips the picker — could store
// any number, while `line_total = quantity × unit_price_snapshot` fed the order
// total, its tax and the journal. `max_quantity` was validated as a manifest
// requirement and then never enforced.
//
// A value that is not a whole number is refused rather than truncated: silently
// turning 2.5 into 2 changes what the caller asked for, and "how many" has no
// fractional answer.
//
// An ABSENT value is left alone — the field's own `default` fills it, and
// `required` covers the case where it does not. Absence is therefore already
// governed by declarations the manifest author wrote; refusing it here would add a
// second, invisible rule.
func validatePickerQuantities(fieldName string, p *spec.PickerDecl, rows []any) error {
	qf := p.Map.QuantityField
	if qf == "" {
		// The picker declares no quantity: each pick is one row (a checklist).
		return nil
	}
	max := p.Map.MaxQuantity
	for i, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("child field %q row %d: row is not an object", fieldName, i+1)
		}
		value, present := row[qf]
		if !present || value == nil {
			continue
		}
		qty, ok := quantityAsInt(value)
		if !ok {
			return fmt.Errorf("child field %q row %d: %s must be a whole number, got %v", fieldName, i+1, qf, value)
		}
		if qty < 1 {
			return fmt.Errorf("child field %q row %d: %s must be at least 1, got %d — a picked row with no quantity is not a row", fieldName, i+1, qf, qty)
		}
		if max > 0 && qty > int64(max) {
			return fmt.Errorf("child field %q row %d: %s is at most %d (the picker's map.max_quantity), got %d", fieldName, i+1, qf, max, qty)
		}
	}
	return nil
}

// quantityAsInt reads a quantity as a whole number.
//
// Values arrive as float64 over JSON and as int/int64 when the payload was built
// in-process (scripts, seeds, tests), so all of those are accepted; a
// non-integral number is REFUSED rather than rounded, so the caller is told their
// value was not usable instead of quietly getting a different one.
func quantityAsInt(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case float32:
		return wholeFloat(float64(n))
	case float64:
		return wholeFloat(n)
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return i, true
		}
		return 0, false
	case string:
		if i, err := strconv.ParseInt(strings.TrimSpace(n), 10, 64); err == nil {
			return i, true
		}
		return 0, false
	default:
		return 0, false
	}
}

// wholeFloat accepts a float that is exactly a whole number.
func wholeFloat(f float64) (int64, bool) {
	if math.IsNaN(f) || math.IsInf(f, 0) || f != math.Trunc(f) {
		return 0, false
	}
	return int64(f), true
}

// lookupNarrowing resolves a lookup's `scope` into query filters.
//
// Two kinds of entry, and they are read differently on purpose:
//
//   - a literal (`value`) is a manifest constant, so it narrows the query as-is
//     (with the operator's own value shape);
//   - a sourced entry (`from: session|route`) names a field THIS record carries,
//     and the record's value is used. It is the value the row will be stored
//     with — already validated by whatever pinned the dimension — and matching
//     the lookup by anything else could produce a value from a dimension the
//     record does not belong to.
//
// A sourced entry whose field is not set is an ERROR, not a skipped check:
// resolving the lookup without it would match an arbitrary row, which is the
// failure this whole mechanism exists to prevent.
func (f *HandlerFactory) lookupNarrowing(body, current map[string]any, lk *spec.PickerLookup) (map[string]db.FilterOp, error) {
	out := make(map[string]db.FilterOp, len(lk.Scope))
	for i := range lk.Scope {
		sc := &lk.Scope[i]
		if sc.Field == "" {
			continue
		}
		op := sc.Op
		if op == "" {
			op = "eq"
		}
		if sc.From == "" {
			value, err := scopeLiteralValue(op, sc.Value)
			if err != nil {
				return nil, fmt.Errorf("lookup.scope on %s: %w", sc.Field, err)
			}
			out[sc.Field] = db.FilterOp{Op: op, Value: value}
			continue
		}
		// A sourced entry is matched against the record's own value, so only an
		// equality is meaningful — a range on a single scalar would be a
		// declaration that cannot be satisfied.
		if op != "eq" {
			return nil, fmt.Errorf("lookup.scope on %s: operator %q cannot be matched against the record's own value — use a literal `value` for anything but equality", sc.Field, op)
		}
		value := createScopeString(body[sc.Field])
		if value == "" && current != nil {
			value = createScopeString(current[sc.Field])
		}
		if value == "" {
			return nil, fmt.Errorf("lookup %s is narrowed by %q (from: %s) but this record carries none — set the field, or the lookup cannot be matched to the record it belongs to",
				lk.Entity, sc.Field, sc.From)
		}
		out[sc.Field] = db.FilterOp{Op: "eq", Value: value}
	}
	return out, nil
}

// describeNarrowing renders the narrowing half of an error message, sorted so
// the same situation always reads the same way.
func describeNarrowing(narrow map[string]db.FilterOp) string {
	if len(narrow) == 0 {
		return ""
	}
	keys := make([]string, 0, len(narrow))
	for k := range narrow {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := ""
	for _, k := range keys {
		parts += fmt.Sprintf(", %s=%v", k, narrow[k].Value)
	}
	return parts
}
