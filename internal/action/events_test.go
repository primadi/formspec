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
		if got := ResolveEmission(events, "", data); got != nil {
			t.Errorf("got %+v, want nil", got)
		}
	})

	t.Run("no matching event returns nil", func(t *testing.T) {
		if got := ResolveEmission(events, "does_not_exist", data); got != nil {
			t.Errorf("got %+v, want nil", got)
		}
	})

	t.Run("matches by name, projects payload fields, carries deliver", func(t *testing.T) {
		got := ResolveEmission(events, "completed", data)
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
		got := ResolveEmission(events, "created", data)
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
