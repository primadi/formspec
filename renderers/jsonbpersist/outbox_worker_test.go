package db

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestOutboxWorker_StartStop(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "worker_startstop.db"), nil)
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
	handler := EventHandlerFunc(func(_ context.Context, _, _, _, _ string) error {
		return nil
	})

	worker := NewOutboxWorker(store, handler,
		WithPollInterval(100*time.Millisecond),
		WithBatchSize(5),
	)

	// Should not be running before Start
	if worker.IsRunning() {
		t.Fatal("worker should not be running before Start")
	}

	worker.Start(ctx)

	// Should be running after Start
	if !worker.IsRunning() {
		t.Fatal("worker should be running after Start")
	}

	// Start again should be a no-op
	worker.Start(ctx)

	// Stop
	worker.Stop()

	// Should not be running after Stop
	if worker.IsRunning() {
		t.Fatal("worker should not be running after Stop")
	}

	// Double stop should be safe
	worker.Stop()
}

func TestOutboxWorker_DeliversEvents(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "worker_deliver.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	outboxStore := NewOutboxStore(d, DriverSQLite)

	var delivered atomic.Int32
	handler := EventHandlerFunc(func(_ context.Context, _, eventName, _, _ string) error {
		delivered.Add(1)
		return nil
	})

	// Enqueue an event before starting worker
	_, err = outboxStore.Enqueue(ctx, "tenant-1", "order.created", "billing/order", `{"id": "ord-001"}`)
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	worker := NewOutboxWorker(outboxStore, handler,
		WithPollInterval(50*time.Millisecond),
		WithBatchSize(5),
	)

	worker.Start(ctx)
	defer worker.Stop()

	// Wait for delivery
	time.Sleep(300 * time.Millisecond)

	if n := delivered.Load(); n != 1 {
		t.Errorf("expected 1 delivered event, got %d", n)
	}
}

func TestOutboxWorker_RetryOnFailure(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "worker_retry.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	outboxStore := NewOutboxStore(d, DriverSQLite)

	var attemptCount atomic.Int32
	handler := EventHandlerFunc(func(_ context.Context, _, _, _, _ string) error {
		n := attemptCount.Add(1)
		if n == 1 {
			return context.DeadlineExceeded // simulated failure
		}
		return nil
	})

	_, err = outboxStore.Enqueue(ctx, "tenant-1", "order.created", "billing/order", `{"id": "ord-001"}`)
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	worker := NewOutboxWorker(outboxStore, handler,
		WithPollInterval(50*time.Millisecond),
		WithBatchSize(5),
		WithMaxRetries(3),
	)

	worker.Start(ctx)

	// Wait for first attempt to happen
	time.Sleep(300 * time.Millisecond)

	if n := attemptCount.Load(); n != 1 {
		t.Fatalf("expected 1 attempt so far, got %d", n)
	}

	// Manually reset next_retry_at to now so retry happens immediately
	// (bypass exponential backoff for test speed)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = d.ExecContext(ctx,
		"UPDATE forma_outbox SET next_retry_at = ? WHERE status = 'pending'", now)
	if err != nil {
		t.Fatalf("update next_retry_at failed: %v", err)
	}

	// Wait for retry to be picked up
	time.Sleep(200 * time.Millisecond)

	worker.Stop()

	if n := attemptCount.Load(); n != 2 {
		t.Errorf("expected 2 attempts (1 fail + 1 retry), got %d", n)
	}
}

func TestOutboxWorker_DoesNotProcessCompleted(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "worker_completed.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	outboxStore := NewOutboxStore(d, DriverSQLite)

	var delivered atomic.Int32
	handler := EventHandlerFunc(func(_ context.Context, _, _, _, _ string) error {
		delivered.Add(1)
		return nil
	})

	// Enqueue and manually mark as completed
	id, err := outboxStore.Enqueue(ctx, "tenant-1", "order.created", "billing/order", `{}`)
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}
	if err := outboxStore.MarkCompleted(ctx, id); err != nil {
		t.Fatalf("MarkCompleted failed: %v", err)
	}

	worker := NewOutboxWorker(outboxStore, handler,
		WithPollInterval(50*time.Millisecond),
	)

	worker.Start(ctx)
	defer worker.Stop()

	time.Sleep(200 * time.Millisecond)

	if n := delivered.Load(); n != 0 {
		t.Errorf("expected 0 deliveries for completed event, got %d", n)
	}
}
