package action

import (
	"reflect"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

func TestResolveEmission(t *testing.T) {
	events := []spec.EventDecl{
		{
			Name:    "completed",
			Type:    spec.EventTypeAsync,
			Payload: &spec.PayloadDecl{Fields: []string{"id", "total"}},
			Deliver: []spec.EventDeliveryDecl{{Channel: "audit_log"}, {Channel: "websocket"}},
		},
		{
			Name:    "created",
			Type:    spec.EventTypeAsync,
			Publish: &spec.PublishDecl{Durable: true},
		},
	}
	data := map[string]any{"id": "v1", "total": 100, "diagnosis": "flu"}

	t.Run("empty emits returns nil", func(t *testing.T) {
		if got := ResolveEmission(events, "", "", data); got != nil {
			t.Errorf("got %+v, want nil", got)
		}
	})

	t.Run("no matching event returns nil", func(t *testing.T) {
		if got := ResolveEmission(events, "does_not_exist", "", data); got != nil {
			t.Errorf("got %+v, want nil", got)
		}
	})

	t.Run("matches by name, projects payload fields, carries deliver", func(t *testing.T) {
		got := ResolveEmission(events, "completed", "", data)
		if got == nil {
			t.Fatal("got nil, want an emission")
		}
		if got.Name != "completed" {
			t.Errorf("Name = %q, want \"completed\"", got.Name)
		}
		if got.Durable {
			t.Error("Durable = true, want false (no publish.durable set)")
		}
		want := map[string]any{"id": "v1", "total": 100}
		if !reflect.DeepEqual(got.Payload, want) {
			t.Errorf("Payload = %+v, want %+v (projected to declared fields only)", got.Payload, want)
		}
		if len(got.DeliverTo) != 2 {
			t.Errorf("DeliverTo = %+v, want 2 entries", got.DeliverTo)
		}
	})

	t.Run("no payload.fields declared uses full record data", func(t *testing.T) {
		got := ResolveEmission(events, "created", "", data)
		if got == nil {
			t.Fatal("got nil, want an emission")
		}
		if !got.Durable {
			t.Error("Durable = false, want true (publish.durable: true)")
		}
		if !reflect.DeepEqual(got.Payload, data) {
			t.Errorf("Payload = %+v, want the full record data %+v", got.Payload, data)
		}
	})
}

// TestResolveTransitionEmission pins S13 (item 6.1): the transition↔event link
// is explicit. An event is emitted because the transition declares `emit`, not
// because its name happens to match a state.
func TestResolveTransitionEmission(t *testing.T) {
	events := []spec.EventDecl{
		{Name: "on_paid", Type: spec.EventTypeAsync, Publish: &spec.PublishDecl{Durable: true}},
		{Name: "on_cancel", Type: spec.EventTypeAsync, Publish: &spec.PublishDecl{Durable: true}},
	}
	sm := &spec.StateMachine{
		Field: "status",
		Transitions: []spec.TransitionDecl{
			{From: spec.StateList{"awaiting_payment"}, To: "paid", Action: "confirm-payment", Emit: "on_paid"},
			{From: spec.StateList{"paid", "in_kitchen", "ready", "served"}, To: "cancelled", Action: "void-order", Emit: "on_cancel"},
			{From: spec.StateList{"paid"}, To: "in_kitchen", Action: "start-preparing"}, // no emit
		},
	}
	data := map[string]any{"id": "o1", "status": "paid"}

	t.Run("transition with emit publishes its event", func(t *testing.T) {
		got := ResolveTransitionEmission(sm, events, "awaiting_payment", "paid", "", data)
		if got == nil || got.Name != "on_paid" {
			t.Fatalf("got %+v, want on_paid", got)
		}
		if !got.Durable {
			t.Error("Durable = false, want true")
		}
	})

	t.Run("multi-origin transition matches any origin", func(t *testing.T) {
		for _, from := range []string{"paid", "in_kitchen", "ready", "served"} {
			got := ResolveTransitionEmission(sm, events, from, "cancelled", "", data)
			if got == nil || got.Name != "on_cancel" {
				t.Errorf("from %s: got %+v, want on_cancel", from, got)
			}
		}
	})

	t.Run("transition without emit publishes nothing", func(t *testing.T) {
		if got := ResolveTransitionEmission(sm, events, "paid", "in_kitchen", "", data); got != nil {
			t.Errorf("got %+v, want nil (transition declares no emit)", got)
		}
	})

	t.Run("no matching transition returns nil", func(t *testing.T) {
		if got := ResolveTransitionEmission(sm, events, "draft", "paid", "", data); got != nil {
			t.Errorf("got %+v, want nil", got)
		}
	})

	t.Run("no state change returns nil", func(t *testing.T) {
		if got := ResolveTransitionEmission(sm, events, "paid", "paid", "", data); got != nil {
			t.Errorf("got %+v, want nil (no transition)", got)
		}
	})
}

// A record's id is a table column, not a field, so `data["id"]` is absent for
// every real record. A `payload.fields: [id]` projection therefore produced a
// null id, and every consumer that addressed the record by it failed —
// gl-balance's update action died on `resource.fetch("gl.journal-entry",
// params.id)` with "got NoneType, want string", so the balance projection was
// never written even though the journal posted correctly.
func TestResolveEmission_RecordIDComesFromTheColumn(t *testing.T) {
	events := []spec.EventDecl{{
		Name:    "journal-posted",
		Type:    spec.EventTypeAsync,
		Publish: &spec.PublishDecl{Durable: true},
		Payload: &spec.PayloadDecl{Fields: []string{"id", "number", "entry_date"}},
	}}
	// Note: no "id" key — exactly what the store hands back.
	recordData := map[string]any{"number": "JRN-2026-000001", "entry_date": "2026-09-21"}

	t.Run("declared id resolves from the column", func(t *testing.T) {
		got := ResolveEmission(events, "journal-posted", "01a0c332-d45b-7039-b2d3-f8f52d0e63dc", recordData)
		if got == nil {
			t.Fatal("got nil, want an emission")
		}
		if got.Payload["id"] != "01a0c332-d45b-7039-b2d3-f8f52d0e63dc" {
			t.Errorf("Payload[id] = %v, want the record id from the column", got.Payload["id"])
		}
		if got.Payload["number"] != "JRN-2026-000001" {
			t.Errorf("Payload[number] = %v, want the projected field", got.Payload["number"])
		}
	})

	t.Run("declared fields stay present-but-null", func(t *testing.T) {
		// A publisher promised these fields; a consumer may rely on the key.
		got := ResolveEmission(events, "journal-posted", "rec-1", map[string]any{"number": "JRN-1"})
		if _, ok := got.Payload["entry_date"]; !ok {
			t.Error("a declared field missing from the record must still appear as a key")
		}
		if got.Payload["entry_date"] != nil {
			t.Errorf("entry_date = %v, want nil", got.Payload["entry_date"])
		}
	})

	t.Run("unknown id is null, not empty string", func(t *testing.T) {
		// An empty string would read as a real (but wrong) identifier.
		got := ResolveEmission(events, "journal-posted", "", recordData)
		if got.Payload["id"] != nil {
			t.Errorf("Payload[id] = %v, want nil when the id is unknown", got.Payload["id"])
		}
	})

	t.Run("an entity's own id field wins", func(t *testing.T) {
		// A data map that really carries "id" is that entity's field.
		got := ResolveEmission(events, "journal-posted", "column-id",
			map[string]any{"id": "declared-field", "number": "JRN-1"})
		if got.Payload["id"] != "declared-field" {
			t.Errorf("Payload[id] = %v, want the entity's own field to win", got.Payload["id"])
		}
	})

	t.Run("no payload.fields still carries the id", func(t *testing.T) {
		full := []spec.EventDecl{{
			Name:    "on_paid",
			Type:    spec.EventTypeAsync,
			Publish: &spec.PublishDecl{Durable: true},
		}}
		src := map[string]any{"status": "paid"}
		got := ResolveEmission(full, "on_paid", "order-1", src)
		if got.Payload["id"] != "order-1" {
			t.Errorf("Payload[id] = %v, want the record id", got.Payload["id"])
		}
		// The caller's live record must not be mutated — an "id" key added to
		// it would be persisted.
		if _, leaked := src["id"]; leaked {
			t.Error("the caller's record data must not gain an id key")
		}
	})
}

// The transition-declared emit path takes the same record id.
func TestResolveTransitionEmission_CarriesRecordID(t *testing.T) {
	sm := &spec.StateMachine{
		Field: "status",
		Transitions: []spec.TransitionDecl{
			{From: spec.StateList{"paid"}, To: "cancelled", Emit: "order-cancelled"},
		},
	}
	events := []spec.EventDecl{{
		Name:    "order-cancelled",
		Type:    spec.EventTypeAsync,
		Publish: &spec.PublishDecl{Durable: true},
		Payload: &spec.PayloadDecl{Fields: []string{"id", "number"}},
	}}

	got := ResolveTransitionEmission(sm, events, "paid", "cancelled", "order-9", map[string]any{"number": "ORD-9"})
	if got == nil {
		t.Fatal("got nil, want an emission")
	}
	if got.Payload["id"] != "order-9" {
		t.Errorf("Payload[id] = %v, want the record id", got.Payload["id"])
	}
}
