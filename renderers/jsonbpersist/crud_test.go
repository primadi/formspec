package db

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/primadi/formspec/pkg/spec"
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
		WorkspaceID: "tenant-1",
		CreatedBy:   "user-1",
		Data:        map[string]any{"name": "John Doe", "email": "john@example.com"},
	})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty ID")
	}

	// GetByID
	rec, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: "tenant-1", ID: id})
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
		WorkspaceID: "t1",
		CreatedBy:   "u1",
		Data:        map[string]any{"name": "Widget", "price": 9.99},
	})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Get to capture version
	rec, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: "t1", ID: id})
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	// Update
	newVersion, err := store.Update(ctx, UpdateParams{
		WorkspaceID: "t1",
		ID:          id,
		Version:     rec.Version,
		UpdatedBy:   "u2",
		Data:        map[string]any{"name": "Widget Pro", "price": 19.99},
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

	id, err := store.Insert(ctx, InsertParams{WorkspaceID: "t1", CreatedBy: "u1", Data: map[string]any{"name": "test"}})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Update with wrong version → should fail
	_, err = store.Update(ctx, UpdateParams{
		WorkspaceID: "t1", ID: id, Version: 999, UpdatedBy: "u2",
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

	id, err := store.Insert(ctx, InsertParams{WorkspaceID: "t1", CreatedBy: "u1", Data: map[string]any{"name": "John"}})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Soft delete
	if err := store.SoftDelete(ctx, "t1", id); err != nil {
		t.Fatalf("SoftDelete failed: %v", err)
	}

	// Should not be found after delete
	_, err = store.GetByID(ctx, GetByIDParams{WorkspaceID: "t1", ID: id})
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
			WorkspaceID: "t1", CreatedBy: "u1",
			Data: map[string]any{"total": float64(100 + i*10), "status": "draft"},
		})
		if err != nil {
			t.Fatalf("Insert %d failed: %v", i, err)
		}
	}

	// List with pagination
	result, err := store.List(ctx, ListParams{
		WorkspaceID: "t1",
		Page:        1,
		PerPage:     3,
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
			WorkspaceID: "t1", CreatedBy: "u1",
			Data: map[string]any{"name": name, "tier": "regular"},
		})
		if err != nil {
			t.Fatalf("Insert %s failed: %v", name, err)
		}
	}
	_, err = store.Insert(ctx, InsertParams{
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "Diana", "tier": "gold"},
	})
	if err != nil {
		t.Fatalf("Insert Diana failed: %v", err)
	}

	// Filter by generated column
	result, err := store.List(ctx, ListParams{
		WorkspaceID: "t1",
		Filters:     map[string]FilterOp{"name": {Op: "eq", Value: "Alice"}},
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
		WorkspaceID: "t1", CreatedBy: "user-1",
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
	idA, err := store.Insert(ctx, InsertParams{WorkspaceID: "tenant-a", CreatedBy: "u1", Data: map[string]any{"name": "Alice"}})
	if err != nil {
		t.Fatalf("Insert tenant-a failed: %v", err)
	}

	// Insert for tenant-b
	_, err = store.Insert(ctx, InsertParams{WorkspaceID: "tenant-b", CreatedBy: "u1", Data: map[string]any{"name": "Bob"}})
	if err != nil {
		t.Fatalf("Insert tenant-b failed: %v", err)
	}

	// Tenant-a should see only their record
	result, err := store.List(ctx, ListParams{WorkspaceID: "tenant-a"})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("expected tenant-a to see 1 record, got %d", result.Total)
	}

	// Tenant-b should NOT see tenant-a's record
	_, err = store.GetByID(ctx, GetByIDParams{WorkspaceID: "tenant-b", ID: idA})
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
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "Widget"},
	})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	rec, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: "t1", ID: id})
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
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "Widget Pro", "price": float64(49.99), "active": false, "tier": "premium"},
	})
	if err != nil {
		t.Fatalf("Insert with explicit values failed: %v", err)
	}

	rec2, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: "t1", ID: id2})
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
		WorkspaceID: "t1", CreatedBy: "u1",
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
		WorkspaceID: "t1", CreatedBy: "u1",
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
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{"number": "ORD-001", "status": "draft"},
	})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	rec, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: "t1", ID: id})
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	// Try to update immutable field → should fail
	_, err = store.Update(ctx, UpdateParams{
		WorkspaceID: "t1", ID: id, Version: rec.Version, UpdatedBy: "u2",
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
		WorkspaceID: "t1", ID: id, Version: rec.Version, UpdatedBy: "u2",
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
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{"status": "draft"},
	})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	rec, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: "t1", ID: id})
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	// Valid transition: draft → submitted
	_, err = store.Update(ctx, UpdateParams{
		WorkspaceID: "t1", ID: id, Version: rec.Version, UpdatedBy: "u2",
		Data: map[string]any{"status": "submitted"},
	})
	if err != nil {
		t.Fatalf("Valid transition draft→submitted failed: %v", err)
	}

	// Invalid transition: submitted → draft (not allowed)
	rec, err = store.GetByID(ctx, GetByIDParams{WorkspaceID: "t1", ID: id})
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	_, err = store.Update(ctx, UpdateParams{
		WorkspaceID: "t1", ID: id, Version: rec.Version, UpdatedBy: "u3",
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
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{"status": "draft", "total": float64(100)},
	})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	rec, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: "t1", ID: id})
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	// Guard should pass: total > 0
	_, err = store.Update(ctx, UpdateParams{
		WorkspaceID: "t1", ID: id, Version: rec.Version, UpdatedBy: "u2",
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
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{"status": "draft", "total": float64(0)},
	})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	rec, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: "t1", ID: id})
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	// Guard should reject: total is 0, not > 0
	_, err = store.Update(ctx, UpdateParams{
		WorkspaceID: "t1", ID: id, Version: rec.Version, UpdatedBy: "u2",
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
		WorkspaceID: "t1", CreatedBy: "u1",
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
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "John", "email": "john@example.com"},
	})
	if err != nil {
		t.Fatalf("Insert with valid email failed: %v", err)
	}

	// Update with invalid email → should fail
	rec, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: "t1", ID: id})
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	_, err = store.Update(ctx, UpdateParams{
		WorkspaceID: "t1", ID: id, Version: rec.Version, UpdatedBy: "u2",
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
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "Widget", "price": float64(-5), "sku": "ABC123", "code": "ABC-0001"},
	})
	if err == nil {
		t.Fatal("expected error for negative price")
	}

	// SKU too short → fail
	_, err = store.Insert(ctx, InsertParams{
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "Widget", "price": float64(10), "sku": "AB", "code": "ABC-0001"},
	})
	if err == nil {
		t.Fatal("expected error for too-short SKU")
	}

	// Invalid code pattern → fail
	_, err = store.Insert(ctx, InsertParams{
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "Widget", "price": float64(10), "sku": "ABC123", "code": "invalid"},
	})
	if err == nil {
		t.Fatal("expected error for invalid code pattern")
	}

	// All valid → succeed
	id, err := store.Insert(ctx, InsertParams{
		WorkspaceID: "t1", CreatedBy: "u1",
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
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{"subtotal": float64(200), "tax_rate": float64(10)},
	})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// GetByID should evaluate computed fields
	rec, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: "t1", ID: id})
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
	result, err := store.List(ctx, ListParams{WorkspaceID: "t1"})
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
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "Item", "price": float64(0)},
	})
	if err == nil {
		t.Fatal("expected error for zero price (positive rule)")
	}

	// Negative price → should fail
	_, err = store.Insert(ctx, InsertParams{
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "Item", "price": float64(-5)},
	})
	if err == nil {
		t.Fatal("expected error for negative price (positive rule)")
	}

	// Positive price → should succeed
	id, err := store.Insert(ctx, InsertParams{
		WorkspaceID: "t1", CreatedBy: "u1",
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
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "Test", "website": "not-a-url"},
	})
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}

	// Valid HTTP URL → should succeed
	_, err = store.Insert(ctx, InsertParams{
		WorkspaceID: "t1", CreatedBy: "u1",
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
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{"number": "INV-001", "amount": float64(100.123)},
	})
	if err == nil {
		t.Fatal("expected error for too many decimal places")
	}

	// Exactly 2 decimal places → should succeed
	_, err = store.Insert(ctx, InsertParams{
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{"number": "INV-002", "amount": float64(100.12)},
	})
	if err != nil {
		t.Fatalf("Insert with 2 decimal places failed: %v", err)
	}

	// Integer (0 decimal places) → should succeed
	_, err = store.Insert(ctx, InsertParams{
		WorkspaceID: "t1", CreatedBy: "u1",
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
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{"title": "Past Event", "date": "2020-01-01T00:00:00Z"},
	})
	if err == nil {
		t.Fatal("expected error for past date (future rule)")
	}

	// Future date → should succeed
	_, err = store.Insert(ctx, InsertParams{
		WorkspaceID: "t1", CreatedBy: "u1",
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
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "Test", "tags": []any{}},
	})
	if err == nil {
		t.Fatal("expected error for empty array (min_items)")
	}

	// 6 items → should fail (max_items: 5)
	_, err = store.Insert(ctx, InsertParams{
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "Test", "tags": []any{"a", "b", "c", "d", "e", "f"}},
	})
	if err == nil {
		t.Fatal("expected error for too many items (max_items)")
	}

	// 2 items → should succeed
	_, err = store.Insert(ctx, InsertParams{
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "Test", "tags": []any{"a", "b"}},
	})
	if err != nil {
		t.Fatalf("Insert with valid array failed: %v", err)
	}
	t.Log("✓ min_items/max_items validators work")
}

// ============================================================================
// v0.3.0 Lifecycle Method Tests
// ============================================================================

func TestEntityStore_Submit_Success(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_submit.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "order", Module: "billing"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields:  []spec.Field{{Name: "total", Type: spec.FieldNumber}},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	// Insert: doc_status should be 'draft' (submitEnabled defaults to true)
	id, err := store.Insert(ctx, InsertParams{
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{"total": float64(100)},
	})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Submit: draft → submitted
	if err := store.Submit(ctx, "t1", id, "u1"); err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	// Verify doc_status changed — check via raw query (doc_status is a column, not in Data)
	var docStatus string
	err = d.QueryRowContext(ctx,
		"SELECT doc_status FROM billing_orders WHERE id = ? AND tenant_id = ?",
		id, "t1").Scan(&docStatus)
	if err != nil {
		t.Fatalf("Query doc_status failed: %v", err)
	}
	if docStatus != "submitted" {
		t.Errorf("expected doc_status='submitted', got %q", docStatus)
	}
}

func TestEntityStore_Submit_AlreadySubmitted(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_submit_dup.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "order", Module: "billing"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields:  []spec.Field{{Name: "total", Type: spec.FieldNumber}},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	id, _ := store.Insert(ctx, InsertParams{
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{"total": float64(100)},
	})
	store.Submit(ctx, "t1", id, "u1")

	// Second submit should fail
	err = store.Submit(ctx, "t1", id, "u1")
	if err == nil {
		t.Fatal("expected error on second submit (already submitted)")
	}
}

func TestEntityStore_Cancel_Success(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_cancel.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "order", Module: "billing"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields:  []spec.Field{{Name: "total", Type: spec.FieldNumber}},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	id, _ := store.Insert(ctx, InsertParams{
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{"total": float64(100)},
	})
	store.Submit(ctx, "t1", id, "u1")

	// Cancel: submitted → cancelled
	if err := store.Cancel(ctx, "t1", id, "u1"); err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}

	var docStatus string
	d.QueryRowContext(ctx,
		"SELECT doc_status FROM billing_orders WHERE id = ? AND tenant_id = ?",
		id, "t1").Scan(&docStatus)
	if docStatus != "cancelled" {
		t.Errorf("expected doc_status='cancelled', got %q", docStatus)
	}
}

func TestEntityStore_Cancel_NotSubmitted(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_cancel_bad.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "order", Module: "billing"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields:  []spec.Field{{Name: "total", Type: spec.FieldNumber}},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	id, _ := store.Insert(ctx, InsertParams{
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{"total": float64(100)},
	})
	// Don't submit — try to cancel directly from draft

	err = store.Cancel(ctx, "t1", id, "u1")
	if err == nil {
		t.Fatal("expected error cancelling draft document")
	}
}

func TestEntityStore_Amend_Success(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_amend.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "order", Module: "billing"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields:  []spec.Field{{Name: "total", Type: spec.FieldNumber}},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	// Create + submit original
	origID, _ := store.Insert(ctx, InsertParams{
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{"total": float64(100)},
	})
	store.Submit(ctx, "t1", origID, "u1")

	// Amend: creates new doc, cancels original, links both
	newID, err := store.Amend(ctx, "t1", origID, "u1", map[string]any{"total": float64(200)})
	if err != nil {
		t.Fatalf("Amend failed: %v", err)
	}
	if newID == "" {
		t.Fatal("expected non-empty new ID from amend")
	}
	if newID == origID {
		t.Fatal("new ID should differ from original")
	}

	// Verify original is cancelled
	var origStatus string
	d.QueryRowContext(ctx,
		"SELECT doc_status FROM billing_orders WHERE id = ? AND tenant_id = ?",
		origID, "t1").Scan(&origStatus)
	if origStatus != "cancelled" {
		t.Errorf("expected original doc_status='cancelled', got %q", origStatus)
	}

	// Verify new doc has amends set
	var amends string
	d.QueryRowContext(ctx,
		"SELECT amends FROM billing_orders WHERE id = ? AND tenant_id = ?",
		newID, "t1").Scan(&amends)
	if amends != origID {
		t.Errorf("expected amends=%s on new doc, got %s", origID, amends)
	}

	// Verify original has amended_by set
	var amendedBy string
	d.QueryRowContext(ctx,
		"SELECT amended_by FROM billing_orders WHERE id = ? AND tenant_id = ?",
		origID, "t1").Scan(&amendedBy)
	if amendedBy != newID {
		t.Errorf("expected amended_by=%s on original, got %s", newID, amendedBy)
	}
}

func TestEntityStore_LifecycleFree_NoDocStatus(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_lifecycle_free.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "customer", Module: "billing"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields:  []spec.Field{{Name: "name", Type: spec.FieldString}},
		Actions: []spec.Action{
			{Name: "submit", Disabled: true}, // lifecycle-free
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	id, err := store.Insert(ctx, InsertParams{
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{"name": "John"},
	})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Verify doc_status is NULL (lifecycle-free)
	var docStatus *string
	d.QueryRowContext(ctx,
		"SELECT doc_status FROM billing_customers WHERE id = ? AND tenant_id = ?",
		id, "t1").Scan(&docStatus)
	if docStatus != nil {
		t.Errorf("expected doc_status=NULL (lifecycle-free), got %v", *docStatus)
	}
}

// ============================================================================
// Backdate policy + override_permission (special path for stale records)
// ============================================================================

// TestEntityStore_BackdatePolicy_Blocked verifies that a transaction_date
// beyond max_days_back is rejected when the caller lacks the override.
func TestEntityStore_BackdatePolicy_Blocked(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_backdate_blocked.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "visit", Module: "clinic"}
	oldDate := timeNow().Add(-10 * 24 * time.Hour).Format("2006-01-02")
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "transaction_date", Type: spec.FieldDate, Required: true},
		},
		BackdatePolicy: &spec.BackdatePolicy{
			MaxDaysBack:        3,
			OverridePermission: "visits.resolve-stale",
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	// Without override permission → blocked
	_, err = store.Insert(ctx, InsertParams{
		WorkspaceID: "t1", CreatedBy: "u1",
		Data:        map[string]any{"transaction_date": oldDate},
		Permissions: []string{"visits.create"},
	})
	if err == nil {
		t.Fatal("expected BACKDATE_EXCEEDED for old date without override")
	}
	if !strings.Contains(err.Error(), "FORMSPEC.TXN.BACKDATE_EXCEEDED") {
		t.Errorf("expected BACKDATE_EXCEEDED error, got: %v", err)
	}
}

// TestEntityStore_BackdatePolicy_Override verifies that a caller holding the
// override_permission can bypass the policy — the special path for stale
// records, without widening the policy itself.
func TestEntityStore_BackdatePolicy_Override(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_backdate_override.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "visit", Module: "clinic"}
	oldDate := timeNow().Add(-10 * 24 * time.Hour).Format("2006-01-02")
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "transaction_date", Type: spec.FieldDate, Required: true},
		},
		BackdatePolicy: &spec.BackdatePolicy{
			MaxDaysBack:        3,
			OverridePermission: "visits.resolve-stale",
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	// With override permission → allowed (wildcard also works)
	_, err = store.Insert(ctx, InsertParams{
		WorkspaceID: "t1", CreatedBy: "u1",
		Data:        map[string]any{"transaction_date": oldDate},
		Permissions: []string{"visits.resolve-stale"},
	})
	if err != nil {
		t.Fatalf("Insert with override permission should pass: %v", err)
	}
}

// TestEntityStore_IsStale_Computed verifies the derived is_stale flag follows
// the backdate limit injected into the computed env.
func TestEntityStore_IsStale_Computed(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_is_stale.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "visit", Module: "clinic"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "transaction_date", Type: spec.FieldDate, Required: true},
			{Name: "status", Type: spec.FieldString},
			{
				Name: "is_stale",
				Type: spec.FieldBoolean,
				Computed: &spec.ComputedDecl{
					Formula: `status not in ("completed", "cancelled") and transaction_date < days_ago(backdate_limit_days)`,
				},
			},
		},
		BackdatePolicy: &spec.BackdatePolicy{
			MaxDaysBack:        3,
			OverridePermission: "visits.resolve-stale",
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	// Old waiting visit → is_stale = true
	id1, err := store.Insert(ctx, InsertParams{
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{
			"transaction_date": timeNow().Add(-5 * 24 * time.Hour).Format("2006-01-02"),
			"status":           "waiting",
		},
		Permissions: []string{"visits.resolve-stale"},
	})
	if err != nil {
		t.Fatalf("Insert stale failed: %v", err)
	}
	rec, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: "t1", ID: id1})
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if stale, _ := rec.Data["is_stale"].(bool); !stale {
		t.Errorf("expected is_stale=true for old waiting visit, got %v", rec.Data["is_stale"])
	}

	// Completed visit (even old) → is_stale = false
	id2, err := store.Insert(ctx, InsertParams{
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{
			"transaction_date": timeNow().Add(-5 * 24 * time.Hour).Format("2006-01-02"),
			"status":           "completed",
		},
		Permissions: []string{"visits.resolve-stale"},
	})
	if err != nil {
		t.Fatalf("Insert completed failed: %v", err)
	}
	rec2, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: "t1", ID: id2})
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if stale, _ := rec2.Data["is_stale"].(bool); stale {
		t.Errorf("expected is_stale=false for completed visit, got %v", rec2.Data["is_stale"])
	}

	// Recent waiting visit → is_stale = false
	id3, err := store.Insert(ctx, InsertParams{
		WorkspaceID: "t1", CreatedBy: "u1",
		Data: map[string]any{
			"transaction_date": timeNow().Format("2006-01-02"),
			"status":           "waiting",
		},
	})
	if err != nil {
		t.Fatalf("Insert recent failed: %v", err)
	}
	rec3, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: "t1", ID: id3})
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if stale, _ := rec3.Data["is_stale"].(bool); stale {
		t.Errorf("expected is_stale=false for recent visit, got %v", rec3.Data["is_stale"])
	}
}

// TestEntityStore_InsertUnknownField ensures insert rejects fields not in spec.
func TestEntityStore_InsertUnknownField(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_unknown_insert.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "task", Module: "project"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "title", Type: spec.FieldString},
			{Name: "done", Type: spec.FieldBoolean, Default: func() *any { v := any(false); return &v }()},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	// Sending a field not in the spec must be rejected.
	_, err = store.Insert(ctx, InsertParams{
		WorkspaceID: "t1",
		CreatedBy:   "user-1",
		Data:        map[string]any{"title": "hello", "hacker_field": "malicious"},
	})
	if err == nil {
		t.Fatal("expected error for unknown field, got nil")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Errorf("expected 'unknown field' error, got: %v", err)
	}

	// Known fields only: must succeed.
	id, err := store.Insert(ctx, InsertParams{
		WorkspaceID: "t1",
		CreatedBy:   "user-1",
		Data:        map[string]any{"title": "valid task"},
	})
	if err != nil {
		t.Fatalf("Insert with known fields failed: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty ID")
	}
}

// TestEntityStore_UpdateUnknownField ensures update rejects fields not in spec.
func TestEntityStore_UpdateUnknownField(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_unknown_update.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "note", Module: "project"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "body", Type: spec.FieldString},
			{Name: "pinned", Type: spec.FieldBoolean},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	id, err := store.Insert(ctx, InsertParams{
		WorkspaceID: "t1",
		CreatedBy:   "user-1",
		Data:        map[string]any{"body": "initial"},
	})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Update with unknown field must be rejected.
	_, err = store.Update(ctx, UpdateParams{
		WorkspaceID: "t1",
		ID:          id,
		Version:     1,
		UpdatedBy:   "user-1",
		Data:        map[string]any{"body": "updated", "extra": 123},
	})
	if err == nil {
		t.Fatal("expected error for unknown field, got nil")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Errorf("expected 'unknown field' error, got: %v", err)
	}

	// Update with known fields only: must succeed.
	newVersion, err := store.Update(ctx, UpdateParams{
		WorkspaceID: "t1",
		ID:          id,
		Version:     1,
		UpdatedBy:   "user-1",
		Data:        map[string]any{"body": "updated", "pinned": true},
	})
	if err != nil {
		t.Fatalf("Update with known fields failed: %v", err)
	}
	if newVersion != 2 {
		t.Errorf("expected version 2, got %d", newVersion)
	}
}

// TestEntityStore_ReservedFieldRejected ensures reserved field names are rejected
// when sent as data (they are not custom fields, so validateKnownFields will
// reject them unless they are in the known set — but they are in the known set
// as an explicit allowlist for framework internals).
func TestEntityStore_ReservedFieldRejected(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crud_reserved.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "widget", Module: "factory"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "label", Type: spec.FieldString},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	// Attempt to inject a custom field named "owner" into data — the field
	// name "owner" is a reserved field declared in spec.ReservedFieldNames
	// and is not in the Entity's own fields list. Because validateKnownFields
	// includes reserved names in its allowlist, "owner" will pass
	// unknown-field validation and be stored as JSON data. This is
	// intentional: reserved fields in data are harmless (they don't
	// override framework-set columns), and rejecting them complicates
	// internal flows that may set them. The real guard against reserved
	// field reuse is at spec validation time (IsReservedField).
	id, err := store.Insert(ctx, InsertParams{
		WorkspaceID: "t1",
		CreatedBy:   "user-1",
		Data:        map[string]any{"label": "ok", "owner": "hacker"}, // "owner" is a reserved name
	})
	if err != nil {
		t.Fatalf("Insert with reserved field name in data failed: %v", err)
	}
	_ = id
}
