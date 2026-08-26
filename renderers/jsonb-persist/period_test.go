package db

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/primadi/formspec/pkg/spec"
)

// setupPeriodEnv creates a transaction entity with a period guard wired.
func setupPeriodEnv(t *testing.T, guard func(ctx context.Context, workspaceID, period string) (bool, error)) *EntityStore {
	t.Helper()
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "period.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	meta := spec.Metadata{Name: "journal-entry", Module: "gl"}
	entity := &spec.EntitySpec{
		Version:        "v1",
		Characteristic: spec.CharTransaction,
		Fields: []spec.Field{
			{Name: "transaction_date", Type: spec.FieldDate},
			{Name: "amount", Type: spec.FieldDecimal},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)
	store.SetPeriodGuard(guard)
	return store
}

func TestPeriodGuard_ClosedPeriodRejected(t *testing.T) {
	// The current month is closed.
	now := time.Now()
	closedPeriod := now.Format("2006-01")
	store := setupPeriodEnv(t, func(_ context.Context, _, period string) (bool, error) {
		return period == closedPeriod, nil
	})
	ctx := context.Background()

	// Insert today (in the closed current month) → rejected.
	_, err := store.Insert(ctx, InsertParams{
		WorkspaceID: "tenant-1",
		CreatedBy:   "user-1",
		Data:        map[string]any{"transaction_date": now.Format("2006-01-02"), "amount": 100},
	})
	if err == nil {
		t.Fatal("expected insert in closed period to fail")
	}
	if !strings.Contains(err.Error(), "FORMSPEC.PERIOD.CLOSED") {
		t.Errorf("expected FORMSPEC.PERIOD.CLOSED, got: %v", err)
	}
}

func TestPeriodGuard_OpenPeriodAllowed(t *testing.T) {
	// A period OTHER than the current month is closed; today (current month)
	// is open.
	now := time.Now()
	closedPeriod := now.AddDate(0, -1, 0).Format("2006-01") // last month closed
	store := setupPeriodEnv(t, func(_ context.Context, _, period string) (bool, error) {
		return period == closedPeriod, nil
	})
	ctx := context.Background()

	id, err := store.Insert(ctx, InsertParams{
		WorkspaceID: "tenant-1",
		CreatedBy:   "user-1",
		Data:        map[string]any{"transaction_date": now.Format("2006-01-02"), "amount": 100},
	})
	if err != nil {
		t.Fatalf("insert in open period: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty id")
	}
}

func TestPeriodGuard_NoGuardDisabled(t *testing.T) {
	// No guard wired → period guard disabled.
	store := setupPeriodEnv(t, nil)
	ctx := context.Background()

	_, err := store.Insert(ctx, InsertParams{
		WorkspaceID: "tenant-1",
		CreatedBy:   "user-1",
		Data:        map[string]any{"transaction_date": time.Now().Format("2006-01-02"), "amount": 100},
	})
	if err != nil {
		t.Fatalf("insert with no guard should succeed: %v", err)
	}
}

func TestPeriodGuard_NonTransactionIgnored(t *testing.T) {
	// A non-transaction entity (master) is not subject to the period guard.
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "period_master.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	meta := spec.Metadata{Name: "product", Module: "inv"}
	entity := &spec.EntitySpec{
		Version:        "v1",
		Characteristic: spec.CharMaster,
		Fields:         []spec.Field{{Name: "name", Type: spec.FieldString}},
	}
	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatal(err)
	}
	store := NewEntityStore(d, DriverSQLite, meta, entity)
	store.SetPeriodGuard(func(_ context.Context, _, _ string) (bool, error) { return true, nil })

	_, err = store.Insert(ctx, InsertParams{
		WorkspaceID: "tenant-1",
		CreatedBy:   "user-1",
		Data:        map[string]any{"name": "widget"},
	})
	if err != nil {
		t.Fatalf("master insert should ignore period guard: %v", err)
	}
}