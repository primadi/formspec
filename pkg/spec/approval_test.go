package spec

import (
	"strings"
	"testing"
)

// TestValidateApprovalSpec_RequiresSteps pins that an approval gate with no step
// can never reach quorum — refused rather than left as a gate that silently
// never opens.
func TestValidateApprovalSpec_RequiresSteps(t *testing.T) {
	if err := ValidateApprovalSpec(nil); err != nil {
		t.Fatalf("no approval is valid (the transition simply does not gate): %v", err)
	}
	err := ValidateApprovalSpec(&ApprovalSpec{})
	if err == nil {
		t.Fatal("an approval with no steps must be refused")
	}
	if !strings.Contains(err.Error(), "no `steps`") {
		t.Errorf("the refusal should name the missing `steps`, got %q", err)
	}
	ok := &ApprovalSpec{Steps: []ApprovalStep{{Name: "check", Permission: "check"}}}
	if err := ValidateApprovalSpec(ok); err != nil {
		t.Fatalf("a one-step approval must be accepted: %v", err)
	}
}

// TestValidateEntitySpec_ApprovalIsValidatedInPlace pins that the approval
// declared ON a transition is validated by ValidateEntitySpec — where the gate
// now lives — not only by the (removed) separate-manifest path.
func TestValidateEntitySpec_ApprovalIsValidatedInPlace(t *testing.T) {
	es := &EntitySpec{
		Version:        "v1",
		Characteristic: CharTransaction,
		Fields:         []Field{{Name: "status", Type: FieldString}},
		StateMachine: &StateMachine{
			Field:   "status",
			Initial: "draft",
			States:  []StateDecl{{Name: "draft"}, {Name: "done"}},
			Transitions: []TransitionDecl{{
				From:     StateList{"draft"},
				To:       "done",
				Action:   "finish",
				Approval: &ApprovalSpec{},
			}},
		},
	}
	err := ValidateEntitySpec(es)
	if err == nil || !strings.Contains(err.Error(), "no `steps`") {
		t.Fatalf("a transition's empty approval must be refused at the entity level, got %v", err)
	}
}
