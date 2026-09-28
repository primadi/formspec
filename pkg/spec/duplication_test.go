package spec

import (
	"strings"
	"testing"
)

// L4 (plan docs_internal/plan/l4-validator-anti-duplikat.md): a `via` that is
// ALSO declared under `actions:` with nothing of its own to say is a leftover.
//
// It is a leftover because L3 made the transition a full action source: it gets
// the permission, the route (when it declares `impl`), the bundle entry, and the
// button. Before L3 the entry was load-bearing; now it is a second home for the
// same name.
func TestValidateDuplication_RejectsPureDuplicate(t *testing.T) {
	d := &EntitySpec{
		Plural: "dining-tables",
		Actions: []Action{
			{Name: "release", Description: "Kosongkan meja"},
		},
		StateMachine: &StateMachine{
			Field:   "table_status",
			Initial: "available",
			Transitions: []TransitionDecl{
				{From: StateList{"occupied"}, To: "available", Action: "release",
					Description: "Kosongkan meja", RequirePermission: "dining-tables.release"},
			},
		},
	}
	err := ValidateActionTransitionDuplication(d)
	if err == nil {
		t.Fatal("a `via` duplicated in `actions:` must be rejected — the transition " +
			"already declares it, so the entry is a second home for one name")
	}
	if !strings.Contains(err.Error(), "release") {
		t.Errorf("error must name the offending action, got: %v", err)
	}
}

// The two shapes that MUST stay legal. Rejecting either would be a regression,
// so they are pinned here rather than left to the implementation's discretion.
func TestValidateDuplication_AllowsTheTwoLoadBearingShapes(t *testing.T) {
	// (1) A RESERVED name. `cancel` gets a generic lifecycle route whose
	// permission is `{module}.{plural}.cancel`; the `actions:` entry is the only
	// way to narrow it. Measured: examples/cafe/.../order/entity.yaml narrows it
	// to the SINGULAR `cafe-order.order.cancel`, and without that entry every
	// holder of `update` could cancel a money-bearing order. Six entities depend
	// on this.
	t.Run("reserved name (cancel) that narrows the generic route", func(t *testing.T) {
		d := &EntitySpec{
			Plural: "orders",
			Actions: []Action{
				{Name: "cancel", RequiredPermission: "cafe-order.order.cancel"},
			},
			StateMachine: &StateMachine{
				Field:   "status",
				Initial: "open",
				Transitions: []TransitionDecl{
					{From: StateList{"open"}, To: "cancelled", Action: "cancel"},
				},
			},
		}
		if err := ValidateActionTransitionDuplication(d); err != nil {
			t.Fatalf("`cancel` must be exempt from the duplicate check — rejecting it "+
				"would delete the only way to narrow a lifecycle route's permission: %v", err)
		}
	})

	// (2) An entry carrying an explicit permission the transition does NOT gate
	// on. The route will require the ENTRY's name, so deleting it would move the
	// permission to the fallback — an authorization change, not a cleanup.
	t.Run("entry narrows the route permission", func(t *testing.T) {
		d := &EntitySpec{
			Plural: "orders",
			Actions: []Action{
				{Name: "settle", RequiredPermission: "cafe-order.order.settle"},
			},
			StateMachine: &StateMachine{
				Field:   "status",
				Initial: "open",
				Transitions: []TransitionDecl{
					{From: StateList{"open"}, To: "paid", Action: "settle"},
				},
			},
		}
		if err := ValidateActionTransitionDuplication(d); err != nil {
			t.Fatalf("an explicit required_permission is the entry earning its place: %v", err)
		}
	})
}

// A transition SHARING a `via` with other transitions must produce ONE action,
// not one per transition. Measured on the live meta bundle before this guard:
// `dining-table` reported `['occupy','occupy','mark-table-served','occupy',…]`
// — three `occupy` entries from the three transitions that use it, which
// duplicates React keys in the transition-button list.
func TestActionSources_SharedViaYieldsOneAction(t *testing.T) {
	es := &EntitySpec{
		Plural: "dining-tables",
		StateMachine: &StateMachine{
			Field:   "table_status",
			Initial: "available",
			Transitions: []TransitionDecl{
				{From: StateList{"available"}, To: "occupied", Action: "occupy"},
				{From: StateList{"reserved"}, To: "occupied", Action: "occupy"},
				{From: StateList{"served"}, To: "occupied", Action: "occupy"},
				{From: StateList{"occupied"}, To: "available", Action: "release"},
			},
		},
	}

	seen := map[string]int{}
	for _, a := range es.ActionSources() {
		seen[a.Name]++
	}
	if seen["occupy"] != 1 {
		t.Fatalf("`occupy` appears %d times, want 1 — a `via` shared by several "+
			"transitions is ONE action; one-per-transition duplicates the name",
			seen["occupy"])
	}
	if len(es.ActionSources()) != 2 {
		t.Errorf("want 2 distinct actions (occupy, release), got %d: %+v",
			len(es.ActionSources()), es.ActionSources())
	}
}

// The first transition to name a `via` must be the one that wins, so the
// generator, the runtime lookup (`GetActionSpec`) and this union cannot disagree
// about which transition's `impl` a route runs.
func TestActionSources_FirstTransitionWinsForSharedVia(t *testing.T) {
	first := &ImplDecl{Type: "script_ref", Ref: "a/first"}
	es := &EntitySpec{
		StateMachine: &StateMachine{
			Transitions: []TransitionDecl{
				{From: StateList{"available"}, To: "occupied", Action: "occupy", Impl: first},
				{From: StateList{"reserved"}, To: "occupied", Action: "occupy"},
			},
		},
	}
	for _, a := range es.ActionSources() {
		if a.Name != "occupy" {
			continue
		}
		if a.Impl == nil || a.Impl.Ref != "a/first" {
			t.Fatalf("the FIRST transition naming `occupy` must win (impl=%v), "+
				"otherwise the route's handler and the lookup disagree", a.Impl)
		}
	}
}

// The entry must be KEPT when it carries a field the transition lacks.
//
// This is a regression guard for a real false positive: the first version of
// this validator compared only `impl` and `required_permission`, so it called
// kafe's `confirm-payment` entry "adds nothing" and told the author to delete
// it — silently dropping `audit: true` and its description. Measured
// 2026-09-27 while gating the money path.
func TestValidateDuplication_KeepsEntryWithFieldsTheTransitionLacks(t *testing.T) {
	t.Run("audit present only on the action", func(t *testing.T) {
		d := &EntitySpec{
			Plural:  "orders",
			Actions: []Action{{Name: "confirm-payment", Audit: true}},
			StateMachine: &StateMachine{
				Field: "status", Initial: "awaiting_payment",
				Transitions: []TransitionDecl{
					{From: StateList{"awaiting_payment"}, To: "paid", Action: "confirm-payment",
						RequirePermission: "orders.confirm-payment"},
				},
			},
		}
		if err := ValidateActionTransitionDuplication(d); err != nil {
			t.Fatalf("the entry carries `audit: true` that the transition lacks — "+
				"deleting it would drop the audit trail, so it must be allowed: %v", err)
		}
	})

	t.Run("uses present only on the action", func(t *testing.T) {
		d := &EntitySpec{
			Plural:  "orders",
			Actions: []Action{{Name: "post", Uses: &UsesDecl{Resources: []string{"gl"}}}},
			StateMachine: &StateMachine{
				Field: "status", Initial: "draft",
				Transitions: []TransitionDecl{{From: StateList{"draft"}, To: "posted", Action: "post"}},
			},
		}
		if err := ValidateActionTransitionDuplication(d); err != nil {
			t.Fatalf("`uses` is a consent footprint; dropping it would widen access: %v", err)
		}
	})

	t.Run("truly empty entry is still rejected", func(t *testing.T) {
		d := &EntitySpec{
			Plural:  "orders",
			Actions: []Action{{Name: "post"}},
			StateMachine: &StateMachine{
				Field: "status", Initial: "draft",
				Transitions: []TransitionDecl{
					{From: StateList{"draft"}, To: "posted", Action: "post", Description: "Posting"},
				},
			},
		}
		if err := ValidateActionTransitionDuplication(d); err == nil {
			t.Fatal("an entry with nothing of its own is exactly the leftover this " +
				"validator exists to reject")
		}
	})
}
