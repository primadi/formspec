// Package action — Condition Evaluator
//
// This file evaluates action conditions (state-level validation, spec §13).
// Conditions are Starlark expressions that gate action execution:
//
//	conditions:
//	  - script: "resource.status == 'draft'"
//	    message: "Order yang sudah checkout tidak bisa diedit"
//	  - script: "not customer.load(resource.field.customer_id).field.is_blacklisted"
//	    message: "Customer diblokir, tidak bisa checkout"
//
// The expression is evaluated with the resource and params injected as variables.
// If any condition returns false, the action is blocked with the condition's message.
package action

import (
	"fmt"
	"strings"

	"github.com/forma/forma/internal/starlark"
	"github.com/forma/forma/pkg/spec"
)

// EvaluateConditions checks all conditions on an action against the current resource data.
// Returns nil if all conditions pass, or an error with the first failing condition's message.
func EvaluateConditions(conditions []spec.ConditionDecl, resourceData map[string]any, params map[string]any) error {
	for _, cond := range conditions {
		if err := evaluateCondition(cond, resourceData, params); err != nil {
			return err
		}
	}
	return nil
}

// evaluateCondition evaluates a single condition.
func evaluateCondition(cond spec.ConditionDecl, resourceData map[string]any, params map[string]any) error {
	// Determine the expression to evaluate
	expr := cond.Expression
	if expr == "" {
		expr = cond.Script // backward compat
	}
	if expr == "" {
		// Field + expression shorthand: {field: "status", expression: "== 'draft'"}
		if cond.Field != "" && cond.Expression != "" {
			expr = cond.Expression
			// Wrap field access: if expression is "== 'draft'", make it "resource.field_name == 'draft'"
			if !strings.Contains(expr, "resource.") {
				expr = fmt.Sprintf("resource_%q %s", cond.Field, expr)
				// This is too complex for simple wrapping — use the script value directly
			}
		}
	}
	if expr == "" {
		return nil // no condition to evaluate
	}

	// Build env with resource data as top-level variables
	env := make(map[string]any, len(resourceData)+len(params)+1)
	for k, v := range resourceData {
		env[k] = v
	}
	for k, v := range params {
		env["param_"+k] = v // avoid name collision with resource fields
	}
	// Also inject the whole resource as a nested map for "resource.field" access
	env["resource"] = resourceData
	env["params"] = params

	// Evaluate via Starlark expression evaluator
	result, err := starlark.EvalExpr(expr, env)
	if err != nil {
		return fmt.Errorf("condition evaluation error: %w", err)
	}

	// Result must be true (boolean) or truthy
	passed := false
	switch v := result.(type) {
	case bool:
		passed = v
	case int64:
		passed = v != 0
	case float64:
		passed = v != 0.0
	default:
		// Truthy by default for non-empty results
		passed = result != nil
	}

	if !passed {
		msg := cond.Message
		if msg == "" {
			msg = fmt.Sprintf("condition failed: %s", expr)
		}
		return fmt.Errorf("%s", msg)
	}

	return nil
}
