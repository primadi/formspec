package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/action"
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/service"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// TestServiceAction_HTTP verifies a stateless Service action is exposed via
// POST /api/v1/{module}/{service}/{action} and dispatched through the action
// dispatcher (todo 7.1).
func TestServiceAction_HTTP(t *testing.T) {
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "svc.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := db.NewMigrationRunner(d, db.DriverSQLite)
	ctx := context.Background()
	r.EnsureSystemTables(ctx)

	reg := entity.NewRegistry(d, db.DriverSQLite, dir)
	reg.LoadEntities()

	// Service registry with a native-backed tax-calculator service.
	svcReg := service.NewRegistry()
	svcReg.Add("billing", "tax-calculator", &spec.ServiceSpec{
		Version: "v1",
		Actions: []spec.Action{
			{
				Name:               "calculate",
				RequiredPermission: "billing.tax-calculator.calculate",
				Impl:               &spec.ImplDecl{Type: spec.ImplNative, Ref: "TaxService.Calculate"},
			},
		},
	})

	// Dispatcher with a native executor that implements the service action.
	disp := action.NewDispatcher()
	nativeEx := action.NewNativeExecutor()
	nativeEx.Register("TaxService.Calculate", func(ctx context.Context, params action.ExecuteParams) (any, error) {
		amount, _ := params.Params["amount"].(float64)
		rate, _ := params.Params["rate"].(float64)
		return map[string]any{"tax": amount * rate}, nil
	})
	disp.RegisterExecutor(spec.ImplNative, nativeEx)
	disp.SetNativeExecutor(nativeEx)

	rb := NewRouterBuilder(reg)
	rb.SetDispatcher(disp)
	rb.SetServiceRegistry(svcReg)
	rb.BuildRoutes()
	handler := rb.BuildHTTP()

	// The route requires permission billing.tax-calculator.calculate. In the
	// test harness there is no auth identity, so RequirePermission fails open
	// (no identity → no permissions → denied). To exercise the happy path we
	// call the handler directly via the route with a permission bypass is not
	// possible here; instead verify the route is registered (200/403, not 404)
	// and that the handler dispatches correctly when invoked directly.
	req := httptest.NewRequest("POST", "/demo/api/v1/billing/tax-calculator/calculate", strings.NewReader(`{"amount":100,"rate":0.1}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Without an authenticated identity, RequirePermission denies (403) rather
	// than 404 — proving the route exists and is permission-gated.
	if rec.Code == http.StatusNotFound {
		t.Fatalf("service route not registered: got 404")
	}

	// Now verify the handler itself dispatches correctly by calling it
	// directly (bypassing the permission middleware).
	factory := NewHandlerFactory(reg)
	factory.SetDispatcher(disp)
	factory.SetServiceRegistry(svcReg)
	actionSpec, _ := svcReg.GetAction("billing", "tax-calculator", "calculate")
	h := factory.HandleServiceAction("billing", "tax-calculator", "calculate", *actionSpec)

	req2 := httptest.NewRequest("POST", "/", strings.NewReader(`{"amount":100,"rate":0.1}`))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	h(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec2.Code, rec2.Body.String())
	}
	var resp SingleResponse
	if err := json.NewDecoder(rec2.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected data map, got %T", resp.Data)
	}
	if tax, ok := data["tax"].(float64); !ok || tax != 10.0 {
		t.Errorf("expected tax=10, got %v", data["tax"])
	}
}

// TestServiceAction_Async verifies `call: async` returns 202 Accepted without
// waiting for the handler (todo 7.1.4).
func TestServiceAction_Async(t *testing.T) {
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "svc_async.db"), nil)
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
	svcReg.Add("notify", "sms", &spec.ServiceSpec{
		Actions: []spec.Action{
			{
				Name: "send",
				Call: "async",
				Impl: &spec.ImplDecl{Type: spec.ImplNative, Ref: "SmsService.Send"},
			},
		},
	})

	disp := action.NewDispatcher()
	nativeEx := action.NewNativeExecutor()
	nativeEx.Register("SmsService.Send", func(ctx context.Context, params action.ExecuteParams) (any, error) {
		return map[string]any{"sent": true}, nil
	})
	disp.RegisterExecutor(spec.ImplNative, nativeEx)
	disp.SetNativeExecutor(nativeEx)

	factory := NewHandlerFactory(reg)
	factory.SetDispatcher(disp)
	factory.SetServiceRegistry(svcReg)
	actionSpec, _ := svcReg.GetAction("notify", "sms", "send")
	h := factory.HandleServiceAction("notify", "sms", "send", *actionSpec)

	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"to":"+62"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted, got %d", rec.Code)
	}
}
