package auth

import (
	"context"
	"errors"
	"fmt"

	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// ErrUserNotFound is returned when a username does not exist in the store.
var ErrUserNotFound = errors.New("auth: user not found")

// ErrUserInactive is returned when a user exists but is deactivated.
var ErrUserInactive = errors.New("auth: user inactive")

// User represents an authenticated principal with credentials and grants.
//
// PasswordHash holds a bcrypt hash of the user's password — never the plaintext.
// Permissions and Roles are the effective grants used to build JWT claims
// (todo 6.1.2). WorkspaceID scopes the user to a single workspace (tenancy §1).
type User struct {
	ID           string
	Username     string
	PasswordHash string
	WorkspaceID  string
	Roles        []string
	Permissions  []string
	Active       bool
}

// UserStore resolves a user by username or ID within a workspace.
//
// Implementations:
//   - EntityUserStore: backed by the formspec.core.user entity (default).
//   - Custom:          user-provided entity satisfying the user role contract
//     (username + password_hash), resolved via RoleResolver.
type UserStore interface {
	GetByUsername(ctx context.Context, workspaceID, username string) (*User, error)
	GetByID(ctx context.Context, workspaceID, id string) (*User, error)
}

// EntityUserStore reads users from a FormSpec entity via the registry's
// EntityStore. The entity is resolved by RoleResolver (default
// formspec.core.user) so a user-provided override wins when present.
//
// This is framework code — it calls the store directly without permission
// checks, because the auth service is trusted infrastructure.
type EntityUserStore struct {
	store *db.EntityStore
}

// NewEntityUserStore creates a user store backed by the given EntityStore.
func NewEntityUserStore(store *db.EntityStore) *EntityUserStore {
	return &EntityUserStore{store: store}
}

// GetByUsername looks up a user by username within a workspace.
func (s *EntityUserStore) GetByUsername(ctx context.Context, workspaceID, username string) (*User, error) {
	rec, err := s.store.FindByField(ctx, workspaceID, "username", username)
	if err != nil {
		return nil, ErrUserNotFound
	}
	if rec == nil {
		return nil, ErrUserNotFound
	}
	return userFromRecord(rec, workspaceID), nil
}

// GetByID looks up a user by record ID. Used by refresh rotation, where the
// session stores the user's ID (UUID), not the username.
func (s *EntityUserStore) GetByID(ctx context.Context, workspaceID, id string) (*User, error) {
	rec, err := s.store.GetByID(ctx, db.GetByIDParams{WorkspaceID: workspaceID, ID: id})
	if err != nil {
		return nil, ErrUserNotFound
	}
	if rec == nil {
		return nil, ErrUserNotFound
	}
	return userFromRecord(rec, workspaceID), nil
}

// userFromRecord converts an entity record into a User.
func userFromRecord(rec *db.EntityRecord, workspaceID string) *User {
	return &User{
		ID:           rec.ID,
		Username:     stringField(rec.Data, "username"),
		WorkspaceID:  workspaceID,
		PasswordHash: stringField(rec.Data, "password_hash"),
		Roles:        stringSliceField(rec.Data, "roles"),
		Permissions:  stringSliceField(rec.Data, "permissions"),
		Active:       boolField(rec.Data, "active", true),
	}
}

// CreateUser inserts a new user record. Used by dev seeding and admin tooling.
// The password is hashed before storage — plaintext never touches the store.
func (s *EntityUserStore) CreateUser(ctx context.Context, workspaceID string, u *User) error {
	hash, err := HashPassword(u.PasswordHash)
	if err != nil {
		return fmt.Errorf("auth: hash password: %w", err)
	}
	_, err = s.store.Insert(ctx, db.InsertParams{
		WorkspaceID: workspaceID,
		CreatedBy:   u.Username,
		Data: map[string]any{
			"username":      u.Username,
			"password_hash": hash,
			"display_name":  u.Username,
			"roles":         u.Roles,
			"permissions":   u.Permissions,
			"active":        u.Active,
		},
	})
	return err
}

// stringField extracts a string field from record data.
func stringField(data map[string]any, key string) string {
	if v, ok := data[key].(string); ok {
		return v
	}
	return ""
}

// stringSliceField extracts a []string field from record data, tolerating
// []any (JSON round-trip) and nil.
func stringSliceField(data map[string]any, key string) []string {
	v, ok := data[key]
	if !ok || v == nil {
		return nil
	}
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

// boolField extracts a bool field from record data, returning def when absent.
func boolField(data map[string]any, key string, def bool) bool {
	if v, ok := data[key].(bool); ok {
		return v
	}
	return def
}

// ensureEntityUserStore is a compile-time helper documenting the store
// construction path used by the auth service.
func ensureEntityUserStore(store *db.EntityStore) (*EntityUserStore, error) {
	if store == nil {
		return nil, fmt.Errorf("auth: nil entity store for user role")
	}
	return NewEntityUserStore(store), nil
}
