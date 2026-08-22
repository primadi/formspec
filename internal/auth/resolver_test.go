package auth

import (
	"context"
	"path/filepath"
	"sort"
	"testing"

	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/ui"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// setupResolver builds a PermissionResolver over core entities + a UI registry
// (order-list page → billing.orders.list/view) for materialization.
func setupResolver(t *testing.T) (*PermissionResolver, *entity.Registry, db.DB) {
	t.Helper()
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "resolver_test.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	reg := entity.NewRegistry(d, db.DriverSQLite, "")
	if err := RegisterCoreEntities(reg); err != nil {
		t.Fatalf("RegisterCoreEntities: %v", err)
	}
	for _, e := range []struct{ name, plural string }{
		{"order", "orders"}, {"customer", "customers"},
	} {
		if err := reg.RegisterCoreEntity("billing", e.name, "test", &spec.EntitySpec{
			Version: "v1", Plural: e.plural, Characteristic: spec.CharMaster,
			Fields: []spec.Field{{Name: "name", Type: spec.FieldString}},
		}); err != nil {
			t.Fatalf("RegisterCoreEntity %s: %v", e.name, err)
		}
	}
	if _, err := reg.SyncSchema(context.Background()); err != nil {
		t.Fatalf("SyncSchema: %v", err)
	}

	uiReg := ui.NewRegistry()
	uiReg.Tables["order-table"] = &ui.Entry[spec.TableSpec]{
		Name: "order-table", Module: "billing",
		Spec: &spec.TableSpec{Entity: "order"},
	}
	uiReg.Pages["order-list"] = &ui.Entry[spec.PageSpec]{
		Name: "order-list", Module: "billing",
		Spec: &spec.PageSpec{Route: "/orders", Blocks: []spec.PageBlock{{Table: &spec.BlockRef{Ref: "order-table"}}}},
	}

	roles := NewRoleResolver(reg)
	userStore, _ := roles.Resolve(RoleUser)
	roleStore, _ := roles.Resolve(RoleRole)
	mat := NewMaterializer(uiReg, reg)

	resolver := NewPermissionResolver(NewEntityUserStore(userStore), NewRoleStore(roleStore), mat)
	return resolver, reg, d
}

func TestPermissionResolver_ResolveAndCache(t *testing.T) {
	resolver, reg, _ := setupResolver(t)
	ctx := context.Background()

	// Create a role with a grant on the order-list page (→ billing.orders.list/view).
	roleStore, err := reg.GetEntityStore(CoreModule, "role")
	if err != nil {
		t.Fatalf("role store: %v", err)
	}
	roleID, err := roleStore.Insert(ctx, db.InsertParams{
		WorkspaceID: "demo", CreatedBy: "test",
		Data: map[string]any{
			"name": "sales-admin",
			"app":  "",
			"grants": []map[string]any{
				{"page": "order-list", "actions": []map[string]any{{"name": "list"}, {"name": "view"}}},
			},
		},
	})
	if err != nil {
		t.Fatalf("insert role: %v", err)
	}

	// Create a user holding that role.
	userStore, err := reg.GetEntityStore(CoreModule, "user")
	if err != nil {
		t.Fatalf("user store: %v", err)
	}
	userID, err := userStore.Insert(ctx, db.InsertParams{
		WorkspaceID: "demo", CreatedBy: "test",
		Data: map[string]any{
			"username":      "alice",
			"password_hash": "x",
			"roles":         []string{"sales-admin"},
			"permissions":   []string{"billing.customers.list"},
			"active":        true,
		},
	})
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	user := &User{ID: userID, WorkspaceID: "demo", Roles: []string{"sales-admin"}, Permissions: []string{"billing.customers.list"}}

	// Resolve → direct + materialized role grants.
	perms, err := resolver.Resolve(ctx, "demo", user)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	sort.Strings(perms)
	want := []string{"billing.customers.list", "billing.orders.list", "billing.orders.view"}
	if len(perms) != len(want) {
		t.Fatalf("got %v, want %v", perms, want)
	}
	for i := range want {
		if perms[i] != want[i] {
			t.Errorf("perms[%d]=%q, want %q", i, perms[i], want[i])
		}
	}

	// Second resolve hits the cache (same result).
	perms2, _ := resolver.Resolve(ctx, "demo", user)
	if len(perms2) != len(want) {
		t.Fatalf("cached resolve mismatch: %v", perms2)
	}

	// Invalidate → still resolves correctly (fresh).
	resolver.Invalidate(userID)
	perms3, _ := resolver.Resolve(ctx, "demo", user)
	if len(perms3) != len(want) {
		t.Fatalf("post-invalidate resolve mismatch: %v", perms3)
	}

	_ = roleID
}
