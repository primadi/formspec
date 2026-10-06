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
// target action → delivered, or backoff retry → dead-letter", and per §7 the
// worker "cek idempotency" first).
//
// The whole delivery entry is passed rather than just its target, because the
// entry carries the `idempotency_key` the retry check must use: retrying a
// consequence that already ran is a double write, and only the entry knows the
// key that identifies "this same delivery".
//
// resource is the event's own "module/entity"; ch.Target is its declared
// DeliveryTarget.
//
// Returning an error marks the outbox entry failed so the worker retries it —
// which is the whole point of the channel: a publisher that promises a
// consequence is not allowed to lose it silently.
type ActionDispatch func(ctx context.Context, workspaceID, resource, eventName string, payload map[string]any, ch spec.EventDeliveryDecl) error

// PubSub is the minimal pub/sub contract the delivery handler needs for the
// `pubsub` channel (todo 7.3.5) — non-durable, at-most-once.
type PubSub interface {
	Publish(ctx context.Context, channel string, payload any) error
}

// JobDispatch runs a `queue` deliver entry's `job:` — a Service action named
// `service.action` or `module.service.action` (todo 7.7.6).
//
// It is a separate callback from ActionDispatch because the reference and the
// target kind differ: `reliable_event` names an Entity/Service target with its
// own action, while a queue entry names the Service action directly. Keeping
// them apart means one resolution rule per declaration instead of one function
// guessing which shape it was handed.
//
// resource is the event's own "module/entity" — the publishing module resolves
// a bare `service.action`. Returning an error marks the outbox entry failed so
// the worker retries, then dead-letters.
type JobDispatch func(ctx context.Context, workspaceID, resource, eventName string, payload map[string]any, job string) error

// NotificationDispatch runs a `notification` deliver entry (todo 7.7.6): it
// writes the in-app notification row (addressed to `notification.recipient`) and
// then, when the entry names `handler:`, hands the payload to that Service
// action for channels outside in-app.
//
// It mirrors JobDispatch for the same reason: the entry's declared fields differ
// per channel, so each channel gets its own resolution rule instead of one
// function guessing which shape it was handed.
type NotificationDispatch func(ctx context.Context, workspaceID, resource, eventName string, payload map[string]any, decl *spec.NotificationDecl, handler string) error

// WebhookDispatch posts an event to an endpoint declared on a `webhook` entry
// (todo 7.7.6, unsigned version). It receives the already-marshaled event body so
// every channel sends the same wire shape.
type WebhookDispatch func(ctx context.Context, decl *spec.WebhookDeliveryDecl, payload map[string]any, body []byte) error

// DeliveryEventHandler implements OutboxWorker's EventHandler, delivering
// durable events enqueued by internal/action.DeliverEvents.
type DeliveryEventHandler struct {
	Hub      events.Hub
	EventLog *EventLogStore
	Lookup   SpecLookup
	// PubSub, when non-nil, backs the `pubsub` delivery channel (todo 7.3.5):
	// the event payload is published to a channel (non-durable, at-most-once).
	PubSub PubSub
	// Jobs, when non-nil, backs the `queue` delivery channel (todo 7.7.6): the
	// entry's `job:` is dispatched to a Service action.
	Jobs JobDispatch
	// Notifications, when non-nil, backs the `notification` delivery channel
	// (todo 7.7.6).
	Notifications NotificationDispatch
	// Webhooks, when non-nil, backs the `webhook` delivery channel (todo 7.7.6,
	// unsigned).
	Webhooks WebhookDispatch
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
		case "queue":
			// Background job (todo 7.7.6): the entry names the Service action to
			// run. The outbox IS the queue — this code is reached by the worker,
			// which is what gives the job retry and dead-letter.
			if strings.TrimSpace(ch.Job) == "" {
				return fmt.Errorf("event %s: `queue` entry has no `job:` — the worker has nothing to call", eventName)
			}
			if h.Jobs == nil {
				// A wiring gap must not look like a delivered job: reporting it
				// as a failure lets the outbox dead-letter it, which is visible.
				return fmt.Errorf("event %s: `queue` job %q cannot run — no job dispatcher is wired", eventName, ch.Job)
			}
			if err := h.Jobs(ctx, workspaceID, resource, eventName, msg.Payload, ch.Job); err != nil {
				return fmt.Errorf("queue %s → %s: %w", eventName, ch.Job, err)
			}
		case "notification":
			// In-app notification (todo 7.7.6), plus an optional Service handler for
			// channels outside in-app. The row is what `kind: NotificationCenter`
			// lists — so this channel and that page now share one source.
			if h.Notifications == nil {
				return fmt.Errorf("event %s: `notification` delivery cannot run — no notification dispatcher is wired", eventName)
			}
			if err := h.Notifications(ctx, workspaceID, resource, eventName, msg.Payload, ch.Notification, ch.Handler); err != nil {
				return fmt.Errorf("notification %s: %w", eventName, err)
			}
		case "webhook":
			// Outbound POST to an endpoint declared on the entry (todo 7.7.6,
			// unsigned). A non-2xx or a transport failure is returned so the outbox
			// retries and then dead-letters — the only way a failed delivery becomes
			// visible.
			if h.Webhooks == nil {
				return fmt.Errorf("event %s: `webhook` delivery cannot run — no webhook sender is wired", eventName)
			}
			if err := h.Webhooks(ctx, ch.Webhook, msg.Payload, []byte(payload)); err != nil {
				return fmt.Errorf("webhook %s: %w", eventName, err)
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
			if err := h.Actions(ctx, workspaceID, resource, eventName, msg.Payload, ch); err != nil {
				return fmt.Errorf("reliable_event %s → %s.%s: %w",
					eventName, ch.Target.Resource, ch.Target.Action, err)
			}
		default:
			// An unknown/undelivered channel is a FAILURE, not a success.
			//
			// This used to `break` (treated as delivered) so the worker would
			// not retry a channel "this pass never promised to support" — but
			// that turned a missing consequence into a COMPLETED outbox entry,
			// so the gap was invisible in the data and `formspec validate`
			// stayed green (todo 7.7.6). Returning an error makes the outbox
			// retry and then dead-letter it (`status='failed'`), which is
			// exactly what an operator needs to see: the delivery did not
			// happen.
			reason, known := spec.ChannelUnsupported(string(ch.Channel))
			if !known {
				reason = "the runtime has no delivery branch for it"
			}
			return fmt.Errorf("event %s: channel %q is not delivered — %s", eventName, ch.Channel, reason)
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
