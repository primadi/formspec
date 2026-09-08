// ─── Dev JWT secret persistence ───
//
// Auth is uniform across dev and prod (always real JWT). In dev, when no
// explicit `jwt-secret` is configured, a secret is generated once and
// persisted to `<state-dir>/dev-jwt-secret` so issued tokens (sessions)
// survive restarts. ProdMode never uses this — it requires an explicit
// secret or public key.

package devsecret

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileName is the persisted dev secret file inside the state directory.
const FileName = "dev-jwt-secret"

// Resolve returns the dev JWT secret: reads <stateDir>/dev-jwt-secret when
// it exists, otherwise generates a fresh 32-byte hex secret and persists it
// (file mode 0600). Returns the secret and whether a new one was generated.
func Resolve(stateDir string) (secret string, generated bool, err error) {
	path := filepath.Join(stateDir, FileName)

	if data, readErr := os.ReadFile(path); readErr == nil {
		if s := strings.TrimSpace(string(data)); s != "" {
			return s, false, nil
		}
	} else if !os.IsNotExist(readErr) {
		return "", false, fmt.Errorf("read dev jwt secret: %w", readErr)
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", false, fmt.Errorf("generate dev jwt secret: %w", err)
	}
	secret = hex.EncodeToString(buf)

	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return "", false, fmt.Errorf("create state dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(secret+"\n"), 0o600); err != nil {
		return "", false, fmt.Errorf("persist dev jwt secret: %w", err)
	}
	return secret, true, nil
}
