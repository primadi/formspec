package starlark

import "fmt"

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
//   - `resource` / `data` — the resource data map
//   - `sum_line_<field>` — per-field sums over the `lines` child array
//   - `item_count` / `line_count` — lengths of the `items` / `lines` arrays
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
	env["resource"] = resourceData
	env["data"] = resourceData

	// Pre-compute sum_line helpers for GL-style guards.
	if lines, ok := resourceData["lines"]; ok {
		if lineList, ok := lines.([]any); ok {
			for field, total := range computeSums(lineList) {
				env["sum_line_"+field] = total
			}
		}
	}

	// Pre-compute len() helpers.
	if v, ok := resourceData["items"]; ok {
		if items, ok := v.([]any); ok {
			env["item_count"] = int64(len(items))
		}
	}
	if v, ok := resourceData["lines"]; ok {
		if lines, ok := v.([]any); ok {
			env["line_count"] = int64(len(lines))
		}
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
