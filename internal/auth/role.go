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

// roleFromRecord converts a role entity record into a Role.
func roleFromRecord(rec *db.EntityRecord) *Role {
	r := &Role{
		ID:          rec.ID,
		Name:        stringField(rec.Data, "name"),
		App:         stringField(rec.Data, "app"),
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
