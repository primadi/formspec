package subscription

import (
	"context"
	"fmt"
	"strings"

	"github.com/primadi/formspec/internal/action"
	"github.com/primadi/formspec/pkg/spec"
)

// Dispatcher dispatches emitted events to matching subscription handlers.
//
// A subscription's `handler` is an ImplDecl — the implementation of the
// handler itself (script_ref, native, compiled, sidecar), dispatched through
// the action dispatcher exactly like an action impl or hook impl. The event
// payload is passed as the handler's params, with the originating event
// metadata under the reserved `_event` key.
type Dispatcher struct {
	reg        *Registry
	dispatcher *action.Dispatcher
}

// NewDispatcher creates a subscription dispatcher bound to the given
// subscription registry and action dispatcher.
func NewDispatcher(reg *Registry, dispatcher *action.Dispatcher) *Dispatcher {
	return &Dispatcher{reg: reg, dispatcher: dispatcher}
}

// Dispatch delivers an emitted event to every subscription that subscribes to
// it. The event name is the fully-qualified resource event (e.g.
// "billing.invoice.on_submit"). Each matching subscription's handler is
// invoked with the event payload as its params.
//
// Errors are aggregated: a failure in one subscription handler does not stop
// the others from running. The caller (outbox worker) decides whether a
// returned error is permanent (dead-letter) or retryable.
func (d *Dispatcher) Dispatch(ctx context.Context, workspaceID, eventName, resource string, payload map[string]any) error {
	if d.reg == nil {
		return nil
	}
	subs := d.reg.ForEvent(eventName)
	if len(subs) == 0 {
		return nil
	}

	var errs []string
	for _, sub := range subs {
		if err := d.dispatchOne(ctx, workspaceID, eventName, resource, payload, sub); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", sub.Handler.Ref, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("subscription dispatch: %s", strings.Join(errs, "; "))
	}
	return nil
}

// dispatchOne invokes a single subscription's handler (an ImplDecl) via the
// action dispatcher.
func (d *Dispatcher) dispatchOne(ctx context.Context, workspaceID, eventName, resource string, payload map[string]any, sub *spec.SubscriptionSpec) error {
	if d.dispatcher == nil {
		return fmt.Errorf("action dispatcher not wired")
	}
	if sub.Handler.Type == "" {
		return fmt.Errorf("subscription handler has no impl type")
	}

	// The event payload becomes the handler's params. Merge the event
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

	// A subscription handler is a stateless computation — model it as a
	// synthetic action whose Impl is the subscription's handler, dispatched
	// through the same uniform path as service/entity actions.
	actionSpec := spec.Action{
		Name: "handle",
		Impl: &sub.Handler,
	}
	execParams := action.ExecuteParams{
		Module:      "subscription",
		Entity:      "subscription",
		ActionName:  "handle",
		Params:      params,
		WorkspaceID: workspaceID,
	}

	_, err := d.dispatcher.Dispatch(ctx, actionSpec, execParams)
	return err
}
