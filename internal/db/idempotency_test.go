package db

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestIdempotencyStore_TryClaim_NewKey(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "idem_new.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	store := NewIdempotencyStore(d, DriverSQLite)

	// New key → should be claimed
	claimed, existing, err := store.TryClaim(ctx, "t1", "checkout", "key-001")
	if err != nil {
		t.Fatalf("TryClaim failed: %v", err)
	}
	if !claimed {
		t.Fatal("expected claimed=true for new key")
	}
	if existing != nil {
		t.Fatal("expected existing=nil for new key")
	}
}

func TestIdempotencyStore_RecordAndReplay(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "idem_replay.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	store := NewIdempotencyStore(d, DriverSQLite)

	// Claim
	claimed, _, err := store.TryClaim(ctx, "t1", "checkout", "key-001")
	if err != nil {
		t.Fatalf("TryClaim #1 failed: %v", err)
	}
	if !claimed {
		t.Fatal("expected claimed=true")
	}

	// Record completed
	if err := store.RecordCompleted(ctx, "t1", "checkout", "key-001", `{"order_id":"ORD-001"}`); err != nil {
		t.Fatalf("RecordCompleted failed: %v", err)
	}

	// Second claim → should replay
	claimed2, existing2, err := store.TryClaim(ctx, "t1", "checkout", "key-001")
	if err != nil {
		t.Fatalf("TryClaim #2 failed: %v", err)
	}
	if claimed2 {
		t.Fatal("expected claimed=false for existing completed key")
	}
	if existing2 == nil {
		t.Fatal("expected existing record for replay")
	}
	if existing2.Status != "completed" {
		t.Errorf("expected status=completed, got %s", existing2.Status)
	}
	if existing2.Response != `{"order_id":"ORD-001"}` {
		t.Errorf("expected response, got %q", existing2.Response)
	}
}

func TestIdempotencyStore_RetryPending(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "idem_retry.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	store := NewIdempotencyStore(d, DriverSQLite)

	// Claim
	store.TryClaim(ctx, "t1", "checkout", "key-001")

	// Claim again before recording → pending exists, should allow retry
	claimed, existing, err := store.TryClaim(ctx, "t1", "checkout", "key-001")
	if err != nil {
		t.Fatalf("TryClaim retry failed: %v", err)
	}
	if !claimed {
		t.Fatal("expected claimed=true for retry of pending key")
	}
	if existing != nil {
		t.Fatal("expected existing=nil for pending retry")
	}
}

func TestIdempotencyStore_RecordFailed(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "idem_fail.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	store := NewIdempotencyStore(d, DriverSQLite)

	// Claim
	store.TryClaim(ctx, "t1", "checkout", "key-001")

	// Record failed
	if err := store.RecordFailed(ctx, "t1", "checkout", "key-001", `{"error":"timeout"}`); err != nil {
		t.Fatalf("RecordFailed failed: %v", err)
	}

	// Claim again after failure → should allow retry (claimed=true)
	claimed, existing, err := store.TryClaim(ctx, "t1", "checkout", "key-001")
	if err != nil {
		t.Fatalf("TryClaim after failure failed: %v", err)
	}
	if !claimed {
		t.Fatal("expected claimed=true for retry after failure")
	}
	if existing != nil {
		t.Fatal("expected existing=nil for failed retry")
	}
}

func TestIdempotencyStore_GetResult(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "idem_result.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	store := NewIdempotencyStore(d, DriverSQLite)

	// GetResult for non-existent key → nil
	result, err := store.GetResult(ctx, "t1", "checkout", "key-001")
	if err != nil {
		t.Fatalf("GetResult non-existent failed: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil for non-existent key")
	}

	// Claim + complete
	store.TryClaim(ctx, "t1", "checkout", "key-001")
	store.RecordCompleted(ctx, "t1", "checkout", "key-001", `{"ok":true}`)

	// GetResult → should return record
	result2, err := store.GetResult(ctx, "t1", "checkout", "key-001")
	if err != nil {
		t.Fatalf("GetResult existing failed: %v", err)
	}
	if result2 == nil {
		t.Fatal("expected record for completed key")
	}
	if result2.Response != `{"ok":true}` {
		t.Errorf("expected response, got %q", result2.Response)
	}
}

func TestIdempotencyStore_TenantIsolation(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "idem_tenant.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	store := NewIdempotencyStore(d, DriverSQLite)

	// Tenant A completes key-001
	store.TryClaim(ctx, "ta", "checkout", "key-001")
	store.RecordCompleted(ctx, "ta", "checkout", "key-001", `{"a":true}`)

	// Tenant B should get a fresh claim
	claimed, _, err := store.TryClaim(ctx, "tb", "checkout", "key-001")
	if err != nil {
		t.Fatalf("TryClaim tenant B failed: %v", err)
	}
	if !claimed {
		t.Fatal("expected claimed=true for different tenant")
	}
}

func TestIdempotencyStore_CleanupExpired(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "idem_clean.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	// Use a store with very short TTL
	store := NewIdempotencyStore(d, DriverSQLite).WithTTL(1 * time.Millisecond)

	// Claim and complete
	store.TryClaim(ctx, "t1", "checkout", "key-001")
	store.RecordCompleted(ctx, "t1", "checkout", "key-001", `{"done":true}`)

	// Wait for expiry
	time.Sleep(50 * time.Millisecond)

	// Cleanup
	deleted, err := store.CleanupExpired(ctx)
	if err != nil {
		t.Fatalf("CleanupExpired failed: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}

	// Key should be gone → new claim possible
	claimed, _, err := store.TryClaim(ctx, "t1", "checkout", "key-001")
	if err != nil {
		t.Fatalf("TryClaim after expiry failed: %v", err)
	}
	if !claimed {
		t.Fatal("expected claimed=true after expiry and cleanup")
	}
}

func TestIdempotencyStore_RecordCompleted_DifferentAction(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "idem_action.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	store := NewIdempotencyStore(d, DriverSQLite)

	// Same key, different actions → independent
	store.TryClaim(ctx, "t1", "checkout", "shared-key")
	store.RecordCompleted(ctx, "t1", "checkout", "shared-key", `{"checkout":true}`)

	claimed, _, _ := store.TryClaim(ctx, "t1", "refund", "shared-key")
	if !claimed {
		t.Fatal("expected claimed=true for different action with same key")
	}
}
