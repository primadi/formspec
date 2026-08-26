package subscription

import (
	"context"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

func TestRecordToSubscription(t *testing.T) {
	ds, ok := RecordToSubscription(map[string]any{
		"name":         "audit-orders",
		"events":       []any{"billing.order.on_submit"},
		"handler_type": "script_ref",
		"handler_ref":  "billing/audit-order",
		"active":       true,
	})
	if !ok {
		t.Fatal("expected valid record to convert")
	}
	if ds.Name != "audit-orders" {
		t.Errorf("name: want audit-orders, got %q", ds.Name)
	}
	if len(ds.Spec.Events) != 1 || ds.Spec.Events[0] != "billing.order.on_submit" {
		t.Errorf("events: want [billing.order.on_submit], got %v", ds.Spec.Events)
	}
	if ds.Spec.Handler.Type != spec.ImplScriptRef || ds.Spec.Handler.Ref != "billing/audit-order" {
		t.Errorf("handler: want script_ref billing/audit-order, got %s %s", ds.Spec.Handler.Type, ds.Spec.Handler.Ref)
	}
}

func TestRecordToSubscription_Tier2Fields(t *testing.T) {
	ds, ok := RecordToSubscription(map[string]any{
		"name":         "stream-audit",
		"events":       []any{"billing.order.on_submit"},
		"handler_type": "script_ref",
		"handler_ref":  "billing/audit-order",
		"durability":   "durable",
		"store":        "redis",
		"position":     "earliest",
		"filter":       "amount > 100",
		"transform":    `{"total": amount}`,
		"max_retry":    float64(5),
		"retention":    "7d",
		"active":       true,
	})
	if !ok {
		t.Fatal("expected valid record to convert")
	}
	if ds.Spec.Durable != "durable" || ds.Spec.Store != "redis" || ds.Spec.Position != "earliest" {
		t.Errorf("tier2 fields: got durable=%q store=%q position=%q", ds.Spec.Durable, ds.Spec.Store, ds.Spec.Position)
	}
	if ds.Spec.Filter != "amount > 100" || ds.Spec.Transform != `{"total": amount}` {
		t.Errorf("filter/transform: got %q / %q", ds.Spec.Filter, ds.Spec.Transform)
	}
	if ds.Spec.MaxRetry != 5 || ds.Spec.Retention != "7d" {
		t.Errorf("max_retry/retention: got %d / %q", ds.Spec.MaxRetry, ds.Spec.Retention)
	}
}

func TestRecordToSubscription_SkipsInactiveAndMalformed(t *testing.T) {
	cases := []map[string]any{
		// inactive
		{"name": "x", "events": []any{"e.on_x"}, "handler_type": "script_ref", "handler_ref": "m/h", "active": false},
		// no name
		{"events": []any{"e.on_x"}, "handler_type": "script_ref", "handler_ref": "m/h", "active": true},
		// no events
		{"name": "x", "handler_type": "script_ref", "handler_ref": "m/h", "active": true},
		// no handler
		{"name": "x", "events": []any{"e.on_x"}, "active": true},
	}
	for i, data := range cases {
		if _, ok := RecordToSubscription(data); ok {
			t.Errorf("case %d: expected skip, got ok", i)
		}
	}
}

func TestRegistry_MergeDynamic(t *testing.T) {
	reg := NewRegistry()
	// A manifest subscription (static) — must survive dynamic merges.
	reg.Add("billing", "manifest-audit", &spec.SubscriptionSpec{
		Events:  []string{"billing.invoice.on_submit"},
		Handler: spec.ImplDecl{Type: spec.ImplNative, Ref: "billing.audit"},
	})

	reg.MergeDynamic([]DynamicSubscription{
		{Name: "dyn-1", Spec: &spec.SubscriptionSpec{
			Events:  []string{"billing.invoice.on_submit"},
			Handler: spec.ImplDecl{Type: spec.ImplNative, Ref: "billing.dyn1"},
		}},
	})

	// Both manifest + dynamic match the event.
	if got := len(reg.ForEvent("billing.invoice.on_submit")); got != 2 {
		t.Fatalf("ForEvent: want 2 (manifest + dynamic), got %d", got)
	}
	if got := len(reg.Durable()); got != 0 {
		t.Fatalf("Durable: want 0, got %d", got)
	}

	// Re-merge replaces dynamic entries (no stale mappings).
	reg.MergeDynamic([]DynamicSubscription{
		{Name: "dyn-1", Spec: &spec.SubscriptionSpec{
			Events:  []string{"billing.invoice.on_cancel"},
			Handler: spec.ImplDecl{Type: spec.ImplNative, Ref: "billing.dyn1"},
		}},
	})
	if got := len(reg.ForEvent("billing.invoice.on_submit")); got != 1 {
		t.Fatalf("ForEvent(on_submit) after re-merge: want 1 (manifest only), got %d", got)
	}
	if got := len(reg.ForEvent("billing.invoice.on_cancel")); got != 1 {
		t.Fatalf("ForEvent(on_cancel) after re-merge: want 1 (dynamic), got %d", got)
	}
	// Manifest subscription untouched.
	if _, ok := reg.Get("billing", "manifest-audit"); !ok {
		t.Fatal("manifest subscription should survive dynamic merges")
	}
}

func TestRegistry_MergeDynamicDurable(t *testing.T) {
	reg := NewRegistry()
	reg.MergeDynamic([]DynamicSubscription{
		{Name: "dyn-stream", Spec: &spec.SubscriptionSpec{
			Events:  []string{"billing.invoice.on_submit"},
			Handler: spec.ImplDecl{Type: spec.ImplNative, Ref: "billing.dyn1"},
			Durable: "durable",
		}},
	})
	durable := reg.Durable()
	if len(durable) != 1 {
		t.Fatalf("Durable: want 1, got %d", len(durable))
	}
	if durable[0].Module != CoreModule || durable[0].Name != "dyn-stream" {
		t.Errorf("Durable[0]: want formspec.core/dyn-stream, got %s/%s", durable[0].Module, durable[0].Name)
	}
}

func TestDynamicRefresher_Refresh(t *testing.T) {
	reg := NewRegistry()
	source := func(_ context.Context, _ string) ([]DynamicSubscription, error) {
		return []DynamicSubscription{
			{Name: "dyn-1", Spec: &spec.SubscriptionSpec{
				Events:  []string{"billing.invoice.on_submit"},
				Handler: spec.ImplDecl{Type: spec.ImplNative, Ref: "billing.dyn1"},
			}},
		}, nil
	}
	w := NewDynamicRefresher(reg, source, "ws-1")
	if err := w.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := len(reg.ForEvent("billing.invoice.on_submit")); got != 1 {
		t.Fatalf("ForEvent after refresh: want 1, got %d", got)
	}
}
