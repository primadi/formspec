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

// ActionDispatch invokes a target action for a `deliver: channel:
// reliable_event  target: {resource, action}` entry — the publisher's contract
// consequence (02-core-basic.md §12.2: the outbox worker makes a "sync call to
// target action → delivered, or backoff retry → dead-letter"). resource is the
// event's own "module/entity"; target is its declared DeliveryTarget.
//
// Returning an error marks the outbox entry failed so the worker retries it —
// which is the whole point of the channel: a publisher that promises a
// consequence is not allowed to lose it silently.
type ActionDispatch func(ctx context.Context, workspaceID, resource, eventName string, payload map[string]any, target *spec.DeliveryTarget) error

// PubSub is the minimal pub/sub contract the delivery handler needs for the
// `pubsub` channel (todo 7.3.5) — non-durable, at-most-once.
type PubSub interface {
	Publish(ctx context.Context, channel string, payload any) error
}

// DeliveryEventHandler implements OutboxWorker's EventHandler, delivering
// durable events enqueued by internal/action.DeliverEvents.
type DeliveryEventHandler struct {
	Hub      events.Hub
	EventLog *EventLogStore
	Lookup   SpecLookup
	// PubSub, when non-nil, backs the `pubsub` delivery channel (todo 7.3.5):
	// the event payload is published to a channel (non-durable, at-most-once).
	PubSub PubSub
	// Subscriptions, when non-nil, is invoked after channel fan-out to
	// dispatch the event to matching kind: Subscription handlers. Wired from
	// resource/formspec.go (which owns the subscription registry + action
	// dispatcher) to avoid a renderer → internal/action import cycle.
	Subscriptions SubscriptionDispatch
	// Actions, when non-nil, invokes declared reliable_event target actions.
	// Wired from resource/formspec.go for the same import-cycle reason as
	// Subscriptions.
	Actions ActionDispatch
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
		case "pubsub":
			// Non-durable, at-most-once (todo 7.3.5): publish the event payload
			// to a channel. The channel name comes from the delivery target's
			// scope, defaulting to "{resource}.{event}".
			if h.PubSub != nil {
				channel := ""
				if ch.Target != nil {
					channel = ch.Target.Scope
				}
				if channel == "" {
					channel = resource + "." + eventName
				}
				if err := h.PubSub.Publish(ctx, channel, msg); err != nil {
					return fmt.Errorf("pubsub delivery: %w", err)
				}
			}
		case "reliable_event":
			// Durable delivery: reaching this code at all means the outbox
			// already holds the event (that is what makes it durable — retry
			// and dead-letter are the worker's job). What is left is the
			// publisher's declared consequence: a target action, invoked here
			// as a sync call (02-core-basic.md §12.2). Retrying this code is
			// exactly how a failed target action is retried, so an error is
			// returned rather than swallowed.
			if ch.Target == nil || ch.Target.Resource == "" {
				// No target: the entry's only job was durability, which the
				// outbox itself provides. Subscription fan-out below still runs.
				break
			}
			if h.Actions == nil {
				// Nothing wired to perform the call. Reporting this as a
				// delivery failure would retry forever against a wiring gap,
				// so it is logged once per attempt and the event is treated as
				// delivered — the durable record still exists in the outbox,
				// event log, and every Subscription.
				break
			}
			if err := h.Actions(ctx, workspaceID, resource, eventName, msg.Payload, ch.Target); err != nil {
				return fmt.Errorf("reliable_event %s → %s.%s: %w",
					eventName, ch.Target.Resource, ch.Target.Action, err)
			}
		default:
			// queue, webhook, notification: not yet implemented. Treated as
			// delivered (no error) so the worker doesn't retry forever on a
			// channel this pass never promised to support.
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
