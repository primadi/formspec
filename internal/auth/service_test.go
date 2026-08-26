package auth

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/internal/entity"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// setupAuthService builds a registry with core entities synced and an auth
// service wired to it, backed by an in-memory SQLite database.
func setupAuthService(t *testing.T) (*Service, *entity.Registry, db.DB) {
	t.Helper()
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "auth_test.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	reg := entity.NewRegistry(d, db.DriverSQLite, "")
	if err := RegisterCoreEntities(reg); err != nil {
		t.Fatalf("RegisterCoreEntities: %v", err)
	}
	if _, err := reg.SyncSchema(context.Background()); err != nil {
		t.Fatalf("SyncSchema: %v", err)
	}

	roles := NewRoleResolver(reg)
	svc, err := NewService(roles, NewTokenIssuer("test-secret", "formspec", "", 0, 0))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc, reg, d
}

func TestService_LoginSuccess(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()

	if err := svc.SeedDevUser(ctx, "demo", "admin", "admin"); err != nil {
		t.Fatalf("SeedDevUser: %v", err)
	}

	pair, err := svc.Login(ctx, "demo", "", "admin", "admin")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("expected non-empty tokens")
	}
	if pair.ExpiresIn <= 0 {
		t.Fatalf("expected positive ExpiresIn, got %d", pair.ExpiresIn)
	}
}

func TestService_LoginWrongPassword(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()

	if err := svc.SeedDevUser(ctx, "demo", "admin", "admin"); err != nil {
		t.Fatalf("SeedDevUser: %v", err)
	}

	if _, err := svc.Login(ctx, "demo", "", "admin", "wrong"); err != ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestService_LoginUnknownUser(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()

	// Same error as wrong password — no user enumeration.
	if _, err := svc.Login(ctx, "demo", "", "nobody", "x"); err != ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestService_RefreshRotation(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()

	if err := svc.SeedDevUser(ctx, "demo", "admin", "admin"); err != nil {
		t.Fatalf("SeedDevUser: %v", err)
	}

	pair, err := svc.Login(ctx, "demo", "", "admin", "admin")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	// Refresh with the valid token → new pair.
	pair2, err := svc.Refresh(ctx, pair.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if pair2.RefreshToken == pair.RefreshToken {
		t.Fatal("expected rotated refresh token to differ")
	}

	// The old refresh token must now be rejected (rotated/invalidated).
	if _, err := svc.Refresh(ctx, pair.RefreshToken); err != ErrSessionRevoked {
		t.Fatalf("expected ErrSessionRevoked for replayed token, got %v", err)
	}
}

func TestService_RefreshInvalidToken(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()

	if _, err := svc.Refresh(ctx, "not-a-jwt"); err != ErrInvalidRefreshToken {
		t.Fatalf("expected ErrInvalidRefreshToken, got %v", err)
	}
}

func TestService_SeedDevUserIdempotent(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()

	if err := svc.SeedDevUser(ctx, "demo", "admin", "admin"); err != nil {
		t.Fatalf("SeedDevUser #1: %v", err)
	}
	// Second seed must not error (idempotent).
	if err := svc.SeedDevUser(ctx, "demo", "admin", "admin"); err != nil {
		t.Fatalf("SeedDevUser #2: %v", err)
	}
}

func TestRoleResolver_Override(t *testing.T) {
	svc, reg, _ := setupAuthService(t)
	_ = svc

	roles := NewRoleResolver(reg)
	// Default resolves to formspec.core.user.
	store, err := roles.Resolve(RoleUser)
	if err != nil {
		t.Fatalf("Resolve default: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}

	// Explicit override to a non-existent entity → error.
	roles.SetOverride(RoleUser, "nope/missing")
	if _, err := roles.Resolve(RoleUser); err == nil {
		t.Fatal("expected error for unresolvable override")
	}
}
