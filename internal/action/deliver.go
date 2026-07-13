package action

import (
	"context"
	"encoding/json"
	"time"

	"github.com/primadi/forma/internal/db"
	"github.com/primadi/forma/internal/events"
)

// DeliveryDeps bundles the dependencies DeliverEvents needs to fan out an
// emission to its declared channels — injected once from resource/forma.go
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
func DeliverEvents(ctx context.Context, deps DeliveryDeps, workspaceID, resource string, emissions []EventEmission) {
	for _, ev := range emissions {
		msg := events.EventMessage{
			Event:    ev.Name,
			Resource: resource,
			Payload:  ev.Payload,
			Emitted:  time.Now().UTC().Format(time.RFC3339Nano),
		}
		payloadJSON, err := json.Marshal(msg)
		if err != nil {
			deps.logger().Error("event.marshal_failed", map[string]any{"event": ev.Name, "resource": resource, "error": err.Error()})
			continue
		}

		for _, ch := range ev.DeliverTo {
			switch ch.Channel {
			case "websocket":
				if deps.Hub != nil {
					deps.Hub.Broadcast(workspaceID, msg)
				}
				if ev.Durable && deps.Outbox != nil {
					if _, err := deps.Outbox.Enqueue(ctx, workspaceID, ev.Name, resource, string(payloadJSON)); err != nil {
						deps.logger().Error("event.outbox_enqueue_failed", map[string]any{"event": ev.Name, "channel": ch.Channel, "error": err.Error()})
					}
				}
			case "audit_log":
				if ev.Durable && deps.Outbox != nil {
					if _, err := deps.Outbox.Enqueue(ctx, workspaceID, ev.Name, resource, string(payloadJSON)); err != nil {
						deps.logger().Error("event.outbox_enqueue_failed", map[string]any{"event": ev.Name, "channel": ch.Channel, "error": err.Error()})
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
