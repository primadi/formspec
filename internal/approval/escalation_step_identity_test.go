package approval

import (
	"context"
	"testing"
	"time"

	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// Two-step workflow where the FIRST step is skipped by its `when` condition.
//
// This is the shape that exposed the escalation defect: `ApplicableSteps` drops
// the skipped step, so the approving request indexes into a list that is one
// shorter than the authored one — while the escalation worker, which cannot see
// the record and therefore cannot evaluate `when`, indexed into the AUTHORED
// list. With one step skipped the two lists disagree about every index, so the
// worker read a different step than the one awaiting approval and would have
// escalated that step's roles.
func skippedFirstStepWorkflow() *spec.ApprovalSpec {
	return &spec.ApprovalSpec{
		Steps: []spec.ApprovalStep{
			{
				Name: "auto-review",
				// Never applies: the record has no such field, and the guard is
				// false for every record.
				When:      "resource.amount > 999999999",
				Roles:     []string{"gl.supervisor"},
				Approvers: 1,
				// A DIFFERENT escalation than the step that actually runs: if the
				// worker reads the wrong step, this is the role it escalates.
				Escalation: &spec.StepEscalation{
					After:    "1s",
					Reassign: "wrong-head-check",
				},
			},
			{
				Name:      "finance-review",
				Roles:     []string{"gl.finance"},
				Approvers: 1,
				Escalation: &spec.StepEscalation{
					After:    "1s",
					Reassign: "finance-head-check",
				},
			},
		},
	}
}

// TestEscalationWorker_ResolvesStepByNameNotIndex is the regression for the
// defect described on skippedFirstStepWorkflow.
//
// The row below is what the approver's request actually writes: `active_step: 0`
// into the APPLICABLE list (one step, `finance-review`, since the first was
// skipped) plus the step NAME. Before this change the worker ignored the name
// and read `wf.Steps[0]` — the authored list's `auto-review` — escalating
// `gl.wrong-head` instead of `gl.finance-head`.
func TestEscalationWorker_ResolvesStepByNameNotIndex(t *testing.T) {
	reg := NewRegistry()
	regAdd(reg, "gl", "journal-entry", "post", []string{"draft"}, "posted", skippedFirstStepWorkflow().Steps...)

	old := time.Now().UTC().Add(-2 * time.Second).Format(time.RFC3339Nano)
	store := &fakeApprovalStore{rows: []db.ApprovalRequestRow{
		{
			ID:             "1",
			TenantID:       "ws-1",
			Entity:         "gl.journal-entry",
			RecordID:       "rec-1",
			GateModule:     "gl",
			GateName:       gateName("journal-entry", "post"),
			Status:         "pending",
			ActiveStep:     0,
			ActiveStepName: "finance-review", // the step that actually awaits approval
			Approvals:      map[string][]string{},
			EscalatedSteps: map[string][]string{},
			UpdatedAt:      old,
		},
	}}

	w := NewEscalationWorker(store, reg, nil)
	w.checkEscalation(context.Background(), store.rows[0])

	got := store.rows[0].EscalatedSteps["finance-review"]
	want := "workflow.gl.journal-entry.post.finance-head-check"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("escalated duty = %v, want [%s] — the worker must follow the step NAME, not the authored index", got, want)
	}
}

// TestEscalationWorker_FallsBackToIndexForUnnamedSteps keeps rows written before
// the column existed working: an empty name means "use the index", which is what
// every row did before.
func TestEscalationWorker_FallsBackToIndexForUnnamedSteps(t *testing.T) {
	reg := NewRegistry()
	regAdd(reg, "gl", "journal-entry", "post", []string{"draft"}, "posted",
		spec.ApprovalStep{
			Roles:     []string{"gl.supervisor"},
			Approvers: 1,
			Escalation: &spec.StepEscalation{
				After:    "1s",
				Reassign: "head-check",
			},
		})

	old := time.Now().UTC().Add(-2 * time.Second).Format(time.RFC3339Nano)
	store := &fakeApprovalStore{rows: []db.ApprovalRequestRow{
		{
			ID: "1", TenantID: "ws-1", Entity: "gl.journal-entry", RecordID: "rec-1",
			GateModule: "gl", GateName: gateName("journal-entry", "post"), Status: "pending",
			ActiveStep: 0, ActiveStepName: "", // legacy row
			Approvals: map[string][]string{}, EscalatedSteps: map[string][]string{},
			UpdatedAt: old,
		},
	}}

	w := NewEscalationWorker(store, reg, nil)
	w.checkEscalation(context.Background(), store.rows[0])

	got := store.rows[0].EscalatedSteps["#0"]
	want := "workflow.gl.journal-entry.post.head-check"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("legacy row must still escalate via the index, got %v want [%s]", got, want)
	}
}

// TestNameForStep covers the lookup itself, including the miss that makes the
// index fallback reachable.
func TestNameForStep(t *testing.T) {
	steps := []spec.ApprovalStep{{Name: "a"}, {Name: ""}, {Name: "c"}}

	if i, ok := NameForStep(steps, "c"); !ok || i != 2 {
		t.Errorf("NameForStep(c) = %d,%v want 2,true", i, ok)
	}
	if _, ok := NameForStep(steps, "missing"); ok {
		t.Error("an unknown name must not resolve")
	}
	if _, ok := NameForStep(steps, ""); ok {
		t.Error("an empty name must not resolve — it means \"no name\", not \"the first step\"")
	}
}

// TestNewApproval_RecordsFirstStepName pins that a fresh approval knows its step
// name from the start, so the worker never has to fall back for a named step.
func TestNewApproval_RecordsFirstStepName(t *testing.T) {
	wf := &spec.ApprovalSpec{
		Steps: []spec.ApprovalStep{{Name: "first"}, {Name: "second"}},
	}
	a := NewApproval(wf, "gl", "journal-entry.post", "gl.order", "rec-1", "a", "b", "req-1")
	if a.ActiveStepName != "first" {
		t.Fatalf("ActiveStepName = %q, want first", a.ActiveStepName)
	}

	a.Advance(wf.Steps)
	if a.ActiveStepName != "second" {
		t.Fatalf("after Advance: ActiveStepName = %q, want second", a.ActiveStepName)
	}
}
