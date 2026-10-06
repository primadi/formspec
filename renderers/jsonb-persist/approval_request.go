package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// ApprovalRequestStore persists transition approval requests
// (02-core-extended.md §2). One row per (tenant, entity, record, workflow)
// while approval is in flight; rows are updated as steps are approved or the
// request is rejected.
type ApprovalRequestStore struct {
	db     DB
	driver DriverType
}

// ApprovalRequestRow is a row in formspec_workflow_approval.
type ApprovalRequestRow struct {
	ID          string
	TenantID    string
	Entity      string // "module.entity"
	RecordID    string
	GateModule  string
	GateName    string
	FromState   string
	ToState     string
	RequesterID string
	Status      string // pending | approved | rejected
	ActiveStep  int
	// ActiveStepName names the step `active_step` points at, for the steps that
	// declare a name (Spec.ApprovalStep.Name).
	//
	// It exists because the INDEX alone is ambiguous the moment applicability
	// depends on data: `when` is evaluated against the record, so the list of
	// steps in force is not necessarily the list that was authored, and a
	// consumer that indexes into the authored list reads a different step than
	// the one awaiting approval. The escalation worker is exactly such a
	// consumer — it has no entity access, so it cannot re-derive the applicable
	// list and must be told which step is active.
	//
	// Empty for steps without a name, and for rows written before this column
	// existed; consumers then fall back to the index.
	ActiveStepName string
	// Approvals records who approved which step, keyed by the workflow's step key
	// (`{step name}` for a named step, `#{index}` otherwise). Legacy rows keyed by
	// the bare numeric index are still read — JSON object keys are strings either
	// way, so both land in this same map and the reader prefers the name.
	Approvals  map[string][]string
	RejectedBy string
	RejectStep int
	// EscalatedSteps records which steps were escalated (7.4.4), keyed the same
	// way.
	EscalatedSteps map[string][]string
	// Params holds the input values the REQUESTER supplied for the intercepted
	// transition (plan action-input-contract, D5). They are applied to the record
	// together with the state change when approval completes, because the
	// requesting call returns 202 before writing anything — without this the
	// values it collected were dropped on the floor.
	Params    map[string]any
	CreatedAt string
	UpdatedAt string
}

// approvalColumns is the SELECT list shared by every read of
// formspec_workflow_approval. It is a single constant because the column list
// and the scan function must agree exactly: a read that forgets a column does
// not fail loudly, it fails on the NEXT scan with a count mismatch — and the
// four call sites here (by record, by id, two list variants) drifted apart the
// moment a column was added by hand.
const approvalColumns = `id, tenant_id, entity, record_id, workflow_module, workflow_name,
	       from_state, to_state, requester_id, status, active_step, active_step_name,
	       approvals, rejected_by, reject_step, escalated_steps, params, created_at, updated_at`

// NewApprovalRequestStore creates a new approval store.
func NewApprovalRequestStore(db DB, driver DriverType) *ApprovalRequestStore {
	return &ApprovalRequestStore{db: db, driver: driver}
}

// Create inserts a new pending approval request.
func (s *ApprovalRequestStore) Create(ctx context.Context, row ApprovalRequestRow) (string, error) {
	approvalsJSON, err := json.Marshal(row.Approvals)
	if err != nil {
		return "", fmt.Errorf("approval create: marshal approvals: %w", err)
	}
	escalatedJSON, err := json.Marshal(row.EscalatedSteps)
	if err != nil {
		return "", fmt.Errorf("approval create: marshal escalated_steps: %w", err)
	}
	paramsJSON, err := json.Marshal(marshalableParams(row.Params))
	if err != nil {
		return "", fmt.Errorf("approval create: marshal params: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	status := row.Status
	if status == "" {
		status = "pending"
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO formspec_workflow_approval
			(tenant_id, entity, record_id, workflow_module, workflow_name,
			 from_state, to_state, requester_id, status, active_step, active_step_name,
			 approvals, rejected_by, reject_step, escalated_steps, params, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.TenantID, row.Entity, row.RecordID, row.GateModule, row.GateName,
		row.FromState, row.ToState, row.RequesterID, status, row.ActiveStep, row.ActiveStepName,
		string(approvalsJSON), row.RejectedBy, row.RejectStep, string(escalatedJSON),
		string(paramsJSON), now, now,
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
func (s *ApprovalRequestStore) GetByRecord(ctx context.Context, tenantID, entity, recordID string) (*ApprovalRequestRow, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+approvalColumns+`
		FROM formspec_workflow_approval
		WHERE tenant_id = ? AND entity = ? AND record_id = ? AND status = 'pending'
		ORDER BY created_at DESC LIMIT 1`,
		tenantID, entity, recordID,
	)
	return scanApprovalRow(row)
}

// GetPendingByID returns ONE workspace's pending approval by its id, or nil
// when there is none. Scoped by tenant on purpose: a client-facing surface must
// not be able to address another workspace's row by guessing an id.
func (s *ApprovalRequestStore) GetPendingByID(ctx context.Context, tenantID, id string) (*ApprovalRequestRow, error) {
	if tenantID == "" || id == "" {
		return nil, nil
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT `+approvalColumns+`
		FROM formspec_workflow_approval
		WHERE tenant_id = ? AND id = ? AND status = 'pending'
		LIMIT 1`,
		tenantID, id,
	)
	return scanApprovalRow(row)
}

// ListPending returns all pending approval requests (todo 7.4.4 escalation
// worker). limit caps the number of rows returned.
//
// It is deliberately TENANT-BLIND: the escalation worker sweeps every
// workspace in one pass. Anything serving a request must use
// ListPendingForTenant instead — a client-facing list built on this query
// would hand one workspace another workspace's approvals.
func (s *ApprovalRequestStore) ListPending(ctx context.Context, limit int) ([]ApprovalRequestRow, error) {
	return s.listPending(ctx, "", limit)
}

// ListPendingForTenant returns the pending approval requests of ONE workspace.
// Used by the ApprovalInbox surface (kind: ApprovalInbox, plan
// docs_internal/plan/approval-inbox-endpoint.md), where tenant isolation is a
// correctness requirement rather than a convenience.
func (s *ApprovalRequestStore) ListPendingForTenant(ctx context.Context, tenantID string, limit int) ([]ApprovalRequestRow, error) {
	if tenantID == "" {
		// Fail closed: an unscoped "my approvals" query is the leak this
		// method exists to prevent.
		return nil, nil
	}
	return s.listPending(ctx, tenantID, limit)
}

// listPending is the shared body of ListPending / ListPendingForTenant.
// tenantID == "" means "every workspace".
func (s *ApprovalRequestStore) listPending(ctx context.Context, tenantID string, limit int) ([]ApprovalRequestRow, error) {
	if limit <= 0 {
		limit = 100
	}
	query := `
		SELECT ` + approvalColumns + `
		FROM formspec_workflow_approval
		WHERE status = 'pending'`
	args := []any{}
	if tenantID != "" {
		query += ` AND tenant_id = ?`
		args = append(args, tenantID)
	}
	query += ` ORDER BY created_at ASC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("approval list pending: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []ApprovalRequestRow
	for rows.Next() {
		var r ApprovalRequestRow
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

// marshalableParams drops values JSON cannot encode, so storing a request's
// inputs can never fail the whole approval creation.
//
// It matters because these values come straight off a decoded JSON body: in
// practice they are already encodable, but a value injected by a hook (a
// function, a channel) would otherwise turn "start an approval" into a 500 — an
// input that cannot be stored is a reason to store less, not to refuse the
// transition.
func marshalableParams(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	if _, err := json.Marshal(in); err == nil {
		return in
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		if _, err := json.Marshal(v); err == nil {
			out[k] = v
		}
	}
	return out
}

// Update persists an updated approval row.
func (s *ApprovalRequestStore) Update(ctx context.Context, row ApprovalRequestRow) error {
	approvalsJSON, err := json.Marshal(row.Approvals)
	if err != nil {
		return fmt.Errorf("approval update: marshal approvals: %w", err)
	}
	escalatedJSON, err := json.Marshal(row.EscalatedSteps)
	if err != nil {
		return fmt.Errorf("approval update: marshal escalated_steps: %w", err)
	}
	paramsJSON, err := json.Marshal(marshalableParams(row.Params))
	if err != nil {
		return fmt.Errorf("approval update: marshal params: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx, `
		UPDATE formspec_workflow_approval
		SET status = ?, active_step = ?, active_step_name = ?, approvals = ?, rejected_by = ?,
		    reject_step = ?, escalated_steps = ?, params = ?, updated_at = ?
		WHERE id = ?`,
		row.Status, row.ActiveStep, row.ActiveStepName, string(approvalsJSON), row.RejectedBy,
		row.RejectStep, string(escalatedJSON), string(paramsJSON), now, row.ID,
	)
	if err != nil {
		return fmt.Errorf("approval update: %w", err)
	}
	return nil
}

// scanApprovalRow scans a single approval row.
func scanApprovalRow(row *sql.Row) (*ApprovalRequestRow, error) {
	var (
		r          ApprovalRequestRow
		approvals  string
		escalated  string
		params     string
		rejectStep sql.NullInt64
	)
	err := row.Scan(
		&r.ID, &r.TenantID, &r.Entity, &r.RecordID, &r.GateModule,
		&r.GateName, &r.FromState, &r.ToState, &r.RequesterID, &r.Status,
		&r.ActiveStep, &r.ActiveStepName, &approvals, &r.RejectedBy, &rejectStep, &escalated, &params,
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
		r.Approvals = make(map[string][]string)
	}
	if err := json.Unmarshal([]byte(escalated), &r.EscalatedSteps); err != nil {
		r.EscalatedSteps = make(map[string][]string)
	}
	if err := json.Unmarshal([]byte(params), &r.Params); err != nil {
		r.Params = nil
	}
	normalizeApprovalRow(&r)
	return &r, nil
}

// scanApprovalRows scans a single approval row from a *sql.Rows cursor.
func scanApprovalRows(rows *sql.Rows, r *ApprovalRequestRow) error {
	var (
		approvals  string
		escalated  string
		params     string
		rejectStep sql.NullInt64
	)
	err := rows.Scan(
		&r.ID, &r.TenantID, &r.Entity, &r.RecordID, &r.GateModule,
		&r.GateName, &r.FromState, &r.ToState, &r.RequesterID, &r.Status,
		&r.ActiveStep, &r.ActiveStepName, &approvals, &r.RejectedBy, &rejectStep, &escalated, &params,
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
	if err := json.Unmarshal([]byte(params), &r.Params); err != nil {
		r.Params = nil
	}
	if err := json.Unmarshal([]byte(approvals), &r.Approvals); err != nil {
		r.Approvals = make(map[string][]string)
	}
	if err := json.Unmarshal([]byte(escalated), &r.EscalatedSteps); err != nil {
		r.EscalatedSteps = make(map[string][]string)
	}
	normalizeApprovalRow(r)
	return nil
}

// normalizeApprovalRow ensures JSON-map fields are non-nil and reject_step is
// sane after scanning.
func normalizeApprovalRow(r *ApprovalRequestRow) {
	if r.Approvals == nil {
		r.Approvals = make(map[string][]string)
	}
	if r.EscalatedSteps == nil {
		r.EscalatedSteps = make(map[string][]string)
	}
	if r.RejectStep == 0 {
		r.RejectStep = -1
	}
}
