package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/primadi/formspec/internal/auth/oauth"
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

// ErrWeakPassword is returned when a new password does not meet the minimum
// strength requirement (non-empty, at least 8 characters). Applied on
// register, change-password, and reset-password.
var ErrWeakPassword = errors.New("auth: password must be at least 8 characters")

// ErrInvalidResetToken is returned by ResetPassword when the reset token is
// unknown, expired, or already used.
var ErrInvalidResetToken = errors.New("auth: invalid or expired reset token")

// ErrEmailTaken is returned by Register when the email is already registered
// in the workspace (email is the OAuth linking key — duplicates are rejected
// at the service layer to keep GetByEmail unambiguous).
var ErrEmailTaken = errors.New("auth: email already registered")

// ErrEmailUnverified is returned by OAuth login when the matching account's
// email is unverified and the provider did not verify it either. An
// unverified email is a claim, not proof of ownership — linking would enable
// account pre-hijacking.
var ErrEmailUnverified = errors.New("auth: email is not verified")

// ErrAccountLinkRequired is returned by OAuth login when the provider email
// matches a verified account that has a password. The account is never
// silently merged with a different external identity — the user must sign in
// with the password and explicitly link the provider.
var ErrAccountLinkRequired = errors.New("auth: sign in with your password to link this account")

// ErrEmailMismatch is returned by LinkOAuthIdentity when the provider email
// does not match the signed-in account's email.
var ErrEmailMismatch = errors.New("auth: oauth email does not match account email")

// ErrIdentityTaken is returned by LinkOAuthIdentity when the external
// identity is already linked to a different account.
var ErrIdentityTaken = errors.New("auth: oauth identity already linked to another account")

// ErrNotLinked is returned by UnlinkOAuthIdentity when the requested provider
// is not the one linked to the account.
var ErrNotLinked = errors.New("auth: this provider is not linked to your account")

// ErrUnlinkRequiresPassword is returned by UnlinkOAuthIdentity when the
// account has no password — unlinking its only sign-in method would lock the
// user out. They must set a password first.
var ErrUnlinkRequiresPassword = errors.New("auth: set a password before unlinking your only sign-in method")

// ErrInvalidVerifyToken is returned by VerifyEmail when the verification
// token is unknown, expired, or already used.
var ErrInvalidVerifyToken = errors.New("auth: invalid or expired verification token")

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
	oauth       map[string]oauth.Provider
	mailer      Mailer

	// resetTokens holds single-use password-reset tokens (workspace-scoped,
	// TTL'd). In-memory — adequate for a single resource process; a
	// distributed deployment would back this with ctx.cache/ctx.db.
	resetTokens map[string]resetToken
	resetMu     sync.Mutex

	// verifyTokens holds single-use email-verification tokens (workspace-
	// scoped, TTL'd). Same in-memory trade-off as resetTokens.
	verifyTokens map[string]verifyToken
	verifyMu     sync.Mutex

	cleanupStop chan struct{}
	cleanupOnce sync.Once
}

// Mailer is the transactional email sender used by the auth service
// (password reset). *mail.Mailer (internal/mail) satisfies it; tests use a
// fake.
type Mailer interface {
	// Send delivers a message to one recipient.
	Send(to, subject, text, html string) error
	// BaseURL returns the public origin used to build links in emails.
	BaseURL() string
}

// resetToken is a single-use password-reset grant.
type resetToken struct {
	WorkspaceID string
	UserID      string
	Expires     time.Time
}

// resetTokenTTL is how long a password-reset token stays valid.
const resetTokenTTL = 15 * time.Minute

// verifyToken is a single-use email-verification grant.
type verifyToken struct {
	WorkspaceID string
	UserID      string
	Email       string
	Expires     time.Time
}

// verifyTokenTTL is how long an email-verification token stays valid.
const verifyTokenTTL = 24 * time.Hour

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

// SetOAuthProviders wires the configured external auth providers (auth
// redesign Fase 5). Keyed by provider name.
func (s *Service) SetOAuthProviders(providers map[string]oauth.Provider) {
	s.oauth = providers
}

// OAuthProvider returns a configured OAuth provider by name, or nil.
func (s *Service) OAuthProvider(name string) oauth.Provider {
	if s.oauth == nil {
		return nil
	}
	return s.oauth[name]
}

// OAuthProviders returns all configured OAuth provider names (for the login
// screen buttons).
func (s *Service) OAuthProviders() []string {
	out := make([]string, 0, len(s.oauth))
	for name := range s.oauth {
		out = append(out, name)
	}
	return out
}

// SetMailer wires the transactional email sender used for email flows
// (password reset). Nil (default) disables them — RequestPasswordReset
// becomes a no-op that never sends.
func (s *Service) SetMailer(m Mailer) {
	s.mailer = m
}

// GetUserByEmail looks up a user by email within a workspace. Used by admin
// tooling and tests.
func (s *Service) GetUserByEmail(ctx context.Context, workspaceID, email string) (*User, error) {
	return s.users.GetByEmail(ctx, workspaceID, email)
}

// GetUserByID looks up a user by ID within a workspace. Used by the meta
// identity endpoint to expose email-verification state.
func (s *Service) GetUserByID(ctx context.Context, workspaceID, id string) (*User, error) {
	return s.users.GetByID(ctx, workspaceID, id)
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
//
// email is optional but recommended. When provided it must be free within the
// workspace (ErrEmailTaken) and the account is created with
// EmailVerified=false — a verification email is sent (when a mailer is
// configured) so the address can be proven owned. An unverified email can
// never be linked via OAuth (account pre-hijacking protection).
func (s *Service) Register(ctx context.Context, workspaceID, username, email, password string) error {
	if err := ValidateUsername(username); err != nil {
		return err
	}
	if password == "" {
		return ErrInvalidCredentials
	}
	if _, err := s.users.GetByUsername(ctx, workspaceID, username); err == nil {
		return ErrUsernameTaken
	}
	if email != "" {
		if _, err := s.users.GetByEmail(ctx, workspaceID, email); err == nil {
			return ErrEmailTaken
		}
	}

	policy := s.regPolicy.Policy
	if policy == "" {
		policy = RegPolicyOpen
	}
	var user *User
	switch policy {
	case RegPolicyClosed:
		return ErrRegistrationClosed
	case RegPolicyApproval:
		user = &User{
			Username:    username,
			Email:       email,
			WorkspaceID: workspaceID,
			Active:      true,
			Status:      UserStatusPending,
			// PasswordHash carries the PLAINTEXT here — CreateUser hashes it.
			PasswordHash: password,
		}
	default: // open
		roles := []string{}
		if s.regPolicy.DefaultRole != "" {
			roles = append(roles, s.regPolicy.DefaultRole)
		}
		user = &User{
			Username:    username,
			Email:       email,
			WorkspaceID: workspaceID,
			Active:      true,
			Status:      UserStatusActive,
			Roles:       roles,
			// PasswordHash carries the PLAINTEXT here — CreateUser hashes it.
			PasswordHash: password,
		}
	}
	if err := s.users.CreateUser(ctx, workspaceID, user); err != nil {
		return err
	}
	// Email verification: the account starts unverified; a verification email
	// proves ownership. No-op when no mailer is configured (the account stays
	// usable by password, but its email can never be OAuth-linked).
	if email != "" {
		if err := s.RequestEmailVerification(ctx, workspaceID, user.Username); err != nil {
			return err
		}
	}
	return nil
}

// RequestEmailVerification generates a single-use verification token and
// emails a verification link to the user's address. Used at registration and
// for resend. No-op (nil) when the user has no email or no mailer is
// configured.
func (s *Service) RequestEmailVerification(ctx context.Context, workspaceID, username string) error {
	user, err := s.users.GetByUsername(ctx, workspaceID, username)
	if err != nil {
		return err
	}
	if user.Email == "" || s.mailer == nil {
		return nil
	}

	token, err := newResetToken()
	if err != nil {
		return err
	}
	s.verifyMu.Lock()
	if s.verifyTokens == nil {
		s.verifyTokens = map[string]verifyToken{}
	}
	s.verifyTokens[token] = verifyToken{
		WorkspaceID: workspaceID,
		UserID:      user.ID,
		Email:       user.Email,
		Expires:     time.Now().Add(verifyTokenTTL),
	}
	s.verifyMu.Unlock()

	link := fmt.Sprintf("%s/%s/verify-email?verify_token=%s", s.resetBaseURL(), workspaceID, token)
	subject := "Verify your email — FormSpec"
	text := "Confirm this email address for your FormSpec account.\n\n" +
		"Open this link to verify your email (valid for 24 hours):\n\n" +
		link + "\n\n" +
		"If you didn't create this account, you can safely ignore this email."
	html := "<p>Confirm this email address for your FormSpec account.</p>" +
		"<p>Open this link to verify your email (valid for 24 hours):</p>" +
		"<p><a href=\"" + link + "\">" + link + "</a></p>" +
		"<p>If you didn't create this account, you can safely ignore this email.</p>"
	return s.mailer.Send(user.Email, subject, text, html)
}

// VerifyEmail consumes a single-use verification token and marks the user's
// email as verified. Returns ErrInvalidVerifyToken when the token is unknown,
// expired, or used.
func (s *Service) VerifyEmail(ctx context.Context, workspaceID, token string) error {
	s.verifyMu.Lock()
	vt, ok := s.verifyTokens[token]
	if ok {
		delete(s.verifyTokens, token) // single-use
	}
	s.verifyMu.Unlock()
	if !ok {
		return ErrInvalidVerifyToken
	}
	if vt.WorkspaceID != workspaceID || time.Now().After(vt.Expires) {
		return ErrInvalidVerifyToken
	}
	return s.users.SetEmailVerified(ctx, workspaceID, vt.UserID, true)
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

// OAuthLogin completes an external auth login (auth redesign Fase 5): it
// exchanges the provider's authorization code for userinfo, then finds or
// creates the user and issues a token pair.
//
// Linking policy (account pre-hijacking protection — plan
// account-pre-hijacking-fix.md):
//
//  1. Identity match (provider, sub) → login directly (same external identity).
//  2. Email match:
//     - Email unverified + provider verified it → takeover: mark verified,
//     clear the previous claimant's password, attach identity, login.
//     - Email unverified + provider did not verify → ErrEmailUnverified.
//     - Email verified + account has a password → ErrAccountLinkRequired
//     (never silently merge a different identity into a password account).
//     - Email verified + account has no password (pure OAuth) → attach
//     identity, notify, login.
//  3. New user → created with a username derived from the email, status per
//     the registration policy (open → active, approval → pending), and
//     EmailVerified from the provider.
//
// Returns ErrInvalidCredentials when the provider is unknown or the user is
// inactive/pending.
func (s *Service) OAuthLogin(ctx context.Context, workspaceID, providerName, code string) (*TokenPair, error) {
	prov := s.OAuthProvider(providerName)
	if prov == nil {
		return nil, ErrInvalidCredentials
	}
	info, err := prov.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("oauth: %s: %w", providerName, err)
	}
	if info.Email == "" {
		return nil, fmt.Errorf("oauth: %s returned no email", providerName)
	}

	// 1. Identity match — the strongest signal: the same external identity
	// always maps to the same account.
	if info.ID != "" {
		if user, err := s.users.GetByOAuthIdentity(ctx, workspaceID, providerName, info.ID); err == nil {
			if !user.Active || user.Status == UserStatusPending {
				return nil, ErrInvalidCredentials
			}
			return s.issuePair(ctx, user)
		}
	}

	// 2. Email match — gated on email verification.
	if user, err := s.users.GetByEmail(ctx, workspaceID, info.Email); err == nil {
		if !user.Active || user.Status == UserStatusPending {
			return nil, ErrInvalidCredentials
		}
		if !user.EmailVerified {
			// Unverified email = a claim, not proof of ownership. Only a
			// provider-verified OAuth user may claim it (takeover).
			if !info.EmailVerified {
				return nil, ErrEmailUnverified
			}
			// Provider-verified takeover: the OAuth user owns the email, so
			// the unverified account is theirs. Clear any password the
			// previous claimant set so it stops working.
			if err := s.users.TakeoverUnverifiedEmail(ctx, workspaceID, user.ID, providerName, info.ID); err != nil {
				return nil, err
			}
			user.EmailVerified = true
			user.PasswordHash = ""
			user.OAuthProvider = providerName
			user.OAuthSub = info.ID
			s.notifyAccountLinked(ctx, user)
			return s.issuePair(ctx, user)
		}
		// Verified email. A password account is never silently merged with a
		// different external identity — require explicit linking.
		if userHasPassword(user) {
			return nil, ErrAccountLinkRequired
		}
		// Pure OAuth account (no password) — safe to attach a new identity.
		if info.ID != "" {
			if err := s.users.LinkOAuthIdentity(ctx, workspaceID, user.ID, providerName, info.ID); err != nil {
				return nil, err
			}
		}
		s.notifyAccountLinked(ctx, user)
		return s.issuePair(ctx, user)
	}

	// 3. New user — create with a derived username, status per policy.
	username := usernameFromEmail(info.Email)
	// Ensure uniqueness: append a numeric suffix if taken.
	for i := 2; ; i++ {
		if _, err := s.users.GetByUsername(ctx, workspaceID, username); err != nil {
			break
		}
		username = fmt.Sprintf("%s%d", usernameFromEmail(info.Email), i)
	}

	status := UserStatusActive
	if s.regPolicy.Policy == RegPolicyApproval {
		status = UserStatusPending
	}
	roles := []string{}
	if s.regPolicy.Policy == RegPolicyOpen && s.regPolicy.DefaultRole != "" {
		roles = append(roles, s.regPolicy.DefaultRole)
	}
	if err := s.users.CreateUser(ctx, workspaceID, &User{
		Username:      username,
		Email:         info.Email,
		EmailVerified: info.EmailVerified,
		DisplayName:   info.Name,
		WorkspaceID:   workspaceID,
		Active:        true,
		Status:        status,
		Roles:         roles,
		OAuthProvider: providerName,
		OAuthSub:      info.ID,
	}); err != nil {
		return nil, err
	}

	// Pending users cannot log in yet (approval policy).
	if status == UserStatusPending {
		return nil, ErrInvalidCredentials
	}
	user, err := s.users.GetByUsername(ctx, workspaceID, username)
	if err != nil {
		return nil, err
	}
	return s.issuePair(ctx, user)
}

// LinkOAuthIdentity explicitly links an external identity to the signed-in
// user's account (explicit account linking — the user signs in with their
// password, then links a provider). The provider email must match the
// account's verified email, and the identity must not already belong to
// another account.
func (s *Service) LinkOAuthIdentity(ctx context.Context, workspaceID, userID, providerName, code string) error {
	prov := s.OAuthProvider(providerName)
	if prov == nil {
		return ErrInvalidCredentials
	}
	info, err := prov.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("oauth: %s: %w", providerName, err)
	}
	if info.Email == "" {
		return fmt.Errorf("oauth: %s returned no email", providerName)
	}
	user, err := s.users.GetByID(ctx, workspaceID, userID)
	if err != nil {
		return ErrInvalidCredentials
	}
	// The provider email must match the account's verified email — otherwise
	// a user could attach someone else's identity.
	if !strings.EqualFold(user.Email, info.Email) {
		return ErrEmailMismatch
	}
	if !user.EmailVerified {
		return ErrEmailUnverified
	}
	// The identity must not already be linked to another account.
	if info.ID != "" {
		if other, err := s.users.GetByOAuthIdentity(ctx, workspaceID, providerName, info.ID); err == nil && other.ID != user.ID {
			return ErrIdentityTaken
		}
	}
	if info.ID != "" {
		if err := s.users.LinkOAuthIdentity(ctx, workspaceID, user.ID, providerName, info.ID); err != nil {
			return err
		}
	}
	s.notifyAccountLinked(ctx, user)
	return nil
}

// UnlinkOAuthIdentity detaches the external identity from the signed-in
// user's account (explicit unlink). The requested provider must be the one
// currently linked, and the account must keep at least one usable sign-in
// method — unlinking the only method (a pure-OAuth account with no password)
// is rejected with ErrUnlinkRequiresPassword.
func (s *Service) UnlinkOAuthIdentity(ctx context.Context, workspaceID, userID, providerName string) error {
	user, err := s.users.GetByID(ctx, workspaceID, userID)
	if err != nil {
		return ErrInvalidCredentials
	}
	if user.OAuthProvider == "" || !strings.EqualFold(user.OAuthProvider, providerName) {
		return ErrNotLinked
	}
	// A pure-OAuth account (no password) would be locked out — require a
	// password before removing its only sign-in method.
	if !userHasPassword(user) {
		return ErrUnlinkRequiresPassword
	}
	if err := s.users.UnlinkOAuthIdentity(ctx, workspaceID, user.ID); err != nil {
		return err
	}
	s.notifyAccountUnlinked(ctx, user)
	return nil
}

// notifyAccountUnlinked emails the account owner when an OAuth identity is
// removed from their account. No-op when no mailer is configured or the
// account has no email.
func (s *Service) notifyAccountUnlinked(ctx context.Context, user *User) {
	if s.mailer == nil || user.Email == "" {
		return
	}
	subject := "Sign-in method removed — FormSpec"
	text := "An external sign-in method was just removed from your FormSpec account.\n\n" +
		"If this was you, no action is needed.\n\n" +
		"If you didn't do this, someone may have accessed your account — " +
		"reset your password immediately and contact support."
	html := "<p>An external sign-in method was just removed from your FormSpec account.</p>" +
		"<p>If this was you, no action is needed.</p>" +
		"<p>If you didn't do this, someone may have accessed your account — " +
		"<strong>reset your password immediately</strong> and contact support.</p>"
	_ = s.mailer.Send(user.Email, subject, text, html)
}

// notifyAccountLinked emails the account owner when an OAuth identity is
// linked to their account (takeover or attach). No-op when no mailer is
// configured or the account has no email. This makes silent account merging
// visible to the owner.
func (s *Service) notifyAccountLinked(ctx context.Context, user *User) {
	if s.mailer == nil || user.Email == "" {
		return
	}
	subject := "New sign-in method linked — FormSpec"
	text := "Your FormSpec account was just linked to an external sign-in method.\n\n" +
		"If this was you, no action is needed.\n\n" +
		"If you didn't do this, someone may have accessed your account — " +
		"reset your password immediately and contact support."
	html := "<p>Your FormSpec account was just linked to an external sign-in method.</p>" +
		"<p>If this was you, no action is needed.</p>" +
		"<p>If you didn't do this, someone may have accessed your account — " +
		"<strong>reset your password immediately</strong> and contact support.</p>"
	_ = s.mailer.Send(user.Email, subject, text, html)
}

// usernameFromEmail derives a username from an email's local part, sanitized
// to the username pattern (letters, digits, . _ -). Falls back to "user".
func usernameFromEmail(email string) string {
	local := email
	if i := strings.IndexByte(email, '@'); i > 0 {
		local = email[:i]
	}
	// Sanitize: keep [a-zA-Z0-9._-], collapse others.
	var b strings.Builder
	for _, r := range local {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if len(out) < 3 {
		out = "user" + out
	}
	if len(out) > 32 {
		out = out[:32]
	}
	return out
}

// userHasPassword reports whether the account has a real (non-empty)
// password. OAuth-created and taken-over accounts store a hash of the empty
// string as the "no password" marker — a hash that only matches an empty
// password (which Login rejects), so it is not a usable credential.
func userHasPassword(u *User) bool {
	return u.PasswordHash != "" && !VerifyPassword(u.PasswordHash, "")
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
	// Reject empty password at the service layer (defense in depth — the
	// HTTP handler also checks). This also blocks OAuth-created users whose
	// password_hash is a bcrypt hash of "" from logging in with an empty
	// password.
	if password == "" {
		return nil, ErrInvalidCredentials
	}
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

// ChangePassword updates the caller's own password (self-service, profile).
//
//   - currentPassword is verified when the user has a password set (non-empty
//     password_hash). OAuth-created users (empty hash) skip the check — they
//     are already authenticated via session and have no password to verify.
//   - newPassword must be non-empty and at least 8 characters.
//
// Returns ErrInvalidCredentials on a wrong current password, ErrWeakPassword
// on a weak new password.
func (s *Service) ChangePassword(ctx context.Context, workspaceID, userID, currentPassword, newPassword string) error {
	if err := validateNewPassword(newPassword); err != nil {
		return err
	}
	user, err := s.users.GetByID(ctx, workspaceID, userID)
	if err != nil {
		return ErrInvalidCredentials
	}
	if user.PasswordHash != "" && !VerifyPassword(user.PasswordHash, currentPassword) {
		return ErrInvalidCredentials
	}
	return s.users.SetPassword(ctx, workspaceID, userID, newPassword)
}

// validateNewPassword enforces the shared password-strength rule: non-empty
// and at least 8 characters.
func validateNewPassword(pw string) error {
	if pw == "" {
		return ErrInvalidCredentials
	}
	if len(pw) < 8 {
		return ErrWeakPassword
	}
	return nil
}

// RequestPasswordReset starts the email-based reset flow: it generates a
// single-use token and emails a reset link to the user's address.
//
// It never reveals whether an email exists — unknown emails return nil
// (success) without sending. When no mailer is configured, it is a no-op
// (nil) so the endpoint stays uniform.
func (s *Service) RequestPasswordReset(ctx context.Context, workspaceID, email string) error {
	if email == "" || s.mailer == nil {
		return nil
	}
	user, err := s.users.GetByEmail(ctx, workspaceID, email)
	if err != nil {
		// Unknown email — do not leak existence.
		return nil
	}
	if user.Email == "" {
		return nil
	}

	token, err := newResetToken()
	if err != nil {
		return err
	}
	s.resetMu.Lock()
	if s.resetTokens == nil {
		s.resetTokens = map[string]resetToken{}
	}
	s.resetTokens[token] = resetToken{
		WorkspaceID: workspaceID,
		UserID:      user.ID,
		Expires:     time.Now().Add(resetTokenTTL),
	}
	s.resetMu.Unlock()

	// Note: the query param is `reset_token`, NOT `token` — the auth
	// middleware reads `?token=` as a JWT (WebSocket handshake), so a reset
	// token there would be rejected as malformed.
	link := fmt.Sprintf("%s/%s/reset-password?reset_token=%s", s.resetBaseURL(), workspaceID, token)
	subject := "Reset password — FormSpec"
	text := "Someone requested a password reset for your FormSpec account.\n\n" +
		"Open this link to choose a new password (valid for 15 minutes):\n\n" +
		link + "\n\n" +
		"If you didn't request this, you can safely ignore this email."
	html := "<p>Someone requested a password reset for your FormSpec account.</p>" +
		"<p>Open this link to choose a new password (valid for 15 minutes):</p>" +
		"<p><a href=\"" + link + "\">" + link + "</a></p>" +
		"<p>If you didn't request this, you can safely ignore this email.</p>"
	return s.mailer.Send(user.Email, subject, text, html)
}

// resetBaseURL returns the public base URL used to build reset links.
// Falls back to a relative link when unset (works when the SPA and API share
// an origin).
func (s *Service) resetBaseURL() string {
	if s.mailer == nil {
		return ""
	}
	return s.mailer.BaseURL()
}

// ResetPassword consumes a single-use reset token and sets a new password.
// Returns ErrInvalidResetToken when the token is unknown, expired, or used.
func (s *Service) ResetPassword(ctx context.Context, workspaceID, token, newPassword string) error {
	if err := validateNewPassword(newPassword); err != nil {
		return err
	}
	s.resetMu.Lock()
	rt, ok := s.resetTokens[token]
	if ok {
		delete(s.resetTokens, token) // single-use
	}
	s.resetMu.Unlock()
	if !ok {
		return ErrInvalidResetToken
	}
	if rt.WorkspaceID != workspaceID || time.Now().After(rt.Expires) {
		return ErrInvalidResetToken
	}
	return s.users.SetPassword(ctx, workspaceID, rt.UserID, newPassword)
}

// newResetToken returns a cryptographically random 32-byte hex token.
func newResetToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: generate reset token: %w", err)
	}
	return hex.EncodeToString(b), nil
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
