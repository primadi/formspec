package formspec

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/internal/api"
)

// TestKafe_LandedCostAllocationStrategy pins the selectable allocation for
// landed cost (kafe item 10.6) — and, more importantly, that an explicitly
// requested strategy REFUSES to run on data it cannot honour instead of
// silently producing a different number.
//
// The two strategies give different answers on the same purchase order, which is
// the whole reason the choice has to be declarable:
//
//	beras: 10.000 g (10 kg), nilai 150.000  → 89.3% of the weight, 38.5% of the value
//	kopi : 1.200 g,          nilai 240.000  → 10.7% of the weight, 61.5% of the value
//
// With 300.000 of shipping cost: beras is charged 42/unit by weight but 27/unit
// by value. Shipping follows weight, so value would under-charge the heavy item
// — but neither is "wrong" in the abstract, which is why it is configuration.
func TestKafe_LandedCostAllocationStrategy(t *testing.T) {
	for _, tc := range []struct {
		name           string
		berasPerUnit   float64
		kopiPerUnit    float64
		expectStrategy string
	}{
		// weight: beras 15 + (300000*0.893)/10000 = 41.79 → 42
		//         kopi  120 + (300000*0.107)/2000  = 136.07 → 136
		{name: "weight", berasPerUnit: 42, kopiPerUnit: 136},
		// value:  beras 15 + (300000*0.385)/10000 = 26.5 → 27
		//         kopi  120 + (300000*0.615)/2000  = 212.3 → 212
		{name: "value", berasPerUnit: 27, kopiPerUnit: 212},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := bootKafeWithStrategy(t, tc.name)
			token := seedKafeAdminToken(t, app)

			branchID := insertRecord(t, app, "cafe-master", "branch", map[string]any{
				"code": "KFE-AL-01", "name": "Cabang Alokasi", "tax_percent": 10,
			})
			supplierID := insertRecord(t, app, "cafe-stock", "supplier", map[string]any{
				"code": "SUP-AL", "name": "Pemasok Alokasi",
			})
			berasID := insertRecord(t, app, "cafe-stock", "ingredient", map[string]any{
				"code": "BHN-AL-BERAS", "name": "Beras Alokasi", "unit": "gram",
				"cost_per_unit":   map[string]any{"amount": "15", "currency": "IDR"},
				"weight_per_unit": 1,
			})
			kopiID := insertRecord(t, app, "cafe-stock", "ingredient", map[string]any{
				"code": "BHN-AL-KOPI", "name": "Kopi Alokasi", "unit": "gram",
				"cost_per_unit":   map[string]any{"amount": "120", "currency": "IDR"},
				"weight_per_unit": 0.6,
			})

			po := map[string]any{
				"transaction_date": "2026-09-22",
				"branch_id":        branchID,
				"supplier_id":      supplierID,
				"shipping_cost":    map[string]any{"amount": "300000", "currency": "IDR"},
				"lines": []any{
					map[string]any{"line_no": 1, "line_type": "ingredient", "ingredient_id": berasID,
						"quantity": 10000, "unit_cost": map[string]any{"amount": "15", "currency": "IDR"},
						"received_quantity": 10000},
					map[string]any{"line_no": 2, "line_type": "ingredient", "ingredient_id": kopiID,
						"quantity": 2000, "unit_cost": map[string]any{"amount": "120", "currency": "IDR"},
						"received_quantity": 2000},
				},
			}
			status, body := doAuthed(t, app, http.MethodPost,
				"/kafe/_ui/entity/cafe-stock/purchase-order", token, po)
			if status != http.StatusCreated && status != http.StatusOK {
				t.Fatalf("create PO: %d %v", status, body)
			}
			poID, _ := body["data"].(map[string]any)["id"].(string)
			if status, body = doAuthed(t, app, http.MethodPatch,
				"/kafe/_ui/entity/cafe-stock/purchase-order/"+poID, token,
				map[string]any{"status": "submitted"}); status != http.StatusOK {
				t.Fatalf("submit PO: %d %v", status, body)
			}
			if status, body = doAuthed(t, app, http.MethodPost,
				"/kafe/_ui/entity/cafe-stock/purchase-order/"+poID+"/receive-goods", token,
				map[string]any{}); status != http.StatusOK && status != http.StatusCreated {
				t.Fatalf("receive-goods: %d %v", status, body)
			}

			if got := movementUnitCost(t, app, berasID); got != tc.berasPerUnit {
				t.Errorf("strategy %s: beras unit_cost = %v, want %v", tc.name, got, tc.berasPerUnit)
			}
			if got := movementUnitCost(t, app, kopiID); got != tc.kopiPerUnit {
				t.Errorf("strategy %s: kopi unit_cost = %v, want %v", tc.name, got, tc.kopiPerUnit)
			}
		})
	}
}

// TestKafe_WeightStrategyRefusesUnknownWeight pins refuse-not-guess: with
// `weight` requested explicitly, an ingredient whose weight is unknown must fail
// the receive with a message naming the ingredient.
//
// Silently falling back to `value` would produce a plausible number computed on
// a basis nobody chose — the failure mode that costs the most, because no one
// knows to look.
func TestKafe_WeightStrategyRefusesUnknownWeight(t *testing.T) {
	app := bootKafeWithStrategy(t, "weight")
	token := seedKafeAdminToken(t, app)

	branchID := insertRecord(t, app, "cafe-master", "branch", map[string]any{
		"code": "KFE-AL-02", "name": "Cabang Tanpa Berat", "tax_percent": 10,
	})
	supplierID := insertRecord(t, app, "cafe-stock", "supplier", map[string]any{
		"code": "SUP-AL2", "name": "Pemasok Tanpa Berat",
	})
	// No weight_per_unit — that is the point.
	bahanID := insertRecord(t, app, "cafe-stock", "ingredient", map[string]any{
		"code": "BHN-AL-NOBERAT", "name": "Bahan Tanpa Berat", "unit": "gram",
		"cost_per_unit": map[string]any{"amount": "10", "currency": "IDR"},
	})

	po := map[string]any{
		"transaction_date": "2026-09-22",
		"branch_id":        branchID,
		"supplier_id":      supplierID,
		"shipping_cost":    map[string]any{"amount": "100000", "currency": "IDR"},
		"lines": []any{
			map[string]any{"line_no": 1, "line_type": "ingredient", "ingredient_id": bahanID,
				"quantity": 100, "unit_cost": map[string]any{"amount": "10", "currency": "IDR"},
				"received_quantity": 100},
		},
	}
	status, body := doAuthed(t, app, http.MethodPost,
		"/kafe/_ui/entity/cafe-stock/purchase-order", token, po)
	if status != http.StatusCreated && status != http.StatusOK {
		t.Fatalf("create PO: %d %v", status, body)
	}
	poID, _ := body["data"].(map[string]any)["id"].(string)
	doAuthed(t, app, http.MethodPatch,
		"/kafe/_ui/entity/cafe-stock/purchase-order/"+poID, token,
		map[string]any{"status": "submitted"})
	status, body = doAuthed(t, app, http.MethodPost,
		"/kafe/_ui/entity/cafe-stock/purchase-order/"+poID+"/receive-goods", token,
		map[string]any{})

	if status < 400 {
		t.Fatalf("receive-goods succeeded (%d) with allocation_strategy=weight and an "+
			"ingredient whose weight is unknown — it must refuse, not pick a different "+
			"basis silently\nbody=%v", status, body)
	}
	msg := ""
	if e, ok := body["error"].(map[string]any); ok {
		msg, _ = e["message"].(string)
	}
	if !contains(msg, "WEIGHT_MISSING") {
		t.Errorf("error message should name the missing weight (WEIGHT_MISSING), got: %q", msg)
	}
}

// bootKafeWithStrategy boots kafe from a COPY of the spec tree whose stock
// config declares the given allocation strategy.
//
// A copy rather than a runtime override because config values are resolved once
// during boot (they back ctx.config). Patching the example tree in place would
// make the test order-dependent — a test that changes a shared fixture leaves it
// changed for whatever runs next.
func bootKafeWithStrategy(t *testing.T, strategy string) *App {
	t.Helper()
	dir := copyKafeSpec(t)
	cfg := filepath.Join(dir, "modules", "cafe-stock", "config", "stock.yaml")
	raw, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatalf("read stock config: %v", err)
	}
	// The default is the only thing that needs to change: it is what the script
	// reads when nothing else is set.
	patched := bytes.Replace(raw,
		[]byte("      default: \"\""),
		[]byte("      default: \""+strategy+"\""), 1)
	if bytes.Equal(patched, raw) {
		t.Fatalf("could not patch allocation_strategy default in %s", cfg)
	}
	if err := os.WriteFile(cfg, patched, 0o644); err != nil {
		t.Fatalf("write stock config: %v", err)
	}

	api.ResetAuthRateLimiters()
	app, err := New(Config{
		SpecPath:    dir,
		DSN:         "sqlite:" + filepath.Join(t.TempDir(), "alloc.db"),
		WorkspaceID: "kafe",
	})
	if err != nil {
		t.Fatalf("boot kafe copy: %v", err)
	}
	t.Cleanup(func() { _ = app.Close(context.Background()) })
	app.StartBackgroundWorkers()
	return app
}

// copyKafeSpec copies examples/kafe/spec into a temp dir.
func copyKafeSpec(t *testing.T) string {
	t.Helper()
	src := filepath.Join("..", "examples", "kafe", "spec")
	dst := filepath.Join(t.TempDir(), "spec")
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copy kafe spec: %v", err)
	}
	return dst
}

// movementUnitCost reads the numeric amount of a purchase movement's unit_cost
// for one ingredient — the value the allocation produced.
func movementUnitCost(t *testing.T, app *App, ingredientID string) float64 {
	t.Helper()
	store, err := app.Registry().GetEntityStore("cafe-stock", "stock-movement")
	if err != nil {
		t.Fatalf("stock-movement store: %v", err)
	}
	rec, err := store.FindByFields(context.Background(), "kafe", map[string]any{
		"ingredient_id": ingredientID,
	})
	if err != nil || rec == nil {
		t.Fatalf("no movement for ingredient %s: %v", ingredientID, err)
	}
	return numberOf(rec.Data["unit_cost"])
}
