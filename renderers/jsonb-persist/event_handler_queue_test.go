package db

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/events"
	"github.com/primadi/formspec/pkg/spec"
)

// TestDeliveryEventHandler_QueueChannelRunsTheJob pins that a `queue` entry's
// `job:` reaches the job dispatcher — the plumbing that was missing entirely
// before 7.7.6 (`queue` fell through to a default that marked the delivery
// COMPLETED, so the job never ran and nothing showed it).
func TestDeliveryEventHandler_QueueChannelRunsTheJob(t *testing.T) {
	hub := &fakeHub{}
	eventLog := newEventLogStore(t)
	lookup := func(resource, eventName string) ([]spec.EventDeliveryDecl, bool) {
		return []spec.EventDeliveryDecl{{Channel: "queue", Job: "receipt-jobs.generate"}}, true
	}

	var gotJob, gotResource string
	var gotPayload map[string]any
	handler := &DeliveryEventHandler{
		Hub: hub, EventLog: eventLog, Lookup: lookup,
		Jobs: func(_ context.Context, _, resource, _ string, payload map[string]any, job string) error {
			gotJob = job
			gotResource = resource
			gotPayload = payload
			return nil
		},
	}

	payload, _ := json.Marshal(events.EventMessage{
		Event: "paid", Resource: "billing/order", Payload: map[string]any{"id": "ord-1"},
	})
	if err := handler.HandleEvent(context.Background(), "demo", "paid", "billing/order", string(payload)); err != nil {
		t.Fatalf("delivery: %v", err)
	}
	if gotJob != "receipt-jobs.generate" {
		t.Fatalf("job = %q, want receipt-jobs.generate", gotJob)
	}
	// The resource is what resolves a bare `service.action` to the publisher's
	// module, so it must travel with the call.
	if gotResource != "billing/order" {
		t.Errorf("resource = %q, want billing/order", gotResource)
	}
	if gotPayload["id"] != "ord-1" {
		t.Errorf("payload = %v, want the event payload unchanged", gotPayload)
	}
}

// TestDeliveryEventHandler_QueueWithoutJobFails: a queue entry with no `job:` is
// unresolvable. Failing means the outbox retries then dead-letters it — visible
// — instead of completing a delivery that did nothing.
func TestDeliveryEventHandler_QueueWithoutJobFails(t *testing.T) {
	hub := &fakeHub{}
	eventLog := newEventLogStore(t)
	lookup := func(resource, eventName string) ([]spec.EventDeliveryDecl, bool) {
		return []spec.EventDeliveryDecl{{Channel: "queue"}}, true
	}
	handler := &DeliveryEventHandler{Hub: hub, EventLog: eventLog, Lookup: lookup}

	payload, _ := json.Marshal(events.EventMessage{Event: "paid", Resource: "billing/order"})
	err := handler.HandleEvent(context.Background(), "demo", "paid", "billing/order", string(payload))
	if err == nil {
		t.Fatal("a queue entry without `job:` must fail")
	}
	if !strings.Contains(err.Error(), "no `job:`") {
		t.Errorf("error %q should say the job is missing", err.Error())
	}
}

// TestDeliveryEventHandler_QueueWithoutDispatcherFails: an unwired dispatcher is
// a wiring gap, and a wiring gap must not look like a delivered job.
func TestDeliveryEventHandler_QueueWithoutDispatcherFails(t *testing.T) {
	hub := &fakeHub{}
	eventLog := newEventLogStore(t)
	lookup := func(resource, eventName string) ([]spec.EventDeliveryDecl, bool) {
		return []spec.EventDeliveryDecl{{Channel: "queue", Job: "svc.act"}}, true
	}
	handler := &DeliveryEventHandler{Hub: hub, EventLog: eventLog, Lookup: lookup}

	payload, _ := json.Marshal(events.EventMessage{Event: "paid", Resource: "billing/order"})
	err := handler.HandleEvent(context.Background(), "demo", "paid", "billing/order", string(payload))
	if err == nil {
		t.Fatal("a queue job with no dispatcher must fail, not be marked delivered")
	}
	if !strings.Contains(err.Error(), "no job dispatcher") {
		t.Errorf("error %q should say why", err.Error())
	}
}
