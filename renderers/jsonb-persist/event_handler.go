package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/primadi/formspec/internal/events"
	"github.com/primadi/formspec/pkg/spec"
)

// SpecLookup resolves the delivery channels declared for a specific
// (resource, event name) pair by re-reading the live entity registry at
// delivery time — not by snapshotting channels into the outbox payload at
// enqueue time — so a hot-reloaded manifest fix is picked up by retries
// automatically. resource is "module/entity" (e.g. "clinic/visit").
type SpecLookup func(resource, eventName string) (channels []spec.EventDeliveryDecl, ok bool)

// SubscriptionDispatch delivers an emitted event to matching kind: Subscription
// handlers (todo 7.3). eventName is the fully-qualified resource event
// (e.g. "billing.invoice.on_submit"); resource is "module/entity". payload is
// the event's wire payload. A non-nil error is treated by the outbox worker
// as a delivery failure (retryable).
type SubscriptionDispatch func(ctx context.Context, workspaceID, eventName, resource string, payload map[string]any) error

// DeliveryEventHandler implements OutboxWorker's EventHandler, delivering
// durable events enqueued by internal/action.DeliverEvents.
type DeliveryEventHandler struct {
	Hub      events.Hub
	EventLog *EventLogStore
	Lookup   SpecLookup
	// Subscriptions, when non-nil, is invoked after channel fan-out to
	// dispatch the event to matching kind: Subscription handlers. Wired from
	// resource/formspec.go (which owns the subscription registry + action
	// dispatcher) to avoid a renderer → internal/action import cycle.
	Subscriptions SubscriptionDispatch
}

// HandleEvent implements EventHandler. payload is the JSON-marshaled
// events.EventMessage that was enqueued at emission time.
func (h *DeliveryEventHandler) HandleEvent(ctx context.Context, workspaceID, eventName, resource, payload string) error {
	channels, ok := h.Lookup(resource, eventName)
	if !ok {
		return fmt.Errorf("outbox delivery: no channels resolved for %s event %q — resource may have been removed from the current spec", resource, eventName)
	}

	var msg events.EventMessage
	if err := json.Unmarshal([]byte(payload), &msg); err != nil {
		return fmt.Errorf("outbox delivery: unmarshal payload: %w", err)
	}

	for _, ch := range channels {
		switch ch.Channel {
		case "websocket":
			// Listener-gated: no live websocket connection for the workspace
			// → skip the push. Realtime is non-durable, so there is no replay
			// for a client that connects later. Audit log (below) is
			// governance and stays unaffected.
			if h.Hub != nil && h.Hub.HasListeners(workspaceID) {
				h.Hub.Broadcast(workspaceID, msg)
			}
		case "audit_log":
			if err := h.EventLog.Write(ctx, workspaceID, eventName, resource, []byte(payload)); err != nil {
				return err
			}
		default:
			// reliable_event, queue, webhook, notification, pubsub: not yet
			// implemented. Treated as delivered (no error) so the worker
			// doesn't retry forever on a channel this pass never promised
			// to support.
		}
	}

	// Dispatch to kind: Subscription handlers (todo 7.3.1). The fully-qualified
	// event name is "{module}.{entity}.{event}" — derived from the resource
	// ("module/entity") and the short event name. A subscription handler
	// failure is returned so the outbox worker retries (at-least-once).
	if h.Subscriptions != nil {
		fqEvent := fullyQualifiedEvent(resource, eventName)
		if err := h.Subscriptions(ctx, workspaceID, fqEvent, resource, msg.Payload); err != nil {
			return err
		}
	}
	return nil
}

// fullyQualifiedEvent builds the fully-qualified resource event name
// ("{module}.{entity}.{event}") from a resource ("module/entity") and a short
// event name ("on_submit") — the form kind: Subscription declares in
// SubscriptionSpec.Events.
func fullyQualifiedEvent(resource, eventName string) string {
	module, entity, ok := strings.Cut(resource, "/")
	if !ok {
		return eventName
	}
	return module + "." + entity + "." + eventName
}
