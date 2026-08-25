package workflow

import (
	"context"
	"encoding/json"
	"log"
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
//   - marks the step as escalated with its reassign_roles, so approvers
//     holding the reassigned roles gain approval rights (7.4.4).
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
	ListPending(ctx context.Context, limit int) ([]db.WorkflowApprovalRow, error)
	Update(ctx context.Context, row db.WorkflowApprovalRow) error
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

// checkEscalation escalates the active step of a pending approval if its
// escalation.after duration has elapsed since the step became active.
func (w *EscalationWorker) checkEscalation(ctx context.Context, row db.WorkflowApprovalRow) {
	// Resolve the workflow from the registry.
	wf, ok := w.registry.Get(row.WorkflowModule, row.WorkflowName)
	if !ok {
		// Workflow removed from spec — leave the approval as-is.
		return
	}
	if row.ActiveStep < 0 || row.ActiveStep >= len(wf.Steps) {
		return
	}
	step := wf.Steps[row.ActiveStep]
	if step.Escalation == nil || step.Escalation.After == "" {
		return
	}

	// Skip if this step was already escalated.
	if _, escalated := row.EscalatedSteps[row.ActiveStep]; escalated {
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

	// Escalate: mark the step as escalated with its reassign_roles.
	if row.EscalatedSteps == nil {
		row.EscalatedSteps = make(map[int][]string)
	}
	row.EscalatedSteps[row.ActiveStep] = step.Escalation.ReassignRoles

	// Record the escalation in the audit trail (7.4.6).
	if w.audit != nil {
		changes := map[string]any{
			"workflow":       row.WorkflowName,
			"step":           row.ActiveStep,
			"reassign_roles": step.Escalation.ReassignRoles,
		}
		if b, err := json.Marshal(changes); err == nil {
			_ = w.audit(ctx, row.TenantID, row.Entity, row.RecordID, "workflow.escalate", "system", string(b), "")
		}
	}

	if err := w.store.Update(ctx, row); err != nil {
		log.Printf("[workflow-escalation] update escalated approval %s: %v", row.ID, err)
		return
	}
	log.Printf("[workflow-escalation] escalated step %d of approval %s (workflow %s, reassign=%v)",
		row.ActiveStep, row.ID, row.WorkflowName, step.Escalation.ReassignRoles)
}

// specWorkflowStep is a compile-time alias to keep the escalation logic
// readable against the spec type.
type specWorkflowStep = spec.WorkflowStep
