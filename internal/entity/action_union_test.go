package entity

import (
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// GetActionSpec resolves over the UNION of declared actions and state-machine
// transitions (plan docs_internal/plan/via-sebagai-action-penuh.md, L2).
//
// Why: a transition's `via` IS an action — it names what a caller invokes, and
// the path that applies the transition (PATCH) already accepts it. Reading only
// `actions:` made `via` second-class: it could not be dispatched and had to be
// duplicated as an action entry to work at all.
func TestGetActionSpec_ResolvesTransitionVia(t *testing.T) {
	reg := &Registry{specs: map[string]*SpecInfo{}}
	es := &spec.EntitySpec{
		Plural: "dining-tables",
		Actions: []spec.Action{
			{Name: "occupy", Description: "declared action"},
		},
		StateMachine: &spec.StateMachine{
			Field:   "table_status",
			Initial: "available",
			Transitions: []spec.TransitionDecl{
				{From: spec.StateList{"available"}, To: "occupied", Action: "occupy"},
				{
					From: spec.StateList{"occupied"}, To: "served",
					Action:      "mark-table-served",
					Description: "Semua pesanan sudah disajikan",
				},
			},
		},
	}
	reg.specs[entityKey("cafe-master", "dining-table")] = &SpecInfo{
		Metadata:   spec.Metadata{Name: "dining-table", Module: "cafe-master"},
		EntitySpec: es,
	}

	t.Run("declared action still resolves", func(t *testing.T) {
		got, ok := reg.GetActionSpec("cafe-master", "dining-table", "occupy")
		if !ok || got == nil {
			t.Fatal("declared action must still resolve")
		}
	})

	t.Run("declared action wins over a transition of the same name", func(t *testing.T) {
		got, ok := reg.GetActionSpec("cafe-master", "dining-table", "occupy")
		if !ok {
			t.Fatal("occupy must resolve")
		}
		if got.Description != "declared action" {
			t.Fatalf("a declared action must win: got description %q", got.Description)
		}
	})

	t.Run("transition-only via resolves", func(t *testing.T) {
		got, ok := reg.GetActionSpec("cafe-master", "dining-table", "mark-table-served")
		if !ok || got == nil {
			t.Fatal("a `via` with no matching action entry must resolve (L2)")
		}
		if got.Name != "mark-table-served" {
			t.Fatalf("name = %q", got.Name)
		}
		if got.Description != "Semua pesanan sudah disajikan" {
			t.Fatalf("the transition's description belongs to its action: got %q", got.Description)
		}
	})

	// A synthesised action must NOT gain a route: routes require an `impl`, and
	// inventing one here would silently broaden the API surface.
	t.Run("transition-only via carries no impl", func(t *testing.T) {
		got, ok := reg.GetActionSpec("cafe-master", "dining-table", "mark-table-served")
		if !ok {
			t.Fatal("mark-table-served must resolve")
		}
		if got.Impl != nil {
			t.Fatal("a synthesised action must not carry an impl — that would invent a route")
		}
	})

	// It must also not inherit the transition's gate: that is enforced on the
	// PATCH path, and copying it here would read as "the same permission
	// declared twice", which the transition validator rejects.
	t.Run("transition-only via does not duplicate the gate", func(t *testing.T) {
		es.StateMachine.Transitions[1].RequirePermission = "dining-tables.mark-table-served"
		defer func() { es.StateMachine.Transitions[1].RequirePermission = "" }()

		got, ok := reg.GetActionSpec("cafe-master", "dining-table", "mark-table-served")
		if !ok {
			t.Fatal("must resolve")
		}
		if got.RequiredPermission != "" {
			t.Fatalf("gate must stay on the transition, got action RequiredPermission %q",
				got.RequiredPermission)
		}
	})

	t.Run("unknown name still fails", func(t *testing.T) {
		if _, ok := reg.GetActionSpec("cafe-master", "dining-table", "nope"); ok {
			t.Fatal("an unknown name must not resolve")
		}
	})

	// A transition with no `via` declares no action — it must not resolve, and
	// it must not accidentally resolve the empty name.
	t.Run("transition without via is not an action", func(t *testing.T) {
		es.StateMachine.Transitions = append(es.StateMachine.Transitions, spec.TransitionDecl{
			From: spec.StateList{"available"}, To: "occupied",
		})
		defer func() {
			es.StateMachine.Transitions = es.StateMachine.Transitions[:len(es.StateMachine.Transitions)-1]
		}()
		if got, ok := reg.GetActionSpec("cafe-master", "dining-table", ""); ok {
			t.Fatalf("empty name must not resolve, got %+v", got)
		}
	})
}
