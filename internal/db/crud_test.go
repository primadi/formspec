package db

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/forma/forma/pkg/spec"
)

func TestEntityStore_InsertAndGetByID(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_insert.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	// Create table via migration
	meta := spec.Metadata{Name: "customer", Module: "billing"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "name", Type: spec.FieldString},
			{Name: "email", Type: spec.FieldString, Unique: true},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	// Insert
	id, err := store.Insert(ctx, InsertParams{
		TenantID:  "tenant-1",
		CreatedBy: "user-1",
		Data:      map[string]any{"name": "John Doe", "email": "john@example.com"},
	})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty ID")
	}

	// GetByID
	rec, err := store.GetByID(ctx, GetByIDParams{TenantID: "tenant-1", ID: id})
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if rec.Version != 1 {
		t.Errorf("expected version 1, got %d", rec.Version)
	}
}

func TestEntityStore_Update(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_update.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "product", Module: "inventory"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "name", Type: spec.FieldString},
			{Name: "price", Type: spec.FieldNumber},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	// Insert
	id, err := store.Insert(ctx, InsertParams{
		TenantID:  "t1",
		CreatedBy: "u1",
		Data:      map[string]any{"name": "Widget", "price": 9.99},
	})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Get to capture version
	rec, err := store.GetByID(ctx, GetByIDParams{TenantID: "t1", ID: id})
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	// Update
	newVersion, err := store.Update(ctx, UpdateParams{
		TenantID:  "t1",
		ID:        id,
		Version:   rec.Version,
		UpdatedBy: "u2",
		Data:      map[string]any{"name": "Widget Pro", "price": 19.99},
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if newVersion != rec.Version+1 {
		t.Errorf("expected version %d, got %d", rec.Version+1, newVersion)
	}
}

func TestEntityStore_Update_VersionConflict(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_conflict.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "item", Module: "test"}
	entity := &spec.EntitySpec{Version: "v1", Fields: []spec.Field{{Name: "name", Type: spec.FieldString}}}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	id, err := store.Insert(ctx, InsertParams{TenantID: "t1", CreatedBy: "u1", Data: map[string]any{"name": "test"}})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Update with wrong version → should fail
	_, err = store.Update(ctx, UpdateParams{
		TenantID: "t1", ID: id, Version: 999, UpdatedBy: "u2",
		Data: map[string]any{"name": "updated"},
	})
	if err == nil {
		t.Fatal("expected version conflict error")
	}
}

func TestEntityStore_SoftDelete(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_delete.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "customer", Module: "billing"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields:  []spec.Field{{Name: "name", Type: spec.FieldString}},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	id, err := store.Insert(ctx, InsertParams{TenantID: "t1", CreatedBy: "u1", Data: map[string]any{"name": "John"}})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Soft delete
	if err := store.SoftDelete(ctx, "t1", id); err != nil {
		t.Fatalf("SoftDelete failed: %v", err)
	}

	// Should not be found after delete
	_, err = store.GetByID(ctx, GetByIDParams{TenantID: "t1", ID: id})
	if err == nil || err.Error() != "not found" {
		t.Fatalf("expected not found after delete, got: %v", err)
	}
}

func TestEntityStore_List(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_list.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "order", Module: "billing"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "total", Type: spec.FieldNumber},
			{Name: "status", Type: spec.FieldEnum, EnumValues: []string{"draft", "paid"}},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	// Insert multiple records
	for i := 0; i < 5; i++ {
		_, err := store.Insert(ctx, InsertParams{
			TenantID: "t1", CreatedBy: "u1",
			Data: map[string]any{"total": float64(100 + i*10), "status": "draft"},
		})
		if err != nil {
			t.Fatalf("Insert %d failed: %v", i, err)
		}
	}

	// List with pagination
	result, err := store.List(ctx, ListParams{
		TenantID: "t1",
		Page:     1,
		PerPage:  3,
	})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if result.Total != 5 {
		t.Errorf("expected total 5, got %d", result.Total)
	}
	if len(result.Data) != 3 {
		t.Errorf("expected 3 items, got %d", len(result.Data))
	}
	if result.TotalPages != 2 {
		t.Errorf("expected 2 pages, got %d", result.TotalPages)
	}
}

func TestEntityStore_List_FilterByField(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_filter.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "customer", Module: "billing"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "name", Type: spec.FieldString, Index: true},
			{Name: "tier", Type: spec.FieldEnum, EnumValues: []string{"regular", "gold"}},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	// Insert records with different tiers
	for _, name := range []string{"Alice", "Bob", "Charlie"} {
		_, err := store.Insert(ctx, InsertParams{
			TenantID: "t1", CreatedBy: "u1",
			Data: map[string]any{"name": name, "tier": "regular"},
		})
		if err != nil {
			t.Fatalf("Insert %s failed: %v", name, err)
		}
	}
	_, err = store.Insert(ctx, InsertParams{
		TenantID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "Diana", "tier": "gold"},
	})
	if err != nil {
		t.Fatalf("Insert Diana failed: %v", err)
	}

	// Filter by generated column
	result, err := store.List(ctx, ListParams{
		TenantID: "t1",
		Filters:  map[string]FilterOp{"name": {Op: "eq", Value: "Alice"}},
	})
	if err != nil {
		t.Fatalf("List with filter failed: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("expected 1 result with filter, got %d", result.Total)
	}
}

func TestEntityStore_FindByField(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_findby.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "customer", Module: "billing"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "email", Type: spec.FieldString, Unique: true},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	id, err := store.Insert(ctx, InsertParams{
		TenantID: "t1", CreatedBy: "user-1",
		Data: map[string]any{"email": "test@example.com"},
	})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Find by email (unique field → generated column)
	rec, err := store.FindByField(ctx, "t1", "email", "test@example.com")
	if err != nil {
		t.Fatalf("FindByField failed: %v", err)
	}
	if rec.ID != id {
		t.Errorf("expected ID %s, got %s", id, rec.ID)
	}
}

func TestEntityStore_TenantIsolation(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_tenant.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "customer", Module: "billing"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields:  []spec.Field{{Name: "name", Type: spec.FieldString}},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	// Insert for tenant-a
	idA, err := store.Insert(ctx, InsertParams{TenantID: "tenant-a", CreatedBy: "u1", Data: map[string]any{"name": "Alice"}})
	if err != nil {
		t.Fatalf("Insert tenant-a failed: %v", err)
	}

	// Insert for tenant-b
	_, err = store.Insert(ctx, InsertParams{TenantID: "tenant-b", CreatedBy: "u1", Data: map[string]any{"name": "Bob"}})
	if err != nil {
		t.Fatalf("Insert tenant-b failed: %v", err)
	}

	// Tenant-a should see only their record
	result, err := store.List(ctx, ListParams{TenantID: "tenant-a"})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("expected tenant-a to see 1 record, got %d", result.Total)
	}

	// Tenant-b should NOT see tenant-a's record
	_, err = store.GetByID(ctx, GetByIDParams{TenantID: "tenant-b", ID: idA})
	if err == nil || err.Error() != "not found" {
		t.Errorf("expected tenant-b to not find tenant-a's record, got %v", err)
	}
}

func TestEntityStore_DefaultValue(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_default.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "product", Module: "inventory"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "name", Type: spec.FieldString},
			{Name: "price", Type: spec.FieldNumber, Default: float64(0.99)},
			{Name: "active", Type: spec.FieldBoolean, Default: true},
			{Name: "tier", Type: spec.FieldString, Default: "standard"},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	// Insert without any optional fields — defaults should apply
	id, err := store.Insert(ctx, InsertParams{
		TenantID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "Widget"},
	})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	rec, err := store.GetByID(ctx, GetByIDParams{TenantID: "t1", ID: id})
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if rec.Data["price"] != float64(0.99) {
		t.Errorf("expected price default 0.99, got %v", rec.Data["price"])
	}
	if rec.Data["active"] != true {
		t.Errorf("expected active default true, got %v", rec.Data["active"])
	}
	if rec.Data["tier"] != "standard" {
		t.Errorf("expected tier default 'standard', got %v", rec.Data["tier"])
	}

	// Insert with explicit values should NOT use defaults
	id2, err := store.Insert(ctx, InsertParams{
		TenantID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "Widget Pro", "price": float64(49.99), "active": false, "tier": "premium"},
	})
	if err != nil {
		t.Fatalf("Insert with explicit values failed: %v", err)
	}

	rec2, err := store.GetByID(ctx, GetByIDParams{TenantID: "t1", ID: id2})
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if rec2.Data["price"] != float64(49.99) {
		t.Errorf("expected price 49.99, got %v", rec2.Data["price"])
	}
	if rec2.Data["active"] != false {
		t.Errorf("expected active false, got %v", rec2.Data["active"])
	}
	if rec2.Data["tier"] != "premium" {
		t.Errorf("expected tier 'premium', got %v", rec2.Data["tier"])
	}
}

func TestEntityStore_RequiredField(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_required.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "customer", Module: "billing"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "name", Type: spec.FieldString, Required: true},
			{Name: "email", Type: spec.FieldString, Required: true},
			{Name: "notes", Type: spec.FieldString},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	// Insert without required field → should fail
	_, err = store.Insert(ctx, InsertParams{
		TenantID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "John"}, // missing email
	})
	if err == nil {
		t.Fatal("expected error for missing required field")
	}
	if !strings.Contains(err.Error(), "required field missing") {
		t.Errorf("expected 'required field missing' error, got: %v", err)
	}

	// Insert with all required fields → should succeed
	id, err := store.Insert(ctx, InsertParams{
		TenantID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "John", "email": "john@example.com"},
	})
	if err != nil {
		t.Fatalf("Insert with all required fields failed: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty ID")
	}
}

func TestEntityStore_ImmutableField(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_immutable.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "order", Module: "billing"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "number", Type: spec.FieldString, Immutable: true},
			{Name: "status", Type: spec.FieldString},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	id, err := store.Insert(ctx, InsertParams{
		TenantID: "t1", CreatedBy: "u1",
		Data: map[string]any{"number": "ORD-001", "status": "draft"},
	})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	rec, err := store.GetByID(ctx, GetByIDParams{TenantID: "t1", ID: id})
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	// Try to update immutable field → should fail
	_, err = store.Update(ctx, UpdateParams{
		TenantID: "t1", ID: id, Version: rec.Version, UpdatedBy: "u2",
		Data: map[string]any{"number": "ORD-002"},
	})
	if err == nil {
		t.Fatal("expected error for changing immutable field")
	}
	if !strings.Contains(err.Error(), "immutable field cannot be changed") {
		t.Errorf("expected 'immutable field cannot be changed' error, got: %v", err)
	}

	// Update non-immutable field → should succeed
	newVersion, err := store.Update(ctx, UpdateParams{
		TenantID: "t1", ID: id, Version: rec.Version, UpdatedBy: "u2",
		Data: map[string]any{"status": "paid"},
	})
	if err != nil {
		t.Fatalf("Update mutable field failed: %v", err)
	}
	if newVersion != rec.Version+1 {
		t.Errorf("expected version %d, got %d", rec.Version+1, newVersion)
	}
}

func TestEntityStore_StateMachineTransition(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_sm.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "order", Module: "billing"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "status", Type: spec.FieldString},
		},
		StateMachine: &spec.StateMachine{
			Field:   "status",
			Initial: "draft",
			States: []spec.StateDecl{
				{Name: "draft"},
				{Name: "submitted"},
				{Name: "approved"},
				{Name: "rejected"},
			},
			Transitions: []spec.TransitionDecl{
				{From: spec.StateList{"draft"}, To: "submitted", Action: "submit"},
				{From: spec.StateList{"submitted"}, To: "approved", Action: "approve"},
				{From: spec.StateList{"submitted"}, To: "rejected", Action: "reject"},
			},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	id, err := store.Insert(ctx, InsertParams{
		TenantID: "t1", CreatedBy: "u1",
		Data: map[string]any{"status": "draft"},
	})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	rec, err := store.GetByID(ctx, GetByIDParams{TenantID: "t1", ID: id})
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	// Valid transition: draft → submitted
	_, err = store.Update(ctx, UpdateParams{
		TenantID: "t1", ID: id, Version: rec.Version, UpdatedBy: "u2",
		Data: map[string]any{"status": "submitted"},
	})
	if err != nil {
		t.Fatalf("Valid transition draft→submitted failed: %v", err)
	}

	// Invalid transition: submitted → draft (not allowed)
	rec, err = store.GetByID(ctx, GetByIDParams{TenantID: "t1", ID: id})
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	_, err = store.Update(ctx, UpdateParams{
		TenantID: "t1", ID: id, Version: rec.Version, UpdatedBy: "u3",
		Data: map[string]any{"status": "draft"},
	})
	if err == nil {
		t.Fatal("expected error for invalid state transition")
	}
	if !strings.Contains(err.Error(), "invalid state transition") {
		t.Errorf("expected 'invalid state transition' error, got: %v", err)
	}
}

func TestEntityStore_StateMachineGuard_Passes(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_sm_guard_pass.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "order", Module: "billing"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "status", Type: spec.FieldString},
			{Name: "total", Type: spec.FieldNumber},
		},
		StateMachine: &spec.StateMachine{
			Field:   "status",
			Initial: "draft",
			States: []spec.StateDecl{
				{Name: "draft"},
				{Name: "submitted"},
			},
			Transitions: []spec.TransitionDecl{
				{
					From: spec.StateList{"draft"}, To: "submitted", Action: "submit",
					Guard: &spec.GuardDecl{
						Expression: "total > 0",
						Message:    "Order total must be positive before submitting",
					},
				},
			},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	id, err := store.Insert(ctx, InsertParams{
		TenantID: "t1", CreatedBy: "u1",
		Data: map[string]any{"status": "draft", "total": float64(100)},
	})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	rec, err := store.GetByID(ctx, GetByIDParams{TenantID: "t1", ID: id})
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	// Guard should pass: total > 0
	_, err = store.Update(ctx, UpdateParams{
		TenantID: "t1", ID: id, Version: rec.Version, UpdatedBy: "u2",
		Data: map[string]any{"status": "submitted"},
	})
	if err != nil {
		t.Fatalf("Guard should have passed, got: %v", err)
	}
}

func TestEntityStore_StateMachineGuard_Rejects(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_sm_guard_reject.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "order", Module: "billing"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "status", Type: spec.FieldString},
			{Name: "total", Type: spec.FieldNumber},
		},
		StateMachine: &spec.StateMachine{
			Field:   "status",
			Initial: "draft",
			States: []spec.StateDecl{
				{Name: "draft"},
				{Name: "submitted"},
			},
			Transitions: []spec.TransitionDecl{
				{
					From: spec.StateList{"draft"}, To: "submitted", Action: "submit",
					Guard: &spec.GuardDecl{
						Expression: "total > 0",
						Message:    "Order total must be positive before submitting",
					},
				},
			},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	id, err := store.Insert(ctx, InsertParams{
		TenantID: "t1", CreatedBy: "u1",
		Data: map[string]any{"status": "draft", "total": float64(0)},
	})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	rec, err := store.GetByID(ctx, GetByIDParams{TenantID: "t1", ID: id})
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	// Guard should reject: total is 0, not > 0
	_, err = store.Update(ctx, UpdateParams{
		TenantID: "t1", ID: id, Version: rec.Version, UpdatedBy: "u2",
		Data: map[string]any{"status": "submitted"},
	})
	if err == nil {
		t.Fatal("expected guard to reject transition")
	}
	if !strings.Contains(err.Error(), "Order total must be positive") {
		t.Errorf("expected custom guard message, got: %v", err)
	}
}

func TestEntityStore_FieldRules_Email(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_rules_email.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "customer", Module: "billing"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "name", Type: spec.FieldString},
			{
				Name: "email", Type: spec.FieldString,
				Rules: []spec.ValidationRule{{Name: "email"}},
			},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	// Invalid email → should fail
	_, err = store.Insert(ctx, InsertParams{
		TenantID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "John", "email": "not-an-email"},
	})
	if err == nil {
		t.Fatal("expected error for invalid email")
	}
	if !strings.Contains(err.Error(), "invalid email format") {
		t.Errorf("expected 'invalid email format' error, got: %v", err)
	}

	// Valid email → should succeed
	id, err := store.Insert(ctx, InsertParams{
		TenantID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "John", "email": "john@example.com"},
	})
	if err != nil {
		t.Fatalf("Insert with valid email failed: %v", err)
	}

	// Update with invalid email → should fail
	rec, err := store.GetByID(ctx, GetByIDParams{TenantID: "t1", ID: id})
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	_, err = store.Update(ctx, UpdateParams{
		TenantID: "t1", ID: id, Version: rec.Version, UpdatedBy: "u2",
		Data: map[string]any{"email": "bad"},
	})
	if err == nil {
		t.Fatal("expected error for update with invalid email")
	}
}

func TestEntityStore_FieldRules_MinMax(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_rules_minmax.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "product", Module: "inventory"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "name", Type: spec.FieldString},
			{
				Name: "price", Type: spec.FieldNumber,
				Rules: []spec.ValidationRule{
					{Name: "positive"},
					{Name: "min", Value: float64(0.01)},
					{Name: "max", Value: float64(99999.99)},
				},
			},
			{
				Name: "sku", Type: spec.FieldString,
				Rules: []spec.ValidationRule{
					{Name: "min_length", Value: 3},
					{Name: "max_length", Value: 20},
				},
			},
			{
				Name: "code", Type: spec.FieldString,
				Rules: []spec.ValidationRule{
					{Name: "pattern", Value: "^[A-Z]{3}-[0-9]{4}$"},
				},
			},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	// Negative price → fail
	_, err = store.Insert(ctx, InsertParams{
		TenantID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "Widget", "price": float64(-5), "sku": "ABC123", "code": "ABC-0001"},
	})
	if err == nil {
		t.Fatal("expected error for negative price")
	}

	// SKU too short → fail
	_, err = store.Insert(ctx, InsertParams{
		TenantID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "Widget", "price": float64(10), "sku": "AB", "code": "ABC-0001"},
	})
	if err == nil {
		t.Fatal("expected error for too-short SKU")
	}

	// Invalid code pattern → fail
	_, err = store.Insert(ctx, InsertParams{
		TenantID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "Widget", "price": float64(10), "sku": "ABC123", "code": "invalid"},
	})
	if err == nil {
		t.Fatal("expected error for invalid code pattern")
	}

	// All valid → succeed
	id, err := store.Insert(ctx, InsertParams{
		TenantID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "Widget", "price": float64(19.99), "sku": "ABC123", "code": "ABC-0001"},
	})
	if err != nil {
		t.Fatalf("Insert with valid data failed: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty ID")
	}
}

func TestEntityStore_ComputedField(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_computed.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "invoice", Module: "billing"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "subtotal", Type: spec.FieldNumber},
			{Name: "tax_rate", Type: spec.FieldNumber},
			{
				Name: "total",
				Type: spec.FieldNumber,
				Computed: &spec.ComputedDecl{
					Formula: "subtotal * (1 + tax_rate / 100)",
				},
			},
			{
				Name: "display",
				Type: spec.FieldString,
				Computed: &spec.ComputedDecl{
					Formula: `"$" + str(subtotal)`,
				},
			},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	id, err := store.Insert(ctx, InsertParams{
		TenantID: "t1", CreatedBy: "u1",
		Data: map[string]any{"subtotal": float64(200), "tax_rate": float64(10)},
	})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// GetByID should evaluate computed fields
	rec, err := store.GetByID(ctx, GetByIDParams{TenantID: "t1", ID: id})
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	// total = 200 * (1 + 10/100) = 200 * 1.1 = 220
	total, ok := rec.Data["total"].(float64)
	if !ok {
		t.Fatalf("expected float64 total, got %T: %v", rec.Data["total"], rec.Data["total"])
	}
	if total < 219 || total > 221 {
		t.Errorf("expected total ~220, got %v", total)
	}

	// List should also evaluate computed fields
	result, err := store.List(ctx, ListParams{TenantID: "t1"})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(result.Data) != 1 {
		t.Fatalf("expected 1 record, got %d", len(result.Data))
	}
	total2, ok := result.Data[0].Data["total"].(float64)
	if !ok {
		t.Fatalf("expected float64 total in list, got %T", result.Data[0].Data["total"])
	}
	if total2 < 219 || total2 > 221 {
		t.Errorf("expected total ~220 in list, got %v", total2)
	}
}

// ============================================================================
// 1.6 New Validator Tests
// ============================================================================

func TestEntityStore_FieldRule_Positive(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_rules_positive.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "product", Module: "inventory"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "name", Type: spec.FieldString},
			{Name: "price", Type: spec.FieldNumber, Rules: []spec.ValidationRule{{Name: "positive"}}},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	// Zero price → should fail
	_, err = store.Insert(ctx, InsertParams{
		TenantID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "Item", "price": float64(0)},
	})
	if err == nil {
		t.Fatal("expected error for zero price (positive rule)")
	}

	// Negative price → should fail
	_, err = store.Insert(ctx, InsertParams{
		TenantID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "Item", "price": float64(-5)},
	})
	if err == nil {
		t.Fatal("expected error for negative price (positive rule)")
	}

	// Positive price → should succeed
	id, err := store.Insert(ctx, InsertParams{
		TenantID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "Item", "price": float64(9.99)},
	})
	if err != nil {
		t.Fatalf("Insert with positive price failed: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty ID")
	}
	t.Log("✓ positive validator works")
}

func TestEntityStore_FieldRule_URL(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_rules_url.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "link", Module: "content"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "name", Type: spec.FieldString},
			{Name: "website", Type: spec.FieldString, Rules: []spec.ValidationRule{{Name: "url"}}},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	// Invalid URL → should fail
	_, err = store.Insert(ctx, InsertParams{
		TenantID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "Test", "website": "not-a-url"},
	})
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}

	// Valid HTTP URL → should succeed
	_, err = store.Insert(ctx, InsertParams{
		TenantID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "Test", "website": "https://example.com"},
	})
	if err != nil {
		t.Fatalf("Insert with valid URL failed: %v", err)
	}
	t.Log("✓ url validator works")
}

func TestEntityStore_FieldRule_Precision(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_rules_precision.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "invoice", Module: "billing"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "number", Type: spec.FieldString},
			{Name: "amount", Type: spec.FieldNumber, Rules: []spec.ValidationRule{{Name: "precision", Value: 2}}},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	// Too many decimal places → should fail
	_, err = store.Insert(ctx, InsertParams{
		TenantID: "t1", CreatedBy: "u1",
		Data: map[string]any{"number": "INV-001", "amount": float64(100.123)},
	})
	if err == nil {
		t.Fatal("expected error for too many decimal places")
	}

	// Exactly 2 decimal places → should succeed
	_, err = store.Insert(ctx, InsertParams{
		TenantID: "t1", CreatedBy: "u1",
		Data: map[string]any{"number": "INV-002", "amount": float64(100.12)},
	})
	if err != nil {
		t.Fatalf("Insert with 2 decimal places failed: %v", err)
	}

	// Integer (0 decimal places) → should succeed
	_, err = store.Insert(ctx, InsertParams{
		TenantID: "t1", CreatedBy: "u1",
		Data: map[string]any{"number": "INV-003", "amount": float64(100)},
	})
	if err != nil {
		t.Fatalf("Insert with integer value failed: %v", err)
	}
	t.Log("✓ precision validator works")
}

func TestEntityStore_FieldRule_Future(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_rules_future.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "event", Module: "calendar"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "title", Type: spec.FieldString},
			{Name: "date", Type: spec.FieldDateTime, Rules: []spec.ValidationRule{{Name: "future"}}},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	// Past date → should fail
	_, err = store.Insert(ctx, InsertParams{
		TenantID: "t1", CreatedBy: "u1",
		Data: map[string]any{"title": "Past Event", "date": "2020-01-01T00:00:00Z"},
	})
	if err == nil {
		t.Fatal("expected error for past date (future rule)")
	}

	// Future date → should succeed
	_, err = store.Insert(ctx, InsertParams{
		TenantID: "t1", CreatedBy: "u1",
		Data: map[string]any{"title": "Future Event", "date": "2030-01-01T00:00:00Z"},
	})
	if err != nil {
		t.Fatalf("Insert with future date failed: %v", err)
	}
	t.Log("✓ future validator works")
}

func TestEntityStore_FieldRule_MinMaxItems(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_rules_items.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "config", Module: "core"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "name", Type: spec.FieldString},
			{Name: "tags", Type: spec.FieldJSON,
				Rules: []spec.ValidationRule{
					{Name: "min_items", Value: 1},
					{Name: "max_items", Value: 5},
				},
			},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	// Empty array → should fail (min_items: 1)
	_, err = store.Insert(ctx, InsertParams{
		TenantID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "Test", "tags": []any{}},
	})
	if err == nil {
		t.Fatal("expected error for empty array (min_items)")
	}

	// 6 items → should fail (max_items: 5)
	_, err = store.Insert(ctx, InsertParams{
		TenantID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "Test", "tags": []any{"a", "b", "c", "d", "e", "f"}},
	})
	if err == nil {
		t.Fatal("expected error for too many items (max_items)")
	}

	// 2 items → should succeed
	_, err = store.Insert(ctx, InsertParams{
		TenantID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "Test", "tags": []any{"a", "b"}},
	})
	if err != nil {
		t.Fatalf("Insert with valid array failed: %v", err)
	}
	t.Log("✓ min_items/max_items validators work")
}
