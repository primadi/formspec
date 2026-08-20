package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/primadi/formspec/internal/entity"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// ErrInvalidCredentials is returned when username/password do not match.
var ErrInvalidCredentials = errors.New("auth: invalid credentials")

// ErrSessionRevoked is returned when a refresh token's session no longer exists.
var ErrSessionRevoked = errors.New("auth: session revoked")

// TokenPair is the result of a successful login or refresh.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"` // access token TTL in seconds
}

// RoleResolver maps a logical auth role (user, session, ...) to a concrete
// entity ref ("{module}/{name}") and resolves it to an EntityStore.
//
// Resolution order (user override wins):
//  1. Explicit override — set via SetOverride (auth_config_ref / config Go).
//  2. external/ — a user-provided entity registered under the role's default
//     key (e.g. formspec.core.user) that replaced the built-in default.
//  3. formspec.core — the built-in default entity.
type RoleResolver struct {
	reg       *entity.Registry
	overrides map[string]string // role → "module/name"
}

// NewRoleResolver creates a RoleResolver backed by the entity registry.
func NewRoleResolver(reg *entity.Registry) *RoleResolver {
	return &RoleResolver{
		reg:       reg,
		overrides: map[string]string{},
	}
}

// SetOverride maps a logical role to an explicit entity ref ("module/name").
// This is the highest-priority resolution layer (auth_config_ref / config Go).
func (r *RoleResolver) SetOverride(role, entityRef string) {
	if entityRef == "" {
		delete(r.overrides, role)
		return
	}
	r.overrides[role] = entityRef
}

// Resolve returns the EntityStore for a logical role.
func (r *RoleResolver) Resolve(role string) (*db.EntityStore, error) {
	// 1. Explicit override
	if ref, ok := r.overrides[role]; ok {
		module, name, found := strings.Cut(ref, "/")
		if !found || module == "" || name == "" {
			return nil, fmt.Errorf("auth: invalid entity ref %q for role %q", ref, role)
		}
		store, err := r.reg.GetEntityStore(module, name)
		if err != nil {
			return nil, fmt.Errorf("auth: resolve role %q → %s: %w", role, ref, err)
		}
		return store, nil
	}

	// 2. external/ override — a user-provided entity registered under the
	// default key (formspec.core.user) replaces the built-in default.
	// 3. Default formspec.core.
	store, err := r.reg.GetEntityStore(CoreModule, role)
	if err != nil {
		return nil, fmt.Errorf("auth: resolve role %q: %w", role, err)
	}
	return store, nil
}

// Service is the auth service: login, token issuance, and refresh rotation.
//
// It is backed by FormSpec entities (formspec.core.user/session by default,
// overridable via RoleResolver) and a TokenIssuer. Roles are materialized into
// concrete permission strings via the Materializer (todo 5.12.5).
type Service struct {
	roles       *RoleResolver
	issuer      *TokenIssuer
	users       *EntityUserStore
	session     SessionStore
	roleStore   *RoleStore
	materialize *Materializer
}

// NewService creates an auth service. The user/session stores are built from
// the RoleResolver's resolved entities.
func NewService(roles *RoleResolver, issuer *TokenIssuer) (*Service, error) {
	userStore, err := roles.Resolve(RoleUser)
	if err != nil {
		return nil, err
	}
	sessionStore, err := roles.Resolve(RoleSession)
	if err != nil {
		return nil, err
	}
	return &Service{
		roles:   roles,
		issuer:  issuer,
		users:   NewEntityUserStore(userStore),
		session: NewEntitySessionStore(sessionStore),
	}, nil
}

// SetRoleStore wires the role store (formspec.core.role) used to resolve a
// user's roles into grants.
func (s *Service) SetRoleStore(rs *RoleStore) { s.roleStore = rs }

// SetMaterializer wires the materializer that expands role grants into
// concrete permission strings (todo 5.12.5).
func (s *Service) SetMaterializer(m *Materializer) { s.materialize = m }

// permissionsForUser resolves a user's effective permissions: their direct
// permissions plus the materialized grants of every role they hold.
func (s *Service) permissionsForUser(ctx context.Context, workspaceID string, user *User) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		if p != "" && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for _, p := range user.Permissions {
		add(p)
	}
	if s.roleStore == nil || s.materialize == nil {
		return out, nil
	}
	for _, roleName := range user.Roles {
		role, err := s.roleStore.GetByName(ctx, workspaceID, roleName)
		if err != nil {
			continue // unknown role — skip
		}
		perms, err := s.materialize.Materialize(role.Grants)
		if err != nil {
			continue // malformed grant — skip role
		}
		for _, p := range perms {
			add(p)
		}
	}
	return out, nil
}

// SeedDevUser creates a default user for development mode. It is idempotent —
// if the username already exists, it is left untouched. Only used in dev
// (never in ProdMode).
func (s *Service) SeedDevUser(ctx context.Context, workspaceID, username, password string) error {
	if _, err := s.users.GetByUsername(ctx, workspaceID, username); err == nil {
		return nil // already seeded
	}
	return s.users.CreateUser(ctx, workspaceID, &User{
		Username:     username,
		PasswordHash: password, // hashed inside CreateUser
		WorkspaceID:  workspaceID,
		Roles:        []string{"admin"},
		Permissions:  []string{"*"},
		Active:       true,
	})
}

// Login verifies credentials and issues an access + refresh token pair.
//
// On success it records a session (refresh jti) for rotation (todo 6.1.3).
func (s *Service) Login(ctx context.Context, workspaceID, username, password string) (*TokenPair, error) {
	user, err := s.users.GetByUsername(ctx, workspaceID, username)
	if err != nil {
		// Do not leak whether the user exists — same error for both cases.
		return nil, ErrInvalidCredentials
	}
	if !user.Active {
		return nil, ErrInvalidCredentials
	}
	if !VerifyPassword(user.PasswordHash, password) {
		return nil, ErrInvalidCredentials
	}

	return s.issuePair(ctx, user)
}

// Refresh validates a refresh token, rotates it (invalidates the old jti and
// issues a new pair), and returns a fresh token pair.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	claims, err := s.issuer.ParseRefreshToken(refreshToken)
	if err != nil {
		return nil, ErrInvalidRefreshToken
	}

	// The session must still exist (not revoked/rotated) and be unexpired.
	sess, ok := s.session.Get(ctx, claims.ID)
	if !ok {
		return nil, ErrSessionRevoked
	}
	if sess.UserID != claims.Subject {
		return nil, ErrSessionRevoked
	}

	// Re-fetch the user to ensure they are still active and to get current
	// roles/permissions for the new access token. The session stores the
	// user's ID (UUID), so look up by ID.
	user, err := s.users.GetByID(ctx, sess.WorkspaceID, sess.UserID)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	if !user.Active {
		return nil, ErrInvalidCredentials
	}

	// Rotate: invalidate the old session, then issue a new pair.
	if err := s.session.Delete(ctx, claims.ID); err != nil {
		return nil, fmt.Errorf("auth: rotate session: %w", err)
	}
	return s.issuePair(ctx, user)
}

// issuePair issues an access + refresh token pair and records the session.
func (s *Service) issuePair(ctx context.Context, user *User) (*TokenPair, error) {
	// Materialize the user's effective permissions (direct + role grants)
	// so the access token carries the concrete permission strings (5.12.5).
	perms, err := s.permissionsForUser(ctx, user.WorkspaceID, user)
	if err != nil {
		return nil, fmt.Errorf("auth: resolve permissions: %w", err)
	}
	user.Permissions = perms

	access, err := s.issuer.IssueAccessToken(user)
	if err != nil {
		return nil, fmt.Errorf("auth: issue access token: %w", err)
	}
	refresh, err := s.issuer.IssueRefreshToken(user)
	if err != nil {
		return nil, fmt.Errorf("auth: issue refresh token: %w", err)
	}

	// Record the refresh session for rotation. The jti is embedded in the
	// refresh token; we parse it back to store it.
	claims, err := s.issuer.ParseRefreshToken(refresh)
	if err != nil {
		return nil, fmt.Errorf("auth: parse issued refresh token: %w", err)
	}
	now := time.Now()
	sess := Session{
		JTI:         claims.ID,
		UserID:      user.ID,
		WorkspaceID: user.WorkspaceID,
		ExpiresAt:   claims.ExpiresAt.Time,
		CreatedAt:   now,
	}
	if err := s.session.Create(ctx, sess); err != nil {
		return nil, fmt.Errorf("auth: record session: %w", err)
	}

	return &TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(s.issuer.AccessTTL().Seconds()),
	}, nil
}
