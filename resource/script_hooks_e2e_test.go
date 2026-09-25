package formspec

import (
	"context"
	"net/http"
	"testing"

	"github.com/primadi/formspec/internal/api"
	"github.com/primadi/formspec/internal/auth"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// TestKafe_ReceiveGoodsMaintainsStockLevel pins a REAL gap found while wiring
// kafe item 10.1/10.2 (purchase → stock-movement + landed cost).
//
// `resource.create` from a Starlark script inserted the row with
// `EntityStore.Insert` and stopped there — it never ran the target entity's
// `after create` hooks, although the HTTP path does. Kafe's `stock-movement`
// maintains the `stock-level` projection from exactly such a hook, so receiving
// a purchase through `receive-goods` produced movements that moved no balance:
//
//	movement:    qty=10000 unit_cost=40   (row exists — looked fine)
//	stock-level: (absent)                 (projection never updated)
//
// Nothing failed and no log line appeared; the only symptom was an empty
// projection. That is a silent hole in the "one business action feeds another"
// path, which is how non-trivial apps are wired.
//
// The test runs the real chain over HTTP and asserts the projection exists AND
// carries the landed-cost-adjusted average.
func TestKafe_ReceiveGoodsMaintainsStockLevel(t *testing.T) {
	app := bootKafe(t)
	token := seedKafeAdminToken(t, app)

	branchID := insertRecord(t, app, "cafe-master", "branch", map[string]any{
		"code": "KFE-UJI-01", "name": "Cabang Uji", "tax_percent": 10,
	})
	supplierID := insertRecord(t, app, "cafe-stock", "supplier", map[string]any{
		"code": "SUP-UJI", "name": "Pemasok Uji",
	})
	berasID := insertRecord(t, app, "cafe-stock", "ingredient", map[string]any{
		"code": "BHN-UJI-01", "name": "Beras Uji", "unit": "gram",
		"cost_per_unit": map[string]any{"amount": "15", "currency": "IDR"},
		"min_stock":     100,
	})

	// 10.000 gram @ 15 = 150.000, plus 250.000 of landed cost (200.000 shipping
	// on the order + 50.000 allocated to the ingredient), so the effective cost
	// per unit must be 40 — not 15.
	po := map[string]any{
		"transaction_date": "2026-09-22",
		"branch_id":        branchID,
		"supplier_id":      supplierID,
		"shipping_cost":    map[string]any{"amount": "200000", "currency": "IDR"},
		"lines": []any{
			map[string]any{
				"line_no": 1, "line_type": "ingredient", "ingredient_id": berasID,
				"quantity": 10000, "unit_cost": map[string]any{"amount": "15", "currency": "IDR"},
				"received_quantity": 10000,
			},
			map[string]any{
				"line_no": 2, "line_type": "cost", "cost_label": "Sewa cold storage",
				"cost_amount": map[string]any{"amount": "50000", "currency": "IDR"},
			},
		},
	}
	status, body := doAuthed(t, app, http.MethodPost,
		"/kafe/_ui/entity/cafe-stock/purchase-order", token, po)
	if status != http.StatusCreated && status != http.StatusOK {
		t.Fatalf("create PO: status %d body %v", status, body)
	}
	poData, _ := body["data"].(map[string]any)
	poID, _ := poData["id"].(string)
	if poID == "" {
		t.Fatalf("create PO: no id in response %v", body)
	}

	// A transition WITHOUT `impl` is applied through PATCH with the target
	// status (UI REST contract §2.7); `receive-goods` HAS an impl → POST.
	status, body = doAuthed(t, app, http.MethodPatch,
		"/kafe/_ui/entity/cafe-stock/purchase-order/"+poID, token,
		map[string]any{"status": "submitted"})
	if status != http.StatusOK {
		t.Fatalf("submit PO: status %d body %v", status, body)
	}

	status, body = doAuthed(t, app, http.MethodPost,
		"/kafe/_ui/entity/cafe-stock/purchase-order/"+poID+"/receive-goods", token, map[string]any{})
	if status != http.StatusOK && status != http.StatusCreated {
		t.Fatalf("receive-goods: status %d body %v", status, body)
	}

	ctx := context.Background()

	// 1. The movement exists and carries the ALLOCATED cost, not the list price.
	mvStore, err := app.Registry().GetEntityStore("cafe-stock", "stock-movement")
	if err != nil {
		t.Fatalf("stock-movement store: %v", err)
	}
	mv, err := mvStore.FindByFields(ctx, "kafe", map[string]any{"ingredient_id": berasID})
	if err != nil || mv == nil {
		t.Fatalf("no stock-movement created by receive-goods: %v", err)
	}
	if got := numberOf(mv.Data["unit_cost"]); got != 40 {
		t.Errorf("movement unit_cost = %v, want 40 (15 + 250000/10000 landed cost)", mv.Data["unit_cost"])
	}

	// 2. The projection must exist — this assertion is what fails when
	//    `resource.create` skips the `after create` hook.
	levelStore, err := app.Registry().GetEntityStore("cafe-stock", "stock-level")
	if err != nil {
		t.Fatalf("stock-level store: %v", err)
	}
	level, err := levelStore.FindByFields(ctx, "kafe", map[string]any{
		"branch_id":     branchID,
		"ingredient_id": berasID,
	})
	if err != nil {
		t.Fatalf("find stock-level: %v", err)
	}
	if level == nil {
		t.Fatal("stock-level row was NOT created: the movement exists but the " +
			"projection never learned about it — `resource.create` did not run the " +
			"target entity's `after create` hook (the silent gap this test catches)")
	}
	if qty := numberOf(level.Data["quantity_on_hand"]); qty != 10000 {
		t.Errorf("stock-level quantity_on_hand = %v, want 10000", level.Data["quantity_on_hand"])
	}
	if got := numberOf(level.Data["moving_avg_cost"]); got != 40 {
		t.Errorf("stock-level moving_avg_cost = %v, want 40 (landed cost must reach the projection)", level.Data["moving_avg_cost"])
	}
}

// seedKafeAdminToken seeds an admin user INSIDE the kafe workspace and returns a
// bearer token for it. seedAdminToken (auth harness) writes to workspace
// "default", and a token minted there resolves the request workspace from its
// claims — so calling it here produced `workspace not found` on every /kafe/...
// request. The token must belong to the same workspace as the URLs.
func seedKafeAdminToken(t *testing.T, app *App) string {
	t.Helper()
	api.ResetAuthRateLimiters()
	reg := app.Registry()
	userStore, err := reg.GetEntityStore("formspec.core", "user")
	if err != nil {
		t.Fatalf("user store: %v", err)
	}
	hash, err := auth.HashPassword("admin")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if _, err := userStore.Insert(context.Background(), db.InsertParams{
		WorkspaceID: "kafe", CreatedBy: "test",
		Data: map[string]any{
			"username": "admin", "password_hash": hash,
			"roles": []string{}, "permissions": []string{"*"}, "active": true,
		},
	}); err != nil {
		t.Fatalf("insert admin: %v", err)
	}
	status, body := doJSON(t, app, http.MethodPost, "/kafe/_ui/auth/login", map[string]any{
		"username": "admin", "password": "admin",
	})
	if status != http.StatusOK {
		t.Fatalf("login: status %d body %v", status, body)
	}
	data, _ := body["data"].(map[string]any)
	tok, _ := data["access_token"].(string)
	if tok == "" {
		t.Fatalf("login: no access_token in %v", body)
	}
	return tok
}

// insertRecord creates a record directly through the store. Test setup helper —
// it bypasses permissions deliberately, since authorization is not the subject.
func insertRecord(t *testing.T, app *App, module, entity string, data map[string]any) string {
	t.Helper()
	store, err := app.Registry().GetEntityStore(module, entity)
	if err != nil {
		t.Fatalf("store %s.%s: %v", module, entity, err)
	}
	id, err := store.Insert(context.Background(), db.InsertParams{
		WorkspaceID: "kafe",
		CreatedBy:   "test",
		Data:        data,
	})
	if err != nil {
		t.Fatalf("insert %s.%s: %v", module, entity, err)
	}
	return id
}
