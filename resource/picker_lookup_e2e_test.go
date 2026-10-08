package formspec

import (
	"net/http"
	"strings"
	"testing"
	"time"

	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// kafe 10.79 over the REAL HTTP surface: a line's price is read from the
// CATALOG, never from the request.
//
// The picker already declared where a line's price lives — `lookup.entity`,
// `lookup.key`, `lookup.field`, and the dimension it must match — and the
// browser joined that list and copied the number into `unit_price_snapshot`.
// That made a MONEY FIELD the caller's to author: a guest could send any amount,
// and the order, its tax, and the journal would all agree with it. There is no
// PII at stake, but the amount is the whole point of the transaction.
//
// The framework already answers this shape elsewhere — a `computed` value is
// stripped because "a derived value is never the caller's to set". A picked
// price is the same thing, so the server now resolves it and a caller-supplied
// amount is REPLACED. No script is involved: a hook would be a second place that
// knows how to price a line, and the two would drift.

// orderBody builds the payload the QR form submits, with an explicit (and
// possibly dishonest) line price.
func orderBody(ids kafeSeedIDs, sessionID, guestToken, menuItemID, name string, price string, qty int) map[string]any {
	return map[string]any{
		"transaction_date": time.Now().UTC().Format(time.RFC3339),
		"branch_id":        ids.branchID,
		"channel":          "qr_table",
		"table_session_id": sessionID,
		"guest_token":      guestToken,
		"dining_table_id":  ids.tableID,
		"lines": []any{
			map[string]any{
				"line_no":             1,
				"menu_item_id":        menuItemID,
				"name_snapshot":       name,
				"unit_price_snapshot": map[string]any{"amount": price, "currency": "IDR"},
				"quantity":            qty,
				"prep_station":        "kitchen",
			},
		},
	}
}

// TestKafe_OrderPriceIsNotTheCallersToSend is the fix under test: an anonymous
// guest states a price of 1 and the stored line carries the catalog's 45000.
func TestKafe_OrderPriceIsNotTheCallersToSend(t *testing.T) {
	app := bootKafe(t)
	ids := seedKafeTableScenario(t, app)

	sessionID := insertRecord(t, app, "cafe-order", "table-session", map[string]any{
		"transaction_date": time.Now().UTC().Format(time.RFC3339),
		"dining_table_id":  ids.tableID,
		"branch_id":        ids.branchID,
		"guest_token":      "PRICE-LIE-1",
	})

	// Anonymous, exactly like the QR flow — no staff token softens this.
	status, out := doJSON(t, app, http.MethodPost,
		"/kafe/_ui/entity/cafe-order/order",
		orderBody(ids, sessionID, "PRICE-LIE-1", ids.nasiGoreng, "Nasi Goreng Spesial", "1", 1))
	if status != http.StatusCreated {
		t.Fatalf("anonymous order create: expected 201, got %d (%v)", status, out)
	}
	data, _ := out["data"].(map[string]any)
	line := firstLine(t, data)

	if got := numberOf(line["unit_price_snapshot"]); got != 45000 {
		t.Fatalf("stored line price = %v, want the catalog's 45000 — the caller authored a money field", got)
	}
	if got := numberOf(data["subtotal"]); got != 45000 {
		t.Fatalf("subtotal = %v, want 45000 (derived from the catalog price, not the request)", got)
	}
}

// TestKafe_OrderPriceIsFrozenOnUpdate pins the other half of the rule: the price
// is resolved when the LINE joins the record, and kept afterwards.
//
// Re-deriving on every edit would silently re-price an order the guest already
// agreed to, and it would make "change the quantity" a way to author a price.
func TestKafe_OrderPriceIsFrozenOnUpdate(t *testing.T) {
	app := bootKafe(t)
	ids := seedKafeTableScenario(t, app)
	admin := seedKafeAdminToken(t, app)

	sessionID := insertRecord(t, app, "cafe-order", "table-session", map[string]any{
		"transaction_date": time.Now().UTC().Format(time.RFC3339),
		"dining_table_id":  ids.tableID,
		"branch_id":        ids.branchID,
		"guest_token":      "PRICE-FREEZE-1",
	})

	status, out := doAuthed(t, app, http.MethodPost,
		"/kafe/_ui/entity/cafe-order/order", admin,
		orderBody(ids, sessionID, "PRICE-FREEZE-1", ids.nasiGoreng, "Nasi Goreng Spesial", "45000", 1))
	if status != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d (%v)", status, out)
	}
	created, _ := out["data"].(map[string]any)
	orderID, _ := created["id"].(string)

	// The catalog price moves. The existing line must NOT follow.
	priceStore, err := app.GetEntityStore("cafe-master", "menu-item-price")
	if err != nil {
		t.Fatalf("price store: %v", err)
	}
	res, err := priceStore.List(t.Context(), db.ListParams{
		WorkspaceID: "kafe", Page: 1, PerPage: 100,
		Filters: map[string]db.FilterOp{"menu_item_id": {Op: "eq", Value: ids.nasiGoreng}},
	})
	if err != nil || len(res.Data) == 0 {
		t.Fatalf("price rows: %v err=%v", res, err)
	}
	priceRowID := res.Data[0].ID
	if err := priceStore.UpdateFields(t.Context(), "kafe", priceRowID, map[string]any{
		"price": map[string]any{"amount": "77777", "currency": "IDR"},
	}); err != nil {
		t.Fatalf("reprice the catalog: %v", err)
	}

	// Edit the order: a different (dishonest) price AND a new quantity.
	body := orderBody(ids, sessionID, "PRICE-FREEZE-1", ids.nasiGoreng, "Nasi Goreng Spesial", "1", 2)
	status, out = doAuthed(t, app, http.MethodPatch,
		"/kafe/_ui/entity/cafe-order/order/"+orderID, admin, body)
	if status != http.StatusOK {
		t.Fatalf("update: expected 200, got %d (%v)", status, out)
	}
	updated, _ := out["data"].(map[string]any)
	line := firstLine(t, updated)

	if got := numberOf(line["unit_price_snapshot"]); got != 45000 {
		t.Fatalf("line price after edit = %v, want the FROZEN 45000 (neither the caller's 1 nor the new catalog 77777)", got)
	}
	if got := numberOf(line["quantity"]); got != 2 {
		t.Fatalf("quantity = %v, want 2 — the edit itself must still apply", got)
	}
	if got := numberOf(updated["subtotal"]); got != 90000 {
		t.Fatalf("subtotal = %v, want 90000 (2 × the frozen price)", got)
	}
}

// TestKafe_UnpricedLineIsRefused pins fail-closed: a line the catalog cannot
// price is refused instead of falling back to whatever the request said.
func TestKafe_UnpricedLineIsRefused(t *testing.T) {
	app := bootKafe(t)
	ids := seedKafeTableScenario(t, app)
	admin := seedKafeAdminToken(t, app)

	// A menu item with NO price row for this branch. It needs a category, so the
	// row is valid in every other respect — the ONLY thing missing is its price.
	category := insertRecord(t, app, "cafe-master", "menu-category", map[string]any{
		"name": "Tanpa Harga", "sort_order": 99, "prep_station_default": "kitchen",
	})
	unpriced := insertRecord(t, app, "cafe-master", "menu-item", map[string]any{
		"code": "MKN-NOPRICE", "name": "Menu Tanpa Harga",
		"menu_category_id": category,
		"prep_station":     "kitchen", "is_available": true, "is_taxable": true,
	})

	sessionID := insertRecord(t, app, "cafe-order", "table-session", map[string]any{
		"transaction_date": time.Now().UTC().Format(time.RFC3339),
		"dining_table_id":  ids.tableID,
		"branch_id":        ids.branchID,
		"guest_token":      "PRICE-MISSING-1",
	})

	status, out := doAuthed(t, app, http.MethodPost,
		"/kafe/_ui/entity/cafe-order/order", admin,
		orderBody(ids, sessionID, "PRICE-MISSING-1", unpriced, "Menu Tanpa Harga", "1000", 1))
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("a line the catalog cannot price must be refused with 422, got %d (%v)", status, out)
	}
	errObj, _ := out["error"].(map[string]any)
	msg, _ := errObj["message"].(string)
	if !strings.Contains(msg, "no cafe-master.menu-item-price row matches") {
		t.Errorf("refusal must say the catalog has no matching row, got %q", msg)
	}
	t.Logf("refused as expected: %s", msg)
}

// firstLine returns a record's first child row.
func firstLine(t *testing.T, rec map[string]any) map[string]any {
	t.Helper()
	rows, _ := rec["lines"].([]any)
	if len(rows) == 0 {
		t.Fatalf("record carries no lines: %v", rec)
	}
	line, _ := rows[0].(map[string]any)
	if line == nil {
		t.Fatalf("first line is not an object: %v", rows[0])
	}
	return line
}

// TestKafe_QuantityIsBoundedOnTheWire drives the REAL HTTP surface: the picker's
// quantity bounds are enforced by the SERVER, not only by the browser's clamp.
//
// Why the browser clamp is not enough (measured): `clampQuantity` lives in the
// client, so a direct API caller — or any client that skips the picker — could
// store any number. The row's amount is `quantity × the frozen price`, so that
// number feeds the order total, its tax and the journal; `map.max_quantity` was
// validated as a manifest requirement and then never enforced on the write path.
func TestKafe_QuantityIsBoundedOnTheWire(t *testing.T) {
	app := bootKafe(t)
	ids := seedKafeTableScenario(t, app)

	sessionID := insertRecord(t, app, "cafe-order", "table-session", map[string]any{
		"transaction_date": time.Now().UTC().Format(time.RFC3339),
		"dining_table_id":  ids.tableID,
		"branch_id":        ids.branchID,
		"guest_token":      "QTY-BOUND-1",
	})

	cases := []struct {
		name    string
		qty     int
		wantMsg string
	}{
		{"zero", 0, "at least 1"},
		{"negative", -3, "at least 1"},
		// The picker declares map.max_quantity: 20, so 21 is above the bound its
		// own author chose.
		{"above the declared maximum", 21, "at most 20"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Anonymous, exactly like the QR flow.
			status, out := doJSON(t, app, http.MethodPost,
				"/kafe/_ui/entity/cafe-order/order",
				orderBody(ids, sessionID, "QTY-BOUND-1", ids.nasiGoreng, "Nasi Goreng Spesial", "45000", tc.qty))
			if status != http.StatusUnprocessableEntity {
				t.Fatalf("quantity %d must be refused with 422, got %d (%v)", tc.qty, status, out)
			}
			errObj, _ := out["error"].(map[string]any)
			msg, _ := errObj["message"].(string)
			if !strings.Contains(msg, tc.wantMsg) {
				t.Errorf("refusal must name the bound (%q), got %q", tc.wantMsg, msg)
			}
		})
	}

	// The declared bounds themselves must keep working — a guard that blocks the
	// happy path gets disabled. 1 is the floor, 20 the ceiling.
	for _, qty := range []int{1, 20} {
		status, out := doJSON(t, app, http.MethodPost,
			"/kafe/_ui/entity/cafe-order/order",
			orderBody(ids, sessionID, "QTY-BOUND-1", ids.nasiGoreng, "Nasi Goreng Spesial", "45000", qty))
		if status != http.StatusCreated {
			t.Fatalf("quantity %d is within the declared bounds and must be accepted, got %d (%v)", qty, status, out)
		}
		data, _ := out["data"].(map[string]any)
		if got := numberOf(firstLine(t, data)["quantity"]); got != float64(qty) {
			t.Fatalf("stored quantity = %v, want %d", got, qty)
		}
	}
}

// TestKafe_QuantityRefusalWritesNothing holds the refusal to "and nothing was
// written" — the same standard the state-machine and create_scope guards are held
// to. A rule that reports after writing is a report, not a rule.
func TestKafe_QuantityRefusalWritesNothing(t *testing.T) {
	app := bootKafe(t)
	ids := seedKafeTableScenario(t, app)

	sessionID := insertRecord(t, app, "cafe-order", "table-session", map[string]any{
		"transaction_date": time.Now().UTC().Format(time.RFC3339),
		"dining_table_id":  ids.tableID,
		"branch_id":        ids.branchID,
		"guest_token":      "QTY-NOWRITE-1",
	})

	status, out := doJSON(t, app, http.MethodPost,
		"/kafe/_ui/entity/cafe-order/order",
		orderBody(ids, sessionID, "QTY-NOWRITE-1", ids.nasiGoreng, "Nasi Goreng Spesial", "45000", 0))
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("quantity 0 must be refused, got %d (%v)", status, out)
	}

	store, err := app.GetEntityStore("cafe-order", "order")
	if err != nil {
		t.Fatalf("order store: %v", err)
	}
	res, err := store.List(t.Context(), db.ListParams{WorkspaceID: "kafe", Page: 1, PerPage: 500})
	if err != nil {
		t.Fatalf("list orders: %v", err)
	}
	if len(res.Data) != 0 {
		t.Errorf("an order was written despite the refusal (%d rows)", len(res.Data))
	}
}
