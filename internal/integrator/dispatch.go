package integrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/primadi/formspec/internal/action"
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/service"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// Dispatcher bridges emitted events to matching integrator target actions
// (02-core-extended.md §5). When an event fires, every integrator listening
// to it invokes its target action (`call.resource` + `call.action`) via the
// action dispatcher. The target may be an Entity action or a Service action.
//
// When a saga store is wired and the integrator declares a `compensate`, the
// cross-boundary call is registered to the saga log; on dispatch failure the
// compensate action is invoked (todo 7.7.4).
type Dispatcher struct {
	reg        *Registry
	entityReg  *entity.Registry
	svcReg     *service.Registry
	dispatcher *action.Dispatcher
	saga       *db.SagaStore
}

// NewDispatcher creates an integrator dispatcher bound to the given
// integrator registry, entity registry, service registry, and action
// dispatcher. saga is optional — when non-nil, cross-boundary calls with a
// declared compensate are registered to the saga log (todo 7.7.4).
func NewDispatcher(reg *Registry, entityReg *entity.Registry, svcReg *service.Registry, dispatcher *action.Dispatcher, saga ...*db.SagaStore) *Dispatcher {
	d := &Dispatcher{reg: reg, entityReg: entityReg, svcReg: svcReg, dispatcher: dispatcher}
	if len(saga) > 0 {
		d.saga = saga[0]
	}
	return d
}

// Dispatch delivers an emitted event to every integrator that listens to it.
// The event name is the fully-qualified resource event (e.g.
// "billing.invoice.on_submit"). Each matching integrator invokes its target
// action with the event payload as params.
//
// Errors are aggregated: a failure in one integrator does not stop the others
// from running. The caller (outbox worker) decides whether a returned error is
// permanent (dead-letter) or retryable.
func (d *Dispatcher) Dispatch(ctx context.Context, workspaceID, eventName, resource string, payload map[string]any) error {
	if d.reg == nil {
		return nil
	}
	its := d.reg.ForEvent(eventName)
	if len(its) == 0 {
		return nil
	}

	var errs []string
	for _, it := range its {
		if err := d.dispatchOne(ctx, workspaceID, eventName, resource, payload, it); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", it.Call.Resource+"."+it.Call.Action, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("integrator dispatch: %s", strings.Join(errs, "; "))
	}
	return nil
}

// dispatchOne invokes a single integrator's target action — an Entity action
// or a Service action. When the integrator declares a `compensate` and a saga
// store is wired, the cross-boundary call is registered to the saga log; on
// failure the compensate action is invoked (todo 7.7.4).
func (d *Dispatcher) dispatchOne(ctx context.Context, workspaceID, eventName, resource string, payload map[string]any, it *spec.IntegratorSpec) error {
	if d.dispatcher == nil {
		return fmt.Errorf("action dispatcher not wired")
	}
	if it.Call == nil {
		return fmt.Errorf("integrator has no call declaration")
	}

	// Resolve the target resource/action from call.resource ({module}.{entity})
	// and call.action.
	targetModule, targetName, ok := splitResourceRef(it.Call.Resource)
	if !ok {
		return fmt.Errorf("invalid call.resource: %q", it.Call.Resource)
	}

	// The event payload becomes the target action's params. Merge the event
	// metadata (name, resource) under a reserved key so the handler can
	// inspect the originating event.
	params := make(map[string]any, len(payload)+1)
	for k, v := range payload {
		params[k] = v
	}
	params["_event"] = map[string]any{
		"name":     eventName,
		"resource": resource,
	}

	// When the integrator declares a `map:`, it REPLACES the default
	// payload-passthrough: the target's params are built from the mapping
	// (S6). The two sides rarely share field names, and the mapping is domain
	// knowledge (e.g. "revenue → credit 4-1000") that belongs in the manifest,
	// not in a script owned by the target module.
	if len(it.Call.Map) > 0 {
		mapped := applyCallMap(it.Call.Map, payload)
		// Keep the event metadata available to the target handler.
		mapped["_event"] = params["_event"]
		params = mapped
	}

	// Register the cross-boundary call to the saga log when a compensate is
	// declared (todo 7.7.4).
	var sagaID string
	if d.saga != nil && it.Compensate != "" {
		id, err := d.saga.Register(ctx, workspaceID, eventName, it.Call.Resource+"."+it.Call.Action, it.Compensate)
		if err != nil {
			return fmt.Errorf("saga register: %w", err)
		}
		sagaID = id
	}

	// Try Entity action first, then Service action.
	dispatchErr := d.dispatchTarget(ctx, workspaceID, targetModule, targetName, it.Call.Action, params)

	if sagaID != "" {
		if dispatchErr != nil {
			// Invoke the compensate action on the target resource.
			compErr := d.invokeCompensate(ctx, workspaceID, targetModule, targetName, it.Compensate, params)
			errMsg := dispatchErr.Error()
			if compErr != nil {
				errMsg = fmt.Sprintf("FORMSPEC.SAGA.COMPENSATE_FAILED: %v (original: %v)", compErr, dispatchErr)
			}
			_ = d.saga.MarkCompensated(ctx, sagaID, errMsg)
			return fmt.Errorf("%s", errMsg)
		}
		_ = d.saga.MarkCompleted(ctx, sagaID)
	}

	return dispatchErr
}

// dispatchTarget dispatches the target action (entity or service).
func (d *Dispatcher) dispatchTarget(ctx context.Context, workspaceID, module, name, actionName string, params map[string]any) error {
	if d.entityReg != nil {
		if actionSpec, ok := d.entityReg.GetActionSpec(module, name, actionName); ok {
			_, err := d.dispatcher.Dispatch(ctx, *actionSpec, action.ExecuteParams{
				Module:      module,
				Entity:      name,
				ActionName:  actionName,
				Params:      params,
				WorkspaceID: workspaceID,
			})
			return err
		}
	}
	if d.svcReg != nil {
		if actionSpec, ok := d.svcReg.GetAction(module, name, actionName); ok {
			_, err := d.dispatcher.Dispatch(ctx, *actionSpec, action.ExecuteParams{
				Module:      module,
				Entity:      name,
				ActionName:  actionName,
				Params:      params,
				WorkspaceID: workspaceID,
			})
			return err
		}
	}
	return fmt.Errorf("target action %s.%s.%s not found", module, name, actionName)
}

// invokeCompensate invokes the compensate action on the target resource.
func (d *Dispatcher) invokeCompensate(ctx context.Context, workspaceID, module, name, compensate string, params map[string]any) error {
	return d.dispatchTarget(ctx, workspaceID, module, name, compensate, params)
}

// splitResourceRef splits "{module}.{entity}" into module and entity.
func splitResourceRef(ref string) (module, entity string, ok bool) {
	parts := strings.Split(ref, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// applyCallMap builds the target action's params from an integrator's `map:`
// declaration (S6), interpolating `{dotted.path}` templates against the source
// event payload. Nested maps and lists are interpolated recursively, so a
// `lines:` list of account entries can be built inline.
//
// A value that is EXACTLY one `{path}` token keeps the resolved value's type
// (so `debit: "{order.total_amount}"` yields the money object, not its string
// form); a value with surrounding text is stringified and interpolated.
func applyCallMap(m map[string]any, payload map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = interpolateValue(v, payload)
	}
	return out
}

// interpolateValue resolves templates in a single map value, recursing into
// maps and lists.
func interpolateValue(v any, payload map[string]any) any {
	switch t := v.(type) {
	case string:
		return interpolateString(t, payload)
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, vv := range t {
			out[k] = interpolateValue(vv, payload)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, vv := range t {
			out[i] = interpolateValue(vv, payload)
		}
		return out
	default:
		return v
	}
}

// interpolateString resolves `{dotted.path}` tokens in a string. When the whole
// string is a single token, the resolved value is returned with its original
// type; otherwise tokens are stringified and substituted in place. An
// unresolvable path is left verbatim (so a typo is visible in the payload
// rather than silently becoming an empty string).
func interpolateString(s string, payload map[string]any) any {
	if path, ok := singleToken(s); ok {
		if val, found := resolvePath(payload, path); found {
			return val
		}
		return s
	}
	out := s
	for {
		start := strings.Index(out, "{")
		if start < 0 {
			break
		}
		rel := strings.Index(out[start:], "}")
		if rel < 0 {
			break
		}
		end := start + rel
		path := out[start+1 : end]
		if val, found := resolvePath(payload, path); found {
			out = out[:start] + stringify(val) + out[end+1:]
		} else {
			// Leave the token verbatim; advance past it to avoid a loop.
			out = out[:end+1] + out[end+1:]
			break
		}
	}
	return out
}

// singleToken reports whether s is exactly one `{path}` token, returning the
// path.
func singleToken(s string) (string, bool) {
	if len(s) < 3 || s[0] != '{' || s[len(s)-1] != '}' {
		return "", false
	}
	inner := s[1 : len(s)-1]
	if strings.ContainsAny(inner, "{}") {
		return "", false
	}
	return inner, true
}

// resolvePath resolves a dot-path against the payload, returning the value and
// whether it was found.
func resolvePath(payload map[string]any, path string) (any, bool) {
	var cur any = payload
	for _, part := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// stringify renders a resolved value for in-place substitution.
func stringify(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
