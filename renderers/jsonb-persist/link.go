package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

// LinkRow is a row in formspec_storage_link (plan: storage-links-plan.md
// Fase 2, todo 7.17.6). It backs the 1x-download (one_time) and TTL flows:
// the API issues a token, the consume route validates/increments it, and
// the sweeper deletes objects whose ttl passed without a download.
type LinkRow struct {
	Token             string // opaque random 256-bit hex (primary key)
	TenantID          string // workspace scope (tenant isolation)
	Path              string // object key in the storage backend
	ExpiresAt         string // RFC3339 — link validity
	MaxDownloads      int    // 0 = unlimited
	DownloadCount     int
	Status            string // active | consumed
	DeleteOnDownload  bool   // one_time: remove the object at max_downloads
	DeleteIfUntouched bool   // ttl: sweeper removes the object when expired
	DownloadedAt      string // RFC3339, empty when never downloaded
	CreatedAt         string // RFC3339
}

// StorageLinkStore persists storage link tokens in formspec_storage_link.
type StorageLinkStore struct {
	db     DB
	driver DriverType
}

// NewStorageLinkStore creates a new storage-link store.
func NewStorageLinkStore(db DB, driver DriverType) *StorageLinkStore {
	return &StorageLinkStore{db: db, driver: driver}
}

// newToken returns a random 256-bit hex token.
func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("link token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// Issue creates a link row and returns it. maxDownloads 0 means unlimited;
// deleteOnDownload marks the object for removal when the download budget is
// exhausted; deleteIfUntouched marks it for the TTL sweeper.
func (s *StorageLinkStore) Issue(ctx context.Context, tenantID, path string, ttl time.Duration, maxDownloads int, deleteOnDownload, deleteIfUntouched bool) (*LinkRow, error) {
	token, err := newToken()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	expires := now.Add(ttl)
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO formspec_storage_link
			(token, tenant_id, path, expires_at, max_downloads, download_count,
			 status, delete_on_download, delete_if_untouched, downloaded_at, created_at)
		VALUES (?, ?, ?, ?, ?, 0, 'active', ?, ?, '', ?)`,
		token, tenantID, path, expires.Format(time.RFC3339Nano),
		maxDownloads, boolInt(deleteOnDownload), boolInt(deleteIfUntouched),
		now.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, fmt.Errorf("link issue: %w", err)
	}
	return &LinkRow{
		Token:             token,
		TenantID:          tenantID,
		Path:              path,
		ExpiresAt:         expires.Format(time.RFC3339Nano),
		MaxDownloads:      maxDownloads,
		DownloadCount:     0,
		Status:            "active",
		DeleteOnDownload:  deleteOnDownload,
		DeleteIfUntouched: deleteIfUntouched,
		CreatedAt:         now.Format(time.RFC3339Nano),
	}, nil
}

// Consume atomically validates and increments a link's download counter.
// It returns the link row and whether the download budget is now exhausted
// (row flipped to consumed). Expired/unknown/consumed links return an error.
func (s *StorageLinkStore) Consume(ctx context.Context, token string) (*LinkRow, bool, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)

	// Atomic guard: only an active, unexpired link under its download
	// budget can be consumed — concurrent consumers serialize on the
	// UPDATE's WHERE clause, so downloads never exceed the budget.
	// max_downloads = 0 means unlimited.
	res, err := s.db.ExecContext(ctx, `
		UPDATE formspec_storage_link
		SET download_count = download_count + 1, downloaded_at = ?
		WHERE token = ? AND status = 'active'
		  AND (max_downloads <= 0 OR download_count < max_downloads)
		  AND expires_at > ?`,
		now, token, now,
	)
	if err != nil {
		return nil, false, fmt.Errorf("link consume: %w", err)
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		return nil, false, fmt.Errorf("link consume: link invalid, expired, or exhausted")
	}

	row, err := s.Get(ctx, token)
	if err != nil {
		return nil, false, err
	}
	if row == nil {
		return nil, false, fmt.Errorf("link consume: link vanished after consume")
	}

	exhausted := false
	if row.MaxDownloads > 0 && row.DownloadCount >= row.MaxDownloads {
		exhausted = true
		if _, err := s.db.ExecContext(ctx,
			`UPDATE formspec_storage_link SET status = 'consumed' WHERE token = ?`,
			token); err != nil {
			return row, true, fmt.Errorf("link consume: mark consumed: %w", err)
		}
	}
	return row, exhausted && row.DeleteOnDownload, nil
}

// Get returns a link row by token, or nil when absent.
func (s *StorageLinkStore) Get(ctx context.Context, token string) (*LinkRow, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT token, tenant_id, path, expires_at, max_downloads, download_count,
		       status, delete_on_download, delete_if_untouched, downloaded_at, created_at
		FROM formspec_storage_link
		WHERE token = ?`, token)
	var l LinkRow
	var delOn, delIf int
	var downloadedAt sql.NullString
	err := row.Scan(&l.Token, &l.TenantID, &l.Path, &l.ExpiresAt, &l.MaxDownloads,
		&l.DownloadCount, &l.Status, &delOn, &delIf, &downloadedAt, &l.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("link get: %w", err)
	}
	l.DeleteOnDownload = delOn == 1
	l.DeleteIfUntouched = delIf == 1
	l.DownloadedAt = downloadedAt.String
	return &l, nil
}

// ListExpired returns active links past their expiry (for the sweeper).
func (s *StorageLinkStore) ListExpired(ctx context.Context, now time.Time, limit int) ([]LinkRow, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT token, tenant_id, path, expires_at, max_downloads, download_count,
		       status, delete_on_download, delete_if_untouched, downloaded_at, created_at
		FROM formspec_storage_link
		WHERE status = 'active' AND expires_at <= ?
		LIMIT ?`, now.Format(time.RFC3339Nano), limit)
	if err != nil {
		return nil, fmt.Errorf("link list expired: %w", err)
	}
	defer rows.Close()

	var out []LinkRow
	for rows.Next() {
		var l LinkRow
		var delOn, delIf int
		var downloadedAt sql.NullString
		if err := rows.Scan(&l.Token, &l.TenantID, &l.Path, &l.ExpiresAt, &l.MaxDownloads,
			&l.DownloadCount, &l.Status, &delOn, &delIf, &downloadedAt, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("link list expired scan: %w", err)
		}
		l.DeleteOnDownload = delOn == 1
		l.DeleteIfUntouched = delIf == 1
		l.DownloadedAt = downloadedAt.String
		out = append(out, l)
	}
	return out, rows.Err()
}

// SweepExpired marks expired active links as consumed. Rows flagged
// delete_if_untouched must have their objects removed by the caller
// (using the returned rows) BEFORE this flips them, so the sweeper can
// delete-then-flip in one pass.
func (s *StorageLinkStore) SweepExpired(ctx context.Context, now time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE formspec_storage_link
		SET status = 'consumed'
		WHERE status = 'active' AND expires_at <= ?`,
		now.Format(time.RFC3339Nano),
	)
	if err != nil {
		return 0, fmt.Errorf("link sweep: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// PurgeOld removes consumed/expired link rows older than olderThan.
func (s *StorageLinkStore) PurgeOld(ctx context.Context, olderThan time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM formspec_storage_link
		WHERE status = 'consumed' AND created_at <= ?`,
		olderThan.Format(time.RFC3339Nano),
	)
	if err != nil {
		return 0, fmt.Errorf("link purge: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
