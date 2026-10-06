// Package webhookout delivers an event to an HTTP endpoint declared in the
// manifest — the `webhook` DELIVERY channel (todo 7.7.6).
//
// UNSIGNED, on purpose and for now. `internal/webhook` covers the opposite
// direction (verifying INBOUND provider webhooks); this is the outbound one.
// There is no HMAC signature and no subscriber registry yet: the endpoint is
// declared where the consequence is (`url:` or `url_from: {config: ...}`), which
// is what an app author actually knows. Signing needs a per-subscriber secret
// store, and announcing a signature the runtime cannot produce would be exactly
// the kind of promise this codebase keeps refusing.
package webhookout

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/primadi/formspec/pkg/spec"
)

// DefaultTimeout bounds one delivery attempt. The outbox worker retries on
// failure, so a slow endpoint must not hold the worker: a timeout is a failure
// to retry, not a reason to wait longer.
const DefaultTimeout = 10 * time.Second

// ConfigLookup resolves a Config key to its value (`url_from`). Implemented by
// internal/config.Registry.
type ConfigLookup func(key string) (string, bool)

// Sender posts events to declared endpoints.
type Sender struct {
	client *http.Client
	config ConfigLookup
}

// New builds a Sender. config may be nil when no entry uses `url_from`.
func New(config ConfigLookup) *Sender {
	return &Sender{client: &http.Client{Timeout: DefaultTimeout}, config: config}
}

// SetClient overrides the HTTP client (tests, or a deployment with its own
// transport policy).
func (s *Sender) SetClient(c *http.Client) { s.client = c }

// Send posts one event to the endpoint the entry declares.
//
// The body is the event message as delivered everywhere else (event name,
// resource, payload, emission time) — one wire shape, not a webhook-specific
// one, so a receiver can be moved between channels without relearning the
// payload.
//
// A non-2xx response is an ERROR: the outbox retries and then dead-letters,
// which is the only way a failed delivery becomes visible. Treating it as
// success is how this channel used to "deliver" nothing at all.
func (s *Sender) Send(ctx context.Context, decl *spec.WebhookDeliveryDecl, payload map[string]any, body []byte) error {
	url, err := s.resolveURL(decl)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// Present so a receiver can tell this apart from a browser/HTML post.
	req.Header.Set("User-Agent", "FormSpec-Webhook/1")
	for k, v := range decl.Headers {
		if strings.TrimSpace(k) == "" {
			continue
		}
		req.Header.Set(k, interpolateHeaders(v, payload))
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook: post %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		// Read a bounded slice of the body so the error can say WHY, without
		// letting a hostile endpoint stream unbounded data into the log.
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("webhook: post %s: status %d: %s", url, resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	// Drain so the connection can be reused.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	return nil
}

// resolveURL resolves the endpoint declared on the entry.
//
// Exactly one source must be present — the validator refuses both or neither —
// so this is a resolution, not a precedence rule that could hide a mistake.
func (s *Sender) resolveURL(decl *spec.WebhookDeliveryDecl) (string, error) {
	if decl == nil {
		return "", fmt.Errorf("webhook: entry has no `webhook:` block — there is no endpoint to post to")
	}
	if u := strings.TrimSpace(decl.URL); u != "" {
		return u, nil
	}
	if decl.URLFrom != nil && strings.TrimSpace(decl.URLFrom.Config) != "" {
		key := strings.TrimSpace(decl.URLFrom.Config)
		if s.config == nil {
			return "", fmt.Errorf("webhook: `url_from: {config: %s}` cannot be resolved — no Config registry is wired", key)
		}
		v, ok := s.config(key)
		if !ok || strings.TrimSpace(v) == "" {
			return "", fmt.Errorf("webhook: `url_from: {config: %s}` resolved to nothing — the endpoint is not declared in any Config manifest", key)
		}
		return strings.TrimSpace(v), nil
	}
	return "", fmt.Errorf("webhook: entry declares neither `url:` nor `url_from: {config: ...}` — the delivery has nowhere to post")
}

// interpolateHeaders resolves `{dotted.path}` templates in a header value
// against the event payload. An unresolved token is left verbatim: a header with
// a visible `{id}` is debuggable, one that silently became empty is not.
//
// It advances a CURSOR past each token instead of rewriting and rescanning the
// produced string — the rescanning form makes an unresolved token a no-op (the
// same `{` is found forever), i.e. an infinite loop.
func interpolateHeaders(value string, payload map[string]any) string {
	if !strings.Contains(value, "{") {
		return value
	}
	var b strings.Builder
	rest := value
	for {
		start := strings.Index(rest, "{")
		if start < 0 {
			b.WriteString(rest)
			return b.String()
		}
		rel := strings.Index(rest[start:], "}")
		if rel < 0 {
			b.WriteString(rest)
			return b.String()
		}
		end := start + rel
		b.WriteString(rest[:start])
		path := strings.TrimSpace(rest[start+1 : end])
		if v, ok := lookupPath(payload, path); ok && v != nil {
			b.WriteString(stringify(v))
		} else {
			b.WriteString(rest[start : end+1])
		}
		rest = rest[end+1:]
	}
}

// lookupPath resolves a dotted path against the payload, walking nested maps.
func lookupPath(payload map[string]any, path string) (any, bool) {
	if path == "" {
		return nil, false
	}
	var cur any = payload
	for _, seg := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[seg]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// stringify renders a scalar for a header value; a structured value is JSON so
// nothing is silently lost.
func stringify(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	case map[string]any, []any:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprintf("%v", t)
		}
		return string(b)
	default:
		return fmt.Sprintf("%v", t)
	}
}
