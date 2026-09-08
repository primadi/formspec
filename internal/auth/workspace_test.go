package auth

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/internal/entity"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// setupWorkspaceRegistry builds a registry with core entities synced and a
// WorkspaceRegistry backed by formspec.core.workspace.
func setupWorkspaceRegistry(t *testing.T) *WorkspaceRegistry {
	t.Helper()
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "workspace_test.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	reg := entity.NewRegistry(d, db.DriverSQLite, "")
	if err := RegisterCoreEntities(reg); err != nil {
		t.Fatalf("RegisterCoreEntities: %v", err)
	}
	if _, err := reg.SyncSchema(context.Background()); err != nil {
		t.Fatalf("SyncSchema: %v", err)
	}
	store, err := reg.GetEntityStore(CoreModule, CoreWorkspaceEntity)
	if err != nil {
		t.Fatalf("GetEntityStore: %v", err)
	}
	return NewWorkspaceRegistry(store)
}

func TestWorkspaceRegistry_EnsureListDelete(t *testing.T) {
	reg := setupWorkspaceRegistry(t)
	ctx := context.Background()

	created, err := reg.Ensure(ctx, WorkspaceInfo{Slug: "kopi", DisplayName: "Kopi Kita"})
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if !created {
		t.Fatal("expected first Ensure to create the workspace")
	}

	// Idempotent: second Ensure must not create a duplicate.
	created, err = reg.Ensure(ctx, WorkspaceInfo{Slug: "kopi", DisplayName: "Kopi Kita Renamed"})
	if err != nil {
		t.Fatalf("Ensure (second): %v", err)
	}
	if created {
		t.Fatal("expected second Ensure to be an update, not a create")
	}

	list, err := reg.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].Slug != "kopi" || list[0].DisplayName != "Kopi Kita Renamed" {
		t.Fatalf("unexpected list: %+v", list)
	}

	if err := reg.Delete(ctx, "kopi"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	ok, err := reg.Registered(ctx, "kopi")
	if err != nil {
		t.Fatalf("Registered: %v", err)
	}
	if ok {
		t.Fatal("workspace should be unregistered after Delete")
	}

	if err := reg.Delete(ctx, "kopi"); err != ErrWorkspaceNotFound {
		t.Fatalf("expected ErrWorkspaceNotFound, got %v", err)
	}
}

func TestWorkspaceRegistry_Registered(t *testing.T) {
	reg := setupWorkspaceRegistry(t)
	ctx := context.Background()

	ok, err := reg.Registered(ctx, "default")
	if err != nil {
		t.Fatalf("Registered (unregistered): %v", err)
	}
	if ok {
		t.Fatal("no workspaces seeded — 'default' must not be registered")
	}

	if _, err := reg.Ensure(ctx, WorkspaceInfo{Slug: "default", DisplayName: "Default Workspace"}); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	ok, err = reg.Registered(ctx, "default")
	if err != nil {
		t.Fatalf("Registered (registered): %v", err)
	}
	if !ok {
		t.Fatal("'default' should be registered after Ensure")
	}
}
