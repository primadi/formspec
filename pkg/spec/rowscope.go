package spec

import "fmt"

// FilterOperators is the closed set of filter operators the storage layer can
// express (see renderers/jsonb-persist `filterSQL` — the single implementation
// behind client filters, entity `row_scope`, and grant row scopes).
//
// It exists so validation can REFUSE an operator the query builder does not
// know: an unknown op produces no clause, and for a row restriction "no clause"
// means "no restriction" — a filter that looks like protection while providing
// none. That is the same failure class as a misspelled `row_scope` key, so the
// two are checked together.
var FilterOperators = map[string]bool{
	"eq": true, "neq": true, "gt": true, "gte": true, "lt": true, "lte": true,
	"like": true, "ilike": true,
	"in": true, "nin": true, "between": true,
	"null": true, "notnull": true,
	"descendant_of": true, "child_of": true, "root": true,
}

// valuelessFilterOperators is the subset that takes NO value at all. Keeping
// them in their own set is what lets "exactly one value source" be stated
// without forbidding `op: notnull`, which has no value by definition.
var valuelessFilterOperators = map[string]bool{
	"null": true, "notnull": true, "root": true,
}

// IsValuelessFilterOperator reports whether an operator carries no value.
func IsValuelessFilterOperator(op string) bool { return valuelessFilterOperators[op] }

// ValidateCreateScopeFilters checks an entity's `create_scope` declarations.
//
// The rules that matter are the ones whose violation would be silent: a
// declaration naming a field the entity does not have would never fire, and one
// whose `via`/`via_field` cannot be resolved would either deny every create or,
// worse, compare against nothing and pass.
//
// `fieldExists` may be nil (no schema at hand), which relaxes the existence
// check — never the shape check, exactly like ValidateRowScopeFilters.
func ValidateCreateScopeFilters(origin string, decls []CreateScopeSpec, fieldExists func(string) bool) error {
	seen := make(map[string]bool, len(decls))
	for i := range decls {
		cs := &decls[i]
		if cs.Field == "" {
			return fmt.Errorf("%s[%d]: field is required — a create scope that names no field pins nothing", origin, i)
		}
		if fieldExists != nil && !fieldExists(cs.Field) {
			return fmt.Errorf("%s[%d]: field %q is not declared on this entity", origin, i, cs.Field)
		}
		if seen[cs.Field] {
			return fmt.Errorf("%s[%d]: field %q is declared twice — the second check could contradict the first", origin, i, cs.Field)
		}
		seen[cs.Field] = true

		// `from: record` is the only source defined. It is accepted and also
		// implied when omitted, so a manifest can be terse; anything else is
		// refused rather than ignored (an unknown source would pin nothing).
		switch cs.From {
		case "", "record":
		default:
			return fmt.Errorf("%s[%d] (%s): from must be \"record\", got %q", origin, i, cs.Field, cs.From)
		}
		if cs.RefField == "" {
			return fmt.Errorf("%s[%d] (%s): ref_field is required — it is the payload field whose record supplies the value", origin, i, cs.Field)
		}
		if fieldExists != nil && !fieldExists(cs.RefField) {
			return fmt.Errorf("%s[%d] (%s): ref_field %q is not declared on this entity", origin, i, cs.Field, cs.RefField)
		}
		if cs.Via == "" {
			return fmt.Errorf("%s[%d] (%s): via is required — name the entity the ref_field points at", origin, i, cs.Field)
		}
		if _, ok := NormalizeEntityRef(cs.Via); !ok {
			return fmt.Errorf("%s[%d] (%s): via %q must be \"<module>.<entity>\"", origin, i, cs.Field, cs.Via)
		}
		if cs.ViaField == "" {
			return fmt.Errorf("%s[%d] (%s): via_field is required — name the field on %s holding the expected value", origin, i, cs.Field, cs.Via)
		}
	}
	return nil
}

// ValidateRowScopeFilters checks a list of SERVER-ENFORCED filters — an entity's
// `row_scope`, or a role grant action's `row_scope`.
//
// The rules are the ones whose violation is invisible at runtime:
//
//   - `field` must be named, and (when the caller has a schema) must exist;
//   - `op` must be in the closed set the query builder implements;
//   - exactly ONE value source: `from: session`, `from: route`, or the literal
//     `value` — except for the valueless operators, which need none. `from`
//     together with `value` is refused as well: the field would be resolved
//     twice and the manifest would not say which wins.
//
// `origin` names the declaration in the error message ("row_scope" for an
// entity, "grant row_scope on order-page/list" for a grant) so an operator can
// tell WHICH declaration is wrong. `fieldExists` may be nil when no schema is at
// hand — absence of a schema relaxes the existence check, never the source rule.
func ValidateRowScopeFilters(origin string, filters []FilterSpec, fieldExists func(string) bool) error {
	for i := range filters {
		sc := &filters[i]
		if sc.Field == "" {
			return fmt.Errorf("%s[%d]: field is required", origin, i)
		}
		if fieldExists != nil && !fieldExists(sc.Field) {
			return fmt.Errorf("%s[%d]: field %q is not declared on this entity", origin, i, sc.Field)
		}
		if sc.Op != "" && !FilterOperators[sc.Op] {
			return fmt.Errorf("%s[%d] (%s): unknown operator %q — the storage layer cannot express it, so the filter would not hold", origin, i, sc.Field, sc.Op)
		}

		valueless := IsValuelessFilterOperator(sc.Op)
		if sc.From != "" && sc.Value != "" {
			return fmt.Errorf("%s[%d] (%s): `from: %s` and a literal `value` are mutually exclusive — pick one value source", origin, i, sc.Field, sc.From)
		}

		switch sc.From {
		case "session":
			// `attr` optional: empty means the entity's scope field, else
			// principal_id.
			if sc.Via != "" {
				return fmt.Errorf("%s[%d] (%s): `via` only applies to `from: route` — a session attribute is not a reference to a record", origin, i, sc.Field)
			}
		case "route":
			// `param` optional: empty means the field name. `via` (optional)
			// turns the parameter into a REFERENCE: the value is read from the
			// named record instead of trusted from the query string (10.76).
			if sc.Via != "" {
				if _, ok := NormalizeEntityRef(sc.Via); !ok {
					return fmt.Errorf("%s[%d] (%s): via %q must be \"<module>.<entity>\"", origin, i, sc.Field, sc.Via)
				}
				if sc.ViaField == "" {
					return fmt.Errorf("%s[%d] (%s): via_field is required with `via` — name the field on %s holding the scope value", origin, i, sc.Field, sc.Via)
				}
			} else if sc.ViaField != "" {
				return fmt.Errorf("%s[%d] (%s): via_field is meaningless without `via`", origin, i, sc.Field)
			}
		case "":
			if sc.Via != "" || sc.ViaField != "" {
				return fmt.Errorf("%s[%d] (%s): `via` applies to `from: route` only", origin, i, sc.Field)
			}
			// Literal value — this is the value source that makes a manifest
			// constant expressible (kafe 10.67: only paid orders reach the
			// kitchen). Without one the entry would silently not filter, so an
			// empty one is rejected rather than accepted as a no-op.
			if !valueless && sc.Value == "" {
				return fmt.Errorf("%s[%d] (%s): needs a value source — set `from: session|route`, or a literal `value`", origin, i, sc.Field)
			}
		default:
			return fmt.Errorf("%s[%d] (%s): from must be \"session\", \"route\", or empty (literal `value`), got %q", origin, i, sc.Field, sc.From)
		}
	}
	return nil
}
