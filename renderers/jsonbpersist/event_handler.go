package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/primadi/forma/internal/events"
	"github.com/primadi/forma/pkg/spec"
)

// SpecLookup resolves the delivery channels declared for a specific
// (resource, event name) pair by re-reading the live entity registry at
// delivery time — not by snapshotting channels into the outbox payload at
// enqueue time — so a hot-reloaded manifest fix is picked up by retries
// automatically. resource is "module/entity" (e.g. "clinic/visit").
type SpecLookup func(resource, eventName string) (channels []spec.EventDeliveryDecl, ok bool)

// DeliveryEventHandler implements OutboxWorker's EventHandler, delivering
// durable events enqueued by internal/action.DeliverEvents.
type DeliveryEventHandler struct {
	Hub      events.Hub
	EventLog *EventLogStore
	Lookup   SpecLookup
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
	return nil
}
