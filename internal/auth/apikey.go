package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// ErrApiKeyNotFound is returned when an API key cannot be resolved.
var ErrApiKeyNotFound = errors.New("auth: api key not found")

// ErrApiKeyInvalid is returned when an API key is revoked, expired, or inactive.
var ErrApiKeyInvalid = errors.New("auth: invalid api key")

// ApiKey represents a non-interactive credential for the external surface
// (/api/v1/, header X-FormSpec-Key). Only the hash is stored; the plaintext
// key is returned exactly once at creation.
type ApiKey struct {
	ID          string
	Name        string
	KeyHash     string
	KeyPrefix   string
	Scope       string // "workspace" | "app"
	App         string
	Permissions []string
	ExpiresAt   *time.Time
	RevokedAt   *time.Time
	Active      bool
}

// ApiKeyStore manages API keys backed by the formspec.core.api-key entity.
//
// This is framework code — it calls the store directly without permission
// checks, because the auth service is trusted infrastructure.
type ApiKeyStore struct {
	store *db.EntityStore
}

// NewApiKeyStore creates an API key store backed by the given EntityStore.
func NewApiKeyStore(store *db.EntityStore) *ApiKeyStore {
	return &ApiKeyStore{store: store}
}

// GenerateKey creates a new random API key. It returns the plaintext key
// (shown once), its SHA-256 hash (stored), and a short display prefix.
func GenerateKey(prefix string) (plaintext, hash, short string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", "", err
	}
	secret := hex.EncodeToString(b)
	plaintext = prefix + "_" + secret
	hash = hashApiKey(plaintext)
	short = prefix + "_" + secret[:8]
	return plaintext, hash, short, nil
}

// hashApiKey returns the SHA-256 hex digest of a plaintext API key.
func hashApiKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// Create generates a new API key, stores only its hash, and returns the
// plaintext key exactly once. The caller is responsible for showing it to
// the user immediately — it cannot be recovered later.
func (s *ApiKeyStore) Create(ctx context.Context, workspaceID string, k *ApiKey) (plaintext string, err error) {
	plaintext, hash, short, err := GenerateKey("fs")
	if err != nil {
		return "", fmt.Errorf("auth: generate api key: %w", err)
	}
	_, err = s.store.Insert(ctx, db.InsertParams{
		WorkspaceID: workspaceID,
		CreatedBy:   "system",
		Data: map[string]any{
			"name":        k.Name,
			"key_hash":    hash,
			"key_prefix":  short,
			"scope":       k.Scope,
			"app":         k.App,
			"permissions": k.Permissions,
			"expires_at":  formatTime(k.ExpiresAt),
			"revoked_at":  nil,
			"active":      true,
		},
	})
	if err != nil {
		return "", fmt.Errorf("auth: insert api key: %w", err)
	}
	return plaintext, nil
}

// GetByKey resolves an API key by its plaintext value (hashed for lookup).
func (s *ApiKeyStore) GetByKey(ctx context.Context, workspaceID, key string) (*ApiKey, error) {
	rec, err := s.store.FindByField(ctx, workspaceID, "key_hash", hashApiKey(key))
	if err != nil || rec == nil {
		return nil, ErrApiKeyNotFound
	}
	return apiKeyFromRecord(rec), nil
}

// List returns all API keys with the hash stripped (masked — only the
// display prefix is shown).
func (s *ApiKeyStore) List(ctx context.Context, workspaceID string) ([]ApiKey, error) {
	res, err := s.store.List(ctx, db.ListParams{WorkspaceID: workspaceID})
	if err != nil {
		return nil, err
	}
	out := make([]ApiKey, 0, len(res.Data))
	for _, rec := range res.Data {
		k := apiKeyFromRecord(&rec)
		k.KeyHash = "" // never expose the hash
		out = append(out, *k)
	}
	return out, nil
}

// Revoke marks an API key revoked and inactive.
func (s *ApiKeyStore) Revoke(ctx context.Context, workspaceID, id string) error {
	now := time.Now().UTC()
	return s.store.UpdateFields(ctx, workspaceID, id, map[string]any{
		"revoked_at": now.Format(time.RFC3339),
		"active":     false,
	})
}

// IsValid reports whether the key is usable at the given time (active, not
// revoked, not expired).
func (k *ApiKey) IsValid(now time.Time) bool {
	if k == nil || !k.Active {
		return false
	}
	if k.RevokedAt != nil {
		return false
	}
	if k.ExpiresAt != nil && now.After(*k.ExpiresAt) {
		return false
	}
	return true
}

// Identity builds an auth.Identity for this API key (service account — no
// interactive user). The UserID is a synthetic "apikey:<id>" so the identity
// is authenticated and auditable by api_key_id.
func (k *ApiKey) Identity(workspaceID string) *Identity {
	return &Identity{
		UserID:      "apikey:" + k.ID,
		WorkspaceID: workspaceID,
		Permissions: k.Permissions,
		Roles:       nil,
	}
}

// formatTime renders a *time.Time as RFC3339, or "" when nil.
func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// apiKeyFromRecord converts an entity record into an ApiKey.
func apiKeyFromRecord(rec *db.EntityRecord) *ApiKey {
	k := &ApiKey{
		ID:          rec.ID,
		Name:        stringField(rec.Data, "name"),
		KeyHash:     stringField(rec.Data, "key_hash"),
		KeyPrefix:   stringField(rec.Data, "key_prefix"),
		Scope:       stringField(rec.Data, "scope"),
		App:         stringField(rec.Data, "app"),
		Permissions: stringSliceField(rec.Data, "permissions"),
		Active:      boolField(rec.Data, "active", true),
	}
	if v := stringField(rec.Data, "expires_at"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			k.ExpiresAt = &t
		}
	}
	if v := stringField(rec.Data, "revoked_at"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			k.RevokedAt = &t
		}
	}
	return k
}
