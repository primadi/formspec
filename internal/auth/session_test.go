package auth

import (
	"context"
	"testing"
	"time"
)

func TestService_ConcurrentSessionLimit(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()
	svc.SetMaxSessionsPerUser(1)

	if err := svc.SeedDevUser(ctx, "demo", "admin", "admin"); err != nil {
		t.Fatal(err)
	}

	// First login → 1 session.
	if _, err := svc.Login(ctx, "demo", "admin", "admin"); err != nil {
		t.Fatal(err)
	}
	// Second login → evicts oldest, still capped at 1.
	if _, err := svc.Login(ctx, "demo", "admin", "admin"); err != nil {
		t.Fatal(err)
	}

	user, err := svc.users.GetByUsername(ctx, "demo", "admin")
	if err != nil {
		t.Fatal(err)
	}
	count, err := svc.session.CountForUser(ctx, "demo", user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 session after limit, got %d", count)
	}
}

func TestService_UnlimitedSessionsByDefault(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()

	if err := svc.SeedDevUser(ctx, "demo", "admin", "admin"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err := svc.Login(ctx, "demo", "admin", "admin"); err != nil {
			t.Fatal(err)
		}
	}
	user, _ := svc.users.GetByUsername(ctx, "demo", "admin")
	count, _ := svc.session.CountForUser(ctx, "demo", user.ID)
	if count != 3 {
		t.Fatalf("expected 3 sessions (unlimited), got %d", count)
	}
}

func TestService_LogoutAll(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()

	if err := svc.SeedDevUser(ctx, "demo", "admin", "admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Login(ctx, "demo", "admin", "admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Login(ctx, "demo", "admin", "admin"); err != nil {
		t.Fatal(err)
	}

	user, _ := svc.users.GetByUsername(ctx, "demo", "admin")
	if err := svc.LogoutAll(ctx, "demo", user.ID); err != nil {
		t.Fatalf("LogoutAll: %v", err)
	}
	count, _ := svc.session.CountForUser(ctx, "demo", user.ID)
	if count != 0 {
		t.Fatalf("expected 0 sessions after LogoutAll, got %d", count)
	}
}

func TestService_PurgeExpired(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()

	if err := svc.SeedDevUser(ctx, "demo", "admin", "admin"); err != nil {
		t.Fatal(err)
	}
	// A live login session (future expiry).
	if _, err := svc.Login(ctx, "demo", "admin", "admin"); err != nil {
		t.Fatal(err)
	}
	// Manually insert an already-expired session.
	user, _ := svc.users.GetByUsername(ctx, "demo", "admin")
	expired := Session{
		JTI:         "expired-jti",
		UserID:      user.ID,
		WorkspaceID: "demo",
		ExpiresAt:   time.Now().Add(-time.Hour),
		CreatedAt:   time.Now(),
	}
	if err := svc.session.Create(ctx, expired); err != nil {
		t.Fatal(err)
	}

	n, err := svc.PurgeExpiredSessions(ctx)
	if err != nil {
		t.Fatalf("PurgeExpiredSessions: %v", err)
	}
	if n < 1 {
		t.Fatalf("expected at least 1 purged, got %d", n)
	}
	if _, ok := svc.session.Get(ctx, "demo", "expired-jti"); ok {
		t.Error("expected expired session to be purged")
	}
}
