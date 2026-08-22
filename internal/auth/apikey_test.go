package auth

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/primadi/formspec/internal/entity"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// setupApiKeyStore builds a registry with core entities synced and an
// ApiKeyStore backed by formspec.core.api-key.
func setupApiKeyStore(t *testing.T) (*ApiKeyStore, *entity.Registry, db.DB) {
	t.Helper()
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "apikey_test.db"), nil)
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

	store, err := NewRoleResolver(reg).Resolve(RoleApiKey)
	if err != nil {
		t.Fatalf("resolve api-key: %v", err)
	}
	return NewApiKeyStore(store), reg, d
}

func TestApiKeyStore_CreateAndResolve(t *testing.T) {
	store, _, _ := setupApiKeyStore(t)
	ctx := context.Background()

	plaintext, err := store.Create(ctx, "demo", &ApiKey{
		Name:        "integration",
		Scope:       "workspace",
		Permissions: []string{"billing.customers.list"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if plaintext == "" {
		t.Fatal("expected plaintext key returned once")
	}

	// Resolve by plaintext.
	k, err := store.GetByKey(ctx, "demo", plaintext)
	if err != nil {
		t.Fatalf("GetByKey: %v", err)
	}
	if !k.IsValid(time.Now()) {
		t.Fatal("expected key valid")
	}
	if len(k.Permissions) != 1 || k.Permissions[0] != "billing.customers.list" {
		t.Errorf("unexpected permissions: %v", k.Permissions)
	}

	// The stored hash must not equal the plaintext.
	if k.KeyHash == plaintext {
		t.Error("key_hash must not equal plaintext")
	}

	// Wrong key → not found.
	if _, err := store.GetByKey(ctx, "demo", "wrong-key"); err == nil {
		t.Error("expected error for wrong key")
	}
}

func TestApiKeyStore_ListMasked(t *testing.T) {
	store, _, _ := setupApiKeyStore(t)
	ctx := context.Background()

	if _, err := store.Create(ctx, "demo", &ApiKey{Name: "a", Scope: "workspace"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(ctx, "demo", &ApiKey{Name: "b", Scope: "workspace"}); err != nil {
		t.Fatal(err)
	}

	keys, err := store.List(ctx, "demo")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
	for _, k := range keys {
		if k.KeyHash != "" {
			t.Error("List must not expose key_hash")
		}
		if k.KeyPrefix == "" {
			t.Error("expected key_prefix for display")
		}
	}
}

func TestApiKeyStore_RevokeAndExpiry(t *testing.T) {
	store, _, _ := setupApiKeyStore(t)
	ctx := context.Background()

	plaintext, err := store.Create(ctx, "demo", &ApiKey{Name: "x", Scope: "workspace"})
	if err != nil {
		t.Fatal(err)
	}
	k, _ := store.GetByKey(ctx, "demo", plaintext)

	// Revoke → invalid.
	if err := store.Revoke(ctx, "demo", k.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	k2, _ := store.GetByKey(ctx, "demo", plaintext)
	if k2.IsValid(time.Now()) {
		t.Error("expected revoked key invalid")
	}

	// Expiry in the past → invalid.
	past := time.Now().Add(-time.Hour)
	plaintext2, err := store.Create(ctx, "demo", &ApiKey{Name: "y", Scope: "workspace", ExpiresAt: &past})
	if err != nil {
		t.Fatal(err)
	}
	k3, _ := store.GetByKey(ctx, "demo", plaintext2)
	if k3.IsValid(time.Now()) {
		t.Error("expected expired key invalid")
	}
}

func TestApiKey_Identity(t *testing.T) {
	k := &ApiKey{ID: "abc", Permissions: []string{"*"}}
	id := k.Identity("demo")
	if id.UserID != "apikey:abc" {
		t.Errorf("unexpected UserID: %s", id.UserID)
	}
	if !id.IsAuthenticated() {
		t.Error("expected api key identity authenticated")
	}
	if !id.HasPermission("anything.at.all") {
		t.Error("expected wildcard permission")
	}
}
