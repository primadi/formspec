package spec

import (
	"strings"
	"testing"
)

// kafe 10.60 (plan docs_internal/plan/via-sebagai-action-penuh.md L9): a hook
// (`hooks.on: before|after|on_error`) may target a transition `via`, because a
// `via` IS an action since L3 and L4 REJECTS redeclaring it under `actions:` —
// so via-only is the intended shape.
//
// Regression: `ValidateEntitySpec` passed `d.Actions` to `ValidateHooks`, so a
// hook naming a via-only action was rejected with
// `hook action "…" does not match any declared action` — a false rejection of
// exactly the shape the newer contract requires. This is the same
// "one registry, two sources" class as L3/L5/5.24.3/L8.
//
// Run with: go test ./pkg/spec/ -run TestValidateEntitySpec_HookMayTargetTransitionVia
func TestValidateEntitySpec_HookMayTargetTransitionVia(t *testing.T) {
	entity := func(hookAction string) *EntitySpec {
		return &EntitySpec{
			Version: "v1",
			Plural:  "orders",
			Fields: []Field{
				{Name: "number", Type: FieldString},
				{Name: "status", Type: FieldString},
			},
			Hooks: []HookDecl{{
				On:     HookOnAfter,
				Action: hookAction,
				Impl:   &ImplDecl{Type: "script_ref", Ref: "billing/after"},
			}},
			StateMachine: &StateMachine{
				Field:   "status",
				Initial: "draft",
				States:  []StateDecl{{Name: "draft"}, {Name: "ready"}},
				Transitions: []TransitionDecl{
					// `mark-ready` exists ONLY here — no `actions:` entry, which
					// L4 would reject as a leftover.
					{From: StateList{"draft"}, To: "ready", Action: "mark-ready"},
				},
			},
		}
	}

	if err := ValidateEntitySpec(entity("mark-ready")); err != nil {
		t.Fatalf("a hook targeting a transition `via` must validate: %v", err)
	}

	// Calibration: a genuinely unknown hook action must STILL be reported —
	// otherwise the fix would be indistinguishable from "hook validation off".
	err := ValidateEntitySpec(entity("nope"))
	if err == nil || !strings.Contains(err.Error(), `hook action "nope" does not match any declared action`) {
		t.Fatalf("an unknown hook action must still be reported, got: %v", err)
	}
}
