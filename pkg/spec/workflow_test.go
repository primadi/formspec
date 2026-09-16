package spec

import (
	"strings"
	"testing"
)

// TestValidateWorkflowSpec_TransitionForms pins the S9 trigger contract: exactly
// one form, never both, never neither. Both would make the runtime silently pick
// one; neither would leave a workflow that intercepts nothing — the
// "looks configured, enforces nothing" failure the kafe ledger keeps finding.
func TestValidateWorkflowSpec_TransitionForms(t *testing.T) {
	steps := []WorkflowStep{{Roles: []string{"cafe-order.supervisor"}}}

	cases := []struct {
		name    string
		wf      *WorkflowSpec
		wantErr string
	}{
		{
			name: "name form",
			wf: &WorkflowSpec{
				Entity: "cafe-order.order",
				On:     &WorkflowTrigger{Transition: &WorkflowTransitionRef{Name: "void-order"}},
				Steps:  steps,
			},
		},
		{
			name: "state pair form",
			wf: &WorkflowSpec{
				Entity: "gl.journal-entry",
				On:     &WorkflowTrigger{Transition: &WorkflowTransitionRef{From: "draft", To: "posted"}},
				Steps:  steps,
			},
		},
		{
			name: "both forms",
			wf: &WorkflowSpec{
				Entity: "cafe-order.order",
				On:     &WorkflowTrigger{Transition: &WorkflowTransitionRef{Name: "void-order", From: "paid", To: "cancelled"}},
				Steps:  steps,
			},
			wantErr: "both `name` and `from`/`to`",
		},
		{
			name: "neither form",
			wf: &WorkflowSpec{
				Entity: "cafe-order.order",
				On:     &WorkflowTrigger{Transition: &WorkflowTransitionRef{}},
				Steps:  steps,
			},
			wantErr: "must name the transition",
		},
		{
			name: "half a state pair",
			wf: &WorkflowSpec{
				Entity: "gl.journal-entry",
				On:     &WorkflowTrigger{Transition: &WorkflowTransitionRef{From: "draft"}},
				Steps:  steps,
			},
			wantErr: "needs both `from` and `to`",
		},
		{
			name: "no steps",
			wf: &WorkflowSpec{
				Entity: "cafe-order.order",
				On:     &WorkflowTrigger{Transition: &WorkflowTransitionRef{Name: "void-order"}},
			},
			wantErr: "no `steps`",
		},
		{
			name:    "missing entity",
			wf:      &WorkflowSpec{On: &WorkflowTrigger{Transition: &WorkflowTransitionRef{Name: "x"}}, Steps: steps},
			wantErr: "requires `entity`",
		},
		{
			name:    "missing trigger",
			wf:      &WorkflowSpec{Entity: "cafe-order.order", Steps: steps},
			wantErr: "requires `on.transition`",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateWorkflowSpec(c.wf)
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("expected valid workflow, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q", c.wantErr)
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("error %q should contain %q", err, c.wantErr)
			}
		})
	}
}

// TestWorkflowTransitionRef_ByName documents the discriminator the registry uses
// to decide which index a workflow belongs in.
func TestWorkflowTransitionRef_ByName(t *testing.T) {
	if (&WorkflowTransitionRef{From: "draft", To: "posted"}).ByName() {
		t.Error("state-pair form must not report ByName")
	}
	if !(&WorkflowTransitionRef{Name: "void-order"}).ByName() {
		t.Error("name form must report ByName")
	}
	var nilRef *WorkflowTransitionRef
	if nilRef.ByName() {
		t.Error("nil ref must not report ByName")
	}
}
