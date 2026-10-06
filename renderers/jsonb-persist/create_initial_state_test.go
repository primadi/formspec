package db

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// A record is BORN in the state machine's initial state (kafe 10.72).
//
// Measured before this was enforced — `POST order {status: "paid"}` on the kafe
// dev server was ACCEPTED and stored `status=paid`, while `status: "ngawur"` was
// refused (by the enum CHECK constraint, not by the state machine). So the
// create path accepted every VALID state, not just the initial one.
//
// Why it matters beyond tidiness: a record created mid-lifecycle skips two
// things at once, both silently —
//
//   - the per-transition permission gate (it only runs on Update);
//   - the transition's `emit:`, because emissions are resolved from a state
//     CHANGE on the update path. An order created as `paid` therefore publishes
//     no `on_paid`, so the GL journal and the table-occupancy bridge never hear
//     about it, while the record looks perfectly valid.
func TestEntityStore_CreateMustStartAtInitialState(t *testing.T) {
	newStore := func(t *testing.T) *EntityStore {
		t.Helper()
		dir := t.TempDir()
		d, err := OpenSQLite(filepath.Join(dir, "initial_state.db"), nil)
		if err != nil {
			t.Fatalf("OpenSQLite failed: %v", err)
		}
		t.Cleanup(func() { _ = d.Close() })

		meta := spec.Metadata{Name: "order", Module: "shop"}
		entity := &spec.EntitySpec{
			Version: "v1",
			Fields: []spec.Field{
				{Name: "code", Type: spec.FieldString},
				{Name: "status", Type: spec.FieldEnum,
					EnumValues: []string{"draft", "paid", "shipped"},
					Default:    "draft"},
			},
			StateMachine: &spec.StateMachine{
				Field:   "status",
				Initial: "draft",
				States:  []spec.StateDecl{{Name: "draft"}, {Name: "paid"}, {Name: "shipped"}},
				Transitions: []spec.TransitionDecl{
					{From: spec.StateList{"draft"}, To: "paid", Action: "pay"},
					{From: spec.StateList{"paid"}, To: "shipped", Action: "ship"},
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
	insert := func(store *EntityStore, data map[string]any, system bool) (string, error) {
		return store.Insert(ctx, InsertParams{
			WorkspaceID: "t1", CreatedBy: "clerk",
			Data: data, SystemCaller: system,
		})
	}

	t.Run("omitting the state stores the initial one", func(t *testing.T) {
		store := newStore(t)
		id, err := insert(store, map[string]any{"code": "O-1"}, false)
		if err != nil {
			t.Fatalf("a create that omits the state must succeed: %v", err)
		}
		rec, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: "t1", ID: id})
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got, _ := rec.Data["status"].(string); got != "draft" {
			t.Errorf("status = %q, want the initial %q", got, "draft")
		}
	})

	t.Run("sending the initial state is fine", func(t *testing.T) {
		store := newStore(t)
		if _, err := insert(store, map[string]any{"code": "O-2", "status": "draft"}, false); err != nil {
			t.Fatalf("explicitly sending the initial state must succeed: %v", err)
		}
	})

	t.Run("a VALID but non-initial state is refused", func(t *testing.T) {
		store := newStore(t)
		// `paid` is a declared state — the enum CHECK would accept it — but a
		// record cannot be BORN there.
		_, err := insert(store, map[string]any{"code": "O-3", "status": "paid"}, false)
		if !errors.Is(err, ErrValidationRule) {
			t.Fatalf("expected a validation error for a mid-lifecycle create, got %v", err)
		}
		if !contains(err.Error(), "draft") || !contains(err.Error(), "paid") {
			t.Errorf("the message must name both the expected and the received state, got %q", err.Error())
		}
	})

	t.Run("the later state is reachable through the normal transition", func(t *testing.T) {
		store := newStore(t)
		id, err := insert(store, map[string]any{"code": "O-4"}, false)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		rec, _ := store.GetByID(ctx, GetByIDParams{WorkspaceID: "t1", ID: id})
		if _, err := store.Update(ctx, UpdateParams{
			WorkspaceID: "t1", ID: id, Version: rec.Version, UpdatedBy: "clerk",
			Data: map[string]any{"status": "paid"},
		}); err != nil {
			t.Fatalf("reaching `paid` via update must still work: %v", err)
		}
	})

	t.Run("SystemCaller may create in any state (seed, restore, migration)", func(t *testing.T) {
		store := newStore(t)
		// A restore reproduces a stored row as-is — a different operation from a
		// caller creating a record.
		if _, err := insert(store, map[string]any{"code": "O-5", "status": "shipped"}, true); err != nil {
			t.Fatalf("a system write must be able to reproduce any state: %v", err)
		}
	})

	t.Run("an entity with no state machine is unaffected", func(t *testing.T) {
		dir := t.TempDir()
		d, err := OpenSQLite(filepath.Join(dir, "no_sm.db"), nil)
		if err != nil {
			t.Fatalf("OpenSQLite failed: %v", err)
		}
		defer func() { _ = d.Close() }()

		meta := spec.Metadata{Name: "note", Module: "shop"}
		entity := &spec.EntitySpec{
			Version: "v1",
			Fields: []spec.Field{
				{Name: "kind", Type: spec.FieldString},
			},
		}
		r := NewMigrationRunner(d, DriverSQLite)
		if _, err := r.ApplyMigrations(ctx,
			[]EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
			t.Fatalf("ApplyMigrations failed: %v", err)
		}
		store := NewEntityStore(d, DriverSQLite, meta, entity)
		if _, err := store.Insert(ctx, InsertParams{
			WorkspaceID: "t1", CreatedBy: "clerk",
			Data: map[string]any{"kind": "anything"},
		}); err != nil {
			t.Fatalf("an entity without a state machine must be unaffected: %v", err)
		}
	})
}
