package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// DefaultIdempotencyTTL is the default TTL for idempotency keys (24 hours).
// Spec §11.3/§420: implementations MUST NOT hard-code the window.
// Production deployments SHOULD configure this via core.idempotency_retention.
const DefaultIdempotencyTTL = 24 * time.Hour

// IdempotencyStore manages idempotency keys for action deduplication.
//
// When a client sends a request with an idempotency key:
//  1. If the key EXISTS and status=completed → return cached response (replay)
//  2. If the key EXISTS and status=pending/failed → retry (previous attempt failed)
//  3. If the key does NOT exist → create new pending entry, execute, record result
//
// Keys expire after TTL (default DefaultIdempotencyTTL) and are automatically
// cleaned up. The TTL MUST be configurable via WithTTL; hard-coding is a spec
// violation per §11.3/§420.
type IdempotencyStore struct {
	db     DB
	driver DriverType
	ttl    time.Duration
}

// IdempotencyRecord represents a row in forma_idempotency_keys.
type IdempotencyRecord struct {
	WorkspaceID  string
	Action    string
	Key       string
	Status    string // pending | completed | failed
	Response  string // JSON response body
	ExpiresAt string
	CreatedAt string
}

// NewIdempotencyStore creates a new idempotency store with DefaultIdempotencyTTL.
// Call WithTTL to override — production deployments MUST configure this.
func NewIdempotencyStore(db DB, driver DriverType) *IdempotencyStore {
	return &IdempotencyStore{
		db:     db,
		driver: driver,
		ttl:    DefaultIdempotencyTTL,
	}
}

// WithTTL sets a custom TTL for idempotency keys.
func (s *IdempotencyStore) WithTTL(ttl time.Duration) *IdempotencyStore {
	s.ttl = ttl
	return s
}

// TryClaim attempts to claim an idempotency key for execution.
//
// Returns:
//   - claimed=true, record=nil      → key is new, caller should execute
//   - claimed=false, record!=nil    → existing completed result available (replay)
//   - claimed=false, record=nil, err!=nil → error
func (s *IdempotencyStore) TryClaim(ctx context.Context, workspaceID, action, key string) (claimed bool, existing *IdempotencyRecord, err error) {
	// 1. Check for existing completed key
	existing, err = s.getByPK(ctx, workspaceID, action, key)
	if err != nil {
		return false, nil, fmt.Errorf("idempotency check: %w", err)
	}

	if existing != nil {
		if existing.Status == "completed" {
			// Replay: return cached response
			return false, existing, nil
		}

		// Pending or failed: previous attempt didn't complete
		if existing.Status == "pending" || existing.Status == "failed" {
			// Check if expired
			if s.isExpired(existing) {
				// Expired: reset to pending and allow retry
				if err := s.updateStatus(ctx, workspaceID, action, key, "pending", ""); err != nil {
					return false, nil, fmt.Errorf("idempotency reset expired: %w", err)
				}
				return true, nil, nil
			}

			// Not expired: allow retry (caller can still execute)
			return true, nil, nil
		}

		return false, nil, fmt.Errorf("idempotency unknown status: %s", existing.Status)
	}

	// 2. Key not found: insert a new pending entry
	expiresAt := time.Now().UTC().Add(s.ttl)
	expiresStr := expiresAt.Format(time.RFC3339Nano)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO forma_idempotency_keys (tenant_id, action, key, status, expires_at)
		VALUES (?, ?, ?, 'pending', ?)
	`, workspaceID, action, key, expiresStr)
	if err != nil {
		return false, nil, fmt.Errorf("idempotency insert: %w", err)
	}

	return true, nil, nil
}

// RecordCompleted marks an idempotency key as completed with a response.
func (s *IdempotencyStore) RecordCompleted(ctx context.Context, workspaceID, action, key, response string) error {
	if err := s.updateStatus(ctx, workspaceID, action, key, "completed", response); err != nil {
		return fmt.Errorf("idempotency record completed: %w", err)
	}
	return nil
}

// RecordFailed marks an idempotency key as failed.
func (s *IdempotencyStore) RecordFailed(ctx context.Context, workspaceID, action, key, response string) error {
	if err := s.updateStatus(ctx, workspaceID, action, key, "failed", response); err != nil {
		return fmt.Errorf("idempotency record failed: %w", err)
	}
	return nil
}

// GetResult retrieves the result for a completed idempotency key.
// Returns nil if the key doesn't exist or isn't completed.
func (s *IdempotencyStore) GetResult(ctx context.Context, workspaceID, action, key string) (*IdempotencyRecord, error) {
	rec, err := s.getByPK(ctx, workspaceID, action, key)
	if err != nil {
		return nil, err
	}
	if rec == nil || rec.Status != "completed" {
		return nil, nil
	}
	return rec, nil
}

// CleanupExpired removes expired idempotency keys from the database.
// Returns the number of rows deleted.
func (s *IdempotencyStore) CleanupExpired(ctx context.Context) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM forma_idempotency_keys
		WHERE expires_at < ?
	`, now)
	if err != nil {
		return 0, fmt.Errorf("idempotency cleanup: %w", err)
	}
	return result.RowsAffected()
}

// --- internal helpers ---

func (s *IdempotencyStore) getByPK(ctx context.Context, workspaceID, action, key string) (*IdempotencyRecord, error) {
	var rec IdempotencyRecord
	var response sql.NullString

	err := s.db.QueryRowContext(ctx, `
		SELECT tenant_id, action, key, status, response, expires_at, created_at
		FROM forma_idempotency_keys
		WHERE tenant_id = ? AND action = ? AND key = ?
	`, workspaceID, action, key).Scan(
		&rec.WorkspaceID, &rec.Action, &rec.Key, &rec.Status,
		&response, &rec.ExpiresAt, &rec.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if response.Valid {
		rec.Response = response.String
	}
	return &rec, nil
}

func (s *IdempotencyStore) updateStatus(ctx context.Context, workspaceID, action, key, status, response string) error {
	if response != "" {
		_, err := s.db.ExecContext(ctx, `
			UPDATE forma_idempotency_keys
			SET status = ?, response = ?
			WHERE tenant_id = ? AND action = ? AND key = ?
		`, status, response, workspaceID, action, key)
		return err
	}

	_, err := s.db.ExecContext(ctx, `
		UPDATE forma_idempotency_keys
		SET status = ?
		WHERE tenant_id = ? AND action = ? AND key = ?
	`, status, workspaceID, action, key)
	return err
}

func (s *IdempotencyStore) isExpired(rec *IdempotencyRecord) bool {
	t, err := time.Parse(time.RFC3339Nano, rec.ExpiresAt)
	if err != nil {
		return true // can't parse → treat as expired
	}
	return time.Now().UTC().After(t)
}
