package auth

import (
	"context"
	"encoding/json"
	"fmt"

	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// Role is a named collection of grants (page → tab → action + conditions),
// scoped to an App. It is stored in the formspec.core.role entity.
type Role struct {
	ID          string
	Name        string
	App         string
	Module      string // scope for module-owner (todo 6.3.4)
	Description string
	Grants      []Grant
}

// RoleStore reads roles from the formspec.core.role entity.
type RoleStore struct {
	store *db.EntityStore
}

// NewRoleStore creates a role store backed by the given EntityStore.
func NewRoleStore(store *db.EntityStore) *RoleStore {
	return &RoleStore{store: store}
}

// GetByName returns a role by name within a workspace.
func (s *RoleStore) GetByName(ctx context.Context, workspaceID, name string) (*Role, error) {
	rec, err := s.store.FindByField(ctx, workspaceID, "name", name)
	if err != nil || rec == nil {
		return nil, fmt.Errorf("auth: role %q not found", name)
	}
	return roleFromRecord(rec), nil
}

// CreateRole inserts a new role record.
func (s *RoleStore) CreateRole(ctx context.Context, workspaceID string, r *Role) error {
	_, err := s.store.Insert(ctx, db.InsertParams{
		WorkspaceID: workspaceID,
		CreatedBy:   "system",
		Data: map[string]any{
			"name":        r.Name,
			"app":         r.App,
			"module":      r.Module,
			"description": r.Description,
			"grants":      r.Grants,
		},
	})
	return err
}

// SeedOwnerRoles creates the 4 symmetric owner roles idempotently
// (todo 6.3.4): Workspace Owner, App Owner, Module Owner, Cloud Owner.
func (s *RoleStore) SeedOwnerRoles(ctx context.Context, workspaceID string) error {
	roles := []Role{
		{Name: RoleWorkspaceOwner, Description: "Full control over the workspace"},
		{Name: RoleAppOwner, Description: "Full control over an App"},
		{Name: RoleModuleOwner, Description: "Full control over a Module"},
		{Name: RoleCloudOwner, Description: "Full control over the cloud (control plane)"},
	}
	for _, r := range roles {
		if _, err := s.GetByName(ctx, workspaceID, r.Name); err == nil {
			continue // already exists
		}
		if err := s.CreateRole(ctx, workspaceID, &r); err != nil {
			return err
		}
	}
	return nil
}

// ownerRolePermission returns the wildcard permission granted by an owner
// role within its scope, or "" if the role is not an owner role (todo 6.3.4).
//
//   - workspace-owner / cloud-owner → "*" (full access)
//   - module-owner → "{module}.*" (scoped to one module)
//   - app-owner → "*" (single-server approximation; per-App scoping needs
//     per-App routing, deferred)
func ownerRolePermission(r *Role) string {
	switch r.Name {
	case RoleWorkspaceOwner, RoleCloudOwner, RoleAppOwner:
		return "*"
	case RoleModuleOwner:
		if r.Module != "" {
			return r.Module + ".*"
		}
	}
	return ""
}

// roleFromRecord converts a role entity record into a Role.
func roleFromRecord(rec *db.EntityRecord) *Role {
	r := &Role{
		ID:          rec.ID,
		Name:        stringField(rec.Data, "name"),
		App:         stringField(rec.Data, "app"),
		Module:      stringField(rec.Data, "module"),
		Description: stringField(rec.Data, "description"),
	}
	// Parse grants JSON.
	if raw, ok := rec.Data["grants"]; ok {
		if b, err := json.Marshal(raw); err == nil {
			_ = json.Unmarshal(b, &r.Grants)
		}
	}
	return r
}
