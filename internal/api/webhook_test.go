package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/action"
	"github.com/primadi/formspec/internal/config"
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/service"
	"github.com/primadi/formspec/internal/webhook"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// newWebhookTestHarness builds a HandlerFactory wired with a service registry,
// a webhook registry, and a config-backed key resolver.
func newWebhookTestHarness(t *testing.T) *HandlerFactory {
	t.Helper()
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "wh.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	r := db.NewMigrationRunner(d, db.DriverSQLite)
	ctx := context.Background()
	r.EnsureSystemTables(ctx)

	reg := entity.NewRegistry(d, db.DriverSQLite, dir)
	reg.LoadEntities()

	// Service registry with a native-backed payment-gateway.webhook action.
	svcReg := service.NewRegistry()
	svcReg.Add("payment-gateway", "webhook", &spec.ServiceSpec{
		Actions: []spec.Action{
			{
				Name: "handle",
				Impl: &spec.ImplDecl{Type: spec.ImplNative, Ref: "PGWebhook.Handle"},
			},
		},
	})

	// Dispatcher with a native executor.
	disp := action.NewDispatcher()
	nativeEx := action.NewNativeExecutor()
	nativeEx.Register("PGWebhook.Handle", func(ctx context.Context, params action.ExecuteParams) (any, error) {
		return map[string]any{"received": true, "txn": params.Params["transaction_id"]}, nil
	})
	disp.RegisterExecutor(spec.ImplNative, nativeEx)
	disp.SetNativeExecutor(nativeEx)

	// Config registry with the webhook secret/token keys.
	cfgReg := config.NewRegistry()
	cfgReg.Add("midtrans", &spec.ConfigSpec{
		Keys: map[string]spec.ConfigKey{
			"server_key": {Type: "string", Default: "s3cret", Secret: true},
		},
	})
	cfgReg.Add("internal", &spec.ConfigSpec{
		Keys: map[string]spec.ConfigKey{
			"webhook_token": {Type: "string", Default: "tok-123", Secret: true},
		},
	})

	factory := NewHandlerFactory(reg)
	factory.SetDispatcher(disp)
	factory.SetServiceRegistry(svcReg)
	factory.SetWebhookKeyResolver(cfgReg)
	return factory
}

func TestWebhook_Signature(t *testing.T) {
	factory := newWebhookTestHarness(t)

	wh := &spec.WebhookSpec{
		For: "payment-gateway.webhook",
		Auth: &spec.WebhookAuth{
			Strategy: "signature",
			Signature: &spec.WebhookSigConfig{
				Algorithm: "hmac-sha256",
				Header:    "X-Midtrans-Signature",
				Key:       &spec.WebhookKeyRef{Config: "midtrans.server_key"},
				Payload:   "raw_body",
			},
		},
	}
	h := factory.HandleWebhook("billing", "midtrans-webhook", wh)

	body := `{"transaction_id":"T-123","amount":100}`
	mac := hmac.New(sha256.New, []byte("s3cret"))
	mac.Write([]byte(body))
	valid := hex.EncodeToString(mac.Sum(nil))

	// Valid signature → 200 + dispatched.
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Midtrans-Signature", valid)
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Invalid signature → 401, handler never runs.
	req2 := httptest.NewRequest("POST", "/", strings.NewReader(body))
	req2.Header.Set("X-Midtrans-Signature", strings.Repeat("0", len(valid)))
	rec2 := httptest.NewRecorder()
	h(rec2, req2)
	if rec2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for bad signature, got %d", rec2.Code)
	}
}

func TestWebhook_Token(t *testing.T) {
	factory := newWebhookTestHarness(t)

	wh := &spec.WebhookSpec{
		For: "payment-gateway.webhook",
		Auth: &spec.WebhookAuth{
			Strategy: "token",
			Token: &spec.WebhookTokenConfig{
				Header: "Authorization",
				Key:    &spec.WebhookKeyRef{Config: "internal.webhook_token"},
			},
		},
	}
	h := factory.HandleWebhook("billing", "internal-webhook", wh)

	// Valid token → 200.
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"transaction_id":"T-1"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer tok-123")
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Invalid token → 401.
	req2 := httptest.NewRequest("POST", "/", strings.NewReader(`{"transaction_id":"T-1"}`))
	req2.Header.Set("Authorization", "Bearer wrong")
	rec2 := httptest.NewRecorder()
	h(rec2, req2)
	if rec2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for bad token, got %d", rec2.Code)
	}
}

func TestWebhook_RouteRegistered(t *testing.T) {
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "wh_route.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := db.NewMigrationRunner(d, db.DriverSQLite)
	ctx := context.Background()
	r.EnsureSystemTables(ctx)

	reg := entity.NewRegistry(d, db.DriverSQLite, dir)
	reg.LoadEntities()

	svcReg := service.NewRegistry()
	svcReg.Add("payment-gateway", "webhook", &spec.ServiceSpec{
		Actions: []spec.Action{
			{Name: "handle", Impl: &spec.ImplDecl{Type: spec.ImplNative, Ref: "PGWebhook.Handle"}},
		},
	})

	disp := action.NewDispatcher()
	nativeEx := action.NewNativeExecutor()
	nativeEx.Register("PGWebhook.Handle", func(ctx context.Context, params action.ExecuteParams) (any, error) {
		return map[string]any{"ok": true}, nil
	})
	disp.RegisterExecutor(spec.ImplNative, nativeEx)
	disp.SetNativeExecutor(nativeEx)

	whReg := webhook.NewRegistry()
	whReg.Add("billing", "midtrans-webhook", &spec.WebhookSpec{
		For:    "payment-gateway.webhook",
		Method: "POST",
		Path:   "/webhooks/midtrans",
		Auth: &spec.WebhookAuth{
			Strategy: "token",
			Token: &spec.WebhookTokenConfig{
				Header: "Authorization",
				Key:    &spec.WebhookKeyRef{Config: "internal.webhook_token"},
			},
		},
	})

	cfgReg := config.NewRegistry()
	cfgReg.Add("internal", &spec.ConfigSpec{
		Keys: map[string]spec.ConfigKey{
			"webhook_token": {Type: "string", Default: "tok-123", Secret: true},
		},
	})

	rb := NewRouterBuilder(reg)
	rb.SetDispatcher(disp)
	rb.SetServiceRegistry(svcReg)
	rb.SetWebhookRegistry(whReg)
	rb.SetWebhookKeyResolver(cfgReg)
	rb.BuildRoutes()
	handler := rb.BuildHTTP()

	// Route is registered at /demo/api/v1/webhooks/midtrans (spec.path).
	req := httptest.NewRequest("POST", "/demo/api/v1/webhooks/midtrans", strings.NewReader(`{"x":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer tok-123")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 via registered route, got %d: %s", rec.Code, rec.Body.String())
	}

	// Wrong token via route → 401.
	req2 := httptest.NewRequest("POST", "/demo/api/v1/webhooks/midtrans", strings.NewReader(`{"x":1}`))
	req2.Header.Set("Authorization", "Bearer nope")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 via route, got %d", rec2.Code)
	}
}
