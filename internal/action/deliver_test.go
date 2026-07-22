package action

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/primadi/forma/renderers/jsonbpersist"
	"github.com/primadi/forma/internal/events"
	"github.com/primadi/forma/pkg/spec"
)

type fakeHub struct {
	broadcasts []events.EventMessage
}

func (f *fakeHub) Broadcast(workspaceID string, msg events.EventMessage) {
	f.broadcasts = append(f.broadcasts, msg)
}

func newDeliveryDeps(t *testing.T, hub events.Hub) DeliveryDeps {
	t.Helper()
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "deliver_test.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	r := db.NewMigrationRunner(d, db.DriverSQLite)
	if err := r.EnsureSystemTables(context.Background()); err != nil {
		t.Fatalf("EnsureSystemTables: %v", err)
	}

	outbox := db.NewOutboxStore(d, db.DriverSQLite)
	eventLog := db.NewEventLogStore(d, db.DriverSQLite)
	return DeliveryDeps{Hub: hub, Outbox: outbox, EventLog: eventLog}
}

func TestDeliverEvents_NonDurable_BroadcastsButDoesNotEnqueue(t *testing.T) {
	hub := &fakeHub{}
	deps := newDeliveryDeps(t, hub)

	emissions := []EventEmission{{
		Name:      "completed",
		Durable:   false,
		Payload:   map[string]any{"id": "v1"},
		DeliverTo: []spec.EventDeliveryDecl{{Channel: "websocket"}},
	}}

	DeliverEvents(context.Background(), deps, "demo", "clinic/visit", emissions, false)

	if len(hub.broadcasts) != 1 {
		t.Fatalf("hub.Broadcast called %d times, want 1", len(hub.broadcasts))
	}

	counts, err := deps.Outbox.CountByStatus(context.Background())
	if err != nil {
		t.Fatalf("CountByStatus: %v", err)
	}
	if total := counts["pending"] + counts["completed"] + counts["failed"]; total != 0 {
		t.Errorf("outbox has %d rows, want 0 for a non-durable event", total)
	}
}

func TestDeliverEvents_Durable_BroadcastsAndEnqueues(t *testing.T) {
	hub := &fakeHub{}
	deps := newDeliveryDeps(t, hub)

	emissions := []EventEmission{{
		Name:      "completed",
		Durable:   true,
		Payload:   map[string]any{"id": "v1"},
		DeliverTo: []spec.EventDeliveryDecl{{Channel: "websocket"}},
	}}

	DeliverEvents(context.Background(), deps, "demo", "clinic/visit", emissions, false)

	if len(hub.broadcasts) != 1 {
		t.Fatalf("hub.Broadcast called %d times, want 1 (durable events still get an immediate best-effort push)", len(hub.broadcasts))
	}

	counts, err := deps.Outbox.CountByStatus(context.Background())
	if err != nil {
		t.Fatalf("CountByStatus: %v", err)
	}
	if counts["pending"] != 1 {
		t.Errorf("outbox pending count = %d, want 1 for a durable event", counts["pending"])
	}
}

func TestDeliverEvents_AuditLog_NonDurable_WritesEventLogDirectly(t *testing.T) {
	hub := &fakeHub{}
	deps := newDeliveryDeps(t, hub)

	emissions := []EventEmission{{
		Name:      "completed",
		Durable:   false,
		Payload:   map[string]any{"id": "v1"},
		DeliverTo: []spec.EventDeliveryDecl{{Channel: "audit_log"}},
	}}

	DeliverEvents(context.Background(), deps, "demo", "clinic/visit", emissions, false)

	records, err := deps.EventLog.ListByWorkspace(context.Background(), "demo", "clinic/visit", 10, 0)
	if err != nil {
		t.Fatalf("ListByWorkspace: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("event log has %d records, want 1", len(records))
	}

	counts, _ := deps.Outbox.CountByStatus(context.Background())
	if total := counts["pending"]; total != 0 {
		t.Errorf("outbox pending = %d, want 0 for a non-durable audit_log event", total)
	}
}

func TestDeliverEvents_AuditLog_Durable_GoesThroughOutboxNotDirectWrite(t *testing.T) {
	hub := &fakeHub{}
	deps := newDeliveryDeps(t, hub)

	emissions := []EventEmission{{
		Name:      "completed",
		Durable:   true,
		Payload:   map[string]any{"id": "v1"},
		DeliverTo: []spec.EventDeliveryDecl{{Channel: "audit_log"}},
	}}

	DeliverEvents(context.Background(), deps, "demo", "clinic/visit", emissions, false)

	records, err := deps.EventLog.ListByWorkspace(context.Background(), "demo", "clinic/visit", 10, 0)
	if err != nil {
		t.Fatalf("ListByWorkspace: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("event log has %d records, want 0 — durable audit_log delivery goes through the outbox worker, not a direct write", len(records))
	}

	counts, _ := deps.Outbox.CountByStatus(context.Background())
	if counts["pending"] != 1 {
		t.Errorf("outbox pending = %d, want 1", counts["pending"])
	}
}

func TestDeliverEvents_UnimplementedChannel_DoesNotPanic(t *testing.T) {
	hub := &fakeHub{}
	deps := newDeliveryDeps(t, hub)

	emissions := []EventEmission{{
		Name:      "created",
		Payload:   map[string]any{"id": "v1"},
		DeliverTo: []spec.EventDeliveryDecl{{Channel: "webhook"}},
	}}

	DeliverEvents(context.Background(), deps, "demo", "clinic/visit", emissions, false)

	if len(hub.broadcasts) != 0 {
		t.Errorf("expected no broadcast for an unimplemented channel")
	}
}
