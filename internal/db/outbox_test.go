package db

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestOutboxStore_Enqueue(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "outbox_enq.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	store := NewOutboxStore(d, DriverSQLite)

	id, err := store.Enqueue(ctx, "t1", "order.created", "order", `{"id":"ORD-001"}`)
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty ID")
	}

	// GetByID
	rec, err := store.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if rec == nil {
		t.Fatal("expected record")
	}
	if rec.EventName != "order.created" {
		t.Errorf("expected event_name=order.created, got %s", rec.EventName)
	}
	if rec.Status != "pending" {
		t.Errorf("expected status=pending, got %s", rec.Status)
	}
	if rec.TenantID != "t1" {
		t.Errorf("expected tenant_id=t1, got %s", rec.TenantID)
	}
}

func TestOutboxStore_Dequeue(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "outbox_deq.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	store := NewOutboxStore(d, DriverSQLite)

	// Enqueue 3 events
	store.Enqueue(ctx, "t1", "event.a", "order", `{"a":1}`)
	store.Enqueue(ctx, "t1", "event.b", "order", `{"b":2}`)
	store.Enqueue(ctx, "t1", "event.c", "order", `{"c":3}`)

	// Dequeue 2
	records, err := store.Dequeue(ctx, 2)
	if err != nil {
		t.Fatalf("Dequeue failed: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0].EventName != "event.a" {
		t.Errorf("expected first event.a, got %s", records[0].EventName)
	}
	if records[1].EventName != "event.b" {
		t.Errorf("expected second event.b, got %s", records[1].EventName)
	}

	// Remaining should be 1
	records2, _ := store.Dequeue(ctx, 10)
	if len(records2) != 1 {
		t.Fatalf("expected 1 remaining, got %d", len(records2))
	}
	if records2[0].EventName != "event.c" {
		t.Errorf("expected remaining event.c, got %s", records2[0].EventName)
	}
}

func TestOutboxStore_MarkCompleted(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "outbox_done.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	store := NewOutboxStore(d, DriverSQLite)

	id, _ := store.Enqueue(ctx, "t1", "event.x", "order", `{"x":1}`)

	// Dequeue + complete
	records, _ := store.Dequeue(ctx, 10)
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	if err := store.MarkCompleted(ctx, records[0].ID); err != nil {
		t.Fatalf("MarkCompleted failed: %v", err)
	}

	rec, _ := store.GetByID(ctx, id)
	if rec.Status != "completed" {
		t.Errorf("expected status=completed, got %s", rec.Status)
	}
}

func TestOutboxStore_MarkFailed_Retries(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "outbox_retry.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	store := NewOutboxStore(d, DriverSQLite)

	id, _ := store.Enqueue(ctx, "t1", "event.y", "order", `{"y":1}`)

	// First failure → retry
	if err := store.MarkFailed(ctx, id, 3); err != nil {
		t.Fatalf("MarkFailed #1 failed: %v", err)
	}

	rec1, _ := store.GetByID(ctx, id)
	if rec1.Status != "pending" {
		t.Errorf("expected status=pending after retry, got %s", rec1.Status)
	}
	if rec1.RetryCount != 1 {
		t.Errorf("expected retry_count=1, got %d", rec1.RetryCount)
	}

	// Fail again
	store.MarkFailed(ctx, id, 3)
	store.MarkFailed(ctx, id, 3)
	store.MarkFailed(ctx, id, 3)

	// 4th failure → max retries exceeded → failed permanently
	rec2, _ := store.GetByID(ctx, id)
	if rec2.Status != "failed" {
		t.Errorf("expected status=failed after max retries, got %s", rec2.Status)
	}
	if rec2.RetryCount != 4 {
		t.Errorf("expected retry_count=4, got %d", rec2.RetryCount)
	}
}

func TestOutboxStore_MarkFailed_ExponentialBackoff(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "outbox_backoff.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	store := NewOutboxStore(d, DriverSQLite)

	id, _ := store.Enqueue(ctx, "t1", "event.z", "order", `{"z":1}`)

	// Fail with specific retry counts and check backoff
	for i := 1; i <= 5; i++ {
		store.MarkFailed(ctx, id, 10)
		rec, _ := store.GetByID(ctx, id)

		if rec.RetryCount != i {
			t.Errorf("iteration %d: expected retry_count=%d, got %d", i, i, rec.RetryCount)
		}
	}
}

func TestOutboxStore_Peek(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "outbox_peek.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	store := NewOutboxStore(d, DriverSQLite)

	store.Enqueue(ctx, "t1", "a", "order", `{}`)
	store.Enqueue(ctx, "t1", "b", "order", `{}`)
	store.Enqueue(ctx, "t1", "c", "order", `{}`)

	records, err := store.Peek(ctx, 2)
	if err != nil {
		t.Fatalf("Peek failed: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records from peek, got %d", len(records))
	}
	// Peek returns newest first (DESC)
	if records[0].EventName != "c" || records[1].EventName != "b" {
		t.Errorf("expected newest first, got %s, %s", records[0].EventName, records[1].EventName)
	}
}

func TestOutboxStore_CountByStatus(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "outbox_count.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	store := NewOutboxStore(d, DriverSQLite)

	store.Enqueue(ctx, "t1", "a", "order", `{}`)
	store.Enqueue(ctx, "t1", "b", "order", `{}`)

	// Dequeue + complete one (other stays "delivering")
	records, _ := store.Dequeue(ctx, 10)
	store.MarkCompleted(ctx, records[0].ID)

	counts, err := store.CountByStatus(ctx)
	if err != nil {
		t.Fatalf("CountByStatus failed: %v", err)
	}

	if counts["completed"] != 1 {
		t.Errorf("expected 1 completed, got %d", counts["completed"])
	}
	if counts["delivering"] != 1 {
		t.Errorf("expected 1 delivering, got %d", counts["delivering"])
	}
}

func TestOutboxStore_Cleanup(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "outbox_clean.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	store := NewOutboxStore(d, DriverSQLite)

	// Enqueue + complete
	id, _ := store.Enqueue(ctx, "t1", "old", "order", `{}`)
	records, _ := store.Dequeue(ctx, 10)
	store.MarkCompleted(ctx, records[0].ID)

	// Cleanup with zero duration → should delete everything older than now
	deleted, err := store.Cleanup(ctx, 1*time.Nanosecond)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}

	rec, _ := store.GetByID(ctx, id)
	if rec != nil {
		t.Error("expected record to be deleted")
	}
}

func TestOutboxStore_EnqueueMultipleTenants(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "outbox_mt.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	store := NewOutboxStore(d, DriverSQLite)

	store.Enqueue(ctx, "ta", "event", "order", `{}`)
	store.Enqueue(ctx, "tb", "event", "order", `{}`)

	records, _ := store.Dequeue(ctx, 10)
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
}

func TestOutboxStore_Dequeue_Empty(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "outbox_empty.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	store := NewOutboxStore(d, DriverSQLite)

	records, err := store.Dequeue(ctx, 10)
	if err != nil {
		t.Fatalf("Dequeue empty failed: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected 0 records, got %d", len(records))
	}
}
