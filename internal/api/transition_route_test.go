package api

import (
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// L3 (plan docs_internal/plan/via-sebagai-action-penuh.md): a transition that
// declares `impl` gets its own route under its `via` name, WITHOUT being
// duplicated as an `actions:` entry.
//
// This is what unblocks removing the 85 duplicated declarations in the repo:
// today `gl/journal-entry` must declare BOTH
// `transitions: [- via: post]` and `actions: [- name: post, impl: …]`, because
// only the action entry produced a route.
func TestUICustomActionRoutes_TransitionWithImpl(t *testing.T) {
	es := &spec.EntitySpec{
		Plural: "journal-entries",
		Actions: []spec.Action{
			// Deliberately NO `post` here: the transition is the only place it
			// is declared.
			{Name: "delete", Disabled: true},
		},
		StateMachine: &spec.StateMachine{
			Field:   "status",
			Initial: "draft",
			Transitions: []spec.TransitionDecl{
				{
					From: spec.StateList{"draft"}, To: "posted", Action: "post",
					Impl: &spec.ImplDecl{Type: "script_ref", Ref: "gl/journal_post"},
				},
				// No impl → no route (applied via PATCH instead).
				{From: spec.StateList{"draft"}, To: "cancelled", Action: "void"},
				// No `via` → declares no action at all.
				{From: spec.StateList{"posted"}, To: "reversed"},
			},
		},
	}

	routes := UICustomActionRoutesForEntity("gl", "journal-entry", es)

	byAction := map[string]RouteDescriptor{}
	for _, r := range routes {
		byAction[r.Action] = r
	}

	post, ok := byAction["post"]
	if !ok {
		t.Fatal("a transition with `impl` must get a route under its `via` name — " +
			"without this, removing the duplicated `actions:` entry would break it")
	}
	if post.Path != "/_ui/entity/gl/journal-entry/{id}/post" {
		t.Errorf("path = %q", post.Path)
	}
	if post.RequiredPermission == "" {
		t.Error("route must carry a permission")
	}

	if _, ok := byAction["void"]; ok {
		t.Error("a transition WITHOUT `impl` must not get a route — it has no " +
			"endpoint, and the PATCH path applies it")
	}
	if _, ok := byAction[""]; ok {
		t.Error("a transition without `via` must not produce a route")
	}
}

// A manifest that keeps both spellings — every manifest in the repo today — must
// produce exactly the SAME routes as before. The union is an addition, not a
// migration.
func TestUICustomActionRoutes_DeclaredAndTransitionDoNotDuplicate(t *testing.T) {
	impl := &spec.ImplDecl{Type: "script_ref", Ref: "x/y"}
	es := &spec.EntitySpec{
		Plural: "orders",
		Actions: []spec.Action{
			{Name: "post", Impl: impl, RequiredPermission: "orders.post"},
		},
		StateMachine: &spec.StateMachine{
			Field:   "status",
			Initial: "draft",
			Transitions: []spec.TransitionDecl{
				{From: spec.StateList{"draft"}, To: "posted", Action: "post", Impl: impl},
			},
		},
	}

	routes := UICustomActionRoutesForEntity("billing", "order", es)
	count := 0
	for _, r := range routes {
		if r.Action == "post" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("want exactly 1 route for `post`, got %d — the union must not "+
			"register the same path twice (the second would shadow the first)", count)
	}
}
