package formspec

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/internal/api"
	"github.com/primadi/formspec/internal/auth"
)

// TestUserPasswordHashHook verifies that creating a user via the entity API
// with a plaintext `password` field hashes it into `password_hash` (the
// before-create native hook), never stores plaintext, and the resulting user
// can log in.
func TestUserPasswordHashHook(t *testing.T) {
	dir := t.TempDir()
	buildAuthSpecDir(t, dir)
	api.ResetAuthRateLimiters()

	app, err := New(Config{
		SpecPath:  dir,
		DSN:       "sqlite:" + filepath.Join(t.TempDir(), "user_hash.db"),
		DevAuth:   true,
		JWTSecret: "test-secret",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer app.Close(context.Background())

	adminTok := login(t, app, "admin", "admin")

	// Create a user with a plaintext password via the entity API.
	status, out := doAuthed(t, app, "POST", "/demo/_ui/entity/formspec.core/user", adminTok, map[string]any{
		"username":     "alice",
		"password":     "secret123",
		"display_name": "Alice",
		"active":       true,
	})
	if status != 201 {
		t.Fatalf("create user: status %d, body %v", status, out)
	}
	rec, _ := out["data"].(map[string]any)
	if rec == nil {
		t.Fatalf("create user: no record in response: %v", out)
	}
	if _, hasPw := rec["password"]; hasPw {
		t.Fatalf("plaintext password leaked into stored record")
	}
	// password_hash is masked in the response but must be present (non-empty).
	hash, _ := rec["password_hash"].(string)
	if hash == "" {
		t.Fatalf("expected password_hash in stored record")
	}

	// The new user can log in with the plaintext password.
	aliceTok := login(t, app, "alice", "secret123")
	if aliceTok == "" {
		t.Fatal("expected alice login to succeed")
	}
}

// TestDevAuth_ValidatorSelection verifies that DevAuth opts into real JWT auth
// in dev mode (JWTValidator) while plain dev mode keeps the DevValidator
// bypass, and that a missing secret is auto-generated so issued tokens
// validate against the middleware.
func TestDevAuth_ValidatorSelection(t *testing.T) {
	// Plain dev mode → DevValidator (auth bypass preserved).
	api.SetAuthValidator(nil)
	if err := configureAuth(Config{}); err != nil {
		t.Fatalf("configureAuth dev: %v", err)
	}
	if _, ok := api.GetAuthValidator().(*auth.DevValidator); !ok {
		t.Fatalf("expected DevValidator in dev mode, got %T", api.GetAuthValidator())
	}

	// DevAuth with explicit secret → JWTValidator.
	api.SetAuthValidator(nil)
	if err := configureAuth(Config{DevAuth: true, JWTSecret: "test-secret"}); err != nil {
		t.Fatalf("configureAuth dev-auth: %v", err)
	}
	if _, ok := api.GetAuthValidator().(*auth.JWTValidator); !ok {
		t.Fatalf("expected JWTValidator in dev-auth mode, got %T", api.GetAuthValidator())
	}

	// DevAuth without a secret → New() auto-generates one, still JWTValidator.
	dir := t.TempDir()
	buildAuthSpecDir(t, dir)
	app, err := New(Config{
		SpecPath: dir,
		DSN:      "sqlite:" + filepath.Join(t.TempDir(), "dev_auth.db"),
		DevAuth:  true,
	})
	if err != nil {
		t.Fatalf("New(DevAuth): %v", err)
	}
	defer app.Close(context.Background())
	if _, ok := api.GetAuthValidator().(*auth.JWTValidator); !ok {
		t.Fatalf("expected JWTValidator after New(DevAuth), got %T", api.GetAuthValidator())
	}
}

// TestDevAuth_LoginFlow verifies the full auth flow in dev-auth mode: an
// anonymous request resolves to the "anonymous" identity (no permissions),
// the seeded dev user (admin/admin) can log in, and the issued token
// authenticates subsequent requests as the real user.
func TestDevAuth_LoginFlow(t *testing.T) {
	dir := t.TempDir()
	buildAuthSpecDir(t, dir)
	api.ResetAuthRateLimiters()

	app, err := New(Config{
		SpecPath:  dir,
		DSN:       "sqlite:" + filepath.Join(t.TempDir(), "dev_auth_flow.db"),
		DevAuth:   true,
		JWTSecret: "test-secret",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer app.Close(context.Background())

	// Anonymous → identity is "anonymous" with no permissions (auth enforced,
	// unlike plain dev mode which returns the synthetic developer identity).
	status, out := doJSON(t, app, "GET", "/demo/_ui/_meta/me", nil)
	if status != 200 {
		t.Fatalf("expected 200 for anonymous _meta/me, got %d", status)
	}
	data, _ := out["data"].(map[string]any)
	if uid, _ := data["user_id"].(string); uid != "anonymous" {
		t.Fatalf("expected anonymous identity, got user_id=%q", uid)
	}

	// Seeded dev user logs in (SeedDevUser runs because !ProdMode).
	adminTok := login(t, app, "admin", "admin")

	// Authenticated → identity is the real user (internal user UUID, not the
	// "anonymous" sentinel).
	status, out = doAuthed(t, app, "GET", "/demo/_ui/_meta/me", adminTok, nil)
	if status != 200 {
		t.Fatalf("expected 200 for authed _meta/me, got %d (body %v)", status, out)
	}
	data, _ = out["data"].(map[string]any)
	if uid, _ := data["user_id"].(string); uid == "" || uid == "anonymous" {
		t.Fatalf("expected a real user identity, got user_id=%q", uid)
	}
}
