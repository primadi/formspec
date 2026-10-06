package approval

import (
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

func TestRegistry_AddGetList(t *testing.T) {
	reg := NewRegistry()
	regAdd(reg, "gl", "journal-entry", "post", []string{"draft"}, "posted",
		spec.ApprovalStep{Roles: []string{"gl.supervisor"}, Approvers: 1})

	got, ok := reg.Get("gl", "journal-entry.post")
	if !ok || got == nil {
		t.Fatal("expected the approval gate to be found by {entity}.{transition}")
	}
	if len(got.Steps) != 1 {
		t.Fatalf("steps: want 1, got %d", len(got.Steps))
	}

	infos := reg.List()
	if len(infos) != 1 {
		t.Fatalf("List: want 1, got %d", len(infos))
	}
	if infos[0].Name != "journal-entry.post" || infos[0].Transition != "post" || infos[0].To != "posted" {
		t.Errorf("List[0]: want gl/journal-entry.post draft->posted, got %s/%s %v->%s",
			infos[0].Module, infos[0].Name, infos[0].From, infos[0].To)
	}
}

func TestRegistry_ForTransition(t *testing.T) {
	reg := NewRegistry()
	regAdd(reg, "gl", "journal-entry", "post", []string{"draft"}, "posted",
		spec.ApprovalStep{Roles: []string{"gl.supervisor"}})
	regAdd(reg, "gl", "journal-entry", "cancel", []string{"draft"}, "cancelled",
		spec.ApprovalStep{Roles: []string{"gl.supervisor"}})

	if len(reg.ForTransition("gl.journal-entry", "post")) != 1 {
		t.Fatal("ForTransition(post): want 1")
	}
	if len(reg.ForTransition("gl.journal-entry", "cancel")) != 1 {
		t.Fatal("ForTransition(cancel): want 1")
	}
	// A transition nobody gated has no approval.
	if len(reg.ForTransition("gl.journal-entry", "amend")) != 0 {
		t.Fatal("ForTransition(amend): want 0")
	}
}

// TestRegistry_MultiOriginTransitionIsCoveredByConstruction is the S9 property
// now guaranteed by shape rather than by a reference that must be kept in step:
// a gate declared ON the transition covers EVERY origin state, because the
// lookup is by transition name alone.
func TestRegistry_MultiOriginTransitionIsCoveredByConstruction(t *testing.T) {
	reg := NewRegistry()
	regAdd(reg, "cafe-order", "order", "void-order",
		[]string{"paid", "in_kitchen", "ready", "served"}, "cancelled",
		spec.ApprovalStep{Roles: []string{"supervisor"}})

	// Every origin of the transition resolves to the same gate.
	for _, from := range []string{"paid", "in_kitchen", "ready", "served"} {
		if got := reg.ForTransition("cafe-order.order", "void-order"); len(got) != 1 {
			t.Errorf("void-order from %s: want 1 gate, got %d", from, len(got))
		}
	}

	// A different transition on the same entity must not be gated, even though it
	// shares the target state.
	if got := reg.ForTransition("cafe-order.order", "cancel-order"); len(got) != 0 {
		t.Errorf("cancel-order must not be gated by the void approval, got %d", len(got))
	}

	// And the same transition name on another entity must not match either —
	// the entity is part of the key.
	if got := reg.ForTransition("cafe-stock.order", "void-order"); len(got) != 0 {
		t.Errorf("entity is part of the key, got %d", len(got))
	}
}

// TestRegistry_GetByNameResolvesDerivedName keeps a grant's `workflow:{name}`
// reference resolvable: the name a grant uses is the same `{entity}.{transition}`
// the row stores.
func TestRegistry_GetByNameResolvesDerivedName(t *testing.T) {
	reg := NewRegistry()
	regAdd(reg, "kafe", "order", "void-order", []string{"paid"}, "cancelled",
		spec.ApprovalStep{Name: "s", Permission: "s"})

	module, a, ok := reg.GetByName(gateName("order", "void-order"))
	if !ok || a == nil || module != "kafe" {
		t.Fatalf("GetByName(order.void-order) = (%q, %v, %v)", module, a != nil, ok)
	}
}
