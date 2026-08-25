package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// WorkflowApprovalStore persists kind: Workflow approval requests
// (02-core-extended.md §2). One row per (tenant, entity, record, workflow)
// while approval is in flight; rows are updated as steps are approved or the
// request is rejected.
type WorkflowApprovalStore struct {
	db     DB
	driver DriverType
}

// WorkflowApprovalRow is a row in formspec_workflow_approval.
type WorkflowApprovalRow struct {
	ID             string
	TenantID       string
	Entity         string // "module.entity"
	RecordID       string
	WorkflowModule string
	WorkflowName   string
	FromState      string
	ToState        string
	RequesterID    string
	Status         string // pending | approved | rejected
	ActiveStep     int
	Approvals      map[int][]string // stepIdx -> userIDs
	RejectedBy     string
	RejectStep     int
	EscalatedSteps map[int][]string // stepIdx -> reassign_roles (7.4.4)
	CreatedAt      string
	UpdatedAt      string
}

// NewWorkflowApprovalStore creates a new approval store.
func NewWorkflowApprovalStore(db DB, driver DriverType) *WorkflowApprovalStore {
	return &WorkflowApprovalStore{db: db, driver: driver}
}

// Create inserts a new pending approval request.
func (s *WorkflowApprovalStore) Create(ctx context.Context, row WorkflowApprovalRow) (string, error) {
	approvalsJSON, err := json.Marshal(row.Approvals)
	if err != nil {
		return "", fmt.Errorf("approval create: marshal approvals: %w", err)
	}
	escalatedJSON, err := json.Marshal(row.EscalatedSteps)
	if err != nil {
		return "", fmt.Errorf("approval create: marshal escalated_steps: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	status := row.Status
	if status == "" {
		status = "pending"
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO formspec_workflow_approval
			(tenant_id, entity, record_id, workflow_module, workflow_name,
			 from_state, to_state, requester_id, status, active_step,
			 approvals, rejected_by, reject_step, escalated_steps, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.TenantID, row.Entity, row.RecordID, row.WorkflowModule, row.WorkflowName,
		row.FromState, row.ToState, row.RequesterID, status, row.ActiveStep,
		string(approvalsJSON), row.RejectedBy, row.RejectStep, string(escalatedJSON), now, now,
	)
	if err != nil {
		return "", fmt.Errorf("approval create: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return "", fmt.Errorf("approval create: last insert id: %w", err)
	}
	return fmt.Sprintf("%d", id), nil
}

// GetByRecord returns the active (pending) approval for a record, if any.
func (s *WorkflowApprovalStore) GetByRecord(ctx context.Context, tenantID, entity, recordID string) (*WorkflowApprovalRow, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, entity, record_id, workflow_module, workflow_name,
		       from_state, to_state, requester_id, status, active_step,
		       approvals, rejected_by, reject_step, escalated_steps, created_at, updated_at
		FROM formspec_workflow_approval
		WHERE tenant_id = ? AND entity = ? AND record_id = ? AND status = 'pending'
		ORDER BY created_at DESC LIMIT 1`,
		tenantID, entity, recordID,
	)
	return scanApprovalRow(row)
}

// ListPending returns all pending approval requests (todo 7.4.4 escalation
// worker). limit caps the number of rows returned.
func (s *WorkflowApprovalStore) ListPending(ctx context.Context, limit int) ([]WorkflowApprovalRow, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, entity, record_id, workflow_module, workflow_name,
		       from_state, to_state, requester_id, status, active_step,
		       approvals, rejected_by, reject_step, escalated_steps, created_at, updated_at
		FROM formspec_workflow_approval
		WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("approval list pending: %w", err)
	}
	defer rows.Close()

	var out []WorkflowApprovalRow
	for rows.Next() {
		var r WorkflowApprovalRow
		if err := scanApprovalRows(rows, &r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("approval list pending rows: %w", err)
	}
	return out, nil
}

// Update persists an updated approval row.
func (s *WorkflowApprovalStore) Update(ctx context.Context, row WorkflowApprovalRow) error {
	approvalsJSON, err := json.Marshal(row.Approvals)
	if err != nil {
		return fmt.Errorf("approval update: marshal approvals: %w", err)
	}
	escalatedJSON, err := json.Marshal(row.EscalatedSteps)
	if err != nil {
		return fmt.Errorf("approval update: marshal escalated_steps: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx, `
		UPDATE formspec_workflow_approval
		SET status = ?, active_step = ?, approvals = ?, rejected_by = ?,
		    reject_step = ?, escalated_steps = ?, updated_at = ?
		WHERE id = ?`,
		row.Status, row.ActiveStep, string(approvalsJSON), row.RejectedBy,
		row.RejectStep, string(escalatedJSON), now, row.ID,
	)
	if err != nil {
		return fmt.Errorf("approval update: %w", err)
	}
	return nil
}

// scanApprovalRow scans a single approval row.
func scanApprovalRow(row *sql.Row) (*WorkflowApprovalRow, error) {
	var (
		r          WorkflowApprovalRow
		approvals  string
		escalated  string
		rejectStep sql.NullInt64
	)
	err := row.Scan(
		&r.ID, &r.TenantID, &r.Entity, &r.RecordID, &r.WorkflowModule,
		&r.WorkflowName, &r.FromState, &r.ToState, &r.RequesterID, &r.Status,
		&r.ActiveStep, &approvals, &r.RejectedBy, &rejectStep, &escalated,
		&r.CreatedAt, &r.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("approval scan: %w", err)
	}
	if rejectStep.Valid {
		r.RejectStep = int(rejectStep.Int64)
	} else {
		r.RejectStep = -1
	}
	if err := json.Unmarshal([]byte(approvals), &r.Approvals); err != nil {
		r.Approvals = make(map[int][]string)
	}
	if err := json.Unmarshal([]byte(escalated), &r.EscalatedSteps); err != nil {
		r.EscalatedSteps = make(map[int][]string)
	}
	normalizeApprovalRow(&r)
	return &r, nil
}

// scanApprovalRows scans a single approval row from a *sql.Rows cursor.
func scanApprovalRows(rows *sql.Rows, r *WorkflowApprovalRow) error {
	var (
		approvals  string
		escalated  string
		rejectStep sql.NullInt64
	)
	err := rows.Scan(
		&r.ID, &r.TenantID, &r.Entity, &r.RecordID, &r.WorkflowModule,
		&r.WorkflowName, &r.FromState, &r.ToState, &r.RequesterID, &r.Status,
		&r.ActiveStep, &approvals, &r.RejectedBy, &rejectStep, &escalated,
		&r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("approval scan: %w", err)
	}
	if rejectStep.Valid {
		r.RejectStep = int(rejectStep.Int64)
	} else {
		r.RejectStep = -1
	}
	if err := json.Unmarshal([]byte(approvals), &r.Approvals); err != nil {
		r.Approvals = make(map[int][]string)
	}
	if err := json.Unmarshal([]byte(escalated), &r.EscalatedSteps); err != nil {
		r.EscalatedSteps = make(map[int][]string)
	}
	normalizeApprovalRow(r)
	return nil
}

// normalizeApprovalRow ensures JSON-map fields are non-nil and reject_step is
// sane after scanning.
func normalizeApprovalRow(r *WorkflowApprovalRow) {
	if r.Approvals == nil {
		r.Approvals = make(map[int][]string)
	}
	if r.EscalatedSteps == nil {
		r.EscalatedSteps = make(map[int][]string)
	}
	if r.RejectStep == 0 {
		r.RejectStep = -1
	}
}
