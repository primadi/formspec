package approval

import (
	"context"
	"testing"
	"time"

	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// fakeApprovalStore is an in-memory ApprovalStore for escalation tests.
type fakeApprovalStore struct {
	rows []db.ApprovalRequestRow
}

func (f *fakeApprovalStore) ListPending(ctx context.Context, limit int) ([]db.ApprovalRequestRow, error) {
	return f.rows, nil
}

func (f *fakeApprovalStore) Update(ctx context.Context, row db.ApprovalRequestRow) error {
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
	regAdd(reg, "gl", "journal-entry", "post", []string{"draft"}, "posted",
		spec.ApprovalStep{
			Roles:     []string{"gl.supervisor"},
			Approvers: 1,
			Escalation: &spec.StepEscalation{
				After:    "1s",
				Reassign: "head-check",
			},
		})

	// An approval whose active step started 2s ago (past the 1s escalation).
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
			Approvals:      map[string][]string{},
			EscalatedSteps: map[string][]string{},
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

	// The step declares no `name`, so its key is the reserved index form `#0` —
	// the fallback that keeps unnamed (legacy) steps working. `StepKey` documents
	// why the name form and the index form cannot collide.
	got := store.rows[0].EscalatedSteps["#0"]
	want := "workflow.gl.journal-entry.post.head-check"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("step 0 should be escalated with reassign %s, got %v", want, store.rows[0].EscalatedSteps)
	}
	if len(auditCalls) != 1 || auditCalls[0] != "workflow.escalate" {
		t.Fatalf("expected workflow.escalate audit, got %v", auditCalls)
	}
}

func TestEscalationWorker_NotDueYet(t *testing.T) {
	reg := NewRegistry()
	regAdd(reg, "gl", "journal-entry", "post", []string{"draft"}, "posted",
		spec.ApprovalStep{
			Roles:     []string{"gl.supervisor"},
			Approvers: 1,
			Escalation: &spec.StepEscalation{
				After:    "1h",
				Reassign: "head-check",
			},
		})

	// Approval created just now — not yet due.
	now := time.Now().UTC().Format(time.RFC3339Nano)
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
			Approvals:      map[string][]string{},
			EscalatedSteps: map[string][]string{},
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
	regAdd(reg, "gl", "journal-entry", "post", []string{"draft"}, "posted",
		spec.ApprovalStep{Roles: []string{"gl.supervisor"}, Approvers: 1})

	old := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339Nano)
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
			Approvals:      map[string][]string{},
			EscalatedSteps: map[string][]string{},
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
			ID:             "1",
			TenantID:       "ws-1",
			Entity:         "gl.journal-entry",
			RecordID:       "rec-1",
			GateModule:     "gl",
			GateName:       gateName("journal-entry", "post"),
			Status:         "pending",
			ActiveStep:     0,
			Approvals:      map[string][]string{},
			EscalatedSteps: map[string][]string{"0": {"gl.head"}}, // legacy numeric key, already escalated
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
