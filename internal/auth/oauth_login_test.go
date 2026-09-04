package auth

import (
	"context"
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/auth/oauth"
)

// mockOAuthProvider is a test double for oauth.Provider.
type mockOAuthProvider struct {
	name string
	info *oauth.UserInfo
	err  error
}

func (m *mockOAuthProvider) Name() string { return m.name }
func (m *mockOAuthProvider) AuthorizeURL(state, redirectURL string) string {
	return "https://mock.example/authorize?state=" + state + "&redirect=" + redirectURL
}
func (m *mockOAuthProvider) Exchange(_ context.Context, _ string) (*oauth.UserInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.info, nil
}

// oauthInfo is a shorthand for a provider-verified identity.
func oauthInfo(id, email string) *oauth.UserInfo {
	return &oauth.UserInfo{ID: id, Email: email, Name: "OAuth User", EmailVerified: true}
}

func TestService_OAuthLogin_NewUser(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()

	svc.SetOAuthProviders(map[string]oauth.Provider{
		"github": &mockOAuthProvider{name: "github", info: oauthInfo("gh-1", "dev@example.com")},
	})

	pair, err := svc.OAuthLogin(ctx, "demo", "github", "code123")
	if err != nil {
		t.Fatalf("OAuthLogin: %v", err)
	}
	if pair.AccessToken == "" {
		t.Fatal("expected access token")
	}

	// User created with derived username + email + provider identity.
	user, err := svc.users.GetByEmail(ctx, "demo", "dev@example.com")
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if user.Username != "dev" {
		t.Errorf("expected username 'dev', got %q", user.Username)
	}
	if user.Status != UserStatusActive {
		t.Errorf("expected active status, got %q", user.Status)
	}
	if !user.EmailVerified {
		t.Error("expected provider-verified email on new user")
	}
	if user.OAuthProvider != "github" || user.OAuthSub != "gh-1" {
		t.Errorf("expected oauth identity github/gh-1, got %s/%s", user.OAuthProvider, user.OAuthSub)
	}
}

func TestService_OAuthLogin_SameIdentity(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()

	// Seed a user already linked to this external identity.
	if err := svc.users.CreateUser(ctx, "demo", &User{
		Username:      "existing",
		Email:         "dev@example.com",
		EmailVerified: true,
		WorkspaceID:   "demo",
		Active:        true,
		Status:        UserStatusActive,
		OAuthProvider: "github",
		OAuthSub:      "gh-1",
	}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	svc.SetOAuthProviders(map[string]oauth.Provider{
		"github": &mockOAuthProvider{name: "github", info: oauthInfo("gh-1", "dev@example.com")},
	})

	pair, err := svc.OAuthLogin(ctx, "demo", "github", "code123")
	if err != nil {
		t.Fatalf("OAuthLogin: %v", err)
	}
	if pair.AccessToken == "" {
		t.Fatal("expected access token")
	}

	// No duplicate user created; the same account is returned.
	user, err := svc.users.GetByEmail(ctx, "demo", "dev@example.com")
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if user.Username != "existing" {
		t.Errorf("expected existing user, got %q", user.Username)
	}
}

func TestService_OAuthLogin_UnverifiedEmail_Blocked(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()

	// A user registered with an email that was never verified (e.g. a
	// hacker's claim). The provider also does not verify the email.
	if err := svc.users.CreateUser(ctx, "demo", &User{
		Username:    "claimed",
		Email:       "victim@example.com",
		WorkspaceID: "demo",
		Active:      true,
		Status:      UserStatusActive,
		// PasswordHash carries the PLAINTEXT here — CreateUser hashes it.
		PasswordHash: "hackerpass123",
	}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	svc.SetOAuthProviders(map[string]oauth.Provider{
		"google": &mockOAuthProvider{name: "google", info: &oauth.UserInfo{
			ID: "g-1", Email: "victim@example.com", Name: "Victim", EmailVerified: false,
		}},
	})

	// The real owner's OAuth login must NOT land in the unverified account.
	if _, err := svc.OAuthLogin(ctx, "demo", "google", "code123"); err != ErrEmailUnverified {
		t.Fatalf("expected ErrEmailUnverified, got %v", err)
	}
}

func TestService_OAuthLogin_UnverifiedEmail_Takeover(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()

	// A hacker registered victim@example.com with a password (unverified).
	if err := svc.users.CreateUser(ctx, "demo", &User{
		Username:    "claimed",
		Email:       "victim@example.com",
		WorkspaceID: "demo",
		Active:      true,
		Status:      UserStatusActive,
		// PasswordHash carries the PLAINTEXT here — CreateUser hashes it.
		PasswordHash: "hackerpass123",
	}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// The real owner signs in via Google (provider-verified email) → takeover.
	svc.SetOAuthProviders(map[string]oauth.Provider{
		"google": &mockOAuthProvider{name: "google", info: oauthInfo("g-1", "victim@example.com")},
	})

	pair, err := svc.OAuthLogin(ctx, "demo", "google", "code123")
	if err != nil {
		t.Fatalf("OAuthLogin: %v", err)
	}
	if pair.AccessToken == "" {
		t.Fatal("expected access token")
	}

	user, err := svc.users.GetByEmail(ctx, "demo", "victim@example.com")
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if !user.EmailVerified {
		t.Error("expected email verified after takeover")
	}
	if userHasPassword(user) {
		t.Error("expected the previous claimant's password to be cleared")
	}
	if user.OAuthProvider != "google" || user.OAuthSub != "g-1" {
		t.Errorf("expected oauth identity google/g-1, got %s/%s", user.OAuthProvider, user.OAuthSub)
	}
	// The hacker's password must no longer authenticate.
	if _, err := svc.Login(ctx, "demo", "", "claimed", "hackerpass123"); err == nil {
		t.Fatal("hacker's password should no longer work after takeover")
	}
}

func TestService_OAuthLogin_VerifiedPasswordAccount_LinkRequired(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()

	// A verified account with a password — a different external identity is
	// never silently merged into it.
	if err := svc.users.CreateUser(ctx, "demo", &User{
		Username:      "owner",
		Email:         "owner@example.com",
		EmailVerified: true,
		WorkspaceID:   "demo",
		Active:        true,
		Status:        UserStatusActive,
		// PasswordHash carries the PLAINTEXT here — CreateUser hashes it.
		PasswordHash: "ownerpass123",
	}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	svc.SetOAuthProviders(map[string]oauth.Provider{
		"google": &mockOAuthProvider{name: "google", info: oauthInfo("g-1", "owner@example.com")},
	})

	if _, err := svc.OAuthLogin(ctx, "demo", "google", "code123"); err != ErrAccountLinkRequired {
		t.Fatalf("expected ErrAccountLinkRequired, got %v", err)
	}
}

func TestService_OAuthLogin_VerifiedNoPassword_AttachIdentity(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()

	// A pure OAuth account (verified email, no password) — a second provider
	// with the same verified email may attach.
	if err := svc.users.CreateUser(ctx, "demo", &User{
		Username:      "oauthuser",
		Email:         "dev@example.com",
		EmailVerified: true,
		WorkspaceID:   "demo",
		Active:        true,
		Status:        UserStatusActive,
		OAuthProvider: "github",
		OAuthSub:      "gh-1",
	}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	svc.SetOAuthProviders(map[string]oauth.Provider{
		"google": &mockOAuthProvider{name: "google", info: oauthInfo("g-1", "dev@example.com")},
	})

	pair, err := svc.OAuthLogin(ctx, "demo", "google", "code123")
	if err != nil {
		t.Fatalf("OAuthLogin: %v", err)
	}
	if pair.AccessToken == "" {
		t.Fatal("expected access token")
	}

	user, err := svc.users.GetByEmail(ctx, "demo", "dev@example.com")
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if user.OAuthProvider != "google" || user.OAuthSub != "g-1" {
		t.Errorf("expected attached identity google/g-1, got %s/%s", user.OAuthProvider, user.OAuthSub)
	}
}

func TestService_OAuthLogin_UnknownProvider(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()

	_, err := svc.OAuthLogin(ctx, "demo", "nonexistent", "code")
	if err != ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestService_OAuthLogin_ApprovalPolicy_Pending(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()

	svc.SetRegistrationPolicy(RegistrationPolicy{Policy: RegPolicyApproval})
	svc.SetOAuthProviders(map[string]oauth.Provider{
		"github": &mockOAuthProvider{
			name: "github",
			info: &oauth.UserInfo{ID: "gh-1", Email: "pending@example.com", Name: "Pending"},
		},
	})

	// New user under approval policy → pending → cannot log in yet.
	_, err := svc.OAuthLogin(ctx, "demo", "github", "code123")
	if err != ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials (pending), got %v", err)
	}

	// But the user record exists with pending status.
	user, err := svc.users.GetByEmail(ctx, "demo", "pending@example.com")
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if user.Status != UserStatusPending {
		t.Errorf("expected pending status, got %q", user.Status)
	}
}

func TestService_LinkOAuthIdentity(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()

	// A verified password account — the user signs in, then explicitly links.
	if err := svc.users.CreateUser(ctx, "demo", &User{
		Username:      "owner",
		Email:         "owner@example.com",
		EmailVerified: true,
		WorkspaceID:   "demo",
		Active:        true,
		Status:        UserStatusActive,
		// PasswordHash carries the PLAINTEXT here — CreateUser hashes it.
		PasswordHash: "ownerpass123",
	}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	user, _ := svc.users.GetByUsername(ctx, "demo", "owner")

	svc.SetOAuthProviders(map[string]oauth.Provider{
		"google": &mockOAuthProvider{name: "google", info: oauthInfo("g-1", "owner@example.com")},
	})

	if err := svc.LinkOAuthIdentity(ctx, "demo", user.ID, "google", "code123"); err != nil {
		t.Fatalf("LinkOAuthIdentity: %v", err)
	}

	// The identity is now linked — a subsequent OAuth login works directly.
	pair, err := svc.OAuthLogin(ctx, "demo", "google", "code123")
	if err != nil {
		t.Fatalf("OAuthLogin after link: %v", err)
	}
	if pair.AccessToken == "" {
		t.Fatal("expected access token")
	}
}

func TestService_LinkOAuthIdentity_EmailMismatch(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()

	if err := svc.users.CreateUser(ctx, "demo", &User{
		Username:      "owner",
		Email:         "owner@example.com",
		EmailVerified: true,
		WorkspaceID:   "demo",
		Active:        true,
		Status:        UserStatusActive,
		PasswordHash:  "ownerpass123",
	}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	user, _ := svc.users.GetByUsername(ctx, "demo", "owner")

	// Provider email differs from the account email → reject.
	svc.SetOAuthProviders(map[string]oauth.Provider{
		"google": &mockOAuthProvider{name: "google", info: oauthInfo("g-1", "other@example.com")},
	})

	if err := svc.LinkOAuthIdentity(ctx, "demo", user.ID, "google", "code123"); err != ErrEmailMismatch {
		t.Fatalf("expected ErrEmailMismatch, got %v", err)
	}
}

func TestService_LinkOAuthIdentity_IdentityTaken(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()

	// The identity already belongs to another account.
	if err := svc.users.CreateUser(ctx, "demo", &User{
		Username:      "other",
		Email:         "other@example.com",
		EmailVerified: true,
		WorkspaceID:   "demo",
		Active:        true,
		Status:        UserStatusActive,
		OAuthProvider: "google",
		OAuthSub:      "g-1",
	}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := svc.users.CreateUser(ctx, "demo", &User{
		Username:      "owner",
		Email:         "owner@example.com",
		EmailVerified: true,
		WorkspaceID:   "demo",
		Active:        true,
		Status:        UserStatusActive,
		PasswordHash:  "ownerpass123",
	}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	user, _ := svc.users.GetByUsername(ctx, "demo", "owner")

	svc.SetOAuthProviders(map[string]oauth.Provider{
		"google": &mockOAuthProvider{name: "google", info: oauthInfo("g-1", "owner@example.com")},
	})

	if err := svc.LinkOAuthIdentity(ctx, "demo", user.ID, "google", "code123"); err != ErrIdentityTaken {
		t.Fatalf("expected ErrIdentityTaken, got %v", err)
	}
}

func TestService_UnlinkOAuthIdentity(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()

	// A verified password account with a linked Google identity.
	if err := svc.users.CreateUser(ctx, "demo", &User{
		Username:      "owner",
		Email:         "owner@example.com",
		EmailVerified: true,
		WorkspaceID:   "demo",
		Active:        true,
		Status:        UserStatusActive,
		PasswordHash:  "ownerpass123",
		OAuthProvider: "google",
		OAuthSub:      "g-1",
	}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	user, _ := svc.users.GetByUsername(ctx, "demo", "owner")

	if err := svc.UnlinkOAuthIdentity(ctx, "demo", user.ID, "google"); err != nil {
		t.Fatalf("UnlinkOAuthIdentity: %v", err)
	}

	// The identity is gone — the user is password-only again.
	user, _ = svc.users.GetByUsername(ctx, "demo", "owner")
	if user.OAuthProvider != "" || user.OAuthSub != "" {
		t.Errorf("expected cleared identity, got provider=%q sub=%q",
			user.OAuthProvider, user.OAuthSub)
	}
}

func TestService_UnlinkOAuthIdentity_NotLinked(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()

	// Password account with NO external identity.
	if err := svc.users.CreateUser(ctx, "demo", &User{
		Username:      "owner",
		Email:         "owner@example.com",
		EmailVerified: true,
		WorkspaceID:   "demo",
		Active:        true,
		Status:        UserStatusActive,
		PasswordHash:  "ownerpass123",
	}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	user, _ := svc.users.GetByUsername(ctx, "demo", "owner")

	if err := svc.UnlinkOAuthIdentity(ctx, "demo", user.ID, "google"); err != ErrNotLinked {
		t.Fatalf("expected ErrNotLinked, got %v", err)
	}
}

func TestService_UnlinkOAuthIdentity_RequiresPassword(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()

	// A pure-OAuth account (no password) — unlinking would lock the user out.
	if err := svc.users.CreateUser(ctx, "demo", &User{
		Username:      "oauthuser",
		Email:         "oauth@example.com",
		EmailVerified: true,
		WorkspaceID:   "demo",
		Active:        true,
		Status:        UserStatusActive,
		OAuthProvider: "google",
		OAuthSub:      "g-1",
	}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	user, _ := svc.users.GetByUsername(ctx, "demo", "oauthuser")

	if err := svc.UnlinkOAuthIdentity(ctx, "demo", user.ID, "google"); err != ErrUnlinkRequiresPassword {
		t.Fatalf("expected ErrUnlinkRequiresPassword, got %v", err)
	}
}

func TestService_Register_WithEmail_SendsVerification(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()
	m := &fakeMailer{baseURL: "http://localhost:18080"}
	svc.SetMailer(m)

	if err := svc.Register(ctx, "demo", "newuser", "new@example.com", "password123"); err != nil {
		t.Fatalf("Register: %v", err)
	}

	user, err := svc.users.GetByUsername(ctx, "demo", "newuser")
	if err != nil {
		t.Fatalf("GetByUsername: %v", err)
	}
	if user.EmailVerified {
		t.Error("expected new account to start unverified")
	}
	// A verification email was sent.
	if len(m.sent) != 1 {
		t.Fatalf("expected 1 verification email, got %d", len(m.sent))
	}
	if !strings.Contains(m.sent[0], "Verify your email") {
		t.Errorf("expected verification subject, got %q", m.sent[0])
	}
}

func TestService_VerifyEmail(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()
	m := &fakeMailer{baseURL: "http://localhost:18080"}
	svc.SetMailer(m)

	if err := svc.Register(ctx, "demo", "newuser", "new@example.com", "password123"); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Extract the token from the emailed link.
	token := extractTokenFromMail(t, m.sent[0])
	if err := svc.VerifyEmail(ctx, "demo", token); err != nil {
		t.Fatalf("VerifyEmail: %v", err)
	}

	user, err := svc.users.GetByUsername(ctx, "demo", "newuser")
	if err != nil {
		t.Fatalf("GetByUsername: %v", err)
	}
	if !user.EmailVerified {
		t.Error("expected email verified after VerifyEmail")
	}

	// Token is single-use.
	if err := svc.VerifyEmail(ctx, "demo", token); err != ErrInvalidVerifyToken {
		t.Fatalf("expected ErrInvalidVerifyToken on reuse, got %v", err)
	}
}

func TestService_Register_EmailTaken(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()

	if err := svc.Register(ctx, "demo", "first", "dup@example.com", "password123"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := svc.Register(ctx, "demo", "second", "dup@example.com", "password123"); err != ErrEmailTaken {
		t.Fatalf("expected ErrEmailTaken, got %v", err)
	}
}

// extractTokenFromMail pulls the verify_token query param out of an emailed
// link (the fakeMailer records "to|subject|text").
func extractTokenFromMail(t *testing.T, mail string) string {
	t.Helper()
	idx := strings.Index(mail, "verify_token=")
	if idx < 0 {
		t.Fatalf("no verify_token in mail: %q", mail)
	}
	rest := mail[idx+len("verify_token="):]
	if sp := strings.IndexAny(rest, " \n\r"); sp >= 0 {
		rest = rest[:sp]
	}
	return rest
}
