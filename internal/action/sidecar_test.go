package action

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/primadi/formspec/renderers/jsonb-persist"
	"github.com/primadi/formspec/pkg/spec"
)

// startAppListener runs a minimal lib-formspec-style /invoke listener on a
// unix socket and returns its endpoint URL.
func startAppListener(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()
	socketPath := filepath.Join(t.TempDir(), "app.sock")
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := &http.Server{Handler: handler}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })
	return "unix://" + socketPath
}

func TestSidecarExecutor_InvokeRoundTrip(t *testing.T) {
	var gotPath string
	var gotReq sidecarInvokeRequest

	endpoint := startAppListener(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		json.NewDecoder(r.Body).Decode(&gotReq)
		json.NewEncoder(w).Encode(sidecarInvokeResponse{
			Data:     map[string]any{"approved_at": "2026-07-10T10:00:00Z"},
			NewState: "approved",
			Events:   []sidecarEventEmission{{Name: "invoice.approved", Payload: map[string]any{"id": "inv-001"}}},
		})
	})

	ex, err := NewSidecarExecutorWithEndpoint(endpoint, 5*time.Second)
	if err != nil {
		t.Fatalf("executor: %v", err)
	}

	result, err := ex.Execute(context.Background(), spec.Action{
		Name: "approve",
		Impl: &spec.ImplDecl{Type: spec.ImplSidecar, Ref: "approve-handler"},
	}, ExecuteParams{
		Module: "billing", Entity: "invoice", ActionName: "approve",
		ResourceID: "inv-001",
		Resource:   map[string]any{"status": "draft"},
		Params:     map[string]any{"note": "ok"},
		UserID:     "u-1",
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if gotPath != "/invoke/billing/invoice/approve" {
		t.Errorf("path = %q, want /invoke/billing/invoice/approve", gotPath)
	}
	if gotReq.ResourceID != "inv-001" {
		t.Errorf("request not serialized: %+v", gotReq)
	}
	if result.NewState != "approved" {
		t.Errorf("NewState = %q, want approved", result.NewState)
	}
	if len(result.Events) != 1 || result.Events[0].Name != "invoice.approved" {
		t.Errorf("events = %+v", result.Events)
	}
}

func TestSidecarExecutor_AppError(t *testing.T) {
	endpoint := startAppListener(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(sidecarInvokeResponse{Error: "handler exploded"})
	})

	ex, err := NewSidecarExecutorWithEndpoint(endpoint, 5*time.Second)
	if err != nil {
		t.Fatalf("executor: %v", err)
	}
	_, err = ex.Execute(context.Background(), spec.Action{
		Impl: &spec.ImplDecl{Type: spec.ImplSidecar},
	}, ExecuteParams{Module: "m", Entity: "e", ActionName: "a"})
	if err == nil || !strings.Contains(err.Error(), "handler exploded") {
		t.Fatalf("err = %v, want app error message", err)
	}
}

func TestSidecarExecutor_Timeout(t *testing.T) {
	endpoint := startAppListener(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	})

	ex, err := NewSidecarExecutorWithEndpoint(endpoint, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("executor: %v", err)
	}
	_, err = ex.Execute(context.Background(), spec.Action{
		Impl: &spec.ImplDecl{Type: spec.ImplSidecar},
	}, ExecuteParams{Module: "m", Entity: "e", ActionName: "slow"})
	if err == nil || !strings.Contains(err.Error(), "did not respond within") {
		t.Fatalf("err = %v, want gateway timeout", err)
	}
}

func TestNewSidecarExecutorWithEndpoint_BadScheme(t *testing.T) {
	if _, err := NewSidecarExecutorWithEndpoint("grpc://localhost:1234", 0); err == nil {
		t.Fatal("expected error for unsupported scheme")
	}
}

// TestSidecarExecutor_ForwardsScopeIdHeader verifies the sending half of
// the cross-process TxScope correlation described in
// renderers/jsonb-persist/txscope.go: when ctx carries an active scope,
// Execute must forward its registry id as X-FormSpec-Scope-Id on the
// outbound /invoke/... request — internal/sidecar/ctx.go's receiving half
// (TestCtxHandler_ScopeIdJoinsSameTransaction) already proves the id gets
// resolved back to the live scope; this proves it actually gets sent.
func TestSidecarExecutor_ForwardsScopeIdHeader(t *testing.T) {
	var gotHeader string
	endpoint := startAppListener(t, func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-FormSpec-Scope-Id")
		json.NewEncoder(w).Encode(sidecarInvokeResponse{Data: map[string]any{}})
	})

	ex, err := NewSidecarExecutorWithEndpoint(endpoint, 5*time.Second)
	if err != nil {
		t.Fatalf("executor: %v", err)
	}

	scope := db.NewTxScope()
	scopeID := db.RegisterScope(scope)
	defer db.UnregisterScope(scopeID)
	ctx := db.WithTxScope(context.Background(), scope, scopeID)

	if _, err := ex.Execute(ctx, spec.Action{
		Name: "sell",
		Impl: &spec.ImplDecl{Type: spec.ImplSidecar, Ref: "sell-handler"},
	}, ExecuteParams{Module: "pharmacy", Entity: "otc-sale", ActionName: "sell"}); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if gotHeader != scopeID {
		t.Fatalf("X-FormSpec-Scope-Id header = %q, want %q", gotHeader, scopeID)
	}
}

// TestSidecarExecutor_NoScopeMeansNoHeader confirms the fallback: without
// an active TxScope in ctx, Execute sends no scope header at all — the app
// process then has nothing to echo back, and /ctx/... callbacks commit
// independently, exactly as before this feature existed.
func TestSidecarExecutor_NoScopeMeansNoHeader(t *testing.T) {
	var sawHeader bool
	endpoint := startAppListener(t, func(w http.ResponseWriter, r *http.Request) {
		_, sawHeader = r.Header["X-FormSpec-Scope-Id"]
		json.NewEncoder(w).Encode(sidecarInvokeResponse{Data: map[string]any{}})
	})

	ex, err := NewSidecarExecutorWithEndpoint(endpoint, 5*time.Second)
	if err != nil {
		t.Fatalf("executor: %v", err)
	}

	if _, err := ex.Execute(context.Background(), spec.Action{
		Name: "sell",
		Impl: &spec.ImplDecl{Type: spec.ImplSidecar, Ref: "sell-handler"},
	}, ExecuteParams{Module: "pharmacy", Entity: "otc-sale", ActionName: "sell"}); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if sawHeader {
		t.Fatal("expected no X-FormSpec-Scope-Id header when ctx carries no TxScope")
	}
}
