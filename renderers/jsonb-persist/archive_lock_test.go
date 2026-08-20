package db

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

func TestSoftDelete_ArchiveLocked(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "archlock.db"), nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "customer", Module: "billing"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "name", Type: spec.FieldString},
			{Name: "locked_for_deletion", Type: spec.FieldBoolean},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	// Normal record — delete allowed.
	id, err := store.Insert(ctx, InsertParams{WorkspaceID: "demo", Data: map[string]any{"name": "A"}})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := store.SoftDelete(ctx, "demo", id); err != nil {
		t.Fatalf("expected delete allowed for unlocked record, got %v", err)
	}

	// Record flagged locked_for_deletion — delete blocked (4.9.3).
	id2, err := store.Insert(ctx, InsertParams{WorkspaceID: "demo", Data: map[string]any{"name": "B", "locked_for_deletion": true}})
	if err != nil {
		t.Fatalf("insert locked: %v", err)
	}
	err = store.SoftDelete(ctx, "demo", id2)
	if err == nil {
		t.Fatal("expected delete blocked for locked_for_deletion record")
	}
	if !strings.Contains(err.Error(), string(spec.ErrorArchiveLockedForDeletion)) {
		t.Fatalf("expected %s in error, got %v", spec.ErrorArchiveLockedForDeletion, err)
	}
}
