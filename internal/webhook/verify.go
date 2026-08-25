package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"net/http"
	"strings"

	"github.com/primadi/formspec/pkg/spec"
)

// KeyResolver resolves a config-referenced secret/token value for webhook
// verification. Implemented by the config registry (ResolveKey) and wired in
// resource/formspec.go.
type KeyResolver interface {
	// ResolveKey returns the string value of keyName within the named Config
	// manifest, and whether it was found.
	ResolveKey(configName, keyName string) (string, bool)
}

// Verify checks an inbound webhook request against its declared auth strategy.
// It returns an error (with a stable code) when verification fails; the
// handler must reject the request BEFORE running any handler logic.
//
// Strategies:
//   - "signature" — HMAC over the raw body (or parsed JSON) using the key
//     referenced via config. The signature header is compared in constant time.
//   - "token" — static token from config compared against a request header in
//     constant time.
func Verify(r *http.Request, wh *spec.WebhookSpec, keys KeyResolver) error {
	if wh == nil || wh.Auth == nil {
		// No auth declared — accept (framework default is permissive for
		// inbound webhooks; the manifest author chooses to secure or not).
		return nil
	}
	switch wh.Auth.Strategy {
	case "signature":
		return verifySignature(r, wh.Auth.Signature, keys)
	case "token":
		return verifyToken(r, wh.Auth.Token, keys)
	default:
		return fmt.Errorf("WEBHOOK_AUTH_UNSUPPORTED: unknown auth strategy %q", wh.Auth.Strategy)
	}
}

func verifySignature(r *http.Request, sig *spec.WebhookSigConfig, keys KeyResolver) error {
	if sig == nil {
		return fmt.Errorf("WEBHOOK_AUTH_ERROR: signature strategy requires a signature config")
	}
	if sig.Key == nil {
		return fmt.Errorf("WEBHOOK_AUTH_ERROR: signature config requires a key reference")
	}
	configName, keyName, ok := splitKeyRef(sig.Key.Config)
	if !ok {
		return fmt.Errorf("WEBHOOK_AUTH_ERROR: invalid signature key ref %q (want {config}.{key})", sig.Key.Config)
	}
	secret, ok := keys.ResolveKey(configName, keyName)
	if !ok {
		return fmt.Errorf("WEBHOOK_AUTH_ERROR: signature key %s not found in config", sig.Key.Config)
	}

	// Read the raw body for signing. For payload: parsed, we sign the
	// canonical JSON; for raw_body (default), we sign the exact bytes.
	body, err := readBody(r)
	if err != nil {
		return fmt.Errorf("WEBHOOK_AUTH_ERROR: read body: %w", err)
	}

	expected := computeHMAC(sig.Algorithm, secret, body)
	provided := r.Header.Get(sig.Header)
	if provided == "" {
		return fmt.Errorf("WEBHOOK_AUTH_FAILED: missing signature header %q", sig.Header)
	}
	// Accept both raw hex and "sha512=..." prefixed forms (common provider
	// conventions). Strip any "algorithm=" prefix before comparing.
	provided = stripSignaturePrefix(provided)
	if !hmac.Equal([]byte(strings.ToLower(provided)), []byte(strings.ToLower(expected))) {
		return fmt.Errorf("WEBHOOK_AUTH_FAILED: signature mismatch")
	}
	return nil
}

func verifyToken(r *http.Request, tok *spec.WebhookTokenConfig, keys KeyResolver) error {
	if tok == nil {
		return fmt.Errorf("WEBHOOK_AUTH_ERROR: token strategy requires a token config")
	}
	if tok.Key == nil {
		return fmt.Errorf("WEBHOOK_AUTH_ERROR: token config requires a key reference")
	}
	configName, keyName, ok := splitKeyRef(tok.Key.Config)
	if !ok {
		return fmt.Errorf("WEBHOOK_AUTH_ERROR: invalid token key ref %q (want {config}.{key})", tok.Key.Config)
	}
	expected, ok := keys.ResolveKey(configName, keyName)
	if !ok {
		return fmt.Errorf("WEBHOOK_AUTH_ERROR: token key %s not found in config", tok.Key.Config)
	}
	provided := r.Header.Get(tok.Header)
	if provided == "" {
		return fmt.Errorf("WEBHOOK_AUTH_FAILED: missing token header %q", tok.Header)
	}
	// Support "Bearer <token>" prefix on the Authorization header.
	provided = strings.TrimSpace(strings.TrimPrefix(provided, "Bearer "))
	if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
		return fmt.Errorf("WEBHOOK_AUTH_FAILED: token mismatch")
	}
	return nil
}

// splitKeyRef parses a config key reference of the form "{config}.{key}" into
// its parts. Returns ok=false when malformed.
func splitKeyRef(ref string) (configName, keyName string, ok bool) {
	parts := strings.SplitN(ref, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// computeHMAC computes the hex-encoded HMAC of body using the given algorithm.
// Supported: hmac-sha256, hmac-sha512 (default hmac-sha256).
func computeHMAC(algorithm, secret, body string) string {
	var h func() hash.Hash
	switch strings.ToLower(algorithm) {
	case "hmac-sha512", "sha512":
		h = sha512.New
	case "hmac-sha256", "sha256", "":
		h = sha256.New
	default:
		h = sha256.New
	}
	mac := hmac.New(h, []byte(secret))
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

// stripSignaturePrefix removes a leading "sha256=" / "sha512=" / "hmac-sha512="
// prefix from a provider-supplied signature header, leaving the bare hex.
func stripSignaturePrefix(v string) string {
	if i := strings.Index(v, "="); i >= 0 {
		return v[i+1:]
	}
	return v
}

// readBody reads the request body and restores it so the handler can read it
// again. Returns the body as a string.
func readBody(r *http.Request) (string, error) {
	if r.Body == nil {
		return "", nil
	}
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return "", err
	}
	// Restore the body for downstream handlers.
	r.Body = io.NopCloser(bytes.NewReader(b))
	return string(b), nil
}
