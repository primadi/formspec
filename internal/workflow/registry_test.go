package workflow

import (
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

func TestRegistry_AddGetList(t *testing.T) {
	reg := NewRegistry()
	wf := &spec.WorkflowSpec{
		Entity: "gl.journal-entry",
		On:     &spec.WorkflowTrigger{Transition: &spec.WorkflowTransitionRef{From: "draft", To: "posted"}},
		Steps:  []spec.WorkflowStep{{Roles: []string{"gl.supervisor"}, Approvers: 1}},
	}
	reg.Add("gl", "journal-posting-approval", wf)

	got, ok := reg.Get("gl", "journal-posting-approval")
	if !ok {
		t.Fatal("expected workflow to be found")
	}
	if got.Entity != "gl.journal-entry" {
		t.Errorf("entity: want gl.journal-entry, got %q", got.Entity)
	}

	infos := reg.List()
	if len(infos) != 1 {
		t.Fatalf("List: want 1, got %d", len(infos))
	}
	if infos[0].Name != "journal-posting-approval" || infos[0].From != "draft" || infos[0].To != "posted" {
		t.Errorf("List[0]: want gl/journal-posting-approval draft->posted, got %s/%s %s->%s",
			infos[0].Module, infos[0].Name, infos[0].From, infos[0].To)
	}
}

func TestRegistry_ForTransition(t *testing.T) {
	reg := NewRegistry()
	reg.Add("gl", "wf1", &spec.WorkflowSpec{
		Entity: "gl.journal-entry",
		On:     &spec.WorkflowTrigger{Transition: &spec.WorkflowTransitionRef{From: "draft", To: "posted"}},
	})
	reg.Add("gl", "wf2", &spec.WorkflowSpec{
		Entity: "gl.journal-entry",
		On:     &spec.WorkflowTrigger{Transition: &spec.WorkflowTransitionRef{From: "draft", To: "posted"}},
	})
	reg.Add("gl", "wf3", &spec.WorkflowSpec{
		Entity: "gl.journal-entry",
		On:     &spec.WorkflowTrigger{Transition: &spec.WorkflowTransitionRef{From: "draft", To: "cancelled"}},
	})

	if len(reg.ForTransition("gl.journal-entry", "post", "draft", "posted")) != 2 {
		t.Fatal("ForTransition(draft->posted): want 2")
	}
	if len(reg.ForTransition("gl.journal-entry", "cancel", "draft", "cancelled")) != 1 {
		t.Fatal("ForTransition(draft->cancelled): want 1")
	}
	if len(reg.ForTransition("gl.journal-entry", "post", "posted", "draft")) != 0 {
		t.Fatal("ForTransition(posted->draft): want 0")
	}
}

func TestRegistry_ReAddReplacesIndex(t *testing.T) {
	reg := NewRegistry()
	reg.Add("gl", "wf", &spec.WorkflowSpec{
		Entity: "gl.journal-entry",
		On:     &spec.WorkflowTrigger{Transition: &spec.WorkflowTransitionRef{From: "draft", To: "posted"}},
	})
	// Re-register with a different transition — old index must be removed.
	reg.Add("gl", "wf", &spec.WorkflowSpec{
		Entity: "gl.journal-entry",
		On:     &spec.WorkflowTrigger{Transition: &spec.WorkflowTransitionRef{From: "draft", To: "cancelled"}},
	})

	if len(reg.ForTransition("gl.journal-entry", "post", "draft", "posted")) != 0 {
		t.Fatal("old transition index should be removed on re-registration")
	}
	if len(reg.ForTransition("gl.journal-entry", "cancel", "draft", "cancelled")) != 1 {
		t.Fatal("new transition index should be present after re-registration")
	}
}

// TestRegistry_ForTransitionByName covers S9: a workflow naming its transition
// must intercept it from EVERY origin state. A state pair can only ever describe
// one origin, which is how `void-order` (reachable from paid/in_kitchen/ready/
// served) previously left three of its four origins unguarded.
func TestRegistry_ForTransitionByName(t *testing.T) {
	reg := NewRegistry()
	reg.Add("cafe-order", "order-void-approval", &spec.WorkflowSpec{
		Entity: "cafe-order.order",
		On:     &spec.WorkflowTrigger{Transition: &spec.WorkflowTransitionRef{Name: "void-order"}},
	})

	for _, from := range []string{"paid", "in_kitchen", "ready", "served"} {
		if got := reg.ForTransition("cafe-order.order", "void-order", from, "cancelled"); len(got) != 1 {
			t.Errorf("void-order from %s: want 1 workflow, got %d", from, len(got))
		}
	}

	// A different transition on the same entity must not be intercepted, even
	// though it shares the target state.
	if got := reg.ForTransition("cafe-order.order", "cancel-order", "awaiting_payment", "cancelled"); len(got) != 0 {
		t.Errorf("cancel-order must not be intercepted by the void workflow, got %d", len(got))
	}

	// And the same transition name on another entity must not match either.
	if got := reg.ForTransition("cafe-stock.order", "void-order", "paid", "cancelled"); len(got) != 0 {
		t.Errorf("entity is part of the key, got %d", len(got))
	}
}

// TestRegistry_ListExposesTransitionName keeps `formspec describe`/admin output
// honest about which form a workflow uses.
func TestRegistry_ListExposesTransitionName(t *testing.T) {
	reg := NewRegistry()
	reg.Add("cafe-order", "by-name", &spec.WorkflowSpec{
		Entity: "cafe-order.order",
		On:     &spec.WorkflowTrigger{Transition: &spec.WorkflowTransitionRef{Name: "void-order"}},
	})
	reg.Add("gl", "by-pair", &spec.WorkflowSpec{
		Entity: "gl.journal-entry",
		On:     &spec.WorkflowTrigger{Transition: &spec.WorkflowTransitionRef{From: "draft", To: "posted"}},
	})

	infos := reg.List()
	if len(infos) != 2 {
		t.Fatalf("List: got %d, want 2", len(infos))
	}
	for _, info := range infos {
		switch info.Name {
		case "by-name":
			if info.Transition != "void-order" || info.From != "" || info.To != "" {
				t.Errorf("by-name info: %+v", info)
			}
		case "by-pair":
			if info.Transition != "" || info.From != "draft" || info.To != "posted" {
				t.Errorf("by-pair info: %+v", info)
			}
		}
	}
}

func TestEngine_RequiresApproval(t *testing.T) {
	reg := NewRegistry()
	reg.Add("gl", "wf", &spec.WorkflowSpec{
		Entity: "gl.journal-entry",
		On:     &spec.WorkflowTrigger{Transition: &spec.WorkflowTransitionRef{From: "draft", To: "posted"}},
	})
	e := NewEngine(reg)

	if !e.RequiresApproval("gl.journal-entry", "post", "draft", "posted") {
		t.Fatal("expected approval required for draft->posted")
	}
	if e.RequiresApproval("gl.journal-entry", "cancel", "draft", "cancelled") {
		t.Fatal("expected no approval for draft->cancelled")
	}
}

func TestEngine_ApplicableSteps_When(t *testing.T) {
	reg := NewRegistry()
	wf := &spec.WorkflowSpec{
		Entity: "gl.journal-entry",
		On:     &spec.WorkflowTrigger{Transition: &spec.WorkflowTransitionRef{From: "draft", To: "posted"}},
		Steps: []spec.WorkflowStep{
			{Roles: []string{"gl.supervisor"}, Approvers: 1},
			{Roles: []string{"gl.controller"}, Approvers: 1, When: "resource.amount > 100000000"},
		},
	}
	e := NewEngine(reg)

	// Small amount → only step 1 applies.
	steps, err := e.ApplicableSteps(wf, map[string]any{"amount": 1000})
	if err != nil {
		t.Fatalf("ApplicableSteps: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("small amount: want 1 step, got %d", len(steps))
	}

	// Large amount → both steps apply.
	steps, err = e.ApplicableSteps(wf, map[string]any{"amount": 200000000})
	if err != nil {
		t.Fatalf("ApplicableSteps: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("large amount: want 2 steps, got %d", len(steps))
	}
}

func TestEngine_CanApprove_RoleEligibility(t *testing.T) {
	reg := NewRegistry()
	wf := &spec.WorkflowSpec{
		Entity: "gl.journal-entry",
		On:     &spec.WorkflowTrigger{Transition: &spec.WorkflowTransitionRef{From: "draft", To: "posted"}},
		Steps:  []spec.WorkflowStep{{Roles: []string{"gl.supervisor"}, Approvers: 1}},
	}
	e := NewEngine(reg)

	// User with the required role can approve.
	if ok, reason := e.CanApprove(wf, 0, "approver-1", []string{"gl.supervisor"}, "", nil, nil); !ok {
		t.Fatalf("expected approval allowed, got: %s", reason)
	}
	// User without the role cannot.
	if ok, _ := e.CanApprove(wf, 0, "approver-1", []string{"gl.other"}, "", nil, nil); ok {
		t.Fatal("expected approval denied for wrong role")
	}
}

func TestEngine_CanApprove_RequesterExcluded(t *testing.T) {
	reg := NewRegistry()
	wf := &spec.WorkflowSpec{
		Entity: "gl.journal-entry",
		On:     &spec.WorkflowTrigger{Transition: &spec.WorkflowTransitionRef{From: "draft", To: "posted"}},
		Steps:  []spec.WorkflowStep{{Roles: []string{"gl.supervisor"}, Approvers: 1}},
	}
	e := NewEngine(reg)

	// The requester (created_by) holds the role but must NOT be able to
	// approve their own request (7.4.5).
	if ok, reason := e.CanApprove(wf, 0, "user-1", []string{"gl.supervisor"}, "user-1", nil, nil); ok {
		t.Fatalf("expected requester to be excluded, got allowed: %s", reason)
	}
	// A different user with the role CAN approve.
	if ok, reason := e.CanApprove(wf, 0, "approver-1", []string{"gl.supervisor"}, "user-1", nil, nil); !ok {
		t.Fatalf("expected non-requester to be allowed, got: %s", reason)
	}
}

func TestApproval_QuorumAndAdvance(t *testing.T) {
	reg := NewRegistry()
	wf := &spec.WorkflowSpec{
		Entity: "gl.journal-entry",
		On:     &spec.WorkflowTrigger{Transition: &spec.WorkflowTransitionRef{From: "draft", To: "posted"}},
		Steps: []spec.WorkflowStep{
			{Roles: []string{"gl.supervisor"}, Approvers: 1},
			{Roles: []string{"gl.controller"}, Approvers: 1},
		},
	}
	e := NewEngine(reg)
	steps, _ := e.ApplicableSteps(wf, nil)

	a := NewApproval(wf, "gl", "gl.journal-entry", "rec-1", "draft", "posted", "requester-1")

	// Step 0: one approval reaches quorum (approvers: 1).
	if err := a.Approve("approver-1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if !a.StepApproved(steps[0], len(steps[0].Roles)) {
		t.Fatal("step 0 should be approved after 1 approval")
	}
	a.Advance()

	// Step 1: approve.
	if err := a.Approve("approver-2"); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if !a.StepApproved(steps[1], len(steps[1].Roles)) {
		t.Fatal("step 1 should be approved after 1 approval")
	}
	a.Advance()

	if !a.AllStepsApproved(steps) {
		t.Fatal("all steps should be approved")
	}
}

func TestApproval_DuplicateApproveRejected(t *testing.T) {
	reg := NewRegistry()
	wf := &spec.WorkflowSpec{
		Entity: "gl.journal-entry",
		On:     &spec.WorkflowTrigger{Transition: &spec.WorkflowTransitionRef{From: "draft", To: "posted"}},
		Steps:  []spec.WorkflowStep{{Roles: []string{"gl.supervisor"}, Approvers: 1}},
	}
	e := NewEngine(reg)
	steps, _ := e.ApplicableSteps(wf, nil)

	a := NewApproval(wf, "gl", "gl.journal-entry", "rec-1", "draft", "posted", "requester-1")
	if err := a.Approve("approver-1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if err := a.Approve("approver-1"); err == nil {
		t.Fatal("expected duplicate approve to fail")
	}
	_ = steps
}

func TestApproval_Reject(t *testing.T) {
	reg := NewRegistry()
	wf := &spec.WorkflowSpec{
		Entity: "gl.journal-entry",
		On:     &spec.WorkflowTrigger{Transition: &spec.WorkflowTransitionRef{From: "draft", To: "posted"}},
		Steps:  []spec.WorkflowStep{{Roles: []string{"gl.supervisor"}, Approvers: 1}},
	}
	e := NewEngine(reg)
	steps, _ := e.ApplicableSteps(wf, nil)

	a := NewApproval(wf, "gl", "gl.journal-entry", "rec-1", "draft", "posted", "requester-1")
	a.Reject("approver-1")
	if a.Status != ApprovalRejected {
		t.Fatalf("status: want rejected, got %s", a.Status)
	}
	if a.RejectedBy != "approver-1" {
		t.Errorf("rejected_by: want approver-1, got %q", a.RejectedBy)
	}
	_ = steps
}

func TestApproval_RoundTrip(t *testing.T) {
	a := &Approval{
		ID:             "1",
		WorkflowModule: "gl",
		WorkflowName:   "wf",
		Entity:         "gl.journal-entry",
		RecordID:       "rec-1",
		From:           "draft",
		To:             "posted",
		RequesterID:    "requester-1",
		Status:         ApprovalPending,
		ActiveStep:     0,
		Approvals:      map[int][]string{0: {"approver-1"}},
		RejectStep:     -1,
	}
	row := a.ToRow()
	back := FromRow(row)
	if back.Entity != a.Entity || back.RecordID != a.RecordID {
		t.Errorf("round-trip mismatch: %#v vs %#v", back, a)
	}
	if len(back.Approvals[0]) != 1 || back.Approvals[0][0] != "approver-1" {
		t.Errorf("approvals round-trip mismatch: %#v", back.Approvals)
	}
}
