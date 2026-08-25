package workflow

import (
	"fmt"
	"strings"

	"github.com/primadi/formspec/internal/starlark"
	"github.com/primadi/formspec/pkg/spec"
)

// Engine evaluates workflows for state-machine transitions and checks
// approver eligibility (02-core-extended.md §2).
type Engine struct {
	reg *Registry
}

// NewEngine creates a workflow engine bound to the given registry.
func NewEngine(reg *Registry) *Engine {
	return &Engine{reg: reg}
}

// RequiresApproval reports whether the given transition is intercepted by at
// least one workflow. entity is "module.entity" (e.g. "gl.journal-entry").
func (e *Engine) RequiresApproval(entity, from, to string) bool {
	if e.reg == nil {
		return false
	}
	return len(e.reg.ForTransition(entity, from, to)) > 0
}

// WorkflowsFor returns the workflows intercepting the given transition.
func (e *Engine) WorkflowsFor(entity, from, to string) []*spec.WorkflowSpec {
	if e.reg == nil {
		return nil
	}
	return e.reg.ForTransition(entity, from, to)
}

// ApplicableSteps returns the steps of a workflow that apply to the given
// resource data — steps whose `when` condition (FormSpecExpr over `resource`)
// evaluates true are included; steps with a false `when` are skipped without
// holding the transition (02-core-extended.md §2.1).
func (e *Engine) ApplicableSteps(wf *spec.WorkflowSpec, resourceData map[string]any) ([]spec.WorkflowStep, error) {
	if wf == nil {
		return nil, nil
	}
	steps := make([]spec.WorkflowStep, 0, len(wf.Steps))
	for _, step := range wf.Steps {
		if step.When == "" {
			steps = append(steps, step)
			continue
		}
		passed, _, err := starlark.EvaluateGuard(step.When, resourceData)
		if err != nil {
			return nil, fmt.Errorf("workflow step when: %w", err)
		}
		if passed {
			steps = append(steps, step)
		}
	}
	return steps, nil
}

// StepMode returns the effective mode of a step ("all" default).
func StepMode(step spec.WorkflowStep) string {
	if step.Mode == "" {
		return "all"
	}
	return step.Mode
}

// Quorum returns the number of approvals required for a step. For mode "all"
// the quorum is the number of distinct eligible approvers (computed from the
// roles); for "any" it's step.Approvers (default 1); for "sequential" it's 1
// (one approver per role in the chain).
func Quorum(step spec.WorkflowStep, eligibleCount int) int {
	switch StepMode(step) {
	case "any":
		if step.Approvers <= 0 {
			return 1
		}
		return step.Approvers
	case "sequential":
		return 1
	default: // "all"
		if eligibleCount <= 0 {
			return 1
		}
		return eligibleCount
	}
}

// CanApprove checks whether a user (by ID and roles) may approve the given
// step of a workflow for a record. It enforces:
//   - requester exclusion (7.4.5): the requester (record created_by) can
//     never approve their own request
//   - role eligibility: the user must hold at least one of the step's roles,
//     OR one of the escalated reassign_roles (7.4.4) when the step has been
//     escalated
//
// stepIdx is the 0-based index of the step within the workflow's steps.
// escalatedRoles are the reassign_roles granted by escalation for this step
// (empty when the step has not been escalated).
func (e *Engine) CanApprove(wf *spec.WorkflowSpec, stepIdx int, userID string, userRoles []string, requesterID string, escalatedRoles []string, resourceData map[string]any) (bool, string) {
	if wf == nil || stepIdx < 0 || stepIdx >= len(wf.Steps) {
		return false, "workflow step out of range"
	}
	step := wf.Steps[stepIdx]

	// Requester exclusion (7.4.5): the requester can never approve their own
	// request. requesterID is the record's created_by.
	if requesterID != "" && userID != "" && userID == requesterID {
		return false, "requester cannot approve their own request"
	}

	// Role eligibility: user must hold at least one of the step's roles, or
	// one of the escalated reassign_roles (7.4.4).
	eligible := append(append([]string{}, step.Roles...), escalatedRoles...)
	if !hasAnyRole(userRoles, eligible) {
		return false, "user does not hold any of the step's required roles"
	}

	return true, ""
}

// hasAnyRole reports whether the user's roles intersect the required roles.
func hasAnyRole(userRoles, required []string) bool {
	for _, r := range userRoles {
		for _, req := range required {
			if r == req {
				return true
			}
		}
	}
	return false
}

// ApprovalStatus is the lifecycle state of an approval request.
type ApprovalStatus string

const (
	ApprovalPending  ApprovalStatus = "pending"
	ApprovalApproved ApprovalStatus = "approved"
	ApprovalRejected ApprovalStatus = "rejected"
)

// Approval is the runtime state of one approval request for a record.
type Approval struct {
	ID             string         `json:"id"`
	WorkflowModule string         `json:"workflow_module"`
	WorkflowName   string         `json:"workflow_name"`
	Entity         string         `json:"entity"` // "module.entity"
	RecordID       string         `json:"record_id"`
	From           string         `json:"from"`
	To             string         `json:"to"`
	RequesterID    string         `json:"requester_id"`
	Status         ApprovalStatus `json:"status"`
	// ActiveStep is the 0-based index of the step currently awaiting approval.
	ActiveStep int `json:"active_step"`
	// Approvals records who approved which step: stepIdx -> userID.
	Approvals map[int][]string `json:"approvals"`
	// RejectedBy records who rejected (and at which step).
	RejectedBy string `json:"rejected_by,omitempty"`
	RejectStep int    `json:"reject_step,omitempty"`
	// EscalatedSteps records which steps were escalated (7.4.4) and the
	// reassign_roles that gained approval rights: stepIdx -> reassign_roles.
	EscalatedSteps map[int][]string `json:"escalated_steps,omitempty"`
}

// NewApproval creates a pending approval for a record.
func NewApproval(wf *spec.WorkflowSpec, module, entity, recordID, from, to, requesterID string) *Approval {
	return &Approval{
		WorkflowModule: module,
		WorkflowName:   wfName(wf),
		Entity:         entity,
		RecordID:       recordID,
		From:           from,
		To:             to,
		RequesterID:    requesterID,
		Status:         ApprovalPending,
		ActiveStep:     0,
		Approvals:      make(map[int][]string),
		EscalatedSteps: make(map[int][]string),
	}
}

// wfName returns a stable name for a workflow (best-effort; the registry key
// is authoritative).
func wfName(wf *spec.WorkflowSpec) string {
	if wf == nil {
		return ""
	}
	return fmt.Sprintf("%p", wf)
}

// StepApproved reports whether the given step has reached its quorum.
func (a *Approval) StepApproved(step spec.WorkflowStep, eligibleCount int) bool {
	approvers := a.Approvals[a.ActiveStep]
	return len(approvers) >= Quorum(step, eligibleCount)
}

// AllStepsApproved reports whether every applicable step has been approved.
func (a *Approval) AllStepsApproved(steps []spec.WorkflowStep) bool {
	return a.ActiveStep >= len(steps)
}

// Approve records an approval by userID for the active step. Returns an error
// if the user already approved this step.
func (a *Approval) Approve(userID string) error {
	for _, u := range a.Approvals[a.ActiveStep] {
		if u == userID {
			return fmt.Errorf("user %s already approved step %d", userID, a.ActiveStep)
		}
	}
	a.Approvals[a.ActiveStep] = append(a.Approvals[a.ActiveStep], userID)
	return nil
}

// Reject records a rejection by userID at the active step.
func (a *Approval) Reject(userID string) {
	a.Status = ApprovalRejected
	a.RejectedBy = userID
	a.RejectStep = a.ActiveStep
}

// Advance moves to the next step (called when the active step reaches quorum).
func (a *Approval) Advance() {
	a.ActiveStep++
	if a.ActiveStep >= len(a.Approvals) {
		// No more steps recorded — the caller tracks step count separately.
	}
}

// splitEntityRef splits "module.entity" into module and entity.
func splitEntityRef(ref string) (module, entity string, ok bool) {
	parts := strings.Split(ref, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// ToRow converts an Approval into a persistence row. The row's ID/tenant/
// timestamps are left for the store to manage.
func (a *Approval) ToRow() *ApprovalRow {
	return &ApprovalRow{
		Entity:         a.Entity,
		RecordID:       a.RecordID,
		WorkflowModule: a.WorkflowModule,
		WorkflowName:   a.WorkflowName,
		FromState:      a.From,
		ToState:        a.To,
		RequesterID:    a.RequesterID,
		Status:         string(a.Status),
		ActiveStep:     a.ActiveStep,
		Approvals:      a.Approvals,
		RejectedBy:     a.RejectedBy,
		RejectStep:     a.RejectStep,
		EscalatedSteps: a.EscalatedSteps,
	}
}

// FromRow converts a persistence row back into an Approval.
func FromRow(row *ApprovalRow) *Approval {
	if row == nil {
		return nil
	}
	return &Approval{
		ID:             row.ID,
		WorkflowModule: row.WorkflowModule,
		WorkflowName:   row.WorkflowName,
		Entity:         row.Entity,
		RecordID:       row.RecordID,
		From:           row.FromState,
		To:             row.ToState,
		RequesterID:    row.RequesterID,
		Status:         ApprovalStatus(row.Status),
		ActiveStep:     row.ActiveStep,
		Approvals:      row.Approvals,
		RejectedBy:     row.RejectedBy,
		RejectStep:     row.RejectStep,
		EscalatedSteps: row.EscalatedSteps,
	}
}

// ApprovalRow mirrors db.WorkflowApprovalRow without importing the renderer
// package (avoids an internal → renderer dependency for the workflow engine).
type ApprovalRow struct {
	ID             string
	TenantID       string
	Entity         string
	RecordID       string
	WorkflowModule string
	WorkflowName   string
	FromState      string
	ToState        string
	RequesterID    string
	Status         string
	ActiveStep     int
	Approvals      map[int][]string
	RejectedBy     string
	RejectStep     int
	EscalatedSteps map[int][]string
	CreatedAt      string
	UpdatedAt      string
}
