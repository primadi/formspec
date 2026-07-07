package auth

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestJWTValidator_Validate(t *testing.T) {
	secret := "test-secret-key"
	issuer := "forma-test"
	validator := NewJWTValidator(secret, issuer, "")

	// Helper: create a valid token
	makeToken := func(sub, workspace string, perms, roles []string) string {
		claims := jwt.MapClaims{
			"sub": sub,
			"ws":  workspace,
			"iss": issuer,
			"iat": time.Now().Unix(),
			"exp": time.Now().Add(time.Hour).Unix(),
		}
		if len(perms) > 0 {
			claims["perms"] = perms
		}
		if len(roles) > 0 {
			claims["roles"] = roles
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signed, err := token.SignedString([]byte(secret))
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}
		return signed
	}

	ctx := context.Background()

	t.Run("valid token with all claims", func(t *testing.T) {
		token := makeToken("user-123", "workspace-abc",
			[]string{"billing.invoices.*", "billing.customers.list"},
			[]string{"billing-admin"})

		id, err := validator.Validate(ctx, token)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id.UserID != "user-123" {
			t.Errorf("UserID = %q, want %q", id.UserID, "user-123")
		}
		if id.WorkspaceID != "workspace-abc" {
			t.Errorf("WorkspaceID = %q, want %q", id.WorkspaceID, "workspace-abc")
		}
		if len(id.Permissions) != 2 {
			t.Errorf("Permissions len = %d, want 2", len(id.Permissions))
		}
		if len(id.Roles) != 1 {
			t.Errorf("Roles len = %d, want 1", len(id.Roles))
		}
	})

	t.Run("valid token without optional claims", func(t *testing.T) {
		token := makeToken("user-456", "ws-xyz", nil, nil)

		id, err := validator.Validate(ctx, token)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id.UserID != "user-456" {
			t.Errorf("UserID = %q, want %q", id.UserID, "user-456")
		}
		if len(id.Permissions) != 0 {
			t.Errorf("Permissions should be empty, got %v", id.Permissions)
		}
	})

	t.Run("empty token", func(t *testing.T) {
		_, err := validator.Validate(ctx, "")
		if err == nil {
			t.Error("expected error for empty token")
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		_, err := validator.Validate(ctx, "not.a.valid.jwt")
		if err == nil {
			t.Error("expected error for invalid token")
		}
	})

	t.Run("expired token", func(t *testing.T) {
		claims := jwt.MapClaims{
			"sub": "user-exp",
			"ws":  "ws-exp",
			"iss": issuer,
			"exp": time.Now().Add(-time.Hour).Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signed, err := token.SignedString([]byte(secret))
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}

		_, err = validator.Validate(ctx, signed)
		if err == nil {
			t.Error("expected error for expired token")
		}
	})

	t.Run("wrong issuer", func(t *testing.T) {
		claims := jwt.MapClaims{
			"sub": "user-wrong",
			"ws":  "ws-wrong",
			"iss": "wrong-issuer",
			"exp": time.Now().Add(time.Hour).Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signed, err := token.SignedString([]byte(secret))
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}

		_, err = validator.Validate(ctx, signed)
		if err == nil {
			t.Error("expected error for wrong issuer")
		}
	})

	t.Run("wrong secret", func(t *testing.T) {
		claims := jwt.MapClaims{
			"sub": "user-secret",
			"ws":  "ws-secret",
			"iss": issuer,
			"exp": time.Now().Add(time.Hour).Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signed, err := token.SignedString([]byte("wrong-secret"))
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}

		_, err = validator.Validate(ctx, signed)
		if err == nil {
			t.Error("expected error for token signed with wrong secret")
		}
	})

	t.Run("missing workspace claim", func(t *testing.T) {
		claims := jwt.MapClaims{
			"sub": "user-no-ws",
			"iss": issuer,
			"exp": time.Now().Add(time.Hour).Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signed, err := token.SignedString([]byte(secret))
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}

		_, err = validator.Validate(ctx, signed)
		if err == nil {
			t.Error("expected error for missing workspace claim")
		}
	})
}

func TestDevValidator(t *testing.T) {
	v := NewDevValidator()

	id, err := v.Validate(context.Background(), "anything")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id.UserID != "developer" {
		t.Errorf("UserID = %q, want %q", id.UserID, "developer")
	}
	if id.WorkspaceID != "" {
		t.Errorf("WorkspaceID = %q, want empty (adopts URL workspace, avoids cross-tenant 404 in dev mode)", id.WorkspaceID)
	}
	if !id.HasPermission("anything.at.all") {
		t.Error("dev identity should have wildcard '*' permission")
	}
}
