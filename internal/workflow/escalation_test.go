package workflow

import (
	"context"
	"testing"
	"time"

	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// fakeApprovalStore is an in-memory ApprovalStore for escalation tests.
type fakeApprovalStore struct {
	rows []db.WorkflowApprovalRow
}

func (f *fakeApprovalStore) ListPending(ctx context.Context, limit int) ([]db.WorkflowApprovalRow, error) {
	return f.rows, nil
}

func (f *fakeApprovalStore) Update(ctx context.Context, row db.WorkflowApprovalRow) error {
	for i := range f.rows {
		if f.rows[i].ID == row.ID {
			f.rows[i] = row
			return nil
		}
	}
	return nil
}

func TestEscalationWorker_EscalatesAfterTimeout(t *testing.T) {
	reg := NewRegistry()
	reg.Add("gl", "wf", &spec.WorkflowSpec{
		Entity: "gl.journal-entry",
		On:     &spec.WorkflowTrigger{Transition: &spec.WorkflowTransitionRef{From: "draft", To: "posted"}},
		Steps: []spec.WorkflowStep{
			{
				Roles:     []string{"gl.supervisor"},
				Approvers: 1,
				Escalation: &spec.StepEscalation{
					After:         "1s",
					ReassignRoles: []string{"gl.head"},
				},
			},
		},
	})

	// An approval whose active step started 2s ago (past the 1s escalation).
	old := time.Now().UTC().Add(-2 * time.Second).Format(time.RFC3339Nano)
	store := &fakeApprovalStore{rows: []db.WorkflowApprovalRow{
		{
			ID:             "1",
			TenantID:       "ws-1",
			Entity:         "gl.journal-entry",
			RecordID:       "rec-1",
			WorkflowModule: "gl",
			WorkflowName:   "wf",
			Status:         "pending",
			ActiveStep:     0,
			Approvals:      map[int][]string{},
			EscalatedSteps: map[int][]string{},
			UpdatedAt:      old,
		},
	}}

	var auditCalls []string
	audit := func(ctx context.Context, workspaceID, entity, entityID, action, actor, changes, requestID string) error {
		auditCalls = append(auditCalls, action)
		return nil
	}

	w := NewEscalationWorker(store, reg, audit, WithEscalationInterval(10*time.Millisecond))
	w.now = func() time.Time { return time.Now().UTC() }

	// Run one check directly (not the background loop).
	w.checkEscalation(context.Background(), store.rows[0])

	if len(store.rows[0].EscalatedSteps[0]) != 1 || store.rows[0].EscalatedSteps[0][0] != "gl.head" {
		t.Fatalf("step 0 should be escalated with reassign gl.head, got %v", store.rows[0].EscalatedSteps[0])
	}
	if len(auditCalls) != 1 || auditCalls[0] != "workflow.escalate" {
		t.Fatalf("expected workflow.escalate audit, got %v", auditCalls)
	}
}

func TestEscalationWorker_NotDueYet(t *testing.T) {
	reg := NewRegistry()
	reg.Add("gl", "wf", &spec.WorkflowSpec{
		Entity: "gl.journal-entry",
		On:     &spec.WorkflowTrigger{Transition: &spec.WorkflowTransitionRef{From: "draft", To: "posted"}},
		Steps: []spec.WorkflowStep{
			{
				Roles:     []string{"gl.supervisor"},
				Approvers: 1,
				Escalation: &spec.StepEscalation{
					After:         "1h",
					ReassignRoles: []string{"gl.head"},
				},
			},
		},
	})

	// Approval created just now — not yet due.
	now := time.Now().UTC().Format(time.RFC3339Nano)
	store := &fakeApprovalStore{rows: []db.WorkflowApprovalRow{
		{
			ID:             "1",
			TenantID:       "ws-1",
			Entity:         "gl.journal-entry",
			RecordID:       "rec-1",
			WorkflowModule: "gl",
			WorkflowName:   "wf",
			Status:         "pending",
			ActiveStep:     0,
			Approvals:      map[int][]string{},
			EscalatedSteps: map[int][]string{},
			UpdatedAt:      now,
		},
	}}

	var auditCalls []string
	audit := func(ctx context.Context, workspaceID, entity, entityID, action, actor, changes, requestID string) error {
		auditCalls = append(auditCalls, action)
		return nil
	}

	w := NewEscalationWorker(store, reg, audit)
	w.now = func() time.Time { return time.Now().UTC() }
	w.checkEscalation(context.Background(), store.rows[0])

	if len(store.rows[0].EscalatedSteps) != 0 {
		t.Fatalf("step should NOT be escalated yet, got %v", store.rows[0].EscalatedSteps)
	}
	if len(auditCalls) != 0 {
		t.Fatalf("no audit expected, got %v", auditCalls)
	}
}

func TestEscalationWorker_NoEscalationDeclared(t *testing.T) {
	reg := NewRegistry()
	reg.Add("gl", "wf", &spec.WorkflowSpec{
		Entity: "gl.journal-entry",
		On:     &spec.WorkflowTrigger{Transition: &spec.WorkflowTransitionRef{From: "draft", To: "posted"}},
		Steps:  []spec.WorkflowStep{{Roles: []string{"gl.supervisor"}, Approvers: 1}},
	})

	old := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339Nano)
	store := &fakeApprovalStore{rows: []db.WorkflowApprovalRow{
		{
			ID:             "1",
			TenantID:       "ws-1",
			Entity:         "gl.journal-entry",
			RecordID:       "rec-1",
			WorkflowModule: "gl",
			WorkflowName:   "wf",
			Status:         "pending",
			ActiveStep:     0,
			Approvals:      map[int][]string{},
			EscalatedSteps: map[int][]string{},
			UpdatedAt:      old,
		},
	}}

	var auditCalls []string
	audit := func(ctx context.Context, workspaceID, entity, entityID, action, actor, changes, requestID string) error {
		auditCalls = append(auditCalls, action)
		return nil
	}

	w := NewEscalationWorker(store, reg, audit)
	w.now = func() time.Time { return time.Now().UTC() }
	w.checkEscalation(context.Background(), store.rows[0])

	if len(store.rows[0].EscalatedSteps) != 0 {
		t.Fatalf("step should NOT be escalated without escalation declaration, got %v", store.rows[0].EscalatedSteps)
	}
	if len(auditCalls) != 0 {
		t.Fatalf("no audit expected, got %v", auditCalls)
	}
}

func TestEscalationWorker_AlreadyEscalated(t *testing.T) {
	reg := NewRegistry()
	reg.Add("gl", "wf", &spec.WorkflowSpec{
		Entity: "gl.journal-entry",
		On:     &spec.WorkflowTrigger{Transition: &spec.WorkflowTransitionRef{From: "draft", To: "posted"}},
		Steps: []spec.WorkflowStep{
			{
				Roles:     []string{"gl.supervisor"},
				Approvers: 1,
				Escalation: &spec.StepEscalation{
					After:         "1s",
					ReassignRoles: []string{"gl.head"},
				},
			},
		},
	})

	old := time.Now().UTC().Add(-2 * time.Second).Format(time.RFC3339Nano)
	store := &fakeApprovalStore{rows: []db.WorkflowApprovalRow{
		{
			ID:             "1",
			TenantID:       "ws-1",
			Entity:         "gl.journal-entry",
			RecordID:       "rec-1",
			WorkflowModule: "gl",
			WorkflowName:   "wf",
			Status:         "pending",
			ActiveStep:     0,
			Approvals:      map[int][]string{},
			EscalatedSteps: map[int][]string{0: {"gl.head"}}, // already escalated
			UpdatedAt:      old,
		},
	}}

	var auditCalls []string
	audit := func(ctx context.Context, workspaceID, entity, entityID, action, actor, changes, requestID string) error {
		auditCalls = append(auditCalls, action)
		return nil
	}

	w := NewEscalationWorker(store, reg, audit)
	w.now = func() time.Time { return time.Now().UTC() }
	w.checkEscalation(context.Background(), store.rows[0])

	if len(auditCalls) != 0 {
		t.Fatalf("no re-escalation audit expected, got %v", auditCalls)
	}
}

func TestEscalationWorker_StartStop(t *testing.T) {
	reg := NewRegistry()
	store := &fakeApprovalStore{}
	w := NewEscalationWorker(store, reg, nil, WithEscalationInterval(10*time.Millisecond))

	if w.IsRunning() {
		t.Fatal("worker should not be running before Start")
	}
	w.Start(context.Background())
	if !w.IsRunning() {
		t.Fatal("worker should be running after Start")
	}
	w.Stop()
	if w.IsRunning() {
		t.Fatal("worker should not be running after Stop")
	}
}