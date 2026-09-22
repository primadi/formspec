package starlark

import (
	"fmt"

	"go.starlark.net/starlark"
)

// childRows normalizes a child collection value to []any, reporting whether it
// was a collection at all.
//
// A child field reaches a guard in one of two shapes depending on which side of
// the persistence layer produced it:
//   - []any            — the shape a client sends and the shape that lives in
//     the parent's JSONB
//   - []map[string]any — the shape ChildStore.Hydrate produces when children
//     live in their own table (storage: table)
//
// Both are ordinary child collections to a guard, so both must be accepted.
// Accepting only []any made every guard over a table-stored child silently sum
// to zero — the type assertion failed, the pre-computed helpers were never
// injected, and the guard reported "not balanced" for data that balanced.
func childRows(v any) ([]any, bool) {
	switch rows := v.(type) {
	case []any:
		return rows, true
	case []map[string]any:
		out := make([]any, len(rows))
		for i, r := range rows {
			out[i] = r
		}
		return out, true
	default:
		return nil, false
	}
}

// newSumLineBuiltin builds the `sum_line(field)` aggregate available in guard
// expressions. It sums one field across the record's `lines` child collection,
// money-aware: a column of money values totals to money (so debit can be
// compared with credit), a column of plain numbers totals to a number. A
// missing field and an empty collection both sum to 0 — a guard asking for a
// sum that isn't there should see zero, not an error.
func newSumLineBuiltin(resourceData map[string]any) *starlark.Builtin {
	return starlark.NewBuiltin("sum_line", func(
		_ *starlark.Thread,
		_ *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var field string
		if err := starlark.UnpackArgs("sum_line", args, kwargs, "field", &field); err != nil {
			return nil, err
		}

		lines, _ := childRows(resourceData["lines"])
		values := make([]starlark.Value, 0, len(lines))
		for _, l := range lines {
			line, ok := l.(map[string]any)
			if !ok {
				continue
			}
			raw, ok := line[field]
			if !ok || raw == nil {
				continue
			}
			sv, err := toStarlark(raw)
			if err != nil {
				return nil, fmt.Errorf("sum_line(%q): %w", field, err)
			}
			values = append(values, sv)
		}
		if len(values) == 0 {
			return starlark.Float(0), nil
		}
		return sumValues(starlark.NewList(values))
	})
}

// EvaluateGuard evaluates a state-machine guard expression against resource
// data, returning whether the guard passed and an optional failure message.
//
// This is the SINGLE shared implementation of guard evaluation used by both:
//   - internal/entity/state_machine.go (StateMachineEngine.CanTransition) — the
//     full engine used by HandleCustomAction, and
//   - renderers/jsonb-persist/crud.go (EntityStore.validateStateTransition) —
//     the transition check applied during Update.
//
// Unifying them here (todo 7.5.4) removes the duplicated env-building and
// sum_line/len helper injection so both paths behave identically.
//
// The guard expression has access to resource data fields directly, plus
// pre-computed helpers:
//   - `resource` / `data` — the resource data map, so both dot notation
//     (resource.amount) and bracket notation (resource["amount"]) work
//   - `sum_line(field)` — sums one numeric/money field over the `lines` child
//     collection (the documented form, 02-core-extended.md §1); `sum_line_<field>`
//     is the equivalent pre-computed identifier
//   - `len(resource.items)` / `len(resource.lines)` — child-collection sizes,
//     also available as `item_count` / `line_count`
//
// A nil guard or empty expression passes trivially.
func EvaluateGuard(expression string, resourceData map[string]any) (bool, string, error) {
	if expression == "" {
		return true, "", nil
	}

	// Build evaluation environment with pre-computed helpers.
	env := make(map[string]any, len(resourceData)+5)
	for k, v := range resourceData {
		env[k] = v
	}
	// `resource` / `data` are FieldMap values so BOTH dot notation
	// (resource.amount) and bracket notation (resource["amount"]) work in
	// guard/when expressions.
	env["resource"] = NewFieldMap(resourceData)
	env["data"] = NewFieldMap(resourceData)

	// `sum_line(field)` — the documented aggregate builtin over the `lines`
	// child collection (02-core-extended.md §1). It sums whole money values
	// money-aware (a list of money sums to money, so `sum_line('debit') ==
	// sum_line('credit')` compares money to money), falling back to a plain
	// field walk when the collection holds scalars.
	env["sum_line"] = newSumLineBuiltin(resourceData)

	// Pre-compute sum_line helpers for GL-style guards.
	if lineList, ok := childRows(resourceData["lines"]); ok {
		for field, total := range computeSums(lineList) {
			env["sum_line_"+field] = total
		}
	}

	// Pre-compute len() helpers.
	if items, ok := childRows(resourceData["items"]); ok {
		env["item_count"] = int64(len(items))
	}
	if lines, ok := childRows(resourceData["lines"]); ok {
		env["line_count"] = int64(len(lines))
	}

	result, err := EvalExpr(expression, env)
	if err != nil {
		return false, "", fmt.Errorf("guard expression %q: %w", expression, err)
	}

	passed := false
	switch v := result.(type) {
	case bool:
		passed = v
	case int64:
		passed = v != 0
	case float64:
		passed = v != 0.0
	default:
		passed = result != nil
	}

	return passed, "", nil
}

// computeSums pre-computes field sums for child arrays (used by GL-style
// guards). It sums numeric fields across every element of a child array.
func computeSums(lineList []any) map[string]float64 {
	sums := make(map[string]float64)
	for _, l := range lineList {
		line, ok := l.(map[string]any)
		if !ok {
			continue
		}
		for field, v := range line {
			switch n := v.(type) {
			case float64:
				sums[field] += n
			case int:
				sums[field] += float64(n)
			case int64:
				sums[field] += float64(n)
			}
		}
	}
	return sums
}
