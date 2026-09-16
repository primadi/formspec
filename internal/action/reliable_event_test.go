package action

import (
	"context"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// TestDeliverEvents_ReliableEvent_GoesThroughOutbox pins the durable channel
// documented in Core Extended §12.1 and required by ValidateEventDurability.
//
// Before this channel was implemented it fell through to the "not implemented"
// default and was silently discarded — so a manifest that validated cleanly
// still produced no delivery at all, and every projection fed by it stayed
// empty. That is the failure mode this test exists to prevent.
func TestDeliverEvents_ReliableEvent_GoesThroughOutbox(t *testing.T) {
	hub := &fakeHub{}
	deps := newDeliveryDeps(t, hub)

	emissions := []EventEmission{{
		Name:      "on_paid",
		Durable:   true,
		Payload:   map[string]any{"id": "ORD-1"},
		DeliverTo: []spec.EventDeliveryDecl{{Channel: "reliable_event"}},
	}}

	DeliverEvents(context.Background(), deps, "demo", "cafe-order/order", emissions, false)

	counts, err := deps.Outbox.CountByStatus(context.Background())
	if err != nil {
		t.Fatalf("CountByStatus: %v", err)
	}
	if counts["pending"] != 1 {
		t.Errorf("outbox pending = %d, want 1 — reliable_event must be durable", counts["pending"])
	}
}

// TestDeliverEvents_ReliableEvent_NotDoubleEnqueued: the create/update path
// already enqueues the durable entry atomically alongside the entity mutation,
// so delivery must not add a second one.
func TestDeliverEvents_ReliableEvent_NotDoubleEnqueued(t *testing.T) {
	hub := &fakeHub{}
	deps := newDeliveryDeps(t, hub)

	emissions := []EventEmission{{
		Name:      "on_paid",
		Durable:   true,
		Payload:   map[string]any{"id": "ORD-1"},
		DeliverTo: []spec.EventDeliveryDecl{{Channel: "reliable_event"}},
	}}

	DeliverEvents(context.Background(), deps, "demo", "cafe-order/order", emissions, true)

	counts, err := deps.Outbox.CountByStatus(context.Background())
	if err != nil {
		t.Fatalf("CountByStatus: %v", err)
	}
	if counts["pending"] != 0 {
		t.Errorf("outbox pending = %d, want 0 when the caller already enqueued it", counts["pending"])
	}
}
