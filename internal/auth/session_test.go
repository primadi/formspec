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
	if _, err := svc.Login(ctx, "demo", "", "admin", "admin"); err != nil {
		t.Fatal(err)
	}
	// Second login → evicts oldest, still capped at 1.
	if _, err := svc.Login(ctx, "demo", "", "admin", "admin"); err != nil {
		t.Fatal(err)
	}

	user, err := svc.users.GetByUsername(ctx, "demo", "admin")
	if err != nil {
		t.Fatal(err)
	}
	count, err := svc.session.CountForUser(ctx, "demo", user.ID, "")
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
		if _, err := svc.Login(ctx, "demo", "", "admin", "admin"); err != nil {
			t.Fatal(err)
		}
	}
	user, _ := svc.users.GetByUsername(ctx, "demo", "admin")
	count, _ := svc.session.CountForUser(ctx, "demo", user.ID, "")
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
	if _, err := svc.Login(ctx, "demo", "", "admin", "admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Login(ctx, "demo", "", "admin", "admin"); err != nil {
		t.Fatal(err)
	}

	user, _ := svc.users.GetByUsername(ctx, "demo", "admin")
	if err := svc.LogoutAll(ctx, "demo", user.ID); err != nil {
		t.Fatalf("LogoutAll: %v", err)
	}
	count, _ := svc.session.CountForUser(ctx, "demo", user.ID, "")
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
	if _, err := svc.Login(ctx, "demo", "", "admin", "admin"); err != nil {
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

// TestService_SessionLimitPerApp: the concurrent-session cap is applied per
// (user, App), so signing into a second App must not evict the session the
// user still holds in the first one (plan app-scoped-login.md D2).
func TestService_SessionLimitPerApp(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()
	svc.SetMaxSessionsPerUser(1)

	if err := svc.SeedDevUser(ctx, "demo", "admin", "admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Login(ctx, "demo", "kafe-pos", "admin", "admin"); err != nil {
		t.Fatalf("login kafe-pos: %v", err)
	}
	if _, err := svc.Login(ctx, "demo", "kafe-kds", "admin", "admin"); err != nil {
		t.Fatalf("login kafe-kds: %v", err)
	}

	user, err := svc.users.GetByUsername(ctx, "demo", "admin")
	if err != nil {
		t.Fatal(err)
	}
	// One session per App survives — the cap is not global.
	for _, app := range []string{"kafe-pos", "kafe-kds"} {
		count, err := svc.session.CountForUser(ctx, "demo", user.ID, app)
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("sessions for app %q = %d, want 1", app, count)
		}
	}

	// A second login in the SAME App still evicts the older one.
	if _, err := svc.Login(ctx, "demo", "kafe-pos", "admin", "admin"); err != nil {
		t.Fatal(err)
	}
	count, _ := svc.session.CountForUser(ctx, "demo", user.ID, "kafe-pos")
	if count != 1 {
		t.Fatalf("sessions for kafe-pos after re-login = %d, want 1", count)
	}
}
