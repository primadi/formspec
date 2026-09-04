package auth

import (
	"context"
	"strings"
	"testing"
	"time"
)

// fakeMailer records sent messages for password-reset tests.
type fakeMailer struct {
	sent    []string // "to|subject|text"
	baseURL string
}

func (f *fakeMailer) Send(to, subject, text, html string) error {
	f.sent = append(f.sent, to+"|"+subject+"|"+text)
	return nil
}

func (f *fakeMailer) BaseURL() string { return f.baseURL }

func TestService_LoginEmptyPassword(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()
	if err := svc.SeedDevUser(ctx, "demo", "admin", "admin"); err != nil {
		t.Fatalf("SeedDevUser: %v", err)
	}

	// Empty password must be rejected at the service layer — even though the
	// stored hash is bcrypt("") for OAuth-created users, an empty password
	// must never authenticate.
	if _, err := svc.Login(ctx, "demo", "", "admin", ""); err == nil {
		t.Fatal("expected empty password to be rejected")
	}
}

func TestService_ChangePassword(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()
	if err := svc.SeedDevUser(ctx, "demo", "admin", "admin"); err != nil {
		t.Fatalf("SeedDevUser: %v", err)
	}
	user, err := svc.users.GetByUsername(ctx, "demo", "admin")
	if err != nil {
		t.Fatalf("GetByUsername: %v", err)
	}

	if err := svc.ChangePassword(ctx, "demo", user.ID, "admin", "newpass123"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	// Old password no longer works; new one does.
	if _, err := svc.Login(ctx, "demo", "", "admin", "admin"); err == nil {
		t.Fatal("old password should no longer work")
	}
	if _, err := svc.Login(ctx, "demo", "", "admin", "newpass123"); err != nil {
		t.Fatalf("new password should work: %v", err)
	}
}

func TestService_ChangePassword_WrongCurrent(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()
	if err := svc.SeedDevUser(ctx, "demo", "admin", "admin"); err != nil {
		t.Fatalf("SeedDevUser: %v", err)
	}
	user, _ := svc.users.GetByUsername(ctx, "demo", "admin")

	if err := svc.ChangePassword(ctx, "demo", user.ID, "wrong", "newpass123"); err == nil {
		t.Fatal("expected wrong current password to be rejected")
	}
}

func TestService_ChangePassword_Weak(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()
	if err := svc.SeedDevUser(ctx, "demo", "admin", "admin"); err != nil {
		t.Fatalf("SeedDevUser: %v", err)
	}
	user, _ := svc.users.GetByUsername(ctx, "demo", "admin")

	if err := svc.ChangePassword(ctx, "demo", user.ID, "admin", "short"); err == nil {
		t.Fatal("expected weak password to be rejected")
	}
}

func TestService_ChangePassword_OAuthUser(t *testing.T) {
	// OAuth-created users have an empty password_hash — they can set a
	// password without a current password (they are already authenticated).
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()
	if err := svc.users.CreateUser(ctx, "demo", &User{
		Username:    "oauthuser",
		Email:       "oauth@example.com",
		WorkspaceID: "demo",
		Active:      true,
		Status:      UserStatusActive,
	}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	user, _ := svc.users.GetByUsername(ctx, "demo", "oauthuser")

	if err := svc.ChangePassword(ctx, "demo", user.ID, "", "newpass123"); err != nil {
		t.Fatalf("OAuth user should set a password without current: %v", err)
	}
	if _, err := svc.Login(ctx, "demo", "", "oauthuser", "newpass123"); err != nil {
		t.Fatalf("new password should work: %v", err)
	}
}

func TestService_RequestPasswordReset_UnknownEmail(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	svc.SetMailer(&fakeMailer{baseURL: "http://localhost:18080"})
	ctx := context.Background()

	// Unknown email → nil (no leak), no email sent.
	if err := svc.RequestPasswordReset(ctx, "demo", "nobody@example.com"); err != nil {
		t.Fatalf("RequestPasswordReset: %v", err)
	}
	if m := svc.mailer.(*fakeMailer); len(m.sent) != 0 {
		t.Fatalf("expected no email for unknown address, got %d", len(m.sent))
	}
}

func TestService_RequestPasswordReset_NoMailer(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()
	if err := svc.SeedDevUser(ctx, "demo", "admin", "admin"); err != nil {
		t.Fatalf("SeedDevUser: %v", err)
	}
	// No mailer wired → no-op, no error.
	if err := svc.RequestPasswordReset(ctx, "demo", "admin@example.com"); err != nil {
		t.Fatalf("RequestPasswordReset: %v", err)
	}
}

func TestService_ResetPassword_FullFlow(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	m := &fakeMailer{baseURL: "http://localhost:18080"}
	svc.SetMailer(m)
	ctx := context.Background()
	if err := svc.SeedDevUser(ctx, "demo", "admin", "admin"); err != nil {
		t.Fatalf("SeedDevUser: %v", err)
	}
	// Give the seeded user an email.
	user, _ := svc.users.GetByUsername(ctx, "demo", "admin")
	if err := svc.users.UpdateUser(ctx, "demo", &User{
		ID: user.ID, Username: "admin", Email: "admin@example.com",
		Active: true, Status: UserStatusActive,
	}); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}

	if err := svc.RequestPasswordReset(ctx, "demo", "admin@example.com"); err != nil {
		t.Fatalf("RequestPasswordReset: %v", err)
	}
	if len(m.sent) != 1 {
		t.Fatalf("expected 1 email, got %d", len(m.sent))
	}
	if !strings.Contains(m.sent[0], "admin@example.com") {
		t.Fatalf("email should be addressed to admin@example.com: %q", m.sent[0])
	}
	if !strings.Contains(m.sent[0], "reset-password?reset_token=") {
		t.Fatalf("email should contain a reset link: %q", m.sent[0])
	}

	// Extract the token from the email and use it.
	rest := m.sent[0][strings.Index(m.sent[0], "reset-password?reset_token=")+len("reset-password?reset_token="):]
	token := strings.Fields(rest)[0]

	if err := svc.ResetPassword(ctx, "demo", token, "resetpass123"); err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}
	if _, err := svc.Login(ctx, "demo", "", "admin", "resetpass123"); err != nil {
		t.Fatalf("reset password should work: %v", err)
	}

	// Token is single-use — second attempt fails.
	if err := svc.ResetPassword(ctx, "demo", token, "another123"); err == nil {
		t.Fatal("expected reused token to be rejected")
	}
}

func TestService_ResetPassword_InvalidToken(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()
	if err := svc.ResetPassword(ctx, "demo", "bogus-token", "newpass123"); err == nil {
		t.Fatal("expected invalid token to be rejected")
	}
}

func TestService_ResetPassword_Expired(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()
	if err := svc.SeedDevUser(ctx, "demo", "admin", "admin"); err != nil {
		t.Fatalf("SeedDevUser: %v", err)
	}
	user, _ := svc.users.GetByUsername(ctx, "demo", "admin")

	// Inject an already-expired token directly.
	svc.resetMu.Lock()
	if svc.resetTokens == nil {
		svc.resetTokens = map[string]resetToken{}
	}
	svc.resetTokens["expired-token"] = resetToken{
		WorkspaceID: "demo",
		UserID:      user.ID,
		Expires:     time.Now().Add(-time.Minute),
	}
	svc.resetMu.Unlock()

	if err := svc.ResetPassword(ctx, "demo", "expired-token", "newpass123"); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}
