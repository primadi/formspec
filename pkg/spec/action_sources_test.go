package spec

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// ActionSources = declared `actions:` ∪ transition `via` (plan
// docs_internal/plan/via-sebagai-action-penuh.md, L2/L3).
//
// It exists so `via` can be the ONE place a transition is declared. Before it,
// an action carrying `impl` had to be duplicated under `actions:` for the
// transition to get a route — the duplication this plan removes.

func sourcesEntity() *EntitySpec {
	return &EntitySpec{
		Plural: "dining-tables",
		Actions: []Action{
			{Name: "release", Description: "declared release"},
			{Name: "delete", Disabled: true},
		},
		StateMachine: &StateMachine{
			Field:   "table_status",
			Initial: "available",
			Transitions: []TransitionDecl{
				// Declared already → the declared entry must win.
				{From: StateList{"occupied"}, To: "available", Action: "release"},
				// Transition-only, with a route.
				{
					From: StateList{"available"}, To: "occupied", Action: "occupy",
					Description: "Tamu membayar",
					Impl:        &ImplDecl{Type: "script_ref", Ref: "cafe/occupy"},
					Audit:       true,
					UI:          &ActionUIHint{ButtonLabel: "Dudukkan tamu"},
				},
				// Transition without `via` → declares no action.
				{From: StateList{"available"}, To: "reserved"},
			},
		},
	}
}

func names(actions []Action) map[string]Action {
	m := map[string]Action{}
	for _, a := range actions {
		m[a.Name] = a
	}
	return m
}

func TestActionSources(t *testing.T) {
	got := names(sourcesEntity().ActionSources())

	t.Run("declared actions are kept", func(t *testing.T) {
		if _, ok := got["release"]; !ok {
			t.Fatal("declared action must be present")
		}
		if _, ok := got["delete"]; !ok {
			t.Fatal("a disabled declared action must still be present")
		}
	})

	t.Run("declared action wins over the same-named transition", func(t *testing.T) {
		if got["release"].Description != "declared release" {
			t.Fatalf("declared entry must win: got description %q", got["release"].Description)
		}
		if got["release"].Impl != nil {
			t.Fatal("the declared entry has no impl, so the result must not either")
		}
	})

	t.Run("transition-only via becomes an action", func(t *testing.T) {
		occupy, ok := got["occupy"]
		if !ok {
			t.Fatal("a transition-only `via` must become an action (L3)")
		}
		if occupy.Description != "Tamu membayar" {
			t.Errorf("description = %q", occupy.Description)
		}
		if occupy.Impl == nil || occupy.Impl.Ref != "cafe/occupy" {
			t.Errorf("impl must carry through — a route depends on it: %+v", occupy.Impl)
		}
		if !occupy.Audit {
			t.Error("audit must carry through")
		}
		if occupy.UI == nil || occupy.UI.ButtonLabel != "Dudukkan tamu" {
			t.Errorf("ui must carry through: %+v", occupy.UI)
		}
	})

	// The gate is enforced on the PATCH path. Copying it here would read as "the
	// same permission declared twice", which ValidateTransitionPermissions
	// rejects as two sources free to drift.
	t.Run("transition gate is not copied onto the action", func(t *testing.T) {
		es := sourcesEntity()
		es.StateMachine.Transitions[1].RequirePermission = "dining-tables.occupy"
		got := names(es.ActionSources())
		if got["occupy"].RequiredPermission != "" {
			t.Fatalf("gate must stay on the transition, got %q",
				got["occupy"].RequiredPermission)
		}
	})

	t.Run("transition without via declares no action", func(t *testing.T) {
		// Nothing may be synthesised for it — least of all an action called "".
		for name := range got {
			if name == "" {
				t.Fatal("an empty action name must never be synthesised")
			}
		}
	})

	t.Run("idempotency carries only when set", func(t *testing.T) {
		if got["occupy"].Idempotent {
			t.Error("occupy did not declare `idempotent`, so it must not be set")
		}
		es := sourcesEntity()
		es.StateMachine.Transitions[1].Idempotent = true
		es.StateMachine.Transitions[1].IdempotencyKey = &IdempotencyDecl{From: "server"}
		got := names(es.ActionSources())
		if !got["occupy"].Idempotent || got["occupy"].IdempotencyKey == nil {
			t.Error("idempotent + idempotency_key must carry through")
		}
	})
}

// The union must be a pure ADDITION for manifests that keep both spellings —
// which is every manifest in the repo today. Regressing this would change
// behaviour for all of them at once.
func TestActionSources_DeclaredOnlyManifestIsUnchanged(t *testing.T) {
	es := &EntitySpec{
		Plural: "orders",
		Actions: []Action{
			{Name: "checkout", RequiredPermission: "orders.checkout"},
			{Name: "delete", Disabled: true},
		},
		StateMachine: &StateMachine{
			Field:   "status",
			Initial: "draft",
			Transitions: []TransitionDecl{
				{From: StateList{"draft"}, To: "paid", Action: "checkout"},
			},
		},
	}
	got := es.ActionSources()
	if len(got) != len(es.Actions) {
		t.Fatalf("want the declared actions unchanged (%d), got %d — the union "+
			"must not duplicate a name that is already declared", len(es.Actions), len(got))
	}
	if names(got)["checkout"].RequiredPermission != "orders.checkout" {
		t.Error("the declared action's own permission must survive untouched")
	}
}

// L1 completeness (plan docs_internal/plan/via-sebagai-action-penuh.md):
// `via` must be able to carry EVERY field an `actions:` entry can, otherwise
// deleting the duplicated entry during the L4 migration would silently drop
// configuration.
//
// Measured need (2026-09-27): of 83 duplicated declarations, 76 carried
// `required_permission` (moves to `require_permission`), 11 carried `uses`, and
// 2 carried `params`. The first is a gate and belongs on the transition; the
// other two are configuration the ACTION needs and must survive the migration.
func TestActionSources_TransitionCarriesUsesAndParams(t *testing.T) {
	src := `
state_machine:
  field: status
  initial: draft
  states: [{name: draft}, {name: posted}]
  transitions:
    - from: draft
      to: posted
      via: post
      description: "Posting jurnal"
      impl: {type: script_ref, ref: gl/journal_post}
      uses:
        resources: [journal-entry]
        primitives: [db]
        db: {write: [gl.journal_entries]}
      params:
        type: object
        required: [reason]
      expose: [rest]
      rate_limit: {requests: 5, window: "1m"}
`
	var holder struct {
		SM StateMachine `yaml:"state_machine"`
	}
	if err := yaml.Unmarshal([]byte(src), &holder); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	es := &EntitySpec{Plural: "journal-entries", StateMachine: &holder.SM}

	var got *Action
	for i, a := range es.ActionSources() {
		if a.Name == "post" {
			got = &es.ActionSources()[i]
			break
		}
	}
	if got == nil {
		t.Fatal("transition with `via: post` produced no action — the union is broken")
	}

	if got.Uses == nil {
		t.Fatal("`uses` dropped: a transition's consent footprint must survive " +
			"the L4 migration, or deleting the duplicated action entry would " +
			"silently widen what the action may touch")
	}
	if len(got.Uses.Resources) != 1 || got.Uses.Resources[0] != "journal-entry" {
		t.Errorf("uses.resources dropped: %+v", got.Uses.Resources)
	}
	if got.Uses.Db == nil || len(got.Uses.Db.Write) != 1 {
		t.Errorf("uses.db.write dropped: %+v", got.Uses.Db)
	}
	if got.Params == nil {
		t.Error("`params` dropped: the input contract must survive the migration")
	}
	if len(got.Expose) != 1 || got.Expose[0] != "rest" {
		t.Errorf("expose dropped: %+v", got.Expose)
	}
	if got.RateLimit == nil {
		t.Error("rate_limit dropped: a per-action limit must survive the migration")
	}
	if got.Impl == nil || got.Impl.Ref != "gl/journal_post" {
		t.Errorf("impl dropped: %+v", got.Impl)
	}
}
