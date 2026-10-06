package db

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// The field-level write guard lives in the STORE, not only in the HTTP handler.
//
// Why it matters which layer: HTTP and `resource.save()` from a script both end
// up here, while a guard placed on one of those paths leaves the other open —
// which is exactly how kafe 10.46 (transition gates) and 10.71 (field-level
// §5.3) came to exist. Enforcing at the store makes the invariant hold for every
// writer, including ones added later.
//
// The declaration is read from the same place in both layers
// (`field.RequiredPermission`), so this is one rule checked twice, not two rules.
func TestEntityStore_FieldWritePermission(t *testing.T) {
	newStore := func(t *testing.T) (*EntityStore, DB) {
		t.Helper()
		dir := t.TempDir()
		d, err := OpenSQLite(filepath.Join(dir, "field_perm.db"), nil)
		if err != nil {
			t.Fatalf("OpenSQLite failed: %v", err)
		}
		t.Cleanup(func() { _ = d.Close() })

		meta := spec.Metadata{Name: "employee", Module: "hr"}
		entity := &spec.EntitySpec{
			Version: "v1",
			Fields: []spec.Field{
				{Name: "name", Type: spec.FieldString},
				// The gated field of 05-field-types.md §5.3.
				{Name: "salary", Type: spec.FieldInteger, RequiredPermission: "hr.salary.view"},
			},
		}
		r := NewMigrationRunner(d, DriverSQLite)
		if _, err := r.ApplyMigrations(context.Background(),
			[]EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
			t.Fatalf("ApplyMigrations failed: %v", err)
		}
		return NewEntityStore(d, DriverSQLite, meta, entity), d
	}

	ctx := context.Background()

	t.Run("insert without the permission is refused", func(t *testing.T) {
		store, _ := newStore(t)
		_, err := store.Insert(ctx, InsertParams{
			WorkspaceID: "t1", CreatedBy: "u1",
			Permissions: []string{"hr.employees.create"},
			Data:        map[string]any{"name": "Alice", "salary": 999999},
		})
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("expected ErrForbidden, got %v", err)
		}
		// The message must name the field: "forbidden" alone tells the caller
		// nothing about which input to drop.
		if !contains(err.Error(), "salary") {
			t.Errorf("error must name the offending field, got %q", err.Error())
		}
	})

	t.Run("insert with the permission is allowed", func(t *testing.T) {
		store, _ := newStore(t)
		if _, err := store.Insert(ctx, InsertParams{
			WorkspaceID: "t1", CreatedBy: "u1",
			Permissions: []string{"hr.employees.create", "hr.salary.view"},
			Data:        map[string]any{"name": "Alice", "salary": 100000},
		}); err != nil {
			t.Fatalf("a caller holding the field permission must be allowed: %v", err)
		}
	})

	t.Run("a wildcard holder is allowed", func(t *testing.T) {
		store, _ := newStore(t)
		if _, err := store.Insert(ctx, InsertParams{
			WorkspaceID: "t1", CreatedBy: "admin",
			Permissions: []string{"*"},
			Data:        map[string]any{"name": "Bob", "salary": 1},
		}); err != nil {
			t.Fatalf("`*` must satisfy the field permission: %v", err)
		}
	})

	t.Run("not sending the field is not a violation", func(t *testing.T) {
		store, _ := newStore(t)
		if _, err := store.Insert(ctx, InsertParams{
			WorkspaceID: "t1", CreatedBy: "u1",
			Permissions: []string{"hr.employees.create"},
			Data:        map[string]any{"name": "Carol"},
		}); err != nil {
			t.Fatalf("a payload that omits the gated field must pass: %v", err)
		}
	})

	t.Run("update without the permission is refused, and the value survives", func(t *testing.T) {
		store, _ := newStore(t)
		id, err := store.Insert(ctx, InsertParams{
			WorkspaceID: "t1", CreatedBy: "admin",
			Permissions: []string{"*"},
			Data:        map[string]any{"name": "Dan", "salary": 100000},
		})
		if err != nil {
			t.Fatalf("seed insert: %v", err)
		}
		rec, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: "t1", ID: id})
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}

		_, err = store.Update(ctx, UpdateParams{
			WorkspaceID: "t1", ID: id, Version: rec.Version, UpdatedBy: "u1",
			Permissions: []string{"hr.employees.update"},
			Data:        map[string]any{"salary": 1},
		})
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("expected ErrForbidden, got %v", err)
		}

		// The stored value must be untouched.
		after, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: "t1", ID: id})
		if err != nil {
			t.Fatalf("GetByID after refusal: %v", err)
		}
		if got := numberOfAny(after.Data["salary"]); got != 100000 {
			t.Errorf("salary = %v, want the original 100000 untouched", got)
		}
	})

	t.Run("updating an unrelated field does not trip the guard", func(t *testing.T) {
		store, _ := newStore(t)
		id, err := store.Insert(ctx, InsertParams{
			WorkspaceID: "t1", CreatedBy: "admin",
			Permissions: []string{"*"},
			Data:        map[string]any{"name": "Eve", "salary": 100000},
		})
		if err != nil {
			t.Fatalf("seed insert: %v", err)
		}
		rec, _ := store.GetByID(ctx, GetByIDParams{WorkspaceID: "t1", ID: id})

		// A caller without the field permission changes only the name. The
		// record CARRIES `salary`, but they did not send it — intent is what is
		// judged, so this must pass.
		if _, err := store.Update(ctx, UpdateParams{
			WorkspaceID: "t1", ID: id, Version: rec.Version, UpdatedBy: "u1",
			Permissions: []string{"hr.employees.update", "hr.employees.view"},
			Data:        map[string]any{"name": "Eve Updated"},
		}); err != nil {
			t.Fatalf("an update that does not touch the gated field must pass: %v", err)
		}
	})

	t.Run("SystemCaller bypasses, and must be explicit", func(t *testing.T) {
		store, _ := newStore(t)
		// A framework write with no user behind it (seed, migration, restore).
		if _, err := store.Insert(ctx, InsertParams{
			WorkspaceID: "t1", CreatedBy: "seed",
			SystemCaller: true,
			Data:         map[string]any{"name": "Seeded", "salary": 50000},
		}); err != nil {
			t.Fatalf("a system caller must not be blocked: %v", err)
		}

		// …and the SAME payload without the marker is refused, so the bypass is
		// a decision a site makes, never something it stumbles into by leaving
		// Permissions empty.
		if _, err := store.Insert(ctx, InsertParams{
			WorkspaceID: "t1", CreatedBy: "seed",
			Data: map[string]any{"name": "Unmarked", "salary": 50000},
		}); !errors.Is(err, ErrForbidden) {
			t.Fatalf("an UNMARKED caller with no permissions must be refused, got %v", err)
		}
	})
}
