package approval

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// EscalationWorker is a background goroutine that periodically checks pending
// approval requests and escalates steps whose `escalation.after` duration has
// elapsed (02-core-extended.md §2 — Timeout & eskalasi). On escalation it:
//
//   - records an audit entry (workflow.escalate) via the audit writer, and
//   - marks the step as escalated with its reassignment DUTY, so a caller
//     holding that permission gains approval rights (7.4.4).
//
// A step is escalated at most once (tracked in Approval.EscalatedSteps).
type EscalationWorker struct {
	store    ApprovalStore
	registry *Registry
	audit    AuditWriter
	interval time.Duration
	now      func() time.Time

	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	mu      sync.RWMutex
	running bool
}

// ApprovalStore is the subset of the approval store the escalation worker
// needs — kept as an interface so the worker is testable without a DB.
type ApprovalStore interface {
	ListPending(ctx context.Context, limit int) ([]db.ApprovalRequestRow, error)
	Update(ctx context.Context, row db.ApprovalRequestRow) error
}

// AuditWriter records an audit entry (workflow.escalate).
type AuditWriter func(ctx context.Context, workspaceID, entity, entityID, action, actor, changes, requestID string) error

// EscalationWorkerOption configures the worker.
type EscalationWorkerOption func(*EscalationWorker)

// WithEscalationInterval sets the poll interval (default 1s).
func WithEscalationInterval(d time.Duration) EscalationWorkerOption {
	return func(w *EscalationWorker) { w.interval = d }
}

// NewEscalationWorker creates an escalation worker.
func NewEscalationWorker(store ApprovalStore, registry *Registry, audit AuditWriter, opts ...EscalationWorkerOption) *EscalationWorker {
	w := &EscalationWorker{
		store:    store,
		registry: registry,
		audit:    audit,
		interval: 1 * time.Second,
		now:      time.Now,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// Start begins the background escalation loop.
func (w *EscalationWorker) Start(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.running {
		return
	}
	w.ctx, w.cancel = context.WithCancel(ctx)
	w.running = true
	w.wg.Add(1)
	go w.runLoop()
	log.Printf("[workflow-escalation] started (poll=%v)", w.interval)
}

// Stop signals the worker to shut down and waits for completion.
func (w *EscalationWorker) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.running = false
	w.mu.Unlock()
	if w.cancel != nil {
		w.cancel()
	}
	w.wg.Wait()
	log.Printf("[workflow-escalation] stopped")
}

// IsRunning reports whether the worker is running.
func (w *EscalationWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// runLoop polls pending approvals on the configured interval.
func (w *EscalationWorker) runLoop() {
	defer w.wg.Done()
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.processPending(w.ctx)
		}
	}
}

// processPending checks all pending approvals for escalation.
func (w *EscalationWorker) processPending(ctx context.Context) {
	if w.store == nil || w.registry == nil {
		return
	}
	rows, err := w.store.ListPending(ctx, 100)
	if err != nil {
		log.Printf("[workflow-escalation] list pending: %v", err)
		return
	}
	for _, row := range rows {
		select {
		case <-ctx.Done():
			return
		default:
		}
		w.checkEscalation(ctx, row)
	}
}

// activeStep resolves which step a pending approval is waiting on, returning its
// position in `wf.Steps` as well.
//
// It prefers the stored step NAME, because the recorded index is only meaningful
// against the SAME step list — and where that list comes from depends on data
// the worker cannot see. `when` is evaluated against the record, so
// `ApplicableSteps` (what the approving request indexes into) may be a shorter
// list than the authored `wf.Steps`; reading `wf.Steps[ActiveStep]` here would
// then describe a DIFFERENT step than the one awaiting approval, escalating the
// wrong roles. That was the behaviour before this column existed, and it was
// silent: an escalation for step 1 would name step 2's reassign_roles.
//
// The index remains the fallback for rows written before the column existed and
// for steps that declare no name.
func (w *EscalationWorker) activeStep(wf *spec.ApprovalSpec, row db.ApprovalRequestRow) (int, spec.ApprovalStep, bool) {
	if idx, found := NameForStep(wf.Steps, row.ActiveStepName); found {
		return idx, wf.Steps[idx], true
	}
	if row.ActiveStep < 0 || row.ActiveStep >= len(wf.Steps) {
		return 0, spec.ApprovalStep{}, false
	}
	return row.ActiveStep, wf.Steps[row.ActiveStep], true
}

// isEscalated reports whether the step at idx has already been escalated,
// accepting the legacy numeric key for rows written before keys were names.
func isEscalated(m map[string][]string, steps []spec.ApprovalStep, idx int) bool {
	if _, ok := m[StepKey(steps, idx)]; ok {
		return true
	}
	_, ok := m[strconv.Itoa(idx)]
	return ok
}

// checkEscalation escalates the active step of a pending approval if its
// escalation.after duration has elapsed since the step became active.
func (w *EscalationWorker) checkEscalation(ctx context.Context, row db.ApprovalRequestRow) {
	// Resolve the workflow from the registry.
	wf, ok := w.registry.Get(row.GateModule, row.GateName)
	if !ok {
		// Workflow removed from spec — leave the approval as-is.
		return
	}
	stepIdx, step, ok := w.activeStep(wf, row)
	if !ok {
		return
	}
	if step.Escalation == nil || step.Escalation.After == "" {
		return
	}

	// The step's own key, so this bucket and the `approvals` bucket name the same
	// step. The legacy numeric key is accepted on read, so a row written before
	// the key shape changed is not escalated twice.
	stepKey := StepKey(wf.Steps, stepIdx)
	if isEscalated(row.EscalatedSteps, wf.Steps, stepIdx) {
		return
	}

	// Parse the escalation.after duration.
	after, err := time.ParseDuration(step.Escalation.After)
	if err != nil {
		return
	}

	// The step became active when the approval was last updated (or created).
	activeSince, err := time.Parse(time.RFC3339Nano, row.UpdatedAt)
	if err != nil {
		activeSince = w.now()
	}
	if w.now().Sub(activeSince) < after {
		return // not yet due
	}

	// Escalate: mark the step as escalated with the DUTY that takes over.
	//
	// The stored value is the qualified permission, not the manifest's short name:
	// `CanApprove` reads it back without the entity/transition context, and a value
	// whose meaning depends on where it was written is exactly the coupling the
	// step-name key removed for step identity.
	if row.EscalatedSteps == nil {
		row.EscalatedSteps = make(map[string][]string)
	}
	// Migrate a legacy numeric bucket onto the step key first, so the escalation
	// does not split the same step's history across two keys.
	if _, hasPrimary := row.EscalatedSteps[stepKey]; !hasPrimary {
		if legacy, ok := row.EscalatedSteps[strconv.Itoa(stepIdx)]; ok {
			row.EscalatedSteps[stepKey] = legacy
			delete(row.EscalatedSteps, strconv.Itoa(stepIdx))
		}
	}
	reassign := spec.EscalationPermission(row.GateModule, row.GateName, step.Escalation)
	row.EscalatedSteps[stepKey] = []string{reassign}

	// Record the escalation in the audit trail (7.4.6).
	if w.audit != nil {
		changes := map[string]any{
			"workflow": row.GateName,
			"step":     stepKey,
			"reassign": reassign,
		}
		if b, err := json.Marshal(changes); err == nil {
			_ = w.audit(ctx, row.TenantID, row.Entity, row.RecordID, "workflow.escalate", "system", string(b), "")
		}
	}

	if err := w.store.Update(ctx, row); err != nil {
		log.Printf("[workflow-escalation] update escalated approval %s: %v", row.ID, err)
		return
	}
	log.Printf("[workflow-escalation] escalated step %d of approval %s (workflow %s, reassign=%s)",
		row.ActiveStep, row.ID, row.GateName, reassign)
}
