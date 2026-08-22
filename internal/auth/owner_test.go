package auth

import (
	"context"
	"testing"

	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

func TestHasPermission_ModuleWildcard(t *testing.T) {
	id := &Identity{Permissions: []string{"billing.*"}}
	if !id.HasPermission("billing.invoices.list") {
		t.Error("billing.* should match billing.invoices.list")
	}
	if !id.HasPermission("billing.customers.view") {
		t.Error("billing.* should match billing.customers.view")
	}
	if id.HasPermission("gl.journal.list") {
		t.Error("billing.* should NOT match gl.journal.list")
	}
	// Entity-level wildcard still matches exactly one more segment.
	id2 := &Identity{Permissions: []string{"billing.invoices.*"}}
	if !id2.HasPermission("billing.invoices.list") {
		t.Error("billing.invoices.* should match billing.invoices.list")
	}
	if id2.HasPermission("billing.invoices.list.extra") {
		t.Error("billing.invoices.* should NOT match deeper path")
	}
}

func TestOwnerRolePermission(t *testing.T) {
	cases := []struct {
		role Role
		want string
	}{
		{Role{Name: RoleWorkspaceOwner}, "*"},
		{Role{Name: RoleCloudOwner}, "*"},
		{Role{Name: RoleAppOwner}, "*"},
		{Role{Name: RoleModuleOwner, Module: "billing"}, "billing.*"},
		{Role{Name: RoleModuleOwner}, ""}, // no module scope → no grant
		{Role{Name: "sales-admin"}, ""},   // not an owner role
	}
	for _, c := range cases {
		if got := ownerRolePermission(&c.role); got != c.want {
			t.Errorf("ownerRolePermission(%s)=%q, want %q", c.role.Name, got, c.want)
		}
	}
}

func TestResolver_OwnerRoleGrantsWildcard(t *testing.T) {
	resolver, reg, _ := setupResolver(t)
	ctx := context.Background()

	// Create a workspace-owner role.
	roleStore, err := reg.GetEntityStore(CoreModule, "role")
	if err != nil {
		t.Fatal(err)
	}
	userID, err := roleStore.Insert(ctx, db.InsertParams{
		WorkspaceID: "demo", CreatedBy: "test",
		Data: map[string]any{"name": RoleWorkspaceOwner, "app": "", "module": "", "grants": []any{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = userID

	user := &User{ID: "u1", WorkspaceID: "demo", Roles: []string{RoleWorkspaceOwner}}
	perms, err := resolver.Resolve(ctx, "demo", user)
	if err != nil {
		t.Fatal(err)
	}
	if len(perms) != 1 || perms[0] != "*" {
		t.Fatalf("expected [*], got %v", perms)
	}
}

func TestSeedOwnerRoles_Idempotent(t *testing.T) {
	svc, reg, _ := setupAuthService(t)
	ctx := context.Background()

	// Wire the role store (setupAuthService does not by default).
	roleStore, err := NewRoleResolver(reg).Resolve(RoleRole)
	if err != nil {
		t.Fatal(err)
	}
	svc.SetRoleStore(NewRoleStore(roleStore))

	if err := svc.SeedOwnerRoles(ctx, "demo"); err != nil {
		t.Fatalf("SeedOwnerRoles #1: %v", err)
	}
	if err := svc.SeedOwnerRoles(ctx, "demo"); err != nil {
		t.Fatalf("SeedOwnerRoles #2 (idempotent): %v", err)
	}
	for _, name := range []string{RoleWorkspaceOwner, RoleAppOwner, RoleModuleOwner, RoleCloudOwner} {
		if _, err := svc.roleStore.GetByName(ctx, "demo", name); err != nil {
			t.Errorf("expected role %q seeded: %v", name, err)
		}
	}
}
