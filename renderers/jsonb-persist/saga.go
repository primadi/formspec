package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SagaStore persists cross-boundary integrator calls and their compensate
// actions (02-core-extended.md §5, todo 7.7.4). A saga entry is registered
// when an integrator dispatches a cross-boundary call; on failure the
// compensate action is invoked and the entry marked compensated.
type SagaStore struct {
	db     DB
	driver DriverType
}

// SagaEntry is a row in formspec_saga_log.
type SagaEntry struct {
	ID         string
	TenantID   string
	Source     string // originating event, e.g. "billing.invoice.on_submit"
	Target     string // target action, e.g. "gl.journal-entry.create"
	Compensate string // compensate action ref
	Status     string // pending | compensated | completed
	Error      string
	CreatedAt  string
	UpdatedAt  string
}

// NewSagaStore creates a new saga store.
func NewSagaStore(db DB, driver DriverType) *SagaStore {
	return &SagaStore{db: db, driver: driver}
}

// Register inserts a new pending saga entry for a cross-boundary call.
func (s *SagaStore) Register(ctx context.Context, tenantID, source, target, compensate string) (string, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO formspec_saga_log (tenant_id, source, target, compensate, status, error, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'pending', '', ?, ?)`,
		tenantID, source, target, compensate, now, now,
	)
	if err != nil {
		return "", fmt.Errorf("saga register: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return "", fmt.Errorf("saga register: last insert id: %w", err)
	}
	return fmt.Sprintf("%d", id), nil
}

// MarkCompleted marks a saga entry as completed (target action succeeded).
func (s *SagaStore) MarkCompleted(ctx context.Context, id string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx,
		"UPDATE formspec_saga_log SET status = 'completed', updated_at = ? WHERE id = ?",
		now, id)
	if err != nil {
		return fmt.Errorf("saga mark completed: %w", err)
	}
	return nil
}

// MarkCompensated marks a saga entry as compensated (compensate action ran).
func (s *SagaStore) MarkCompensated(ctx context.Context, id, errMsg string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx,
		"UPDATE formspec_saga_log SET status = 'compensated', error = ?, updated_at = ? WHERE id = ?",
		errMsg, now, id)
	if err != nil {
		return fmt.Errorf("saga mark compensated: %w", err)
	}
	return nil
}

// ListPending returns pending saga entries (for a compensation worker).
func (s *SagaStore) ListPending(ctx context.Context, limit int) ([]SagaEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, source, target, compensate, status, error, created_at, updated_at
		FROM formspec_saga_log
		WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("saga list pending: %w", err)
	}
	defer rows.Close()

	var out []SagaEntry
	for rows.Next() {
		var e SagaEntry
		if err := rows.Scan(&e.ID, &e.TenantID, &e.Source, &e.Target, &e.Compensate,
			&e.Status, &e.Error, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("saga scan: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("saga list pending rows: %w", err)
	}
	return out, nil
}

// GetByID returns a saga entry by ID.
func (s *SagaStore) GetByID(ctx context.Context, id string) (*SagaEntry, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, source, target, compensate, status, error, created_at, updated_at
		FROM formspec_saga_log
		WHERE id = ?`, id)
	var e SagaEntry
	err := row.Scan(&e.ID, &e.TenantID, &e.Source, &e.Target, &e.Compensate,
		&e.Status, &e.Error, &e.CreatedAt, &e.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("saga get: %w", err)
	}
	return &e, nil
}
