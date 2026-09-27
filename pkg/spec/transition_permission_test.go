package spec

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// Per-transition gating (kafe 10.44). A transition reached through PATCH was
// authorized by `{module}.{plural}.update` alone — one permission for every
// transition of the entity — so "this transition is admin-only, that one is
// cashier-only" could be written in YAML and honored nowhere. TransitionDecl
// gained `require_permission`; FormSpecExpr cannot stand in because it excludes
// identity/permission by design (docs/spec/frontend/08-formspec-expr.md §3).

func transitionEntity(transitions []TransitionDecl, actions []Action) *EntitySpec {
	return &EntitySpec{
		Version:        "v1",
		Plural:         "dining-tables",
		Characteristic: CharMaster,
		Fields:         []Field{{Name: "table_status", Type: FieldEnum}},
		Actions:        actions,
		StateMachine: &StateMachine{
			Field:       "table_status",
			Initial:     "available",
			States:      []StateDecl{{Name: "available"}, {Name: "reserved"}, {Name: "not_available"}},
			Transitions: transitions,
		},
	}
}

func TestTransitionPermission(t *testing.T) {
	t.Run("declared gate is enforced", func(t *testing.T) {
		got := TransitionPermission(TransitionDecl{RequirePermission: "dining-tables.not-available"})
		if got != "dining-tables.not-available" {
			t.Errorf("want the declared gate, got %q", got)
		}
	})

	// The whole point of the additive rule: no manifest declares this field, so
	// absent must mean "behave exactly as before".
	t.Run("absent gate leaves the transition unchanged", func(t *testing.T) {
		if got := TransitionPermission(TransitionDecl{}); got != "" {
			t.Errorf("absent gate → want %q (previous behavior), got %q", "", got)
		}
	})

	// A gate is enforced on its own merits: it does NOT depend on the via
	// action's required_permission, because an action without an `impl` has no
	// route of its own — which is exactly why the gate lives on the transition.
	t.Run("gate is enforced even when the via action declares the same name", func(t *testing.T) {
		got := TransitionPermission(TransitionDecl{
			Action:            "release",
			RequirePermission: "dining-tables.release",
		})
		if got != "dining-tables.release" {
			t.Errorf("gate must be enforced regardless of the action's own value, got %q", got)
		}
	})
}

func TestValidateTransitionPermissions(t *testing.T) {
	t.Run("gate with no matching action is valid", func(t *testing.T) {
		// A transition gate is a permission, not an action reference: requiring
		// a matching action would force a meaningless duplicate declaration (a
		// `via: release` transition could not be gated on the standard
		// `{plural}.delete` without inventing an action named `delete`).
		d := transitionEntity(
			[]TransitionDecl{{
				From: StateList{"available"}, To: "not_available",
				Action: "mark-not-available", RequirePermission: "dining-tables.delete",
			}},
			nil)
		if err := ValidateTransitionPermissions(d); err != nil {
			t.Fatalf("a gate needs no matching action: %v", err)
		}
	})

	t.Run("gate matching the action's own convention name is valid", func(t *testing.T) {
		// The action declares nothing, so `dining-tables.reserve` is just the
		// conventional name — not a duplicate declaration.
		d := transitionEntity(
			[]TransitionDecl{{
				From: StateList{"available"}, To: "reserved",
				Action: "reserve", RequirePermission: "dining-tables.reserve",
			}},
			[]Action{{Name: "reserve"}})
		if err := ValidateTransitionPermissions(d); err != nil {
			t.Fatalf("gate using the action's conventional name must be valid: %v", err)
		}
	})

	t.Run("unqualified gate is valid", func(t *testing.T) {
		d := transitionEntity(
			[]TransitionDecl{{
				From: StateList{"available"}, To: "reserved",
				Action: "reserve", RequirePermission: "reserve",
			}},
			nil)
		if err := ValidateTransitionPermissions(d); err != nil {
			t.Fatalf("unqualified gate must be valid: %v", err)
		}
	})

	t.Run("standard CRUD gate is valid", func(t *testing.T) {
		d := transitionEntity(
			[]TransitionDecl{{
				From: StateList{"available"}, To: "reserved",
				Action: "reserve", RequirePermission: "dining-tables.update",
			}},
			nil)
		if err := ValidateTransitionPermissions(d); err != nil {
			t.Fatalf("gate matching a standard CRUD permission must be valid: %v", err)
		}
	})

	t.Run("no gate declared is valid", func(t *testing.T) {
		d := transitionEntity(
			[]TransitionDecl{{From: StateList{"available"}, To: "reserved", Action: "reserve"}},
			nil)
		if err := ValidateTransitionPermissions(d); err != nil {
			t.Fatalf("a transition without a gate must stay valid: %v", err)
		}
	})

	// The mistake this contract exists for: declaring the same permission on
	// both the action and the transition. Only the transition's copy is
	// enforced, so the two are free to drift — and the difference is
	// unobservable at runtime, which makes it worse than a plain duplicate.
	t.Run("repeating the action's own permission is rejected", func(t *testing.T) {
		d := transitionEntity(
			[]TransitionDecl{{
				From: StateList{"available"}, To: "not_available",
				Action: "mark-not-available", RequirePermission: "dining-tables.not-available",
			}},
			[]Action{{Name: "mark-not-available", RequiredPermission: "dining-tables.not-available"}})
		err := ValidateTransitionPermissions(d)
		if err == nil {
			t.Fatal("declaring the same gate twice must fail: only the transition's copy is enforced")
		}
	})

	t.Run("gate with an empty final segment is rejected", func(t *testing.T) {
		d := transitionEntity(
			[]TransitionDecl{{
				From: StateList{"available"}, To: "reserved",
				Action: "reserve", RequirePermission: "cafe-master.",
			}},
			nil)
		if err := ValidateTransitionPermissions(d); err == nil {
			t.Fatal("a gate whose final segment is empty can never be granted and must be rejected")
		}
	})
}

// The trap that bit twice: every field added to TransitionDecl must also be
// listed in its custom UnmarshalYAML's local struct, because every load goes
// through that method. Manifest validation reads the FILE, so a dropped field
// passes `formspec validate` and is then absent in the running engine — the
// exact failure that once silently discarded `emit` (the durable on_paid never
// reached the outbox).
func TestTransitionDecl_UnmarshalKeepsRequirePermission(t *testing.T) {
	var t2 struct {
		SM StateMachine `yaml:"state_machine"`
	}
	src := `
state_machine:
  field: table_status
  initial: available
  states: [{name: available}, {name: not_available}]
  transitions:
    - { from: available, to: not_available, via: mark-not-available, require_permission: dining-tables.not-available }
`
	if err := yaml.Unmarshal([]byte(src), &t2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(t2.SM.Transitions) != 1 {
		t.Fatalf("want 1 transition, got %d", len(t2.SM.Transitions))
	}
	got := TransitionPermission(t2.SM.Transitions[0])
	if got != "dining-tables.not-available" {
		t.Fatalf("require_permission was dropped at load time: want %q, got %q", "dining-tables.not-available", got)
	}
}

// The same load-time trap as require_permission, for the description field:
// every field added to TransitionDecl must also be listed in the custom
// UnmarshalYAML's local struct, or `formspec validate` accepts a label the
// running engine never sees (it reads the file; the engine reads the struct).
func TestTransitionDecl_UnmarshalKeepsDescription(t *testing.T) {
	var holder struct {
		SM StateMachine `yaml:"state_machine"`
	}
	src := `
state_machine:
  field: table_status
  initial: available
  states: [{name: available}, {name: occupied}]
  transitions:
    - { from: available, to: occupied, via: occupy, description: "Tamu membayar — meja terisi" }
`
	if err := yaml.Unmarshal([]byte(src), &holder); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(holder.SM.Transitions) != 1 {
		t.Fatalf("want 1 transition, got %d", len(holder.SM.Transitions))
	}
	got := holder.SM.Transitions[0].Description
	want := "Tamu membayar — meja terisi"
	if got != want {
		t.Fatalf("description was dropped at load time: want %q, got %q", want, got)
	}
}

// Closes the load-time trap as a CLASS, not per field.
//
// Every field added to TransitionDecl used to be silently dropped at load
// unless it was also listed in the custom UnmarshalYAML's local struct — which
// happened three times (`emit`, then `require_permission`, then `description`),
// each time invisible to `formspec validate` because validation reads the file
// while the engine reads the struct.
//
// The unmarshaler now embeds TransitionDecl with `,inline`, so new fields carry
// automatically. This test fails the moment that embedding is dropped, which is
// what turns "remember to add it in two places" into a mechanic.
func TestTransitionDecl_UnmarshalCarriesEveryField(t *testing.T) {
	// A value for every YAML field of TransitionDecl, keyed by its yaml tag.
	src := `
state_machine:
  field: table_status
  initial: available
  states: [{name: available}, {name: occupied}]
  transitions:
    - from: [available, reserved]
      to: occupied
      via: occupy
      description: "Tamu membayar"
      require_permission: dining-tables.occupy
      emit: on_paid
      guard:
        expression: "occupied_seats < seats"
        message: "meja penuh"
`
	var holder struct {
		SM StateMachine `yaml:"state_machine"`
	}
	if err := yaml.Unmarshal([]byte(src), &holder); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(holder.SM.Transitions) != 1 {
		t.Fatalf("want 1 transition, got %d", len(holder.SM.Transitions))
	}
	got := holder.SM.Transitions[0]

	if len(got.From) != 2 || got.From[0] != "available" || got.From[1] != "reserved" {
		t.Errorf("from dropped: %v", got.From)
	}
	if got.To != "occupied" {
		t.Errorf("to dropped: %q", got.To)
	}
	if got.Action != "occupy" {
		t.Errorf("via dropped: %q", got.Action)
	}
	if got.Description != "Tamu membayar" {
		t.Errorf("description dropped: %q", got.Description)
	}
	if got.RequirePermission != "dining-tables.occupy" {
		t.Errorf("require_permission dropped: %q", got.RequirePermission)
	}
	if got.Emit != "on_paid" {
		t.Errorf("emit dropped: %q", got.Emit)
	}
	if got.Guard == nil || got.Guard.Expression != "occupied_seats < seats" ||
		got.Guard.Message != "meja penuh" {
		t.Errorf("guard dropped: %+v", got.Guard)
	}
}

// The legacy `action:` alias must keep working alongside `via` — it is declared
// separately in the unmarshaler (not part of the embedded struct), so it is the
// one field that can still be dropped by a careless edit.
func TestTransitionDecl_UnmarshalKeepsLegacyActionAlias(t *testing.T) {
	var holder struct {
		SM StateMachine `yaml:"state_machine"`
	}
	src := `
state_machine:
  field: s
  initial: a
  states: [{name: a}, {name: b}]
  transitions:
    - { from: a, to: b, action: ship-it }
`
	if err := yaml.Unmarshal([]byte(src), &holder); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := holder.SM.Transitions[0].Action; got != "ship-it" {
		t.Fatalf("legacy `action:` alias dropped: got %q, want %q", got, "ship-it")
	}
}

// `via` wins when both spellings are present.
func TestTransitionDecl_UnmarshalPrefersViaOverAlias(t *testing.T) {
	var holder struct {
		SM StateMachine `yaml:"state_machine"`
	}
	src := `
state_machine:
  field: s
  initial: a
  states: [{name: a}, {name: b}]
  transitions:
    - { from: a, to: b, via: canonical, action: legacy }
`
	if err := yaml.Unmarshal([]byte(src), &holder); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := holder.SM.Transitions[0].Action; got != "canonical" {
		t.Fatalf("`via` must win over the legacy alias: got %q, want %q", got, "canonical")
	}
}
