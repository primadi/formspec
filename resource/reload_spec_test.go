package formspec

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/internal/api"
)

// TestReloadSpec_PreservesEmbeddedCoreEntities verifies that a hot-reload
// (ReloadSpec) keeps the framework-owned embedded auth entities
// (formspec.core.user/role/...) registered. Regression test for the bug where
// reloading the filesystem spec dropped the embedded module's entities while
// the UI registry still referenced them (user-table/role-table/role-form) →
// "entity not found" and tabs rendering "Table: user-table".
func TestReloadSpec_PreservesEmbeddedCoreEntities(t *testing.T) {
	dir := t.TempDir()
	buildAuthSpecDir(t, dir)
	api.ResetAuthRateLimiters()

	app, err := New(Config{
		SpecPath:  dir,
		DSN:       "sqlite:" + filepath.Join(t.TempDir(), "reload.db"),
		DevAuth:   true,
		JWTSecret: "test-secret",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer app.Close(context.Background())

	// Embedded core entities must be present before reload.
	for _, name := range []string{"user", "role", "session", "api-key"} {
		if _, ok := app.Registry().GetEntity("formspec.core", name); !ok {
			t.Fatalf("before reload: formspec.core.%s not registered", name)
		}
	}

	// Trigger a full spec reload (simulates the dev watcher firing on a
	// spec file change).
	if err := app.ReloadSpec(); err != nil {
		t.Fatalf("ReloadSpec: %v", err)
	}

	// Embedded core entities must survive the reload.
	for _, name := range []string{"user", "role", "session", "api-key"} {
		if _, ok := app.Registry().GetEntity("formspec.core", name); !ok {
			t.Errorf("after reload: formspec.core.%s not registered (dropped by reload)", name)
		}
	}

	// User-spec entities must also survive.
	if _, ok := app.Registry().GetEntity("acme", "customer"); !ok {
		t.Errorf("after reload: acme.customer not registered")
	}
}
