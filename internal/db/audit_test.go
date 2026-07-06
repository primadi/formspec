package db

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditStore_WriteAndListByEntity(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "audit_entity.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	// Write audit logs directly
	err = writeAuditLog(ctx, d, DriverSQLite, "tenant-1", "billing/invoice", "inv-001", "create", "user-1", `{"total": 100}`)
	if err != nil {
		t.Fatalf("writeAuditLog failed: %v", err)
	}

	err = writeAuditLog(ctx, d, DriverSQLite, "tenant-1", "billing/invoice", "inv-001", "update", "user-2", `{"status": {"old": "draft", "new": "sent"}}`)
	if err != nil {
		t.Fatalf("writeAuditLog failed: %v", err)
	}

	err = writeAuditLog(ctx, d, DriverSQLite, "tenant-1", "billing/invoice", "inv-002", "create", "user-1", `{"total": 200}`)
	if err != nil {
		t.Fatalf("writeAuditLog failed: %v", err)
	}

	store := NewAuditStore(d, DriverSQLite)

	// List by entity
	records, err := store.ListByEntity(ctx, "tenant-1", "billing/invoice", "inv-001", 10, 0)
	if err != nil {
		t.Fatalf("ListByEntity failed: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("expected 2 records for inv-001, got %d", len(records))
	}

	// Verify ordering (most recent first)
	if records[0].Action != "update" {
		t.Errorf("expected first record to be 'update', got %q", records[0].Action)
	}
	if records[1].Action != "create" {
		t.Errorf("expected second record to be 'create', got %q", records[1].Action)
	}
}

func TestAuditStore_ListByTenant(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "audit_tenant.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	// Write audit logs for two tenants
	_ = writeAuditLog(ctx, d, DriverSQLite, "tenant-a", "billing/invoice", "inv-001", "create", "user-1", `{}`)
	_ = writeAuditLog(ctx, d, DriverSQLite, "tenant-b", "billing/invoice", "inv-002", "create", "user-2", `{}`)
	_ = writeAuditLog(ctx, d, DriverSQLite, "tenant-a", "billing/order", "ord-001", "create", "user-1", `{}`)

	store := NewAuditStore(d, DriverSQLite)

	// List all for tenant-a
	records, err := store.ListByTenant(ctx, "tenant-a", "", 10, 0)
	if err != nil {
		t.Fatalf("ListByTenant failed: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("expected 2 records for tenant-a, got %d", len(records))
	}

	// Filter by entity type
	records, err = store.ListByTenant(ctx, "tenant-a", "billing/invoice", 10, 0)
	if err != nil {
		t.Fatalf("ListByTenant with filter failed: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("expected 1 invoice record for tenant-a, got %d", len(records))
	}
}

func TestAuditStore_TenantIsolation(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "audit_isolation.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	_ = writeAuditLog(ctx, d, DriverSQLite, "tenant-a", "billing/invoice", "inv-001", "create", "u1", `{}`)
	_ = writeAuditLog(ctx, d, DriverSQLite, "tenant-b", "billing/invoice", "inv-002", "create", "u2", `{}`)

	store := NewAuditStore(d, DriverSQLite)

	// Tenant-b should only see their own records
	records, err := store.ListByTenant(ctx, "tenant-b", "", 10, 0)
	if err != nil {
		t.Fatalf("ListByTenant failed: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("expected 1 record for tenant-b, got %d", len(records))
	}
	if records[0].TenantID != "tenant-b" {
		t.Errorf("expected tenant-b record, got tenant %q", records[0].TenantID)
	}
}

func TestWriteAuditLog_ErrorOnNilDB(t *testing.T) {
	ctx := context.Background()

	// Should fail gracefully when DB is nil (using a closed DB)
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "audit_error.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	if err := NewMigrationRunner(d, DriverSQLite).EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}
	d.Close()

	err = writeAuditLog(ctx, d, DriverSQLite, "t1", "test/entity", "id-1", "create", "u1", `{}`)
	if err == nil {
		t.Fatal("expected error writing audit log to closed DB")
	}
	if !strings.Contains(err.Error(), "write audit log") {
		t.Errorf("expected 'write audit log' error, got: %v", err)
	}
}
