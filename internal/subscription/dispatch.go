package subscription

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/primadi/formspec/internal/action"
	"github.com/primadi/formspec/internal/stream"
	"github.com/primadi/formspec/pkg/spec"
)

// Dispatcher dispatches emitted events to matching subscription handlers.
//
// A subscription's `handler` is an ImplDecl — the implementation of the
// handler itself (script_ref, native, compiled, sidecar), dispatched through
// the action dispatcher exactly like an action impl or hook impl. The event
// payload is passed as the handler's params, with the originating event
// metadata under the reserved `_event` key.
//
// Tier 2 (todo 7.3.2): when a Stream backend is wired via SetStream,
// subscriptions with `durability: durable` append the event to the stream
// (named by the fully-qualified event) instead of dispatching directly — the
// StreamingWorker consumes it with at-least-once, positioned replay,
// filter/transform, retry and dead-letter.
type Dispatcher struct {
	reg        *Registry
	dispatcher *action.Dispatcher
	stream     stream.Stream
}

// NewDispatcher creates a subscription dispatcher bound to the given
// subscription registry and action dispatcher.
func NewDispatcher(reg *Registry, dispatcher *action.Dispatcher) *Dispatcher {
	return &Dispatcher{reg: reg, dispatcher: dispatcher}
}

// SetStream wires the Tier 2 stream backend (todo 7.3.2). When set,
// subscriptions with durability: durable append events to the stream instead
// of dispatching directly.
func (d *Dispatcher) SetStream(s stream.Stream) {
	d.stream = s
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
		// Tier 2 (durable): append to the stream; the StreamingWorker
		// consumes it. Tier 1: dispatch directly.
		if d.stream != nil && sub.Durable == "durable" {
			if err := d.appendToStream(ctx, workspaceID, eventName, resource, payload, sub); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", sub.Handler.Ref, err))
			}
			continue
		}
		if err := d.dispatchOne(ctx, workspaceID, eventName, resource, payload, sub); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", sub.Handler.Ref, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("subscription dispatch: %s", strings.Join(errs, "; "))
	}
	return nil
}

// appendToStream appends an event to the durable subscription's stream. The
// entry carries the delivery metadata (workspace, resource, event, occurred_at)
// plus the wire payload so the StreamingWorker can rebuild the dispatch
// context and the filter/transform environment.
func (d *Dispatcher) appendToStream(ctx context.Context, workspaceID, eventName, resource string, payload map[string]any, sub *spec.SubscriptionSpec) error {
	if d.stream == nil {
		return fmt.Errorf("durable subscription %s has no stream backend configured", sub.Handler.Ref)
	}
	streamName := stream.NormalizeStreamName(eventName)
	data := map[string]any{
		"workspace_id": workspaceID,
		"resource":     resource,
		"event":        eventName,
		"payload":      payload,
		"occurred_at":  time.Now().UTC().Format(time.RFC3339Nano),
	}
	if _, err := d.stream.Append(ctx, streamName, data); err != nil {
		return fmt.Errorf("append to stream %s: %w", streamName, err)
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
