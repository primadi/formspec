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

// User status values (todo: registration policies). A pending user cannot
// log in until an admin approves them; disabled blocks login permanently.
const (
	UserStatusActive   = "active"
	UserStatusPending  = "pending"
	UserStatusDisabled = "disabled"
)

// User represents an authenticated principal with credentials and grants.
//
// PasswordHash holds a bcrypt hash of the user's password — never the plaintext.
// Permissions and Roles are the effective grants used to build JWT claims
// (todo 6.1.2). WorkspaceID scopes the user to a single workspace (tenancy §1).
// Status is active | pending | disabled (default active); pending users cannot
// log in until approved.
type User struct {
	ID           string
	Username     string
	Email        string
	DisplayName  string
	PasswordHash string
	WorkspaceID  string
	// App is the app scope for this login/session (transient — set during
	// login, not persisted). Empty = workspace-level (e.g. _admin).
	App         string
	Roles       []string
	Permissions []string
	Active      bool
	Status      string
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
	status := stringField(rec.Data, "status")
	if status == "" {
		status = UserStatusActive
	}
	return &User{
		ID:           rec.ID,
		Username:     stringField(rec.Data, "username"),
		Email:        stringField(rec.Data, "email"),
		DisplayName:  stringField(rec.Data, "display_name"),
		WorkspaceID:  workspaceID,
		PasswordHash: stringField(rec.Data, "password_hash"),
		Roles:        stringSliceField(rec.Data, "roles"),
		Permissions:  stringSliceField(rec.Data, "permissions"),
		Active:       boolField(rec.Data, "active", true),
		Status:       status,
	}
}

// CreateUser inserts a new user record. Used by dev seeding and admin tooling.
// The password is hashed before storage — plaintext never touches the store.
func (s *EntityUserStore) CreateUser(ctx context.Context, workspaceID string, u *User) error {
	hash, err := HashPassword(u.PasswordHash)
	if err != nil {
		return fmt.Errorf("auth: hash password: %w", err)
	}
	status := u.Status
	if status == "" {
		status = UserStatusActive
	}
	displayName := u.DisplayName
	if displayName == "" {
		displayName = u.Username
	}
	_, err = s.store.Insert(ctx, db.InsertParams{
		WorkspaceID: workspaceID,
		CreatedBy:   u.Username,
		Data: map[string]any{
			"username":      u.Username,
			"email":         u.Email,
			"password_hash": hash,
			"display_name":  displayName,
			"roles":         u.Roles,
			"permissions":   u.Permissions,
			"active":        u.Active,
			"status":        status,
		},
	})
	return err
}

// HasUsers reports whether the workspace has at least one user record.
// Used for first-run setup detection (no users → setup wizard required).
func (s *EntityUserStore) HasUsers(ctx context.Context, workspaceID string) (bool, error) {
	res, err := s.store.List(ctx, db.ListParams{WorkspaceID: workspaceID, Page: 1, PerPage: 1})
	if err != nil {
		return false, err
	}
	return res.Total > 0, nil
}

// UpdateUser updates a user record's mutable fields (status, roles,
// permissions, display_name, email, active). Used by admin approval
// (approval registration policy) and admin tooling. The password is never
// touched here — password changes go through the entity update hook.
// Fetches the current record version for optimistic concurrency.
func (s *EntityUserStore) UpdateUser(ctx context.Context, workspaceID string, u *User) error {
	rec, err := s.store.GetByID(ctx, db.GetByIDParams{WorkspaceID: workspaceID, ID: u.ID})
	if err != nil {
		return err
	}
	// Preserve required fields (username, password_hash) — the entity update
	// validates required fields on the merged data.
	username := u.Username
	if username == "" {
		username = stringField(rec.Data, "username")
	}
	passwordHash := stringField(rec.Data, "password_hash")
	_, err = s.store.Update(ctx, db.UpdateParams{
		WorkspaceID: workspaceID,
		ID:          u.ID,
		Version:     rec.Version,
		UpdatedBy:   username,
		Data: map[string]any{
			"username":      username,
			"password_hash": passwordHash,
			"display_name":  u.DisplayName,
			"email":         u.Email,
			"roles":         u.Roles,
			"permissions":   u.Permissions,
			"active":        u.Active,
			"status":        u.Status,
		},
	})
	return err
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

// stringField extracts a string field from record data.
func stringField(data map[string]any, key string) string {
	if v, ok := data[key].(string); ok {
		return v
	}
	return ""
}

// boolField extracts a bool field from record data, returning def when absent.
func boolField(data map[string]any, key string, def bool) bool {
	if v, ok := data[key].(bool); ok {
		return v
	}
	return def
}
