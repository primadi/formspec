package clinic_test

import (
	"net/http"
	"testing"
)

// TestOTCSale_EndToEnd covers scenario (c) — a walk-in purchase with no
// prescription at all. create (pending) -> sell (completed): total is
// computed in the script (otc_sell.star), and medicine stock decrements,
// mirroring the prescription/dispense idiom on a dedicated, simpler entity.
func TestOTCSale_EndToEnd(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()

	status, env := do(t, handler, "POST", "/demo/_ui/entity/pharmacy/medicine", map[string]any{
		"sku": "SKU-200", "name": "Vitamin C 500mg", "unit": "tablet", "stock": 100, "price": 1000,
	})
	if status != http.StatusCreated {
		t.Fatalf("create medicine: status %d, body %+v", status, env)
	}
	medicineID := dataMap(t, env)["id"].(string)

	status, env = do(t, handler, "POST", "/demo/_ui/entity/pharmacy/otc-sale", map[string]any{
		"transaction_date": "2026-07-12",
		"buyer_name":       "Pembeli Umum",
		"items": []map[string]any{
			{"line_number": 1, "medicine_id": medicineID, "quantity": 4, "unit_price": 1000},
		},
	})
	if status != http.StatusCreated {
		t.Fatalf("create otc-sale: status %d, body %+v", status, env)
	}
	sale := dataMap(t, env)
	saleID := sale["id"].(string)
	if got := sale["status"]; got != "pending" {
		t.Fatalf("expected status pending after create, got %v", got)
	}

	status, env = do(t, handler, "POST", "/demo/_ui/entity/pharmacy/otc-sale/"+saleID+"/sell", nil)
	if status != http.StatusOK {
		t.Fatalf("sell: status %d, body %+v", status, env)
	}

	status, env = do(t, handler, "GET", "/demo/_ui/entity/pharmacy/otc-sale/"+saleID, nil)
	if status != http.StatusOK {
		t.Fatalf("get otc-sale: status %d, body %+v", status, env)
	}
	final := dataMap(t, env)
	if got := final["status"]; got != "completed" {
		t.Fatalf("expected status completed, got %v", got)
	}
	if got := final["total"]; got != float64(4*1000) {
		t.Errorf("expected total %v, got %v", float64(4*1000), got)
	}

	status, env = do(t, handler, "GET", "/demo/_ui/entity/pharmacy/medicine/"+medicineID, nil)
	if status != http.StatusOK {
		t.Fatalf("get medicine: status %d, body %+v", status, env)
	}
	if got := dataMap(t, env)["stock"]; got != float64(96) {
		t.Errorf("expected medicine stock 96 after sell, got %v", got)
	}
}

// TestOTCSale_Cancel proves the pending -> cancelled path leaves stock
// untouched (no stock decrement script runs on cancel).
func TestOTCSale_Cancel(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()

	status, env := do(t, handler, "POST", "/demo/_ui/entity/pharmacy/medicine", map[string]any{
		"sku": "SKU-201", "name": "Antacid", "unit": "tablet", "stock": 10, "price": 500,
	})
	if status != http.StatusCreated {
		t.Fatalf("create medicine: status %d, body %+v", status, env)
	}
	medicineID := dataMap(t, env)["id"].(string)

	status, env = do(t, handler, "POST", "/demo/_ui/entity/pharmacy/otc-sale", map[string]any{
		"transaction_date": "2026-07-12",
		"items": []map[string]any{
			{"line_number": 1, "medicine_id": medicineID, "quantity": 2, "unit_price": 500},
		},
	})
	if status != http.StatusCreated {
		t.Fatalf("create otc-sale: status %d, body %+v", status, env)
	}
	saleID := dataMap(t, env)["id"].(string)

	status, env = do(t, handler, "POST", "/demo/_ui/entity/pharmacy/otc-sale/"+saleID+"/cancel", nil)
	if status != http.StatusOK {
		t.Fatalf("cancel: status %d, body %+v", status, env)
	}

	status, env = do(t, handler, "GET", "/demo/_ui/entity/pharmacy/otc-sale/"+saleID, nil)
	if status != http.StatusOK {
		t.Fatalf("get otc-sale: status %d, body %+v", status, env)
	}
	if got := dataMap(t, env)["status"]; got != "cancelled" {
		t.Fatalf("expected status cancelled, got %v", got)
	}

	status, env = do(t, handler, "GET", "/demo/_ui/entity/pharmacy/medicine/"+medicineID, nil)
	if status != http.StatusOK {
		t.Fatalf("get medicine: status %d, body %+v", status, env)
	}
	if got := dataMap(t, env)["stock"]; got != float64(10) {
		t.Errorf("expected medicine stock untouched at 10, got %v", got)
	}
}

// TestOTCSale_StockGuardRejectsInsufficientStock proves the hooks:
// before/create guard (otc_sale_stock_guard.star) aborts the create — via
// fail() — before the row ever exists, when a line item's quantity exceeds
// available stock.
func TestOTCSale_StockGuardRejectsInsufficientStock(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()

	status, env := do(t, handler, "POST", "/demo/_ui/entity/pharmacy/medicine", map[string]any{
		"sku": "SKU-202", "name": "Low Stock Item", "unit": "tablet", "stock": 2, "price": 1000,
	})
	if status != http.StatusCreated {
		t.Fatalf("create medicine: status %d, body %+v", status, env)
	}
	medicineID := dataMap(t, env)["id"].(string)

	status, env = do(t, handler, "POST", "/demo/_ui/entity/pharmacy/otc-sale", map[string]any{
		"transaction_date": "2026-07-12",
		"items": []map[string]any{
			{"line_number": 1, "medicine_id": medicineID, "quantity": 5, "unit_price": 1000},
		},
	})
	if status == http.StatusCreated {
		t.Fatalf("expected create to be rejected by the stock guard hook (requested 5, stock 2), got 201: %+v", env)
	}
	if status != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 HOOK_ABORTED, got status %d: %+v", status, env)
	}
	if env.Error == nil || env.Error.Code != "HOOK_ABORTED" {
		t.Errorf("expected error code HOOK_ABORTED, got %+v", env.Error)
	}
}

// TestOTCSale_SellRejectsEmptyItems proves the state-machine guard on the
// pending -> completed transition (items non-empty) is enforced.
func TestOTCSale_SellRejectsEmptyItems(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()

	status, env := do(t, handler, "POST", "/demo/_ui/entity/pharmacy/otc-sale", map[string]any{
		"transaction_date": "2026-07-12",
		"buyer_name":       "No Items Buyer",
	})
	if status != http.StatusCreated {
		t.Fatalf("create otc-sale: status %d, body %+v", status, env)
	}
	saleID := dataMap(t, env)["id"].(string)

	status, _ = do(t, handler, "POST", "/demo/_ui/entity/pharmacy/otc-sale/"+saleID+"/sell", nil)
	if status == http.StatusOK {
		t.Fatal("expected sell to fail with no items, but it succeeded")
	}
}
