package db

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// TestEntityStore_OptionalNaturalKeySkipsBlankOnUniqueness pins the "diisi user"
// mode: a natural key where the author chose NOT to set `required`.
//
// Uniqueness is the one guarantee the engine makes, and a blank value on an
// optional key means "not set" rather than "a value" — so two records without a
// code are not duplicates. Before the uniqueness index skipped blanks, the
// second insert died with `UNIQUE constraint failed: ... (_code)`, which made
// the optional mode unusable: the author's declaration was accepted and then
// every second record was refused.
func TestEntityStore_OptionalNaturalKeySkipsBlankOnUniqueness(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "nk_optional.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer func() { _ = d.Close() }()

	meta := spec.Metadata{Name: "account", Module: "gl"}
	entity := &spec.EntitySpec{
		Version:        "v1",
		Characteristic: spec.CharReference,
		Fields: []spec.Field{
			// The author's optional mode: filled by a person, possibly later.
			{Name: "code", Type: spec.FieldString, NaturalKey: true},
			{Name: "name", Type: spec.FieldString},
		},
	}
	if err := spec.ValidateEntitySpec(entity); err != nil {
		t.Fatalf("ValidateEntitySpec: %v", err)
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations: %v", err)
	}
	store := NewEntityStore(d, DriverSQLite, meta, entity)

	// Two records the user left blank.
	for _, name := range []string{"Kas", "Pendapatan"} {
		if _, err := store.Insert(ctx, InsertParams{
			WorkspaceID: "t1", CreatedBy: "u",
			Data: map[string]any{"code": "", "name": name},
		}); err != nil {
			t.Fatalf("insert %s with blank key must be allowed, got %v", name, err)
		}
	}

	// A value that IS supplied must still be unique.
	if _, err := store.Insert(ctx, InsertParams{
		WorkspaceID: "t1", CreatedBy: "u",
		Data: map[string]any{"code": "1-10001", "name": "Kas Kecil"},
	}); err != nil {
		t.Fatalf("first non-blank key: %v", err)
	}
	if _, err := store.Insert(ctx, InsertParams{
		WorkspaceID: "t1", CreatedBy: "u",
		Data: map[string]any{"code": "1-10001", "name": "Duplikat"},
	}); err == nil {
		t.Fatal("a duplicate non-blank key must still be refused")
	}
}
