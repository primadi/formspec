package auth

import (
	"fmt"

	"github.com/primadi/formspec/internal/starlark"
)

// EvaluateGrantConditions evaluates ABAC conditions attached to an action
// grant (todo 6.2.6). Each condition's Expr is a FormSpecExpr evaluated
// against `resource` (the data being submitted) + `params`. If any condition
// evaluates false, the action is rejected with the condition's Message.
//
// This is the enforcement primitive for attribute-based authorization: a role
// may grant an action only under certain attribute constraints (e.g. a branch
// code from the resource data or the caller's membership attributes).
func EvaluateGrantConditions(conditions []ConditionGrant, resourceData, params map[string]any) error {
	for _, c := range conditions {
		if c.Expr == "" {
			continue
		}
		env := map[string]any{}
		if resourceData != nil {
			env["resource"] = resourceData
		}
		if params != nil {
			env["params"] = params
		}
		result, err := starlark.EvalExpr(c.Expr, env)
		if err != nil {
			return fmt.Errorf("abac: evaluate condition %q: %w", c.Expr, err)
		}
		if !truthy(result) {
			msg := c.Message
			if msg == "" {
				msg = "action not permitted by attribute condition"
			}
			return fmt.Errorf("abac: %s", msg)
		}
	}
	return nil
}

// truthy reports whether a Starlark evaluation result is truthy.
func truthy(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case nil:
		return false
	default:
		return true
	}
}
