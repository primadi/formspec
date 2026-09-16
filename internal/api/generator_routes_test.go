package api

import (
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// UIRoutesForEntity / UICustomActionRoutesForEntity exist so the REST contract
// can be *printed* (formspec describe, docs) from the same generator the server
// registers routes with (gap #47). These tests pin the parts a hand-written
// contract gets wrong: which actions a spec actually produces.
func TestUIRoutesForEntity_LifecycleAndDisabled(t *testing.T) {
	lifecycle := &spec.EntitySpec{
		Version: "v1",
		Plural:  "invoices",
		Fields:  []spec.Field{{Name: "number", Type: spec.FieldString}},
	}
	got := actionsOf(UIRoutesForEntity("billing", "invoice", lifecycle))
	for _, want := range []string{"list", "find", "create", "update", "delete", "submit", "cancel", "amend"} {
		if !got[want] {
			t.Errorf("lifecycle entity: expected a route for %q, got %v", want, got)
		}
	}

	// Kafe's `order`: lifecycle-free (submit disabled) and delete explicitly
	// disabled — the printed contract must not promise either.
	order := &spec.EntitySpec{
		Version: "v1",
		Plural:  "orders",
		Actions: []spec.Action{{Name: "delete", Disabled: true}, {Name: "submit", Disabled: true}},
		Fields:  []spec.Field{{Name: "number", Type: spec.FieldString}},
	}
	got = actionsOf(UIRoutesForEntity("cafe-order", "order", order))
	for _, want := range []string{"list", "find", "create", "update"} {
		if !got[want] {
			t.Errorf("order: expected a route for %q, got %v", want, got)
		}
	}
	for _, unwanted := range []string{"delete", "submit", "cancel", "amend"} {
		if got[unwanted] {
			t.Errorf("order: %q must not have a route (disabled or lifecycle-free)", unwanted)
		}
	}

	// Summary projections are read-only on this surface.
	summary := &spec.EntitySpec{
		Version:        "v1",
		Plural:         "stock-levels",
		Characteristic: spec.CharSummary,
		Fields:         []spec.Field{{Name: "quantity_on_hand", Type: spec.FieldDecimal}},
	}
	got = actionsOf(UIRoutesForEntity("cafe-stock", "stock-level", summary))
	if !got["list"] || !got["find"] || got["create"] || got["update"] || got["delete"] {
		t.Errorf("summary: want list+find only, got %v", got)
	}
}

// A state-machine transition without an `impl` has no endpoint of its own — it is
// applied through update. Printing a route for it would document an endpoint the
// server never serves.
func TestUICustomActionRoutesForEntity_OnlyActionsWithImpl(t *testing.T) {
	es := &spec.EntitySpec{
		Version: "v1",
		Plural:  "orders",
		Actions: []spec.Action{
			{Name: "start-preparing"}, // state-machine transition, no impl
			{Name: "void-order", Impl: &spec.ImplDecl{Type: spec.ImplScriptRef, Ref: "cafe-order/void_order"}},
			{Name: "abandon", Disabled: true, Impl: &spec.ImplDecl{Type: spec.ImplScriptRef, Ref: "x"}},
		},
	}
	routes := UICustomActionRoutesForEntity("cafe-order", "order", es)
	if len(routes) != 1 {
		t.Fatalf("want exactly the impl-backed action, got %d route(s)", len(routes))
	}
	rd := routes[0]
	if rd.Action != "void-order" {
		t.Fatalf("got action %q", rd.Action)
	}
	if rd.Method != "POST" || rd.Path != "/_ui/entity/cafe-order/order/{id}/void-order" {
		t.Fatalf("unexpected route %s %s", rd.Method, rd.Path)
	}
	if rd.RequiredPermission != "cafe-order.orders.void-order" {
		t.Fatalf("permission = %q, want the canonical {module}.{plural}.{action} form", rd.RequiredPermission)
	}
}

// actionsOf collapses a route list into the set of actions it exposes.
func actionsOf(routes []RouteDescriptor) map[string]bool {
	out := make(map[string]bool, len(routes))
	for _, rd := range routes {
		out[rd.Action] = true
	}
	return out
}
