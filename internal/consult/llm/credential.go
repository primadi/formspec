package llm

import (
	"fmt"
	"os"

	"github.com/zalando/go-keyring"
)

// CredentialStore resolves the LLM API key (todo 10.2.4, docs/ai/05 §3):
//
//  1. OS keyring (zalando/go-keyring)  → normal desktop case
//  2. Environment variable             → headless/CI case
//  3. (nothing)                        → clear error guiding setup
//
// No encrypted-file fallback — key derivation complexity is not worth it for
// the first release (05 §3).
type CredentialStore struct {
	// KeyringService is the keyring service name ("formspec-consult").
	KeyringService string
	// KeyringUser is the keyring account (the provider name).
	KeyringUser string
	// EnvVar is the environment variable checked second.
	EnvVar string
	// FallbackEnvVars are checked after EnvVar (e.g. OPENAI_API_KEY).
	FallbackEnvVars []string
}

// GetAPIKey resolves the API key, keyring first, then env vars.
func (c CredentialStore) GetAPIKey() (string, error) {
	if c.KeyringService != "" && c.KeyringUser != "" {
		if secret, err := keyring.Get(c.KeyringService, c.KeyringUser); err == nil && secret != "" {
			return secret, nil
		}
		// Keyring unavailable (headless Linux without secret service) or no
		// entry — fall through to env vars silently.
	}
	for _, name := range append([]string{c.EnvVar}, c.FallbackEnvVars...) {
		if name == "" {
			continue
		}
		if v := os.Getenv(name); v != "" {
			return v, nil
		}
	}
	return "", fmt.Errorf(
		"no LLM API key found — set it via OS keyring (`formspec consult` stores it on first run) "+
			"or export %s (BYOK — docs/ai/05-llm-provider-layer.md §3)", c.EnvVar)
}

// StoreAPIKey saves the key into the OS keyring (used by the REPL on first
// run when the user pastes a key).
func (c CredentialStore) StoreAPIKey(secret string) error {
	if c.KeyringService == "" || c.KeyringUser == "" {
		return fmt.Errorf("keyring not configured")
	}
	return keyring.Set(c.KeyringService, c.KeyringUser, secret)
}
