package action

import (
	"context"
	"encoding/json"
	"time"

	"github.com/primadi/formspec/internal/events"
	db "github.com/primadi/formspec/renderers/jsonbpersist"
)

// DeliveryDeps bundles the dependencies DeliverEvents needs to fan out an
// emission to its declared channels — injected once from resource/formspec.go
// into HandlerFactory.
type DeliveryDeps struct {
	Hub      events.Hub
	Outbox   *db.OutboxStore
	EventLog *db.EventLogStore
	Logger   RuntimeLogger
}

func (d DeliveryDeps) logger() RuntimeLogger {
	if d.Logger != nil {
		return d.Logger
	}
	return DefaultLogger
}

// BuildEventMessage marshals the wire payload for an emission exactly as
// DeliverEvents would — exposed so a caller (e.g. the create/update HTTP
// handlers) can enqueue a durable event to the outbox atomically, in the
// same transaction as the entity mutation that produced it, before
// DeliverEvents ever runs (see EntityStore.Insert/Update's PendingEvents
// param in renderers/jsonbpersist). Core Basic §7 requires the entity
// mutation and its durable outbox entry to commit together or not at all.
func BuildEventMessage(resource string, ev EventEmission) ([]byte, error) {
	msg := events.EventMessage{
		Event:    ev.Name,
		Resource: resource,
		Payload:  ev.Payload,
		Emitted:  time.Now().UTC().Format(time.RFC3339Nano),
	}
	return json.Marshal(msg)
}

// NotifyMutation broadcasts a generic entity-change event over the
// workspace's websocket — the Spec Resolution API §5 realtime channel
// `entity:{module}.{name}` with events `created | updated | deleted` (and
// custom action names for lifecycle/state transitions). The CRUD handlers
// call it after a mutation commits, so every mutation is pushed to live
// subscribers — not only declared `events:` with `deliver: websocket`.
//
// Listener-gated: when no websocket connection is registered for the
// workspace this is a no-op — realtime publish work is never done when
// nobody is listening, even if the entity declares a websocket publish.
func NotifyMutation(deps DeliveryDeps, workspaceID, resource, event string) {
	if deps.Hub == nil || !deps.Hub.HasListeners(workspaceID) {
		return
	}
	deps.Hub.Broadcast(workspaceID, events.EventMessage{
		Event:    event,
		Resource: resource,
		Emitted:  time.Now().UTC().Format(time.RFC3339Nano),
	})
}

// DeliverEvents fans each emission out to its declared channels
// (EventEmission.DeliverTo, already resolved by ResolveEmission from the
// entity's events: block). Durability is gated purely by ev.Durable (from
// the event's publish.durable, default false, Core Basic §12.1):
// non-durable events are delivered immediately, in-process, best-effort, on
// every declared channel. Durable events additionally go through the
// outbox so a background worker retries delivery at least once, surviving
// a crash between action commit and delivery. For the websocket channel,
// an immediate best-effort push always happens regardless of durability —
// the outbox enqueue on top is insurance for a client that was
// disconnected at the moment of the immediate push.
//
// outboxAlreadyEnqueued is set by callers (create/update) that already
// enqueued this emission's durable outbox entry atomically alongside the
// entity mutation itself, via EntityStore's PendingEvents — DeliverEvents
// must not enqueue it a second time. Custom actions (which don't yet have
// an atomic path) pass false, preserving the pre-existing best-effort
// behavior for that path (a documented gap — see 01-architecture.md §3).
func DeliverEvents(ctx context.Context, deps DeliveryDeps, workspaceID, resource string, emissions []EventEmission, outboxAlreadyEnqueued bool) {
	for _, ev := range emissions {
		payloadJSON, err := BuildEventMessage(resource, ev)
		if err != nil {
			deps.logger().Error("event.marshal_failed", map[string]any{"event": ev.Name, "resource": resource, "error": err.Error()})
			continue
		}
		msg := events.EventMessage{Event: ev.Name, Resource: resource, Payload: ev.Payload}

		for _, ch := range ev.DeliverTo {
			switch ch.Channel {
			case "websocket":
				// Listener-gated (realtime is non-durable, §5): no websocket
				// connection for the workspace → skip both the immediate push
				// and the outbox insurance — there is nobody to receive it
				// and there is no replay for a late connection.
				if deps.Hub != nil && deps.Hub.HasListeners(workspaceID) {
					deps.Hub.Broadcast(workspaceID, msg)
					if ev.Durable && deps.Outbox != nil && !outboxAlreadyEnqueued {
						if _, err := deps.Outbox.Enqueue(ctx, workspaceID, ev.Name, resource, string(payloadJSON)); err != nil {
							deps.logger().Error("event.outbox_enqueue_failed", map[string]any{"event": ev.Name, "channel": ch.Channel, "error": err.Error()})
						}
					}
				}
			case "audit_log":
				if ev.Durable {
					if deps.Outbox != nil && !outboxAlreadyEnqueued {
						if _, err := deps.Outbox.Enqueue(ctx, workspaceID, ev.Name, resource, string(payloadJSON)); err != nil {
							deps.logger().Error("event.outbox_enqueue_failed", map[string]any{"event": ev.Name, "channel": ch.Channel, "error": err.Error()})
						}
					}
					continue
				}
				if deps.EventLog != nil {
					if err := deps.EventLog.Write(ctx, workspaceID, ev.Name, resource, payloadJSON); err != nil {
						deps.logger().Error("event.audit_log_write_failed", map[string]any{"event": ev.Name, "error": err.Error()})
					}
				}
			default:
				// reliable_event, queue, webhook, notification, pubsub —
				// explicitly out of scope for this pass (see plan notes).
				deps.logger().Warn("event.channel_not_implemented", map[string]any{"event": ev.Name, "channel": ch.Channel})
			}
		}
	}
}
