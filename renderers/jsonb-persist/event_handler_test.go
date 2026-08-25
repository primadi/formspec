package db

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/events"
	"github.com/primadi/formspec/pkg/spec"
)

type fakeHub struct {
	broadcasts []events.EventMessage
	tenants    []string
}

func (f *fakeHub) Broadcast(workspaceID string, msg events.EventMessage) {
	f.tenants = append(f.tenants, workspaceID)
	f.broadcasts = append(f.broadcasts, msg)
}

func (f *fakeHub) HasListeners(string) bool { return true }

func newEventLogStore(t *testing.T) *EventLogStore {
	t.Helper()
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "event_log.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	r := NewMigrationRunner(d, DriverSQLite)
	if err := r.EnsureSystemTables(context.Background()); err != nil {
		t.Fatalf("EnsureSystemTables: %v", err)
	}
	return NewEventLogStore(d, DriverSQLite)
}

func TestDeliveryEventHandler_FansOutToWebsocketAndAuditLog(t *testing.T) {
	hub := &fakeHub{}
	eventLog := newEventLogStore(t)

	lookup := func(resource, eventName string) ([]spec.EventDeliveryDecl, bool) {
		if resource == "clinic/visit" && eventName == "completed" {
			return []spec.EventDeliveryDecl{{Channel: "audit_log"}, {Channel: "websocket"}}, true
		}
		return nil, false
	}

	handler := &DeliveryEventHandler{Hub: hub, EventLog: eventLog, Lookup: lookup}

	payload, _ := json.Marshal(events.EventMessage{Event: "completed", Resource: "clinic/visit", Payload: map[string]any{"id": "v1"}})
	if err := handler.HandleEvent(context.Background(), "demo", "completed", "clinic/visit", string(payload)); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}

	if len(hub.broadcasts) != 1 {
		t.Fatalf("hub.Broadcast called %d times, want 1", len(hub.broadcasts))
	}
	if hub.tenants[0] != "demo" {
		t.Errorf("workspace = %q, want \"demo\"", hub.tenants[0])
	}
	if hub.broadcasts[0].Event != "completed" {
		t.Errorf("Event = %q, want \"completed\"", hub.broadcasts[0].Event)
	}

	records, err := eventLog.ListByWorkspace(context.Background(), "demo", "clinic/visit", 10, 0)
	if err != nil {
		t.Fatalf("ListByWorkspace: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("event log has %d records, want 1", len(records))
	}
	if records[0].EventName != "completed" {
		t.Errorf("EventName = %q, want \"completed\"", records[0].EventName)
	}
}

func TestDeliveryEventHandler_LookupMiss_ReturnsError(t *testing.T) {
	hub := &fakeHub{}
	eventLog := newEventLogStore(t)
	lookup := func(resource, eventName string) ([]spec.EventDeliveryDecl, bool) {
		return nil, false
	}
	handler := &DeliveryEventHandler{Hub: hub, EventLog: eventLog, Lookup: lookup}

	payload, _ := json.Marshal(events.EventMessage{Event: "completed", Resource: "clinic/visit"})
	err := handler.HandleEvent(context.Background(), "demo", "completed", "clinic/visit", string(payload))
	if err == nil {
		t.Fatal("expected an error when the spec lookup misses (e.g. resource removed from current spec)")
	}
	if len(hub.broadcasts) != 0 {
		t.Errorf("hub.Broadcast should not have been called on lookup miss")
	}
}

func TestDeliveryEventHandler_UnimplementedChannel_ReturnsNilNotError(t *testing.T) {
	hub := &fakeHub{}
	eventLog := newEventLogStore(t)
	lookup := func(resource, eventName string) ([]spec.EventDeliveryDecl, bool) {
		return []spec.EventDeliveryDecl{{Channel: "webhook"}}, true
	}
	handler := &DeliveryEventHandler{Hub: hub, EventLog: eventLog, Lookup: lookup}

	payload, _ := json.Marshal(events.EventMessage{Event: "completed", Resource: "clinic/visit"})
	err := handler.HandleEvent(context.Background(), "demo", "completed", "clinic/visit", string(payload))
	if err != nil {
		t.Fatalf("expected nil (treated as delivered, not retried) for an unimplemented channel, got %v", err)
	}
}

func TestDeliveryEventHandler_MalformedPayload_ReturnsError(t *testing.T) {
	hub := &fakeHub{}
	eventLog := newEventLogStore(t)
	lookup := func(resource, eventName string) ([]spec.EventDeliveryDecl, bool) {
		return []spec.EventDeliveryDecl{{Channel: "websocket"}}, true
	}
	handler := &DeliveryEventHandler{Hub: hub, EventLog: eventLog, Lookup: lookup}

	err := handler.HandleEvent(context.Background(), "demo", "completed", "clinic/visit", "not json")
	if err == nil {
		t.Fatal("expected an error for malformed JSON payload")
	}
	if !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("error = %v, want it to mention unmarshal", err)
	}
}

// recordingPubSub captures published payloads for the pubsub channel test.
type recordingPubSub struct {
	channels []string
	payloads []any
}

func (p *recordingPubSub) Publish(_ context.Context, channel string, payload any) error {
	p.channels = append(p.channels, channel)
	p.payloads = append(p.payloads, payload)
	return nil
}

func TestDeliveryEventHandler_PubsubChannel(t *testing.T) {
	hub := &fakeHub{}
	eventLog := newEventLogStore(t)
	ps := &recordingPubSub{}

	lookup := func(resource, eventName string) ([]spec.EventDeliveryDecl, bool) {
		if resource == "clinic/visit" && eventName == "completed" {
			return []spec.EventDeliveryDecl{{Channel: "pubsub", Target: &spec.DeliveryTarget{Scope: "visits"}}}, true
		}
		return nil, false
	}

	handler := &DeliveryEventHandler{Hub: hub, EventLog: eventLog, Lookup: lookup, PubSub: ps}

	payload, _ := json.Marshal(events.EventMessage{Event: "completed", Resource: "clinic/visit", Payload: map[string]any{"id": "v1"}})
	if err := handler.HandleEvent(context.Background(), "demo", "completed", "clinic/visit", string(payload)); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}

	if len(ps.channels) != 1 {
		t.Fatalf("pubsub.Publish called %d times, want 1", len(ps.channels))
	}
	if ps.channels[0] != "visits" {
		t.Errorf("channel = %q, want \"visits\" (from target.scope)", ps.channels[0])
	}
}

func TestDeliveryEventHandler_PubsubChannel_DefaultChannelName(t *testing.T) {
	hub := &fakeHub{}
	eventLog := newEventLogStore(t)
	ps := &recordingPubSub{}

	lookup := func(resource, eventName string) ([]spec.EventDeliveryDecl, bool) {
		if resource == "clinic/visit" && eventName == "completed" {
			return []spec.EventDeliveryDecl{{Channel: "pubsub"}}, true // no target.scope
		}
		return nil, false
	}

	handler := &DeliveryEventHandler{Hub: hub, EventLog: eventLog, Lookup: lookup, PubSub: ps}

	payload, _ := json.Marshal(events.EventMessage{Event: "completed", Resource: "clinic/visit"})
	if err := handler.HandleEvent(context.Background(), "demo", "completed", "clinic/visit", string(payload)); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}

	if len(ps.channels) != 1 {
		t.Fatalf("pubsub.Publish called %d times, want 1", len(ps.channels))
	}
	if ps.channels[0] != "clinic/visit.completed" {
		t.Errorf("channel = %q, want default \"clinic/visit.completed\"", ps.channels[0])
	}
}
