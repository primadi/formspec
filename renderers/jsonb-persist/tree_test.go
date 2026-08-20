package db

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// setupTreeStore creates an entity with a self-referential tree relation
// (parent_id → category, tree: true).
func setupTreeStore(t *testing.T) *EntityStore {
	t.Helper()
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "tree.db"), nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	meta := spec.Metadata{Name: "category", Module: "catalog"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "name", Type: spec.FieldString},
			{
				Name: "parent_id",
				Type: spec.FieldRelation,
				Relation: &spec.RelationDecl{
					Type:     "belongs_to",
					Resource: "category",
				},
				Tree: true,
			},
		},
		// Lifecycle-free (submit disabled) so relation targets are valid.
		Actions: []spec.Action{{Name: "submit", Disabled: true}},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	return NewEntityStore(d, DriverSQLite, meta, entity)
}

func TestTree_MaterializedPath(t *testing.T) {
	store := setupTreeStore(t)
	ctx := context.Background()

	// Root A.
	aID, err := store.Insert(ctx, InsertParams{WorkspaceID: "demo", Data: map[string]any{"name": "A"}})
	if err != nil {
		t.Fatalf("insert A: %v", err)
	}
	// Child B of A.
	bID, err := store.Insert(ctx, InsertParams{WorkspaceID: "demo", Data: map[string]any{"name": "B", "parent_id": aID}})
	if err != nil {
		t.Fatalf("insert B: %v", err)
	}
	// Grandchild C of B.
	cID, err := store.Insert(ctx, InsertParams{WorkspaceID: "demo", Data: map[string]any{"name": "C", "parent_id": bID}})
	if err != nil {
		t.Fatalf("insert C: %v", err)
	}

	// Verify materialized paths via raw query.
	paths := map[string]string{}
	rows, err := store.db.QueryContext(ctx, "SELECT id, _tpath_parent_id FROM catalog_categories")
	if err != nil {
		t.Fatalf("query paths: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, path string
		if err := rows.Scan(&id, &path); err != nil {
			t.Fatalf("scan: %v", err)
		}
		paths[id] = path
	}
	if paths[aID] != aID {
		t.Fatalf("A path = %q, want %q (root)", paths[aID], aID)
	}
	if paths[bID] != aID+"."+bID {
		t.Fatalf("B path = %q, want %q", paths[bID], aID+"."+bID)
	}
	if paths[cID] != aID+"."+bID+"."+cID {
		t.Fatalf("C path = %q, want %q", paths[cID], aID+"."+bID+"."+cID)
	}
}

func TestTree_Operators(t *testing.T) {
	store := setupTreeStore(t)
	ctx := context.Background()

	aID, _ := store.Insert(ctx, InsertParams{WorkspaceID: "demo", Data: map[string]any{"name": "A"}})
	bID, _ := store.Insert(ctx, InsertParams{WorkspaceID: "demo", Data: map[string]any{"name": "B", "parent_id": aID}})
	_, _ = store.Insert(ctx, InsertParams{WorkspaceID: "demo", Data: map[string]any{"name": "C", "parent_id": bID}})
	// A second root D.
	_, _ = store.Insert(ctx, InsertParams{WorkspaceID: "demo", Data: map[string]any{"name": "D"}})

	// root → A and D (2 roots).
	res, err := store.List(ctx, ListParams{
		WorkspaceID: "demo", Page: 1, PerPage: 100,
		Filters: map[string]FilterOp{"parent_id": {Op: "root"}},
	})
	if err != nil {
		t.Fatalf("root: %v", err)
	}
	if res.Total != 2 {
		t.Fatalf("expected 2 roots, got %d", res.Total)
	}

	// child_of A → B only.
	res, err = store.List(ctx, ListParams{
		WorkspaceID: "demo", Page: 1, PerPage: 100,
		Filters: map[string]FilterOp{"parent_id": {Op: "child_of", Value: aID}},
	})
	if err != nil {
		t.Fatalf("child_of: %v", err)
	}
	if res.Total != 1 {
		t.Fatalf("expected 1 child of A, got %d", res.Total)
	}

	// descendant_of A → B and C (2 descendants).
	res, err = store.List(ctx, ListParams{
		WorkspaceID: "demo", Page: 1, PerPage: 100,
		Filters: map[string]FilterOp{"parent_id": {Op: "descendant_of", Value: aID}},
	})
	if err != nil {
		t.Fatalf("descendant_of: %v", err)
	}
	if res.Total != 2 {
		t.Fatalf("expected 2 descendants of A, got %d", res.Total)
	}
}

func TestTree_CycleDetection(t *testing.T) {
	store := setupTreeStore(t)
	ctx := context.Background()

	aID, _ := store.Insert(ctx, InsertParams{WorkspaceID: "demo", Data: map[string]any{"name": "A"}})
	bID, _ := store.Insert(ctx, InsertParams{WorkspaceID: "demo", Data: map[string]any{"name": "B", "parent_id": aID}})
	cID, _ := store.Insert(ctx, InsertParams{WorkspaceID: "demo", Data: map[string]any{"name": "C", "parent_id": bID}})

	// Reparent A under C → cycle (A is an ancestor of C).
	rec, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: "demo", ID: aID})
	if err != nil {
		t.Fatalf("get A: %v", err)
	}
	rec.Data["parent_id"] = cID
	_, err = store.Update(ctx, UpdateParams{
		WorkspaceID: "demo", ID: aID, Version: rec.Version,
		UpdatedBy: "test", Data: rec.Data,
	})
	if err == nil {
		t.Fatal("expected cycle detection to reject reparenting A under C")
	}
	if !strings.Contains(err.Error(), "cycle detected") {
		t.Fatalf("expected cycle detected error, got %v", err)
	}
}
