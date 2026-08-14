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

// OutboxRecord represents a row in formspec_outbox.
type OutboxRecord struct {
	ID             string
	WorkspaceID    string
	EventName      string
	Resource       string
	Payload        string
	Status         string // pending | delivering | completed | failed
	RetryCount     int
	MaxRetries     int
	Backoff        string // exponential | linear | fixed (2.4.4)
	InitialDelayMs int    // ms before first retry (2.4.4)
	CreatedAt      string
	NextRetryAt    string
}

// NewOutboxStore creates a new outbox store.
func NewOutboxStore(db DB, driver DriverType) *OutboxStore {
	return &OutboxStore{db: db, driver: driver}
}

// Enqueue inserts a new event into the outbox for processing.
func (s *OutboxStore) Enqueue(ctx context.Context, workspaceID, eventName, resource, payload string) (string, error) {
	return enqueueOutbox(ctx, s.db, workspaceID, eventName, resource, payload)
}

// EnqueueOutboxTx enqueues a durable event directly onto an already-open
// transaction's DB (e.g. a TxScope's txdb) — used by HandleCustomAction to
// enqueue atomically alongside whatever mutation the action performed,
// instead of going through a fresh OutboxStore bound to the base
// connection. Delegates to the same enqueueOutbox helper OutboxStore uses.
func EnqueueOutboxTx(ctx context.Context, txdb DB, workspaceID, eventName, resource, payload string) (string, error) {
	return enqueueOutbox(ctx, txdb, workspaceID, eventName, resource, payload)
}

// enqueueOutboxParams holds optional parameters for enqueueing an outbox event.
type enqueueOutboxParams struct {
	Backoff        string // exponential | linear | fixed (default: exponential)
	InitialDelayMs int    // ms before first retry (default: 1000)
}

// enqueueOutbox performs the outbox insert against the given DB — a plain
// connection, or a transaction-bound DB (see InTx in tx.go). EntityStore
// uses the latter form to enqueue a durable event atomically alongside the
// entity mutation that produced it (InsertParams/UpdateParams.PendingEvents),
// closing the gap described in
// docs/renderers/jsonb-persist/01-architecture.md §3 where a crash between
// mutation commit and outbox enqueue silently drops the event.
func enqueueOutbox(ctx context.Context, database DB, workspaceID, eventName, resource, payload string) (string, error) {
	return enqueueOutboxWithParams(ctx, database, workspaceID, eventName, resource, payload, enqueueOutboxParams{})
}

// enqueueOutboxParams is the same as enqueueOutbox but with optional retry params.
func enqueueOutboxWithParams(ctx context.Context, database DB, workspaceID, eventName, resource, payload string, params enqueueOutboxParams) (string, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)

	backoff := params.Backoff
	if backoff == "" {
		backoff = "exponential"
	}
	initialDelayMs := params.InitialDelayMs
	if initialDelayMs <= 0 {
		initialDelayMs = 1000
	}

	query := `
		INSERT INTO formspec_outbox (tenant_id, event_name, resource, payload, status, backoff, initial_delay_ms, created_at, next_retry_at)
		VALUES (?, ?, ?, ?, 'pending', ?, ?, ?, ?)
		RETURNING id
	`

	var id string
	err := database.QueryRowContext(ctx, query,
		workspaceID, eventName, resource, payload, backoff, initialDelayMs, now, now,
	).Scan(&id)
	if err != nil {
		// SQLite may not support RETURNING
		return enqueueOutboxFallback(ctx, database, workspaceID, eventName, resource, payload, now, backoff, initialDelayMs)
	}
	return id, nil
}

func enqueueOutboxFallback(ctx context.Context, database DB, workspaceID, eventName, resource, payload, now, backoff string, initialDelayMs int) (string, error) {
	_, err := database.ExecContext(ctx, `
		INSERT INTO formspec_outbox (tenant_id, event_name, resource, payload, status, backoff, initial_delay_ms, created_at, next_retry_at)
		VALUES (?, ?, ?, ?, 'pending', ?, ?, ?, ?)
	`, workspaceID, eventName, resource, payload, backoff, initialDelayMs, now, now)
	if err != nil {
		return "", fmt.Errorf("outbox enqueue: %w", err)
	}

	// Retrieve the last inserted ID
	var id string
	err = database.QueryRowContext(ctx, `SELECT last_insert_rowid()`).Scan(&id)
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
		SELECT id, tenant_id, event_name, resource, payload, status, retry_count, max_retries, backoff, initial_delay_ms, created_at, next_retry_at
		FROM formspec_outbox
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
		if err := rows.Scan(&rec.ID, &rec.WorkspaceID, &rec.EventName, &rec.Resource,
			&rec.Payload, &rec.Status, &rec.RetryCount, &rec.MaxRetries,
			&rec.Backoff, &rec.InitialDelayMs,
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
			UPDATE formspec_outbox SET status = 'delivering' WHERE id = ? AND status = 'pending'
		`, rec.ID); err != nil {
			return nil, fmt.Errorf("outbox claim %s: %w", rec.ID, err)
		}
	}

	return records, nil
}

// MarkCompleted marks an outbox record as successfully delivered.
func (s *OutboxStore) MarkCompleted(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE formspec_outbox SET status = 'completed' WHERE id = ?
	`, id)
	return err
}

// MarkFailed marks an outbox record as failed and schedules a retry.
// Uses the record's backoff strategy (exponential|linear|fixed) and
// initial_delay_ms for scheduling the next attempt (2.4.4).
func (s *OutboxStore) MarkFailed(ctx context.Context, id string, maxRetries int) error {
	// Get current retry count, backoff strategy, and initial delay
	var retryCount int
	var currentStatus, backoff string
	var initialDelayMs int
	err := s.db.QueryRowContext(ctx, `
		SELECT retry_count, status, backoff, initial_delay_ms FROM formspec_outbox WHERE id = ?
	`, id).Scan(&retryCount, &currentStatus, &backoff, &initialDelayMs)
	if err != nil {
		return fmt.Errorf("outbox mark failed get: %w", err)
	}

	newRetryCount := retryCount + 1

	if newRetryCount > maxRetries {
		// Max retries exceeded → mark as failed permanently
		_, err = s.db.ExecContext(ctx, `
			UPDATE formspec_outbox SET status = 'failed', retry_count = ? WHERE id = ?
		`, newRetryCount, id)
		return err
	}

	// Calculate backoff delay: initial_delay_ms is the base for the first retry
	var delayMs float64
	switch backoff {
	case "linear":
		delayMs = float64(initialDelayMs) * float64(newRetryCount)
	case "fixed":
		delayMs = float64(initialDelayMs)
	default: // exponential
		delayMs = float64(initialDelayMs) * math.Pow(2, float64(newRetryCount-1))
	}

	// Cap at 1 hour
	delayMs = math.Min(delayMs, 3600000)

	nextRetry := time.Now().UTC().Add(time.Duration(delayMs) * time.Millisecond).Format(time.RFC3339Nano)

	_, err = s.db.ExecContext(ctx, `
		UPDATE formspec_outbox SET status = 'pending', retry_count = ?, next_retry_at = ? WHERE id = ?
	`, newRetryCount, nextRetry, id)
	return err
}

// CountByStatus returns the count of outbox records grouped by status.
func (s *OutboxStore) CountByStatus(ctx context.Context) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT status, COUNT(*) as cnt FROM formspec_outbox GROUP BY status
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
		DELETE FROM formspec_outbox WHERE status = 'completed' AND created_at < ?
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
		FROM formspec_outbox
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
		if err := rows.Scan(&rec.ID, &rec.WorkspaceID, &rec.EventName, &rec.Resource,
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
		FROM formspec_outbox WHERE id = ?
	`, id).Scan(&rec.ID, &rec.WorkspaceID, &rec.EventName, &rec.Resource,
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
