package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// fakeKeys is a minimal KeyResolver for tests.
type fakeKeys map[string]string

func (f fakeKeys) ResolveKey(configName, keyName string) (string, bool) {
	v, ok := f[configName+"."+keyName]
	return v, ok
}

func TestVerifySignature(t *testing.T) {
	secret := "s3cret"
	body := `{"transaction_id":"T-123","amount":100}`
	keys := fakeKeys{"midtrans.server_key": secret}

	// Compute a valid HMAC-SHA256 over the raw body.
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	valid := hex.EncodeToString(mac.Sum(nil))

	wh := &spec.WebhookSpec{
		Auth: &spec.WebhookAuth{
			Strategy: "signature",
			Signature: &spec.WebhookSigConfig{
				Algorithm: "hmac-sha256",
				Header:    "X-Signature",
				Key:       &spec.WebhookKeyRef{Config: "midtrans.server_key"},
				Payload:   "raw_body",
			},
		},
	}

	// Valid signature → no error.
	req := httptestNewRequest("POST", body)
	req.Header.Set("X-Signature", valid)
	if err := Verify(req, wh, keys); err != nil {
		t.Fatalf("expected valid signature to pass, got: %v", err)
	}

	// Wrong signature → error.
	req2 := httptestNewRequest("POST", body)
	req2.Header.Set("X-Signature", strings.Repeat("0", len(valid)))
	if err := Verify(req2, wh, keys); err == nil {
		t.Fatal("expected wrong signature to fail")
	}

	// Missing header → error.
	req3 := httptestNewRequest("POST", body)
	if err := Verify(req3, wh, keys); err == nil {
		t.Fatal("expected missing header to fail")
	}
}

func TestVerifyToken(t *testing.T) {
	keys := fakeKeys{"internal.webhook_token": "tok-123"}
	wh := &spec.WebhookSpec{
		Auth: &spec.WebhookAuth{
			Strategy: "token",
			Token: &spec.WebhookTokenConfig{
				Header: "Authorization",
				Key:    &spec.WebhookKeyRef{Config: "internal.webhook_token"},
			},
		},
	}

	// Valid token (with Bearer prefix) → pass.
	req := httptestNewRequest("POST", `{}`)
	req.Header.Set("Authorization", "Bearer tok-123")
	if err := Verify(req, wh, keys); err != nil {
		t.Fatalf("expected valid token to pass, got: %v", err)
	}

	// Wrong token → fail.
	req2 := httptestNewRequest("POST", `{}`)
	req2.Header.Set("Authorization", "Bearer wrong")
	if err := Verify(req2, wh, keys); err == nil {
		t.Fatal("expected wrong token to fail")
	}
}

func TestVerifyNoAuth(t *testing.T) {
	// No auth declared → accept.
	wh := &spec.WebhookSpec{}
	req := httptestNewRequest("POST", `{}`)
	if err := Verify(req, wh, fakeKeys{}); err != nil {
		t.Fatalf("expected no-auth webhook to pass, got: %v", err)
	}
}

func TestVerifyUnsupportedStrategy(t *testing.T) {
	wh := &spec.WebhookSpec{Auth: &spec.WebhookAuth{Strategy: "oauth"}}
	req := httptestNewRequest("POST", `{}`)
	if err := Verify(req, wh, fakeKeys{}); err == nil {
		t.Fatal("expected unsupported strategy to fail")
	}
}

func httptestNewRequest(method, body string) *http.Request {
	req, _ := http.NewRequest(method, "/webhooks/test", strings.NewReader(body))
	return req
}
