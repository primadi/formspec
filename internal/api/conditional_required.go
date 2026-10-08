package api

import (
	"fmt"

	"github.com/primadi/formspec/internal/starlark"
	"github.com/primadi/formspec/pkg/spec"
)

// enforceConditionalRequired enforces a field's `required_when` on the write path.
//
// The declaration already existed in the schema and was already honoured by the
// renderer — and by NOTHING on the server. `formspec check` type-checks the
// expression, the client marks or hides the control, and the store's
// `validateRequired` only knows the static `required: true`. So a manifest could
// state "a manual discount must carry a reason" and a direct API call could ignore
// it entirely: measured on kafe, `manual_discount_amount: 40500` (90% of the order)
// was accepted with `manual_discount_reason` absent.
//
// That is the failure shape this codebase keeps removing — a declaration that
// READS like enforcement but has no enforcer. A conditional requirement is a data
// invariant, so it belongs where the writes are; and since the store has no
// expression evaluator (importing one would cycle), the write path that already
// owns the payload is where it lands. The limitation is stated rather than
// implied: a script writing the row through `resource.save()` does not pass here
// (the same shape recorded for 10.46/10.80).
//
// Three decisions, each with a reason:
//
//   - The MERGED view is what gets evaluated. The condition may mention a field
//     the PATCH did not resend ("the reason is required because the STORED
//     discount is set"), so judging the body alone would let a caller satisfy the
//     requirement just by omitting the trigger. Overlaying the stored record means
//     an already-satisfied requirement stays satisfied, and a request that sets the
//     trigger without the consequence fails.
//   - A non-boolean result is an ERROR. `required_when: "fields.x"` silently
//     meaning "truthy" would make a typo'd expression look like a working gate, and
//     a gate that passes by accident is worse than no gate.
//   - Presence follows the store's own rule: absent, nil, or empty string does not
//     satisfy a requirement. Zero DOES — it is a legitimate amount, and treating 0
//     as absence would make `required_when` unusable on money.
func (f *HandlerFactory) enforceConditionalRequired(es *spec.EntitySpec, body, current map[string]any) error {
	if es == nil {
		return nil
	}
	view := body
	if current != nil {
		view = make(map[string]any, len(current)+len(body))
		for k, v := range current {
			view[k] = v
		}
		for k, v := range body {
			view[k] = v
		}
	}

	for i := range es.Fields {
		fld := &es.Fields[i]
		if fld.RequiredWhen == "" {
			continue
		}
		required, err := starlark.EvalFormSpecBool(fld.RequiredWhen,
			starlark.FieldMapEnv("fields", view))
		if err != nil {
			return fmt.Errorf("field %q: required_when %q could not be evaluated: %w", fld.Name, fld.RequiredWhen, err)
		}
		if required && !fieldSatisfied(view[fld.Name]) {
			return fmt.Errorf("field %q is required when %s — this request leaves it empty", fld.Name, fld.RequiredWhen)
		}
	}
	return nil
}

// fieldSatisfied mirrors the store's presence rule (`validateRequired`).
func fieldSatisfied(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case string:
		return t != ""
	default:
		return true
	}
}
