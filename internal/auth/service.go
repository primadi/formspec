package auth

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/primadi/formspec/internal/entity"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// ErrInvalidCredentials is returned when username/password do not match.
var ErrInvalidCredentials = errors.New("auth: invalid credentials")

// ErrSessionRevoked is returned when a refresh token's session no longer exists.
var ErrSessionRevoked = errors.New("auth: session revoked")

// ErrUsernameTaken is returned by Register when the username already exists
// in the workspace (self-service sign-up, registry portal B.3).
var ErrUsernameTaken = errors.New("auth: username already taken")

// ErrInvalidUsername is returned by Register when the username violates the
// format rules (see ValidateUsername).
var ErrInvalidUsername = errors.New("auth: invalid username format")

// ErrRegistrationClosed is returned by Register when the workspace's
// registration policy is "closed" (self-service sign-up disabled; admins
// create users).
var ErrRegistrationClosed = errors.New("auth: self-service registration is disabled")

// ErrNotPending is returned by ApproveUser when the target user is not in
// the pending state (only pending users can be approved).
var ErrNotPending = errors.New("auth: user is not pending")

// RegistrationPolicy is the resolved workspace-level sign-up policy
// (auth redesign Fase 4).
//
//   - open     (default): sign-up creates an active user with DefaultRole.
//   - approval: sign-up creates a pending user; admin approves + assigns role.
//   - closed:   self-service sign-up disabled; admins create users.
type RegistrationPolicy struct {
	Policy      string
	DefaultRole string
}

// Registration policy values.
const (
	RegPolicyOpen     = "open"
	RegPolicyApproval = "approval"
	RegPolicyClosed   = "closed"
)

// usernamePattern constrains self-service usernames: 3–32 chars, letters
// (a-z, A-Z), digits, dot, underscore, hyphen. No spaces or other symbols —
// usernames appear in URLs, CLI flags (--vendor), and lookup-by-username
// paths where free-form text is unsafe.
var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]{3,32}$`)

// ValidateUsername checks the username format rules. Existing accounts are
// unaffected — only new registrations are validated.
func ValidateUsername(username string) error {
	if !usernamePattern.MatchString(username) {
		return fmt.Errorf("%w: must be 3-32 chars (letters, digits, . _ -)",
			ErrInvalidUsername)
	}
	return nil
}

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
	resolver    *PermissionResolver
	maxSessions int // 0 = unlimited (concurrent session limit, todo 6.5.3)
	regPolicy   RegistrationPolicy

	cleanupStop chan struct{}
	cleanupOnce sync.Once
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

// SetRegistrationPolicy sets the workspace-level sign-up policy (auth
// redesign Fase 4). Defaults to open when never set.
func (s *Service) SetRegistrationPolicy(p RegistrationPolicy) {
	if p.Policy == "" {
		p.Policy = RegPolicyOpen
	}
	s.regPolicy = p
}

// SetMaterializer wires the materializer that expands role grants into
// concrete permission strings (todo 5.12.5).
func (s *Service) SetMaterializer(m *Materializer) { s.materialize = m }

// SetMaxSessionsPerUser sets the concurrent session limit per user
// (todo 6.5.3). 0 (default) means unlimited. When exceeded, the oldest
// sessions are evicted on the next login/refresh.
func (s *Service) SetMaxSessionsPerUser(n int) { s.maxSessions = n }

// SeedOwnerRoles creates the 4 symmetric owner roles idempotently
// (todo 6.3.4). No-op when the role store is not wired.
func (s *Service) SeedOwnerRoles(ctx context.Context, workspaceID string) error {
	if s.roleStore == nil {
		return nil
	}
	return s.roleStore.SeedOwnerRoles(ctx, workspaceID)
}

// ErrSetupComplete is returned when first-run setup is attempted but the
// workspace already has users (setup is a one-time bootstrap).
var ErrSetupComplete = errors.New("auth: setup already complete")

// SetupRequired reports whether the workspace needs first-run setup — i.e.
// it has no user records at all. Used to gate the setup wizard (self-hosted
// prod bootstrap without formspec-ctl).
func (s *Service) SetupRequired(ctx context.Context, workspaceID string) (bool, error) {
	has, err := s.users.HasUsers(ctx, workspaceID)
	if err != nil {
		return false, err
	}
	return !has, nil
}

// SetupFirstAdmin creates the first admin user (roles ["admin"], permissions
// ["*"]) and seeds the 4 owner roles. Only valid when the workspace has no
// users yet — the one-time bootstrap for self-hosted production. After the
// first admin exists, normal auth (login/register) applies.
func (s *Service) SetupFirstAdmin(ctx context.Context, workspaceID, username, password, displayName string) error {
	required, err := s.SetupRequired(ctx, workspaceID)
	if err != nil {
		return err
	}
	if !required {
		return ErrSetupComplete
	}
	if err := ValidateUsername(username); err != nil {
		return err
	}
	if len(password) < 8 {
		return ErrInvalidCredentials
	}
	if err := s.users.CreateUser(ctx, workspaceID, &User{
		Username:    username,
		DisplayName: displayName,
		// PasswordHash carries the PLAINTEXT here — CreateUser hashes it.
		PasswordHash: password,
		WorkspaceID:  workspaceID,
		Roles:        []string{"admin"},
		Permissions:  []string{"*"},
		Active:       true,
		Status:       UserStatusActive,
	}); err != nil {
		return err
	}
	return s.SeedOwnerRoles(ctx, workspaceID)
}

// ensureResolver lazily builds the permission resolver once both the role
// store and materializer are wired (they are set after NewService).
func (s *Service) ensureResolver() {
	if s.resolver == nil && s.roleStore != nil && s.materialize != nil {
		s.resolver = NewPermissionResolver(s.users, s.roleStore, s.materialize)
	}
}

// InvalidatePermissions clears the per-session permission cache for a user.
// Call this when a user's roles change (todo 6.2.4).
func (s *Service) InvalidatePermissions(userID string) {
	s.ensureResolver()
	if s.resolver != nil {
		s.resolver.Invalidate(userID)
	}
}

// permissionsForUser resolves a user's effective permissions: their direct
// permissions plus the materialized grants of every role they hold (scoped to
// the given app — roles with a non-empty app only contribute when they match).
// Uses the per-session cache when available (todo 6.2.4).
func (s *Service) permissionsForUser(ctx context.Context, workspaceID, app string, user *User) ([]string, error) {
	s.ensureResolver()
	if s.resolver != nil {
		return s.resolver.Resolve(ctx, workspaceID, app, user)
	}
	// Fallback (no role store/materializer wired): direct permissions only.
	return user.Permissions, nil
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

// Register creates a new user account via self-service sign-up (registry
// portal B.3). The username must be free within the workspace. Behavior
// follows the workspace registration policy (auth redesign Fase 4):
//
//   - open     (default): active user with the policy's DefaultRole.
//   - approval: pending user (cannot log in until an admin approves).
//   - closed:   returns ErrRegistrationClosed (sign-up disabled).
//
// Password is hashed inside CreateUser.
func (s *Service) Register(ctx context.Context, workspaceID, username, password string) error {
	if err := ValidateUsername(username); err != nil {
		return err
	}
	if password == "" {
		return ErrInvalidCredentials
	}
	if _, err := s.users.GetByUsername(ctx, workspaceID, username); err == nil {
		return ErrUsernameTaken
	}

	policy := s.regPolicy.Policy
	if policy == "" {
		policy = RegPolicyOpen
	}
	switch policy {
	case RegPolicyClosed:
		return ErrRegistrationClosed
	case RegPolicyApproval:
		return s.users.CreateUser(ctx, workspaceID, &User{
			Username:    username,
			WorkspaceID: workspaceID,
			Active:      true,
			Status:      UserStatusPending,
			// PasswordHash carries the PLAINTEXT here — CreateUser hashes it.
			PasswordHash: password,
		})
	default: // open
		roles := []string{}
		if s.regPolicy.DefaultRole != "" {
			roles = append(roles, s.regPolicy.DefaultRole)
		}
		return s.users.CreateUser(ctx, workspaceID, &User{
			Username:    username,
			WorkspaceID: workspaceID,
			Active:      true,
			Status:      UserStatusActive,
			Roles:       roles,
			// PasswordHash carries the PLAINTEXT here — CreateUser hashes it.
			PasswordHash: password,
		})
	}
}

// ApproveUser approves a pending user (approval registration policy): sets
// status to active and assigns the given roles. Only pending users can be
// approved. The permission cache for the user is invalidated so the next
// login resolves fresh grants.
func (s *Service) ApproveUser(ctx context.Context, workspaceID, username string, roles []string) error {
	user, err := s.users.GetByUsername(ctx, workspaceID, username)
	if err != nil {
		return err
	}
	if user.Status != UserStatusPending {
		return ErrNotPending
	}
	user.Status = UserStatusActive
	user.Roles = roles
	if err := s.users.UpdateUser(ctx, workspaceID, user); err != nil {
		return err
	}
	s.InvalidatePermissions(user.ID)
	return nil
}

// GrantRoles adds roles and permissions to an existing user (idempotent —
// duplicates are skipped). Used by the vendor-approval flow (Fase 6): when an
// admin approves a vendor application, the owner user is granted the `vendor`
// role + registry permissions. The permission cache is invalidated so the
// next login resolves the new grants.
func (s *Service) GrantRoles(ctx context.Context, workspaceID, username string, roles, permissions []string) error {
	user, err := s.users.GetByUsername(ctx, workspaceID, username)
	if err != nil {
		return err
	}
	user.Roles = mergeStrings(user.Roles, roles)
	user.Permissions = mergeStrings(user.Permissions, permissions)
	if err := s.users.UpdateUser(ctx, workspaceID, user); err != nil {
		return err
	}
	s.InvalidatePermissions(user.ID)
	return nil
}

// mergeStrings appends items to base, skipping duplicates.
func mergeStrings(base, extra []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(base)+len(extra))
	for _, s := range base {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	for _, s := range extra {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// Login verifies credentials and issues an access + refresh token pair.
//
// app scopes the session to one App (role management is per-App): the user's
// effective permissions are resolved for that App only. Empty app = workspace-
// level session (e.g. the _admin surface) — all roles apply.
//
// On success it records a session (refresh jti) for rotation (todo 6.1.3).
func (s *Service) Login(ctx context.Context, workspaceID, app, username, password string) (*TokenPair, error) {
	user, err := s.users.GetByUsername(ctx, workspaceID, username)
	if err != nil {
		// Do not leak whether the user exists — same error for both cases.
		return nil, ErrInvalidCredentials
	}
	if !user.Active {
		return nil, ErrInvalidCredentials
	}
	// Pending users (registration policy "approval") cannot log in until an
	// admin approves them. Disabled is covered by Active=false.
	if user.Status == UserStatusPending {
		return nil, ErrInvalidCredentials
	}
	if !VerifyPassword(user.PasswordHash, password) {
		return nil, ErrInvalidCredentials
	}

	user.App = app
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
	sess, ok := s.session.Get(ctx, claims.Workspace, claims.ID)
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
	// Re-scope to the same App the refresh token was issued for.
	user.App = claims.App

	// Rotate: invalidate the old session, then issue a new pair.
	if err := s.session.Delete(ctx, claims.Workspace, claims.ID); err != nil {
		return nil, fmt.Errorf("auth: rotate session: %w", err)
	}
	return s.issuePair(ctx, user)
}

// issuePair issues an access + refresh token pair and records the session.
func (s *Service) issuePair(ctx context.Context, user *User) (*TokenPair, error) {
	// Materialize the user's effective permissions (direct + role grants,
	// app-scoped) so the access token carries the concrete permission strings
	// (5.12.5).
	perms, err := s.permissionsForUser(ctx, user.WorkspaceID, user.App, user)
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
		App:         user.App,
		ExpiresAt:   claims.ExpiresAt.Time,
		CreatedAt:   now,
	}
	if err := s.session.Create(ctx, sess); err != nil {
		return nil, fmt.Errorf("auth: record session: %w", err)
	}
	// Concurrent session limit (todo 6.5.3): evict oldest sessions beyond the cap.
	if s.maxSessions > 0 {
		if err := s.enforceSessionLimit(ctx, user.ID, user.WorkspaceID); err != nil {
			return nil, err
		}
	}

	return &TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(s.issuer.AccessTTL().Seconds()),
	}, nil
}

// enforceSessionLimit evicts the oldest sessions when a user exceeds the
// configured concurrent session limit (todo 6.5.3).
func (s *Service) enforceSessionLimit(ctx context.Context, userID, workspaceID string) error {
	count, err := s.session.CountForUser(ctx, workspaceID, userID)
	if err != nil {
		return fmt.Errorf("auth: count sessions: %w", err)
	}
	if count <= s.maxSessions {
		return nil
	}
	sessions, err := s.session.ListForUser(ctx, workspaceID, userID)
	if err != nil {
		return fmt.Errorf("auth: list sessions: %w", err)
	}
	toEvict := count - s.maxSessions
	for i := 0; i < toEvict && i < len(sessions); i++ {
		if err := s.session.Delete(ctx, workspaceID, sessions[i].JTI); err != nil {
			return fmt.Errorf("auth: evict session: %w", err)
		}
	}
	return nil
}

// Logout revokes a single session (logout one device).
func (s *Service) Logout(ctx context.Context, workspaceID, jti string) error {
	return s.session.Delete(ctx, workspaceID, jti)
}

// LogoutAll revokes all sessions for a user (logout all devices, todo 6.5.4).
func (s *Service) LogoutAll(ctx context.Context, workspaceID, userID string) error {
	return s.session.DeleteForUser(ctx, workspaceID, userID)
}

// PurgeExpiredSessions deletes all expired sessions (todo 6.5.5).
func (s *Service) PurgeExpiredSessions(ctx context.Context) (int, error) {
	return s.session.PurgeExpired(ctx)
}

// StartSessionCleanup launches a background goroutine that purges expired
// sessions every interval (todo 6.5.5). Idempotent — calling twice is a no-op.
func (s *Service) StartSessionCleanup(interval time.Duration) {
	s.cleanupOnce.Do(func() {
		s.cleanupStop = make(chan struct{})
		go func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					_, _ = s.session.PurgeExpired(ctx)
					cancel()
				case <-s.cleanupStop:
					return
				}
			}
		}()
	})
}

// StopSessionCleanup stops the background session cleanup goroutine.
func (s *Service) StopSessionCleanup() {
	if s.cleanupStop != nil {
		close(s.cleanupStop)
	}
}
