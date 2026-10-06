package db

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// The transition gate must be enforced at the STORE, not only in the HTTP
// handler (kafe 10.46).
//
// The gap: `require_permission` on a transition was checked only on the HTTP
// PATCH path, so a transition reached through `resource.save()` from a script —
// and any future writer — ran unguarded. Measured on kafe before this:
// `table_status_from_order.star` moved a dining table across gated transitions
// with no permission at all (legitimately, as a subscription — but nothing
// distinguished it from a script acting for a user).
//
// This pins: a caller WITHOUT the transition's permission is refused, a caller
// WITH it passes, and a system caller bypasses EXPLICITLY.
func TestEntityStore_TransitionPermission(t *testing.T) {
	newStore := func(t *testing.T) *EntityStore {
		t.Helper()
		dir := t.TempDir()
		d, err := OpenSQLite(filepath.Join(dir, "transition_perm.db"), nil)
		if err != nil {
			t.Fatalf("OpenSQLite failed: %v", err)
		}
		t.Cleanup(func() { _ = d.Close() })

		meta := spec.Metadata{Name: "table", Module: "cafe"}
		entity := &spec.EntitySpec{
			Version: "v1",
			Fields: []spec.Field{
				{Name: "code", Type: spec.FieldString},
				{Name: "table_status", Type: spec.FieldEnum,
					EnumValues: []string{"available", "occupied", "not_available"}},
			},
			StateMachine: &spec.StateMachine{
				Field:   "table_status",
				Initial: "available",
				States: []spec.StateDecl{
					{Name: "available"}, {Name: "occupied"}, {Name: "not_available"},
				},
				Transitions: []spec.TransitionDecl{
					// Gated, with the manifest's OWN-MODULE PREFIX OMITTED — the
					// form that must be auto-prefixed before matching, or the
					// gate can never be opened (kafe 10.47's failure mode).
					{From: spec.StateList{"available"}, To: "not_available",
						Action: "mark-not-available", RequirePermission: "tables.mark-not-available"},
					// Ungated: the ordinary path must keep working untouched.
					{From: spec.StateList{"available"}, To: "occupied", Action: "occupy"},
				},
			},
		}
		r := NewMigrationRunner(d, DriverSQLite)
		if _, err := r.ApplyMigrations(context.Background(),
			[]EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
			t.Fatalf("ApplyMigrations failed: %v", err)
		}
		return NewEntityStore(d, DriverSQLite, meta, entity)
	}

	ctx := context.Background()
	seed := func(t *testing.T, store *EntityStore) (string, int) {
		t.Helper()
		id, err := store.Insert(ctx, InsertParams{
			WorkspaceID: "t1", CreatedBy: "admin",
			SystemCaller: true,
			Data:         map[string]any{"code": "A-01"},
		})
		if err != nil {
			t.Fatalf("seed insert: %v", err)
		}
		rec, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: "t1", ID: id})
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		return id, rec.Version
	}

	t.Run("gated transition without the permission is refused", func(t *testing.T) {
		store := newStore(t)
		id, version := seed(t, store)

		_, err := store.Update(ctx, UpdateParams{
			WorkspaceID: "t1", ID: id, Version: version, UpdatedBy: "kasir",
			Permissions: []string{"cafe.tables.update"},
			Data:        map[string]any{"table_status": "not_available"},
		})
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("expected ErrForbidden for a gated transition, got %v", err)
		}
	})

	t.Run("gated transition WITH the permission passes (auto-prefix works)", func(t *testing.T) {
		store := newStore(t)
		id, version := seed(t, store)

		// The permission the materializer produces: fully qualified.
		if _, err := store.Update(ctx, UpdateParams{
			WorkspaceID: "t1", ID: id, Version: version, UpdatedBy: "manajer",
			Permissions: []string{"cafe.tables.update", "cafe.tables.mark-not-available"},
			Data:        map[string]any{"table_status": "not_available"},
		}); err != nil {
			t.Fatalf("a caller holding the QUALIFIED permission must pass — "+
				"if this fails the gate can never be opened: %v", err)
		}
	})

	t.Run("an unqualified declaration also matches its prefixed holder", func(t *testing.T) {
		// Same as above but asserting the module prefix is applied, not the
		// literal string: the state is reached with the prefixed permission
		// while the manifest declared the bare form.
		store := newStore(t)
		id, version := seed(t, store)
		if _, err := store.Update(ctx, UpdateParams{
			WorkspaceID: "t1", ID: id, Version: version, UpdatedBy: "manajer",
			Permissions: []string{"cafe.tables.mark-not-available"},
			Data:        map[string]any{"table_status": "not_available"},
		}); err != nil {
			t.Fatalf("prefixed permission must satisfy a bare manifest declaration: %v", err)
		}
	})

	t.Run("ungated transition is unaffected", func(t *testing.T) {
		store := newStore(t)
		id, version := seed(t, store)
		if _, err := store.Update(ctx, UpdateParams{
			WorkspaceID: "t1", ID: id, Version: version, UpdatedBy: "kasir",
			Permissions: []string{"cafe.tables.update"},
			Data:        map[string]any{"table_status": "occupied"},
		}); err != nil {
			t.Fatalf("an ungated transition must not require a permission: %v", err)
		}
	})

	t.Run("SystemCaller bypasses, and must be explicit", func(t *testing.T) {
		store := newStore(t)
		id, version := seed(t, store)

		// A subscription/worker (kafe's table_status_from_order) legitimately
		// crosses gated transitions with no user.
		if _, err := store.Update(ctx, UpdateParams{
			WorkspaceID: "t1", ID: id, Version: version, UpdatedBy: "subscription",
			SystemCaller: true,
			Data:         map[string]any{"table_status": "not_available"},
		}); err != nil {
			t.Fatalf("a system caller must not be blocked: %v", err)
		}

		// The SAME write without the marker is refused — so the bypass is a
		// declaration, never something a writer inherits by having no identity.
		id2, version2 := seed(t, store)
		if _, err := store.Update(ctx, UpdateParams{
			WorkspaceID: "t1", ID: id2, Version: version2, UpdatedBy: "anon",
			Data: map[string]any{"table_status": "not_available"},
		}); !errors.Is(err, ErrForbidden) {
			t.Fatalf("an UNMARKED caller with no permissions must be refused "+
				"(an anonymous HTTP caller looks exactly like this), got %v", err)
		}
	})
}
