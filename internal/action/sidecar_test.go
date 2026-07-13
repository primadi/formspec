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

	"github.com/primadi/forma/pkg/spec"
)

// startAppListener runs a minimal lib-forma-style /invoke listener on a
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
