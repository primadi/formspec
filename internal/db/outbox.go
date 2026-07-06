package db

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"
)

// OutboxStore implements the outbox pattern for reliable event publishing.
//
// When a business action creates an event, the event is first written to the
// outbox table (within the same transaction as the business data), then a
// background worker picks up pending events and delivers them to the event bus.
//
// This guarantees at-least-once delivery: if the process crashes after writing
// to the outbox but before publishing, the worker will retry.
type OutboxStore struct {
	db     DB
	driver DriverType
}

// OutboxRecord represents a row in forma_outbox.
type OutboxRecord struct {
	ID          string
	TenantID    string
	EventName   string
	Resource    string
	Payload     string
	Status      string // pending | delivering | completed | failed
	RetryCount  int
	MaxRetries  int
	CreatedAt   string
	NextRetryAt string
}

// NewOutboxStore creates a new outbox store.
func NewOutboxStore(db DB, driver DriverType) *OutboxStore {
	return &OutboxStore{db: db, driver: driver}
}

// Enqueue inserts a new event into the outbox for processing.
func (s *OutboxStore) Enqueue(ctx context.Context, tenantID, eventName, resource, payload string) (string, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	query := `
		INSERT INTO forma_outbox (tenant_id, event_name, resource, payload, status, created_at, next_retry_at)
		VALUES (?, ?, ?, ?, 'pending', ?, ?)
		RETURNING id
	`

	var id string
	err := s.db.QueryRowContext(ctx, query,
		tenantID, eventName, resource, payload, now, now,
	).Scan(&id)
	if err != nil {
		// SQLite may not support RETURNING
		return s.enqueueFallback(ctx, tenantID, eventName, resource, payload, now)
	}
	return id, nil
}

func (s *OutboxStore) enqueueFallback(ctx context.Context, tenantID, eventName, resource, payload, now string) (string, error) {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO forma_outbox (tenant_id, event_name, resource, payload, status, created_at, next_retry_at)
		VALUES (?, ?, ?, ?, 'pending', ?, ?)
	`, tenantID, eventName, resource, payload, now, now)
	if err != nil {
		return "", fmt.Errorf("outbox enqueue: %w", err)
	}

	// Retrieve the last inserted ID
	var id string
	err = s.db.QueryRowContext(ctx, `SELECT last_insert_rowid()`).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("outbox get last id: %w", err)
	}
	return id, nil
}

// Dequeue retrieves a batch of pending events for processing.
// Events are claimed atomically by setting status to "delivering".
func (s *OutboxStore) Dequeue(ctx context.Context, batchSize int) ([]OutboxRecord, error) {
	if batchSize < 1 || batchSize > 100 {
		batchSize = 10
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, event_name, resource, payload, status, retry_count, max_retries, created_at, next_retry_at
		FROM forma_outbox
		WHERE status = 'pending' AND next_retry_at <= ?
		ORDER BY created_at ASC
		LIMIT ?
	`, now, batchSize)
	if err != nil {
		return nil, fmt.Errorf("outbox dequeue: %w", err)
	}
	defer rows.Close()

	var records []OutboxRecord
	for rows.Next() {
		var rec OutboxRecord
		if err := rows.Scan(&rec.ID, &rec.TenantID, &rec.EventName, &rec.Resource,
			&rec.Payload, &rec.Status, &rec.RetryCount, &rec.MaxRetries,
			&rec.CreatedAt, &rec.NextRetryAt); err != nil {
			return nil, fmt.Errorf("outbox scan: %w", err)
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("outbox rows: %w", err)
	}

	// Claim them
	for _, rec := range records {
		if _, err := s.db.ExecContext(ctx, `
			UPDATE forma_outbox SET status = 'delivering' WHERE id = ? AND status = 'pending'
		`, rec.ID); err != nil {
			return nil, fmt.Errorf("outbox claim %s: %w", rec.ID, err)
		}
	}

	return records, nil
}

// MarkCompleted marks an outbox record as successfully delivered.
func (s *OutboxStore) MarkCompleted(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE forma_outbox SET status = 'completed' WHERE id = ?
	`, id)
	return err
}

// MarkFailed marks an outbox record as failed and schedules a retry.
// Uses exponential backoff: 2^retry_count seconds, capped at max_retries.
func (s *OutboxStore) MarkFailed(ctx context.Context, id string, maxRetries int) error {
	// Get current retry count
	var retryCount int
	var currentStatus string
	err := s.db.QueryRowContext(ctx, `
		SELECT retry_count, status FROM forma_outbox WHERE id = ?
	`, id).Scan(&retryCount, &currentStatus)
	if err != nil {
		return fmt.Errorf("outbox mark failed get: %w", err)
	}

	newRetryCount := retryCount + 1

	if newRetryCount > maxRetries {
		// Max retries exceeded → mark as failed permanently
		_, err = s.db.ExecContext(ctx, `
			UPDATE forma_outbox SET status = 'failed', retry_count = ? WHERE id = ?
		`, newRetryCount, id)
		return err
	}

	// Exponential backoff: 2^retry_count seconds (cap at 3600s = 1h)
	backoffSeconds := 1 << uint(newRetryCount)
	backoff := math.Min(float64(backoffSeconds), 3600)
	nextRetry := time.Now().UTC().Add(time.Duration(backoff) * time.Second).Format(time.RFC3339Nano)

	_, err = s.db.ExecContext(ctx, `
		UPDATE forma_outbox SET status = 'pending', retry_count = ?, next_retry_at = ? WHERE id = ?
	`, newRetryCount, nextRetry, id)
	return err
}

// CountByStatus returns the count of outbox records grouped by status.
func (s *OutboxStore) CountByStatus(ctx context.Context) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT status, COUNT(*) as cnt FROM forma_outbox GROUP BY status
	`)
	if err != nil {
		return nil, fmt.Errorf("outbox count: %w", err)
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("outbox count scan: %w", err)
		}
		result[status] = count
	}
	return result, rows.Err()
}

// Cleanup removes completed outbox records older than the specified duration.
func (s *OutboxStore) Cleanup(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-olderThan).Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM forma_outbox WHERE status = 'completed' AND created_at < ?
	`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("outbox cleanup: %w", err)
	}
	return result.RowsAffected()
}

// Peek returns recent outbox records for monitoring without claiming them.
func (s *OutboxStore) Peek(ctx context.Context, limit int) ([]OutboxRecord, error) {
	if limit < 1 || limit > 100 {
		limit = 10
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, event_name, resource, payload, status, retry_count, max_retries, created_at, next_retry_at
		FROM forma_outbox
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("outbox peek: %w", err)
	}
	defer rows.Close()

	var records []OutboxRecord
	for rows.Next() {
		var rec OutboxRecord
		if err := rows.Scan(&rec.ID, &rec.TenantID, &rec.EventName, &rec.Resource,
			&rec.Payload, &rec.Status, &rec.RetryCount, &rec.MaxRetries,
			&rec.CreatedAt, &rec.NextRetryAt); err != nil {
			return nil, fmt.Errorf("outbox peek scan: %w", err)
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

// GetByID retrieves a single outbox record by ID.
func (s *OutboxStore) GetByID(ctx context.Context, id string) (*OutboxRecord, error) {
	var rec OutboxRecord
	var payload sql.NullString

	err := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, event_name, resource, payload, status, retry_count, max_retries, created_at, next_retry_at
		FROM forma_outbox WHERE id = ?
	`, id).Scan(&rec.ID, &rec.TenantID, &rec.EventName, &rec.Resource,
		&payload, &rec.Status, &rec.RetryCount, &rec.MaxRetries,
		&rec.CreatedAt, &rec.NextRetryAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("outbox get: %w", err)
	}
	if payload.Valid {
		rec.Payload = payload.String
	}
	return &rec, nil
}
