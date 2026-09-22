package spec

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// TestTransitionDecl_UnmarshalKeepsEmit pins the S13 link end-to-end through
// the loader path: the custom unmarshaler used to drop `emit`, so a manifest
// could declare it, validate accept it, and the runtime still published
// nothing (found via the scenario-8 journal chain, 2026-09-21).
func TestTransitionDecl_UnmarshalKeepsEmit(t *testing.T) {
	var d EntitySpec
	src := `
state_machine:
  field: status
  initial: draft
  states:
    - { name: awaiting_payment }
    - { name: paid }
  transitions:
    - { from: awaiting_payment, to: paid, via: confirm-payment, emit: on_paid }
`
	if err := yaml.Unmarshal([]byte(src), &d); err != nil {
		t.Fatal(err)
	}
	t0 := d.StateMachine.Transitions[0]
	if t0.Emit != "on_paid" {
		t.Fatalf("Emit = %q, want %q — the custom unmarshaler dropped it", t0.Emit, "on_paid")
	}
	if t0.Action != "confirm-payment" || t0.To != "paid" {
		t.Fatalf("via/to lost: %+v", t0)
	}
}
