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
//
// EmailVerified tracks whether the email address has been proven owned — via
// the email-verification flow (registration) or an OAuth provider's
// `email_verified` claim. OAuth login only links to a verified email (account
// pre-hijacking protection). OAuthProvider/OAuthSub record the external
// identity (provider name + subject id) so the same identity maps to the same
// account.
type User struct {
	ID            string
	Username      string
	Email         string
	EmailVerified bool
	DisplayName   string
	PasswordHash  string
	WorkspaceID   string
	// OAuthProvider/OAuthSub is the external identity this account is linked
	// to (e.g. "google" + the `sub` claim). Empty = password-only account.
	OAuthProvider string
	OAuthSub      string
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

// GetByEmail looks up a user by email within a workspace. Used by OAuth
// login (Fase 5) to link an external identity to an existing account.
func (s *EntityUserStore) GetByEmail(ctx context.Context, workspaceID, email string) (*User, error) {
	rec, err := s.store.FindByField(ctx, workspaceID, "email", email)
	if err != nil {
		return nil, ErrUserNotFound
	}
	if rec == nil {
		return nil, ErrUserNotFound
	}
	return userFromRecord(rec, workspaceID), nil
}

// GetByOAuthIdentity looks up a user by its external identity (provider name
// + subject id). Used by OAuth login to map the same external identity to the
// same account — the strongest linking signal (stronger than email).
func (s *EntityUserStore) GetByOAuthIdentity(ctx context.Context, workspaceID, provider, sub string) (*User, error) {
	if provider == "" || sub == "" {
		return nil, ErrUserNotFound
	}
	res, err := s.store.List(ctx, db.ListParams{
		WorkspaceID: workspaceID,
		Page:        1,
		PerPage:     1,
		Filters: map[string]db.FilterOp{
			"oauth_provider": {Op: "eq", Value: provider},
			"oauth_sub":      {Op: "eq", Value: sub},
		},
	})
	if err != nil {
		return nil, ErrUserNotFound
	}
	if len(res.Data) == 0 {
		return nil, ErrUserNotFound
	}
	return userFromRecord(&res.Data[0], workspaceID), nil
}

// LinkOAuthIdentity attaches an external identity (provider + sub) to an
// existing user. Used when OAuth login links to an existing account or when a
// signed-in user explicitly links a provider.
func (s *EntityUserStore) LinkOAuthIdentity(ctx context.Context, workspaceID, userID, provider, sub string) error {
	if provider == "" || sub == "" {
		return nil
	}
	rec, err := s.store.GetByID(ctx, db.GetByIDParams{WorkspaceID: workspaceID, ID: userID})
	if err != nil {
		return err
	}
	_, err = s.store.Update(ctx, db.UpdateParams{
		WorkspaceID: workspaceID,
		ID:          userID,
		Version:     rec.Version,
		UpdatedBy:   stringField(rec.Data, "username"),
		Data: map[string]any{
			"username":       stringField(rec.Data, "username"),
			"password_hash":  stringField(rec.Data, "password_hash"),
			"email":          stringField(rec.Data, "email"),
			"email_verified": boolField(rec.Data, "email_verified", false),
			"display_name":   stringField(rec.Data, "display_name"),
			"oauth_provider": provider,
			"oauth_sub":      sub,
			"roles":          stringSliceField(rec.Data, "roles"),
			"permissions":    stringSliceField(rec.Data, "permissions"),
			"active":         boolField(rec.Data, "active", true),
			"status":         stringField(rec.Data, "status"),
		},
	})
	return err
}

// UnlinkOAuthIdentity detaches the external identity from a user (clears
// oauth_provider + oauth_sub). Used by the explicit unlink flow — the
// service layer guards against unlinking the only sign-in method.
func (s *EntityUserStore) UnlinkOAuthIdentity(ctx context.Context, workspaceID, userID string) error {
	rec, err := s.store.GetByID(ctx, db.GetByIDParams{WorkspaceID: workspaceID, ID: userID})
	if err != nil {
		return err
	}
	_, err = s.store.Update(ctx, db.UpdateParams{
		WorkspaceID: workspaceID,
		ID:          userID,
		Version:     rec.Version,
		UpdatedBy:   stringField(rec.Data, "username"),
		Data: map[string]any{
			"username":       stringField(rec.Data, "username"),
			"password_hash":  stringField(rec.Data, "password_hash"),
			"email":          stringField(rec.Data, "email"),
			"email_verified": boolField(rec.Data, "email_verified", false),
			"display_name":   stringField(rec.Data, "display_name"),
			"oauth_provider": "",
			"oauth_sub":      "",
			"roles":          stringSliceField(rec.Data, "roles"),
			"permissions":    stringSliceField(rec.Data, "permissions"),
			"active":         boolField(rec.Data, "active", true),
			"status":         stringField(rec.Data, "status"),
		},
	})
	return err
}

// SetEmailVerified updates the user's email-verification flag. Used by the
// email-verification flow (VerifyEmail).
func (s *EntityUserStore) SetEmailVerified(ctx context.Context, workspaceID, userID string, verified bool) error {
	rec, err := s.store.GetByID(ctx, db.GetByIDParams{WorkspaceID: workspaceID, ID: userID})
	if err != nil {
		return err
	}
	_, err = s.store.Update(ctx, db.UpdateParams{
		WorkspaceID: workspaceID,
		ID:          userID,
		Version:     rec.Version,
		UpdatedBy:   stringField(rec.Data, "username"),
		Data: map[string]any{
			"username":       stringField(rec.Data, "username"),
			"password_hash":  stringField(rec.Data, "password_hash"),
			"email":          stringField(rec.Data, "email"),
			"email_verified": verified,
			"display_name":   stringField(rec.Data, "display_name"),
			"oauth_provider": stringField(rec.Data, "oauth_provider"),
			"oauth_sub":      stringField(rec.Data, "oauth_sub"),
			"roles":          stringSliceField(rec.Data, "roles"),
			"permissions":    stringSliceField(rec.Data, "permissions"),
			"active":         boolField(rec.Data, "active", true),
			"status":         stringField(rec.Data, "status"),
		},
	})
	return err
}

// TakeoverUnverifiedEmail claims an unverified account on behalf of the real
// email owner (proven via a provider-verified OAuth login): marks the email
// verified, replaces any password the previous claimant set with a dead hash
// (so it stops working — the account behaves like a pure OAuth account), and
// attaches the OAuth identity. This is the account pre-hijacking recovery
// path.
func (s *EntityUserStore) TakeoverUnverifiedEmail(ctx context.Context, workspaceID, userID, provider, sub string) error {
	rec, err := s.store.GetByID(ctx, db.GetByIDParams{WorkspaceID: workspaceID, ID: userID})
	if err != nil {
		return err
	}
	// A hash of the empty string is the "no password" marker (same as
	// OAuth-created users): no real password authenticates, and the entity's
	// required password_hash stays non-empty.
	deadHash, err := HashPassword("")
	if err != nil {
		return fmt.Errorf("auth: dead hash: %w", err)
	}
	_, err = s.store.Update(ctx, db.UpdateParams{
		WorkspaceID: workspaceID,
		ID:          userID,
		Version:     rec.Version,
		UpdatedBy:   stringField(rec.Data, "username"),
		Data: map[string]any{
			"username":       stringField(rec.Data, "username"),
			"password_hash":  deadHash, // previous claimant's password stops working
			"email":          stringField(rec.Data, "email"),
			"email_verified": true,
			"display_name":   stringField(rec.Data, "display_name"),
			"oauth_provider": provider,
			"oauth_sub":      sub,
			"roles":          stringSliceField(rec.Data, "roles"),
			"permissions":    stringSliceField(rec.Data, "permissions"),
			"active":         boolField(rec.Data, "active", true),
			"status":         stringField(rec.Data, "status"),
		},
	})
	return err
}

// userFromRecord converts an entity record into a User.
func userFromRecord(rec *db.EntityRecord, workspaceID string) *User {
	status := stringField(rec.Data, "status")
	if status == "" {
		status = UserStatusActive
	}
	return &User{
		ID:            rec.ID,
		Username:      stringField(rec.Data, "username"),
		Email:         stringField(rec.Data, "email"),
		EmailVerified: boolField(rec.Data, "email_verified", false),
		DisplayName:   stringField(rec.Data, "display_name"),
		WorkspaceID:   workspaceID,
		PasswordHash:  stringField(rec.Data, "password_hash"),
		OAuthProvider: stringField(rec.Data, "oauth_provider"),
		OAuthSub:      stringField(rec.Data, "oauth_sub"),
		Roles:         stringSliceField(rec.Data, "roles"),
		Permissions:   stringSliceField(rec.Data, "permissions"),
		Active:        boolField(rec.Data, "active", true),
		Status:        status,
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
			"username":       u.Username,
			"email":          u.Email,
			"email_verified": u.EmailVerified,
			"password_hash":  hash,
			"display_name":   displayName,
			"oauth_provider": u.OAuthProvider,
			"oauth_sub":      u.OAuthSub,
			"roles":          u.Roles,
			"permissions":    u.Permissions,
			"active":         u.Active,
			"status":         status,
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
			"username":       username,
			"password_hash":  passwordHash,
			"display_name":   u.DisplayName,
			"email":          u.Email,
			"email_verified": u.EmailVerified,
			"oauth_provider": u.OAuthProvider,
			"oauth_sub":      u.OAuthSub,
			"roles":          u.Roles,
			"permissions":    u.Permissions,
			"active":         u.Active,
			"status":         u.Status,
		},
	})
	return err
}

// SetPassword hashes a plaintext password and updates ONLY the user's
// password_hash, preserving every other field. Used by change-password and
// reset-password flows (self-service). The plaintext never touches the store.
func (s *EntityUserStore) SetPassword(ctx context.Context, workspaceID, userID, plain string) error {
	rec, err := s.store.GetByID(ctx, db.GetByIDParams{WorkspaceID: workspaceID, ID: userID})
	if err != nil {
		return err
	}
	hash, err := HashPassword(plain)
	if err != nil {
		return fmt.Errorf("auth: hash password: %w", err)
	}
	username := stringField(rec.Data, "username")
	_, err = s.store.Update(ctx, db.UpdateParams{
		WorkspaceID: workspaceID,
		ID:          userID,
		Version:     rec.Version,
		UpdatedBy:   username,
		Data: map[string]any{
			"username":       username,
			"password_hash":  hash,
			"email":          stringField(rec.Data, "email"),
			"email_verified": boolField(rec.Data, "email_verified", false),
			"display_name":   stringField(rec.Data, "display_name"),
			"oauth_provider": stringField(rec.Data, "oauth_provider"),
			"oauth_sub":      stringField(rec.Data, "oauth_sub"),
			"roles":          stringSliceField(rec.Data, "roles"),
			"permissions":    stringSliceField(rec.Data, "permissions"),
			"active":         boolField(rec.Data, "active", true),
			"status":         stringField(rec.Data, "status"),
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
