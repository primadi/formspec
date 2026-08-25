package integrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/primadi/formspec/internal/action"
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/service"
	"github.com/primadi/formspec/pkg/spec"
)

// Dispatcher bridges emitted events to matching integrator target actions
// (02-core-extended.md §5). When an event fires, every integrator listening
// to it invokes its target action (`call.resource` + `call.action`) via the
// action dispatcher. The target may be an Entity action or a Service action.
type Dispatcher struct {
	reg        *Registry
	entityReg  *entity.Registry
	svcReg     *service.Registry
	dispatcher *action.Dispatcher
}

// NewDispatcher creates an integrator dispatcher bound to the given
// integrator registry, entity registry, service registry, and action
// dispatcher.
func NewDispatcher(reg *Registry, entityReg *entity.Registry, svcReg *service.Registry, dispatcher *action.Dispatcher) *Dispatcher {
	return &Dispatcher{reg: reg, entityReg: entityReg, svcReg: svcReg, dispatcher: dispatcher}
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
// or a Service action.
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

	// Try Entity action first, then Service action.
	if d.entityReg != nil {
		if actionSpec, ok := d.entityReg.GetActionSpec(targetModule, targetName, it.Call.Action); ok {
			_, err := d.dispatcher.Dispatch(ctx, *actionSpec, action.ExecuteParams{
				Module:      targetModule,
				Entity:      targetName,
				ActionName:  it.Call.Action,
				Params:      params,
				WorkspaceID: workspaceID,
			})
			return err
		}
	}
	if d.svcReg != nil {
		if actionSpec, ok := d.svcReg.GetAction(targetModule, targetName, it.Call.Action); ok {
			_, err := d.dispatcher.Dispatch(ctx, *actionSpec, action.ExecuteParams{
				Module:      targetModule,
				Entity:      targetName,
				ActionName:  it.Call.Action,
				Params:      params,
				WorkspaceID: workspaceID,
			})
			return err
		}
	}

	return fmt.Errorf("target action %s.%s.%s not found", targetModule, targetName, it.Call.Action)
}

// splitResourceRef splits "{module}.{entity}" into module and entity.
func splitResourceRef(ref string) (module, entity string, ok bool) {
	parts := strings.Split(ref, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}