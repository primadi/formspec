package webhookout

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// TestSend_PostsTheEventBody: the body is the event message as delivered on
// every other channel, so a receiver can be moved between channels without
// relearning the payload.
func TestSend_PostsTheEventBody(t *testing.T) {
	var gotBody []byte
	var gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := New(nil)
	payload := map[string]any{"id": "ord-1", "number": "INV-9"}
	body, _ := json.Marshal(map[string]any{"event": "paid", "payload": payload})

	if err := s.Send(context.Background(), &spec.WebhookDeliveryDecl{URL: srv.URL}, payload, body); err != nil {
		t.Fatalf("send: %v", err)
	}
	if gotCT != "application/json" {
		t.Errorf("content-type = %q, want application/json", gotCT)
	}
	var decoded map[string]any
	if err := json.Unmarshal(gotBody, &decoded); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if decoded["event"] != "paid" {
		t.Errorf("body event = %v, want paid — the endpoint must receive the event message", decoded["event"])
	}
}

// TestSend_Non2xxIsAnError is the difference between a delivery and a silent
// no-op: reporting success for a rejected post means the outbox marks it
// completed and nothing ever retries — the exact failure this channel had.
func TestSend_Non2xxIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	s := New(nil)
	err := s.Send(context.Background(), &spec.WebhookDeliveryDecl{URL: srv.URL}, nil, []byte(`{}`))
	if err == nil {
		t.Fatal("a non-2xx response must be an error so the outbox retries and dead-letters")
	}
	// The body snippet must be surfaced: "status 500" alone does not say why.
	if !strings.Contains(err.Error(), "500") || !strings.Contains(err.Error(), "boom") {
		t.Errorf("error %q should carry the status and the response body", err)
	}
}

// TestSend_ConfigResolvedURL: `url_from: {config: ...}` is how the endpoint
// belongs to a deployment rather than to the module.
func TestSend_ConfigResolvedURL(t *testing.T) {
	var hit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hit = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	s := New(func(key string) (string, bool) {
		if key == "billing.webhook_url" {
			return srv.URL, true
		}
		return "", false
	})
	if err := s.Send(context.Background(), &spec.WebhookDeliveryDecl{
		URLFrom: &spec.WebhookURLRef{Config: "billing.webhook_url"},
	}, nil, []byte(`{}`)); err != nil {
		t.Fatalf("send: %v", err)
	}
	if !hit {
		t.Fatal("the resolved endpoint was not called")
	}
}

// TestSend_MissingConfigKeyIsAnError: a key that resolves to nothing must say so,
// naming the key — a silent skip would look like a delivered webhook.
func TestSend_MissingConfigKeyIsAnError(t *testing.T) {
	s := New(func(string) (string, bool) { return "", false })
	err := s.Send(context.Background(), &spec.WebhookDeliveryDecl{
		URLFrom: &spec.WebhookURLRef{Config: "missing.key"},
	}, nil, []byte(`{}`))
	if err == nil || !strings.Contains(err.Error(), "missing.key") {
		t.Fatalf("expected the unresolvable-key error naming the key, got %v", err)
	}
}

// TestSend_NoEndpointIsAnError covers the entry the validator also refuses: with
// no `url:` and no `url_from:`, there is nowhere to post.
func TestSend_NoEndpointIsAnError(t *testing.T) {
	s := New(nil)
	for name, decl := range map[string]*spec.WebhookDeliveryDecl{
		"nil block":  nil,
		"empty defn": {},
	} {
		if err := s.Send(context.Background(), decl, nil, []byte(`{}`)); err == nil {
			t.Errorf("%s: expected an error, got nil", name)
		}
	}
}

// TestSend_HeaderInterpolation covers `headers:` templates.
func TestSend_HeaderInterpolation(t *testing.T) {
	var routing string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		routing = r.Header.Get("X-Order")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := New(nil)
	if err := s.Send(context.Background(), &spec.WebhookDeliveryDecl{
		URL:     srv.URL,
		Headers: map[string]string{"X-Order": "{number}"},
	}, map[string]any{"number": "INV-9"}, []byte(`{}`)); err != nil {
		t.Fatalf("send: %v", err)
	}
	if routing != "INV-9" {
		t.Errorf("header = %q, want INV-9", routing)
	}
}

// TestInterpolateHeaders_UnresolvedTokenTerminates is a REGRESSION TEST for an
// infinite loop.
//
// The first implementation rewrote the produced string and rescanned it from the
// start. For a token that does not resolve, the rewrite was a no-op — the same
// `{` was found forever, hanging the delivery worker. The test needs no
// assertion beyond completing: before the fix it never returned.
func TestInterpolateHeaders_UnresolvedTokenTerminates(t *testing.T) {
	got := interpolateHeaders("order-{missing}-paid", map[string]any{"id": "x"})
	if got != "order-{missing}-paid" {
		t.Fatalf("unresolved token = %q, want it left verbatim so a typo stays visible", got)
	}
	// A malformed template (opening brace, no closer) must also terminate.
	if got := interpolateHeaders("order-{unclosed", nil); got != "order-{unclosed" {
		t.Fatalf("unclosed token = %q, want it left verbatim", got)
	}
}

// TestInterpolateHeaders_StructuredValueIsJSON: a map/list resolved into a header
// is JSON, not Go's map printing, so nothing is lost silently.
func TestInterpolateHeaders_StructuredValueIsJSON(t *testing.T) {
	got := interpolateHeaders("{ids}", map[string]any{"ids": []any{"a", "b"}})
	if got != `["a","b"]` {
		t.Fatalf("structured header = %q, want JSON", got)
	}
}
