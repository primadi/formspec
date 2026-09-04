package db

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// newLinkTestStore opens an in-memory-shape SQLite with system tables.
func newLinkTestStore(t *testing.T) (*StorageLinkStore, context.Context) {
	t.Helper()
	d, err := OpenSQLite(filepath.Join(t.TempDir(), "link.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	r := NewMigrationRunner(d, DriverSQLite)
	if err := r.EnsureSystemTables(context.Background()); err != nil {
		t.Fatalf("EnsureSystemTables: %v", err)
	}
	return NewStorageLinkStore(d, DriverSQLite), context.Background()
}

// TestLinkIssueConsumeBudget verifies the atomic download budget: a 1x link
// is consumable exactly once, then reports exhausted.
func TestLinkIssueConsumeBudget(t *testing.T) {
	store, ctx := newLinkTestStore(t)

	link, err := store.Issue(ctx, "t1", "t1/billing/document/1/attachment/a.pdf",
		15*time.Minute, 1, true, false)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if link.Token == "" || link.Status != "active" {
		t.Fatalf("unexpected link: %+v", link)
	}

	// First consume succeeds, budget exhausted (one_time → delete flag).
	row, deleteNow, err := store.Consume(ctx, link.Token)
	if err != nil {
		t.Fatalf("consume 1: %v", err)
	}
	if row.Path != link.Path || row.DownloadCount != 1 {
		t.Fatalf("consume 1: unexpected row %+v", row)
	}
	if !deleteNow {
		t.Fatalf("consume 1: expected deleteNow=true for one_time link")
	}

	// Second consume fails — budget exhausted.
	if _, _, err := store.Consume(ctx, link.Token); err == nil {
		t.Fatalf("consume 2: expected error for exhausted link")
	}
}

// TestLinkConsumeUnlimited verifies max_downloads=0 links never exhaust.
func TestLinkConsumeUnlimited(t *testing.T) {
	store, ctx := newLinkTestStore(t)

	link, err := store.Issue(ctx, "t1", "t1/x/y", time.Minute, 0, false, false)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	for i := 0; i < 3; i++ {
		if _, deleteNow, err := store.Consume(ctx, link.Token); err != nil || deleteNow {
			t.Fatalf("consume %d: err=%v deleteNow=%v (want nil/false)", i+1, err, deleteNow)
		}
	}
}

// TestLinkConsumeExpired verifies an expired link cannot be consumed.
func TestLinkConsumeExpired(t *testing.T) {
	store, ctx := newLinkTestStore(t)

	link, err := store.Issue(ctx, "t1", "t1/x/y", -time.Minute, 0, false, false)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, _, err := store.Consume(ctx, link.Token); err == nil {
		t.Fatalf("consume: expected error for expired link")
	}
}

// TestLinkSweepAndPurge verifies the sweeper flips expired rows and purges
// old consumed rows.
func TestLinkSweepAndPurge(t *testing.T) {
	store, ctx := newLinkTestStore(t)

	// Expired + untouched (ttl flow).
	if _, err := store.Issue(ctx, "t1", "t1/ttl/a", -time.Minute, 0, false, true); err != nil {
		t.Fatalf("issue ttl: %v", err)
	}
	// Expired but downloaded: an already-expired link cannot be consumed —
	// the consume guard requires expires_at > now — so the sweeper's
	// DownloadedAt filter only has the untouched row to screen here.
	// Consumed long ago → purge candidate.
	old, err := store.Issue(ctx, "t1", "t1/old", time.Minute, 1, true, false)
	if err != nil {
		t.Fatalf("issue old: %v", err)
	}
	if _, _, err := store.Consume(ctx, old.Token); err != nil {
		t.Fatalf("consume old: %v", err)
	}

	expired, err := store.ListExpired(ctx, time.Now().UTC(), 100)
	if err != nil {
		t.Fatalf("list expired: %v", err)
	}
	if len(expired) != 1 {
		t.Fatalf("list expired: want 1 row, got %d", len(expired))
	}
	var untouched []string
	for _, row := range expired {
		if row.DeleteIfUntouched && row.DownloadedAt == "" {
			untouched = append(untouched, row.Path)
		}
	}
	if len(untouched) != 1 || untouched[0] != "t1/ttl/a" {
		t.Fatalf("untouched: want [t1/ttl/a], got %v", untouched)
	}

	if n, err := store.SweepExpired(ctx, time.Now().UTC()); err != nil || n != 1 {
		t.Fatalf("sweep: n=%d err=%v (want 1, nil)", n, err)
	}
	// Both consumed rows (ttl/a via sweep, old via consume) are purge
	// candidates — the test passes `now` as the cutoff.
	if n, err := store.PurgeOld(ctx, time.Now().UTC()); err != nil || n != 2 {
		t.Fatalf("purge: n=%d err=%v (want 2, nil)", n, err)
	}
}
