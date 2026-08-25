package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// JobRow is a row in formspec_job (02-core-extended.md §13, todo 7.13).
type JobRow struct {
	ID          string
	TenantID    string
	Module      string
	Entity      string
	Action      string
	Status      string // pending | running | completed | failed
	Progress    int
	Message     string
	Result      map[string]any
	Error       string
	CallbackURL string // optional callback webhook URL (7.13.4)
	CreatedAt   string
	UpdatedAt   string
}

// JobStore persists async job tracking rows (todo 7.13). A tracked async
// action (`call: async` + `track: true`) creates a job row, reports progress
// via ctx.job.progress, and ends completed/failed — pushed to the `jobs`
// websocket channel by the Tracker.
type JobStore struct {
	db     DB
	driver DriverType
}

// NewJobStore creates a new job store.
func NewJobStore(db DB, driver DriverType) *JobStore {
	return &JobStore{db: db, driver: driver}
}

// Create inserts a new pending job and returns its row.
func (s *JobStore) Create(ctx context.Context, tenantID, module, entity, action, callbackURL string) (*JobRow, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO formspec_job (tenant_id, module, entity, action, status, progress, message, result, error, callback_url, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'pending', 0, '', '{}', '', ?, ?, ?)`,
		tenantID, module, entity, action, callbackURL, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("job create: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("job create: last insert id: %w", err)
	}
	return &JobRow{
		ID:          fmt.Sprintf("%d", id),
		TenantID:    tenantID,
		Module:      module,
		Entity:      entity,
		Action:      action,
		Status:      "pending",
		CallbackURL: callbackURL,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// Update writes a job's status/progress/message/result/error.
func (s *JobStore) Update(ctx context.Context, id, status string, progress int, message string, result map[string]any, errMsg string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	resultJSON := "{}"
	if result != nil {
		b, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("job update: marshal result: %w", err)
		}
		resultJSON = string(b)
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE formspec_job
		SET status = ?, progress = ?, message = ?, result = ?, error = ?, updated_at = ?
		WHERE id = ?`,
		status, progress, message, resultJSON, errMsg, now, id)
	if err != nil {
		return fmt.Errorf("job update: %w", err)
	}
	return nil
}

// Get returns a job row by ID, or nil when absent.
func (s *JobStore) Get(ctx context.Context, id string) (*JobRow, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, module, entity, action, status, progress, message, result, error, callback_url, created_at, updated_at
		FROM formspec_job
		WHERE id = ?`, id)
	var j JobRow
	var resultJSON string
	err := row.Scan(&j.ID, &j.TenantID, &j.Module, &j.Entity, &j.Action,
		&j.Status, &j.Progress, &j.Message, &resultJSON, &j.Error,
		&j.CallbackURL, &j.CreatedAt, &j.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("job get: %w", err)
	}
	if resultJSON != "" && resultJSON != "{}" {
		_ = json.Unmarshal([]byte(resultJSON), &j.Result)
	}
	return &j, nil
}

// ListByWorkspace returns jobs for a workspace, newest first.
func (s *JobStore) ListByWorkspace(ctx context.Context, tenantID string, limit int) ([]JobRow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, module, entity, action, status, progress, message, result, error, callback_url, created_at, updated_at
		FROM formspec_job
		WHERE tenant_id = ?
		ORDER BY created_at DESC
		LIMIT ?`, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("job list: %w", err)
	}
	defer rows.Close()

	var out []JobRow
	for rows.Next() {
		var j JobRow
		var resultJSON string
		if err := rows.Scan(&j.ID, &j.TenantID, &j.Module, &j.Entity, &j.Action,
			&j.Status, &j.Progress, &j.Message, &resultJSON, &j.Error,
			&j.CallbackURL, &j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, fmt.Errorf("job list scan: %w", err)
		}
		if resultJSON != "" && resultJSON != "{}" {
			_ = json.Unmarshal([]byte(resultJSON), &j.Result)
		}
		out = append(out, j)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("job list rows: %w", err)
	}
	return out, nil
}