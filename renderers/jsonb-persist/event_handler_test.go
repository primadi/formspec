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
	t.Cleanup(func() { _ = d.Close() })

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

// TestDeliveryEventHandler_UnwiredChannel_FailsNotSilentlyDelivered is the 7.7.6
// correction.
//
// This test used to pin the opposite: an unimplemented channel returned nil,
// "treated as delivered, not retried". That was the polite reading of a
// delivery that never happened — the outbox marked the entry COMPLETED, so
// nothing in the data showed the consequence was missing, and `formspec
// validate` stayed green over a manifest whose consequence could never occur.
// The honest outcome is a failure: the worker retries, then dead-letters
// (`status='failed'`), which an operator can see and query.
//
// Every channel is now delivered by SOMEONE; what this pins is the WIRING GAP —
// a channel whose dispatcher was not wired must fail, not report success.
func TestDeliveryEventHandler_UnwiredChannel_FailsNotSilentlyDelivered(t *testing.T) {
	hub := &fakeHub{}
	eventLog := newEventLogStore(t)
	lookup := func(resource, eventName string) ([]spec.EventDeliveryDecl, bool) {
		return []spec.EventDeliveryDecl{{Channel: "webhook", Webhook: &spec.WebhookDeliveryDecl{URL: "https://example.com/h"}}}, true
	}
	handler := &DeliveryEventHandler{Hub: hub, EventLog: eventLog, Lookup: lookup}

	payload, _ := json.Marshal(events.EventMessage{Event: "completed", Resource: "clinic/visit"})
	err := handler.HandleEvent(context.Background(), "demo", "completed", "clinic/visit", string(payload))
	if err == nil {
		t.Fatal("an unwired channel must fail, not be reported as delivered")
	}
	if !strings.Contains(err.Error(), "no webhook sender is wired") {
		t.Errorf("error %q should say the wiring is missing", err.Error())
	}
}

// TestDeliveryEventHandler_UnknownChannel_FailsToo keeps a value outside the
// declared set from slipping through: the struct can be built programmatically,
// so the handler must not rely on the schema having filtered it.
func TestDeliveryEventHandler_UnknownChannel_FailsToo(t *testing.T) {
	hub := &fakeHub{}
	eventLog := newEventLogStore(t)
	lookup := func(resource, eventName string) ([]spec.EventDeliveryDecl, bool) {
		return []spec.EventDeliveryDecl{{Channel: "carrier-pigeon"}}, true
	}
	handler := &DeliveryEventHandler{Hub: hub, EventLog: eventLog, Lookup: lookup}

	payload, _ := json.Marshal(events.EventMessage{Event: "completed", Resource: "clinic/visit"})
	err := handler.HandleEvent(context.Background(), "demo", "completed", "clinic/visit", string(payload))
	if err == nil {
		t.Fatal("an unknown channel must fail rather than complete")
	}
	if !strings.Contains(err.Error(), "carrier-pigeon") {
		t.Errorf("error %q should name the channel", err.Error())
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

// TestDeliveryEventHandler_ReliableEventPassesTheWholeEntry is the plumbing half
// of todo 7.7.5: the consequence dispatcher cannot enforce
// `deliver[].idempotency_key` unless the entry that declares it is handed over.
// Passing only `ch.Target` — which is what the handler used to do — dropped the
// key on the floor, so the outbox retry could not check it and re-applied the
// consequence.
func TestDeliveryEventHandler_ReliableEventPassesTheWholeEntry(t *testing.T) {
	hub := &fakeHub{}
	eventLog := newEventLogStore(t)

	want := spec.EventDeliveryDecl{
		Channel:        "reliable_event",
		Target:         &spec.DeliveryTarget{Resource: "gl.gl-balance", Action: "update"},
		IdempotencyKey: "balance.{id}",
	}
	lookup := func(resource, eventName string) ([]spec.EventDeliveryDecl, bool) {
		return []spec.EventDeliveryDecl{want}, true
	}

	var got spec.EventDeliveryDecl
	var called bool
	handler := &DeliveryEventHandler{
		Hub: hub, EventLog: eventLog, Lookup: lookup,
		Actions: func(_ context.Context, _, _, _ string, _ map[string]any, ch spec.EventDeliveryDecl) error {
			called = true
			got = ch
			return nil
		},
	}

	payload, _ := json.Marshal(events.EventMessage{
		Event: "journal-posted", Resource: "gl/journal-entry", Payload: map[string]any{"id": "jrn-1"},
	})
	if err := handler.HandleEvent(context.Background(), "demo", "journal-posted", "gl/journal-entry", string(payload)); err != nil {
		t.Fatalf("delivery: %v", err)
	}
	if !called {
		t.Fatal("the target action dispatch must be invoked for a reliable_event with a target")
	}
	if got.IdempotencyKey != "balance.{id}" {
		t.Fatalf("idempotency_key = %q, want balance.{id} — dropping it makes the retry check impossible", got.IdempotencyKey)
	}
	if got.Target == nil || got.Target.Resource != "gl.gl-balance" || got.Target.Action != "update" {
		t.Fatalf("target = %+v, want gl.gl-balance.update", got.Target)
	}
}

// TestDeliveryEventHandler_ReliableEventWithoutTargetSkipsDispatch keeps the
// durability-only entry (no target) from invoking anything — its job was the
// outbox itself, and Subscription fan-out is a separate pass.
func TestDeliveryEventHandler_ReliableEventWithoutTargetSkipsDispatch(t *testing.T) {
	hub := &fakeHub{}
	eventLog := newEventLogStore(t)
	lookup := func(resource, eventName string) ([]spec.EventDeliveryDecl, bool) {
		return []spec.EventDeliveryDecl{{Channel: "reliable_event"}}, true
	}

	called := false
	handler := &DeliveryEventHandler{
		Hub: hub, EventLog: eventLog, Lookup: lookup,
		Actions: func(_ context.Context, _, _, _ string, _ map[string]any, _ spec.EventDeliveryDecl) error {
			called = true
			return nil
		},
	}

	payload, _ := json.Marshal(events.EventMessage{Event: "completed", Resource: "clinic/visit"})
	if err := handler.HandleEvent(context.Background(), "demo", "completed", "clinic/visit", string(payload)); err != nil {
		t.Fatalf("delivery: %v", err)
	}
	if called {
		t.Fatal("an entry with no target must not invoke a consequence")
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
