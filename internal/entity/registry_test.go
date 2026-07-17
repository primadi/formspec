package entity

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/forma/renderers/jsonbpersist"
)

func setupTestRegistry(t *testing.T, specRelPath string) (*Registry, db.DB) {
	t.Helper()
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "entity_test.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}

	// specRelPath is relative to this test file's directory
	reg := NewRegistry(d, db.DriverSQLite, specRelPath)
	return reg, d
}

// TestRegistry_LoadEntities verifies entity loading from a real spec directory.
func TestRegistry_LoadEntities(t *testing.T) {
	reg, d := setupTestRegistry(t, "registry_fixtures/customer/spec")
	defer d.Close()

	errs := reg.LoadEntities()
	for _, e := range errs {
		t.Logf("loading: %v", e)
	}

	if reg.Count() != 2 {
		t.Fatalf("expected 2 entities, got %d", reg.Count())
	}

	// Check specific entities
	cust, ok := reg.GetEntity("billing", "customer")
	if !ok {
		t.Fatal("expected billing/customer in registry")
	}
	if cust.Metadata.Name != "customer" {
		t.Errorf("expected name=customer, got %s", cust.Metadata.Name)
	}
	if cust.Metadata.Module != "billing" {
		t.Errorf("expected module=billing, got %s", cust.Metadata.Module)
	}

	addr, ok := reg.GetEntity("billing", "address")
	if !ok {
		t.Fatal("expected billing/address in registry")
	}
	if addr.Metadata.Description == "" {
		t.Error("expected non-empty description for address")
	}
}

// TestRegistry_LoadEntities_FiltersNonEntity verifies that only kind: Entity
// manifests are registered (Module, Config, Table, Form, etc. are skipped).
func TestRegistry_LoadEntities_FiltersNonEntity(t *testing.T) {
	reg, d := setupTestRegistry(t, "registry_fixtures/customer/spec")
	defer d.Close()

	errs := reg.LoadEntities()
	for _, e := range errs {
		t.Logf("loading: %v", e)
	}

	if reg.Count() != 2 {
		t.Fatalf("expected only 2 entities (customer, address), got %d", reg.Count())
	}
}

// TestRegistry_ListEntities verifies the ListEntities helper.
func TestRegistry_ListEntities(t *testing.T) {
	reg, d := setupTestRegistry(t, "registry_fixtures/customer/spec")
	defer d.Close()

	reg.LoadEntities()

	list := reg.ListEntities()
	if len(list) != 2 {
		t.Fatalf("expected 2 entities in list, got %d", len(list))
	}

	// Should be sorted by module, then name
	if list[0].Module != "billing" || list[0].Name != "address" {
		t.Errorf("expected first: billing/address, got %s/%s", list[0].Module, list[0].Name)
	}
	if list[1].Module != "billing" || list[1].Name != "customer" {
		t.Errorf("expected second: billing/customer, got %s/%s", list[1].Module, list[1].Name)
	}

	// Verify fields
	for _, info := range list {
		if info.Kind != "Document" {
			t.Errorf("expected kind=Document, got %s", info.Kind)
		}
		if info.Characteristic != "master" {
			t.Errorf("expected characteristic=master, got %s", info.Characteristic)
		}
		if info.Source == "" {
			t.Error("expected non-empty source")
		}
	}
}

// TestRegistry_GetEntitiesByCharacteristic verifies filtering.
func TestRegistry_GetEntitiesByCharacteristic(t *testing.T) {
	reg, d := setupTestRegistry(t, "registry_fixtures/customer/spec")
	defer d.Close()

	reg.LoadEntities()

	masters := reg.GetEntitiesByCharacteristic("master")
	if len(masters) != 2 {
		t.Fatalf("expected 2 master entities, got %d", len(masters))
	}

	transactions := reg.GetEntitiesByCharacteristic("transaction")
	if len(transactions) != 0 {
		t.Fatalf("expected 0 transaction entities, got %d", len(transactions))
	}
}

// TestRegistry_SyncSchema verifies that SyncSchema creates tables in the database.
func TestRegistry_SyncSchema(t *testing.T) {
	reg, d := setupTestRegistry(t, "registry_fixtures/customer/spec")
	defer d.Close()

	ctx := context.Background()

	reg.LoadEntities()

	// Before sync: should have no TableInfo
	for _, info := range reg.ListEntities() {
		specInfo, _ := reg.GetEntity(info.Module, info.Name)
		if specInfo.TableInfo != nil {
			t.Error("expected nil TableInfo before SyncSchema")
		}
	}

	// Sync schema
	applied, err := reg.SyncSchema(ctx)
	if err != nil {
		t.Fatalf("SyncSchema failed: %v", err)
	}
	if applied != 2 {
		t.Fatalf("expected 2 migrations applied, got %d", applied)
	}

	if !reg.IsSynced() {
		t.Error("expected IsSynced()=true after SyncSchema")
	}

	// After sync: TableInfo should be populated
	for _, info := range reg.ListEntities() {
		specInfo, _ := reg.GetEntity(info.Module, info.Name)
		if specInfo.TableInfo == nil {
			t.Errorf("expected TableInfo for %s/%s", info.Module, info.Name)
		}
		if !strings.Contains(specInfo.TableInfo.TableName, info.Module) {
			t.Errorf("table name should contain module, got %s", specInfo.TableInfo.TableName)
		}
	}

	// Tables should exist
	has, err := d.HasTable(ctx, "", "billing_customers")
	if err != nil {
		t.Fatalf("HasTable failed: %v", err)
	}
	if !has {
		t.Error("expected billing_customers table to exist")
	}

	hasAddr, _ := d.HasTable(ctx, "", "billing_addresses")
	if !hasAddr {
		t.Error("expected billing_addresses table to exist")
	}

	// Second sync should be idempotent
	applied2, err := reg.SyncSchema(ctx)
	if err != nil {
		t.Fatalf("SyncSchema #2 failed: %v", err)
	}
	if applied2 != 0 {
		t.Fatalf("expected 0 migrations on second sync, got %d", applied2)
	}
}

// TestRegistry_GetEntityStore verifies that GetEntityStore returns a working store.
func TestRegistry_GetEntityStore(t *testing.T) {
	reg, d := setupTestRegistry(t, "registry_fixtures/customer/spec")
	defer d.Close()

	ctx := context.Background()

	reg.LoadEntities()
	reg.SyncSchema(ctx)

	// Get store for customer
	store, err := reg.GetEntityStore("billing", "customer")
	if err != nil {
		t.Fatalf("GetEntityStore failed: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}

	// Insert
	id, err := store.Insert(ctx, db.InsertParams{
		WorkspaceID: "t1", CreatedBy: "user",
		Data: map[string]any{"name": "Bob", "email": "bob@test.com"},
	})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty ID")
	}

	// GetByID
	rec, err := store.GetByID(ctx, db.GetByIDParams{WorkspaceID: "t1", ID: id})
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if rec.Data["name"] != "Bob" {
		t.Errorf("expected name=Bob, got %v", rec.Data["name"])
	}
	if rec.Data["email"] != "bob@test.com" {
		t.Errorf("expected email=bob@test.com, got %v", rec.Data["email"])
	}

	// Get same store again (cached)
	store2, err := reg.GetEntityStore("billing", "customer")
	if err != nil {
		t.Fatalf("GetEntityStore #2 failed: %v", err)
	}
	if store2 == nil {
		t.Fatal("expected non-nil cached store")
	}
}

// TestRegistry_GetEntityStore_NotFound verifies error for non-existent entity.
func TestRegistry_GetEntityStore_NotFound(t *testing.T) {
	reg, d := setupTestRegistry(t, "registry_fixtures/customer/spec")
	defer d.Close()

	_, err := reg.GetEntityStore("billing", "nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent entity")
	}
}

// TestRegistry_EmptySpecPath verifies loading from an empty directory.
func TestRegistry_EmptySpecPath(t *testing.T) {
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "empty.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	reg := NewRegistry(d, db.DriverSQLite, dir)
	errs := reg.LoadEntities()
	if len(errs) > 0 {
		t.Fatalf("expected no errors from empty spec, got %v", errs)
	}
	if reg.Count() != 0 {
		t.Fatalf("expected 0 entities, got %d", reg.Count())
	}
}

// TestRegistry_LoadEntities_FromGeneralLedger tests loading from a different
// example module with more entities.
func TestRegistry_LoadEntities_FromGeneralLedger(t *testing.T) {
	reg, d := setupTestRegistry(t, "../../verticals/gl/spec")
	defer d.Close()

	errs := reg.LoadEntities()
	for _, e := range errs {
		t.Logf("loading: %v", e)
	}

	if reg.Count() == 0 {
		t.Fatal("expected at least 1 entity from General-Ledger")
	}

	t.Logf("General-Ledger: %d entities registered", reg.Count())
	for _, info := range reg.ListEntities() {
		t.Logf("  %s/%s [%s] (%d fields)", info.Module, info.Name, info.Characteristic, info.FieldCount)
	}
}
