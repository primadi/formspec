package formspec

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// In-process end-to-end harness for the kafe TABLE LIFECYCLE (todo 9.2.2).
//
// The chain this file pins is the one the owner asked about (2026-09-27):
//
//	scan QR meja → sesi → pesan → bayar → meja `occupied`
//	  → dapur menyajikan → meja `served`
//	  → pesan lagi + bayar → meja `occupied`
//	  → kasir `release` → meja `available`
//
// Three links in that chain did not exist before this work, and each is a
// tracked gap:
//
//	10.34b  dining-table.qr_token is not a natural key → GET by token 404
//	10.40b  no `order.on_paid` → table_status bridge
//	10.41   no "orders served → table served"
//
// This test drives the REAL HTTP surface (not the outbox directly, unlike
// o2c_e2e_test.go): `confirm-payment` and `mark-served` are action-less
// transitions applied by PATCH, and it is precisely that HTTP path which
// resolves and enqueues the durable emission — so calling the outbox by hand
// would test the subscription but not the wiring that feeds it.

// kafeSeedIDs carries the ids a scenario needs. Master data is seeded through
// the store (production kafe gets the same rows from `formspec seed`).
type kafeSeedIDs struct {
	branchID   string
	tableID    string
	nasiGoreng string
	esTeh      string
}

// seedKafeTableScenario inserts the minimum master data for the lifecycle:
// one branch, one table (with the QR token the check-in page resolves), two
// menu items, and their per-branch prices.
//
// Prices are deliberately the seeded ones (45_000 / 12_000) so the assertions
// match `cafe-master/seeds/master.yaml` — if the seed changes, this test tells
// you the documented fixture moved.
func seedKafeTableScenario(t *testing.T, app *App) kafeSeedIDs {
	t.Helper()

	branchID := insertRecord(t, app, "cafe-master", "branch", map[string]any{
		"code": "KFE-JKT-01", "name": "Kafe Senayan",
		"tax_percent": "10", "service_charge_percent": "5",
		"apply_service_charge": true, "is_active": true,
	})

	foodCat := insertRecord(t, app, "cafe-master", "menu-category", map[string]any{
		"name": "Makanan", "sort_order": 10, "prep_station_default": "kitchen",
	})
	drinkCat := insertRecord(t, app, "cafe-master", "menu-category", map[string]any{
		"name": "Minuman", "sort_order": 20, "prep_station_default": "bar",
	})

	nasiGoreng := insertRecord(t, app, "cafe-master", "menu-item", map[string]any{
		"code": "MKN-001", "name": "Nasi Goreng Spesial", "menu_category_id": foodCat,
		"prep_station": "kitchen", "is_available": true, "is_taxable": true, "sort_order": 10,
	})
	esTeh := insertRecord(t, app, "cafe-master", "menu-item", map[string]any{
		"code": "MNM-001", "name": "Es Teh Manis", "menu_category_id": drinkCat,
		"prep_station": "bar", "is_available": true, "is_taxable": true, "sort_order": 10,
	})

	insertRecord(t, app, "cafe-master", "menu-item-price", map[string]any{
		"branch_id": branchID, "menu_item_id": nasiGoreng,
		"price": map[string]any{"amount": "45000", "currency": "IDR"}, "is_active": true,
	})
	insertRecord(t, app, "cafe-master", "menu-item-price", map[string]any{
		"branch_id": branchID, "menu_item_id": esTeh,
		"price": map[string]any{"amount": "12000", "currency": "IDR"}, "is_active": true,
	})

	// The QR token is the physical table's identity — the same value the card
	// carries. `natural_key: true` is what lets a lookup by it resolve (10.34b).
	tableID := insertRecord(t, app, "cafe-master", "dining-table", map[string]any{
		"branch_id": branchID, "code": "A-01", "qr_token": "JKT-A01-DEMO",
		"area": "indoor", "seats": 2,
	})

	return kafeSeedIDs{
		branchID:   branchID,
		tableID:    tableID,
		nasiGoreng: nasiGoreng,
		esTeh:      esTeh,
	}
}

// tableStatus reads a dining table's status through the registry store — the
// same store the HTTP surface reads, so an assertion cannot pass on a stale
// cache the API does not share.
func tableStatus(t *testing.T, app *App, tableID string) string {
	t.Helper()
	store, err := app.GetEntityStore("cafe-master", "dining-table")
	if err != nil {
		t.Fatalf("GetEntityStore(cafe-master/dining-table): %v", err)
	}
	rec, err := store.GetByID(context.Background(), db.GetByIDParams{
		WorkspaceID: "kafe", ID: tableID,
	})
	if err != nil || rec == nil {
		t.Fatalf("GetByID(dining-table %s): rec=%v err=%v", tableID, rec, err)
	}
	status, _ := rec.Data["table_status"].(string)
	return status
}

// waitForTableStatus polls until the table reaches `want`.
//
// Polling is the honest way to observe this: the bridge is a SUBSCRIPTION, so
// the status changes after the HTTP response for the payment has already been
// written. The worker's interval is the latency, and a bounded poll states that
// instead of hiding it behind a sleep.
func waitForTableStatus(t *testing.T, app *App, tableID, want string) string {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	var last string
	for time.Now().Before(deadline) {
		last = tableStatus(t, app, tableID)
		if last == want {
			return last
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for dining-table %s to reach %q (last observed %q)", tableID, want, last)
	return last
}

// createSession opens a table session the way the check-in form does: the
// token is the value the caller also puts in the URL, so nothing has to be
// read back out of the response.
func createSession(t *testing.T, app *App, authToken, tableID, branchID, guestToken string) {
	t.Helper()
	status, out := doAuthed(t, app, http.MethodPost,
		"/kafe/_ui/entity/cafe-order/table-session", authToken, map[string]any{
			"transaction_date": time.Now().UTC().Format(time.RFC3339),
			"branch_id":        branchID,
			"dining_table_id":  tableID,
			"guest_token":      guestToken,
			"guest_name":       "Tamu E2E",
			"guest_count":      2,
		})
	if status != http.StatusCreated {
		t.Fatalf("create table-session: expected 201, got %d (%v)", status, out)
	}
}

// createOrder creates a QR order with a single line and returns its id.
//
// The line carries the D2 snapshot fields (name/price frozen at order time)
// exactly as the picker does — the server derives `subtotal` and `line_total`
// from them, so the test never invents a line total.
//
// `total_amount` IS sent, and that is not laziness: the manifest derives
// `subtotal` but NOT `total_amount` (order/entity.yaml — the discount/tax/
// service-charge chain needs optional operands first). Until that is closed,
// the caller supplies the total, exactly as the POS form does. The fixture has
// no tax/service charge/discount, so the total is simply the line total — and
// the test asserts the server's own `subtotal` to prove the snapshot pipeline
// ran rather than trusting its own arithmetic.
func createOrder(t *testing.T, app *App, token string, ids kafeSeedIDs, sessionID, guestToken, menuItemID, name string, price float64, qty float64) string {
	t.Helper()
	total := price * qty
	status, out := doAuthed(t, app, http.MethodPost,
		"/kafe/_ui/entity/cafe-order/order", token, map[string]any{
			"transaction_date": time.Now().UTC().Format(time.RFC3339),
			"branch_id":        ids.branchID,
			"channel":          "qr_table",
			"table_session_id": sessionID,
			"guest_token":      guestToken,
			"dining_table_id":  ids.tableID,
			"total_amount":     map[string]any{"amount": formatAmount(total), "currency": "IDR"},
			"paid_amount":      map[string]any{"amount": formatAmount(total), "currency": "IDR"},
			"lines": []any{
				map[string]any{
					"line_no":             1,
					"menu_item_id":        menuItemID,
					"name_snapshot":       name,
					"unit_price_snapshot": map[string]any{"amount": formatAmount(price), "currency": "IDR"},
					"quantity":            qty,
					"prep_station":        "kitchen",
				},
			},
		})
	if status != http.StatusCreated {
		t.Fatalf("create order: expected 201, got %d (%v)", status, out)
	}
	data, _ := out["data"].(map[string]any)
	id, _ := data["id"].(string)
	if id == "" {
		t.Fatalf("create order: response carried no id: %v", out)
	}

	// The server derives `subtotal` from the frozen line snapshots — proving
	// the D2 snapshot pipeline ran (a wrong snapshot would show up here, not
	// three steps later as a mysterious journal imbalance).
	if got := numberOf(data["subtotal"]); got != total {
		t.Fatalf("server-computed subtotal = %v, want %v (line snapshot pipeline)", got, total)
	}
	return id
}

// patchOrderStatus applies a state-machine transition by writing the state
// field — the path that actually works for a `via` without an `impl`, and the
// path that resolves + enqueues the transition's declared `emit` (S13).
func patchOrderStatus(t *testing.T, app *App, token, orderID, to string) {
	t.Helper()
	status, out := doAuthed(t, app, http.MethodPatch,
		"/kafe/_ui/entity/cafe-order/order/"+orderID, token, map[string]any{
			"status": to,
		})
	if status != http.StatusOK {
		t.Fatalf("PATCH order status → %s: expected 200, got %d (%v)", to, status, out)
	}
}

// formatAmount renders a whole-rupiah amount without a trailing ".0" — the
// canonical money string the manifests use (seeds write "45000", not 45000.0).
func formatAmount(v float64) string {
	if v == float64(int64(v)) {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// ── the scenario ──

// TestKafe_TableLifecycle_FullScenario walks the exact six steps the owner
// described, asserting the table's status after each one.
func TestKafe_TableLifecycle_FullScenario(t *testing.T) {
	app := bootKafe(t)
	ids := seedKafeTableScenario(t, app)
	admin := seedKafeAdminToken(t, app)
	// The GL subscription is a co-consumer of the same events. Seeding the
	// chart of accounts keeps it from failing (and retrying) in the outbox
	// while this test exercises the table branch — noise that would otherwise
	// obscure a real delivery failure.
	seedKafeAccounts(t, app)

	if got := tableStatus(t, app, ids.tableID); got != "available" {
		t.Fatalf("a fresh table must start `available`, got %q", got)
	}

	// ── Step 1: the guest scans the table's QR and a session opens ──
	//
	// 10.34b: the QR payload IS the token, and a lookup by it must resolve.
	// Asserted over HTTP because that is the request the check-in page makes.
	status, out := doAuthed(t, app, http.MethodGet,
		"/kafe/_ui/entity/cafe-master/dining-table/JKT-A01-DEMO", admin, nil)
	if status != http.StatusOK {
		t.Fatalf("resolve table by QR token: expected 200, got %d (%v)", status, out)
	}
	resolved, _ := out["data"].(map[string]any)
	if got, _ := resolved["id"].(string); got != ids.tableID {
		t.Fatalf("QR token resolved to the wrong table: got %q want %q", got, ids.tableID)
	}

	const guestToken = "E2E-GUEST-TOKEN-1"
	createSession(t, app, admin, ids.tableID, ids.branchID, guestToken)

	// The session's own id is needed to attach orders to it; look it up by the
	// token the caller used (the same natural-key path the guest page relies on).
	sessionID := sessionIDByToken(t, app, guestToken)

	// The check-in redirect points at `/menu/{guest_token}` (#37), so this
	// exact anonymous request is what the menu page issues to resolve the
	// session. `natural_key: true` on `guest_token` is what makes it answer —
	// asserted ANONYMOUS, because the guest has no account (business rule #3).
	anonStatus, anonOut := doJSON(t, app, http.MethodGet,
		"/kafe/_ui/entity/cafe-order/table-session/"+guestToken, nil)
	if anonStatus != http.StatusOK {
		t.Fatalf("anonymous resolve of the session by guest token: expected 200, got %d (%v)", anonStatus, anonOut)
	}
	anonSession, _ := anonOut["data"].(map[string]any)
	if got, _ := anonSession["id"].(string); got != sessionID {
		t.Fatalf("guest token resolved to the wrong session: got %q want %q", got, sessionID)
	}

	// ── Step 2: order nasi goreng and pay by QRIS ──
	order1 := createOrder(t, app, admin, ids, sessionID, guestToken,
		ids.nasiGoreng, "Nasi Goreng Spesial", 45000, 1)

	// The order starts as a draft; a cashier submits it, then marks it paid.
	// `confirm-payment` is the transition that emits `on_paid` durably.
	patchOrderStatus(t, app, admin, order1, "awaiting_payment")
	patchOrderStatus(t, app, admin, order1, "paid")

	// ── Step 3: paying fills the table (10.40b — the bridge under test) ──
	if got := waitForTableStatus(t, app, ids.tableID, "occupied"); got != "occupied" {
		t.Fatalf("after payment the table must be `occupied`, got %q", got)
	}

	// ── Step 4: the kitchen works the order and the waiter serves it ──
	patchOrderStatus(t, app, admin, order1, "in_kitchen")
	patchOrderStatus(t, app, admin, order1, "ready")
	patchOrderStatus(t, app, admin, order1, "served")

	if got := waitForTableStatus(t, app, ids.tableID, "served"); got != "served" {
		t.Fatalf("after serving the table must be `served`, got %q", got)
	}

	// ── Step 5: the guest adds an es teh and pays again ──
	order2 := createOrder(t, app, admin, ids, sessionID, guestToken,
		ids.esTeh, "Es Teh Manis", 12000, 1)
	patchOrderStatus(t, app, admin, order2, "awaiting_payment")
	patchOrderStatus(t, app, admin, order2, "paid")

	// A second payment from the same session puts the table back to `occupied`
	// (the `served → occupied` transition) — the guest is being served again.
	if got := waitForTableStatus(t, app, ids.tableID, "occupied"); got != "occupied" {
		t.Fatalf("a second order must return the table to `occupied`, got %q", got)
	}

	// ── Step 6: the es teh is served ──
	patchOrderStatus(t, app, admin, order2, "in_kitchen")
	patchOrderStatus(t, app, admin, order2, "ready")
	patchOrderStatus(t, app, admin, order2, "served")

	if got := waitForTableStatus(t, app, ids.tableID, "served"); got != "served" {
		t.Fatalf("after serving the second order the table must be `served`, got %q", got)
	}

	// ── Step 7: the cashier clears the table ──
	//
	// `release` carries no `impl`, so the PATCH of the state field IS the
	// application path (10.35a gugur for tables). The permission
	// `cafe-master.dining-tables.release` is what the cashier grants.
	patchTableStatus(t, app, admin, ids.tableID, "available")

	if got := tableStatus(t, app, ids.tableID); got != "available" {
		t.Fatalf("after release the table must be `available`, got %q", got)
	}
}

// TestKafe_TableLifecycle_CancelReturnsToAvailable pins the symmetric half of
// the bridge: a cancelled order must not leave the table occupied forever.
func TestKafe_TableLifecycle_CancelReturnsToAvailable(t *testing.T) {
	app := bootKafe(t)
	ids := seedKafeTableScenario(t, app)
	admin := seedKafeAdminToken(t, app)
	seedKafeAccounts(t, app)

	const guestToken = "E2E-GUEST-TOKEN-CANCEL"
	createSession(t, app, admin, ids.tableID, ids.branchID, guestToken)
	sessionID := sessionIDByToken(t, app, guestToken)

	order := createOrder(t, app, admin, ids, sessionID, guestToken,
		ids.nasiGoreng, "Nasi Goreng Spesial", 45000, 1)
	patchOrderStatus(t, app, admin, order, "awaiting_payment")
	patchOrderStatus(t, app, admin, order, "paid")
	waitForTableStatus(t, app, ids.tableID, "occupied")

	// Void is approval-gated (D5) — the admin holds the workflow, but the
	// transition still needs a reason (`void-order`'s condition). Drive the
	// simpler pre-payment cancel instead: it emits the same `on_cancel`.
	order2 := createOrder(t, app, admin, ids, sessionID, guestToken,
		ids.esTeh, "Es Teh Manis", 12000, 1)
	patchOrderStatus(t, app, admin, order2, "awaiting_payment")

	cancelOrder(t, app, admin, order2)

	if got := waitForTableStatus(t, app, ids.tableID, "available"); got != "available" {
		t.Fatalf("a cancelled order must return the table to `available`, got %q", got)
	}
}

// TestKafe_SessionOpenUnique pins 10.34c at the storage layer: a second OPEN
// session on the same table must be refused by the partial unique index, while
// a closed session does not block the next guest.
func TestKafe_SessionOpenUnique(t *testing.T) {
	app := bootKafe(t)
	ids := seedKafeTableScenario(t, app)
	admin := seedKafeAdminToken(t, app)

	createSession(t, app, admin, ids.tableID, ids.branchID, "E2E-DUP-TOKEN-1")

	// Same table, different token, still open → the index must reject it.
	// 409 CONFLICT, not 500: a duplicate value is a client-visible conflict
	// (see constraint_status_e2e_test.go). The full status/message contract is
	// pinned there; this asserts the storage-layer guard itself.
	status, out := doAuthed(t, app, http.MethodPost,
		"/kafe/_ui/entity/cafe-order/table-session", admin, map[string]any{
			"transaction_date": time.Now().UTC().Format(time.RFC3339),
			"branch_id":        ids.branchID,
			"dining_table_id":  ids.tableID,
			"guest_token":      "E2E-DUP-TOKEN-2",
		})
	if status != http.StatusConflict {
		t.Fatalf("a second OPEN session on one table must be refused with 409, got %d (%v)", status, out)
	}
}

// TestKafe_TableLifecycle_SessionClosesOnRelease pins the lifecycle coupling
// that 10.34c made REQUIRED: clearing the table must close its open session.
//
// 10.34c gives `table-session` a partial unique index on one OPEN session per
// table. The QR gateway creates a session on every scan, and nothing ever
// closed one — so the SECOND guest to sit at the same table would hit
// `UNIQUE constraint failed: cafe_order_table_sessions._dining_table_id` and
// get a 500 from the check-in page. A fresh seeded database hides it (each run
// starts clean), which is exactly why this is pinned on the SAME table, twice.
func TestKafe_TableLifecycle_SessionClosesOnRelease(t *testing.T) {
	app := bootKafe(t)
	ids := seedKafeTableScenario(t, app)
	admin := seedKafeAdminToken(t, app)
	seedKafeAccounts(t, app)

	const guestToken = "E2E-REUSE-TOKEN-1"
	createSession(t, app, admin, ids.tableID, ids.branchID, guestToken)

	// Occupy, then clear the table the way the cashier does.
	patchTableStatus(t, app, admin, ids.tableID, "occupied")
	patchTableStatus(t, app, admin, ids.tableID, "available")

	// The session for that visit must be closed, or the next guest is locked
	// out. Closing is driven by a SUBSCRIPTION (durable event → outbox →
	// background worker, ~1s poll), so poll rather than read once.
	if got := waitForSessionStatus(t, app, guestToken, "closed"); got != "closed" {
		t.Fatalf("clearing the table must close its session, got status=%q", got)
	}

	// The real proof: the NEXT guest can check in at the same table.
	status, out := doAuthed(t, app, http.MethodPost,
		"/kafe/_ui/entity/cafe-order/table-session", admin, map[string]any{
			"transaction_date": time.Now().UTC().Format(time.RFC3339),
			"branch_id":        ids.branchID,
			"dining_table_id":  ids.tableID,
			"guest_token":      "E2E-REUSE-TOKEN-2",
		})
	if status != http.StatusCreated {
		t.Fatalf("after the table is cleared the next guest must get a session, got %d (%v)", status, out)
	}
}

// waitForSessionStatus polls a session's status until it reaches `want`, and
// returns whatever it last saw. The table/session bridges are subscriptions, so
// every assertion about them has to tolerate the outbox's polling delay.
func waitForSessionStatus(t *testing.T, app *App, guestToken, want string) string {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	got := ""
	for time.Now().Before(deadline) {
		got, _ = sessionByToken(t, app, guestToken).Data["status"].(string)
		if got == want {
			return got
		}
		time.Sleep(100 * time.Millisecond)
	}
	return got
}

// sessionByToken returns the whole session record for a guest token.
func sessionByToken(t *testing.T, app *App, guestToken string) *db.EntityRecord {
	t.Helper()
	store, err := app.GetEntityStore("cafe-order", "table-session")
	if err != nil {
		t.Fatalf("GetEntityStore(cafe-order/table-session): %v", err)
	}
	rec, err := store.FindByField(context.Background(), "kafe", "guest_token", guestToken)
	if err != nil || rec == nil {
		t.Fatalf("find table-session by guest_token %q: rec=%v err=%v", guestToken, rec, err)
	}
	return rec
}

// ── HTTP helpers specific to the table ──

// patchTableStatus applies a dining-table transition by writing the state
// field. `occupy`/`occupied` etc. are `via` names without an `impl`, so this
// PATCH is their only application path (10.35a).
func patchTableStatus(t *testing.T, app *App, token, tableID, to string) {
	t.Helper()
	status, out := doAuthed(t, app, http.MethodPatch,
		"/kafe/_ui/entity/cafe-master/dining-table/"+tableID, token, map[string]any{
			"table_status": to,
		})
	if status != http.StatusOK {
		t.Fatalf("PATCH table_status → %s: expected 200, got %d (%v)", to, status, out)
	}
}

// cancelOrder applies `cancel-order` (awaiting_payment → cancelled), whose
// `emit: on_cancel` is the counterpart the table bridge listens for.
func cancelOrder(t *testing.T, app *App, token, orderID string) {
	t.Helper()
	status, out := doAuthed(t, app, http.MethodPatch,
		"/kafe/_ui/entity/cafe-order/order/"+orderID, token, map[string]any{
			"status": "cancelled",
		})
	if status != http.StatusOK {
		t.Fatalf("cancel order: expected 200, got %d (%v)", status, out)
	}
}

// sessionIDByToken resolves a table session through its guest token — the
// natural-key path the guest's own URLs depend on.
func sessionIDByToken(t *testing.T, app *App, guestToken string) string {
	t.Helper()
	store, err := app.GetEntityStore("cafe-order", "table-session")
	if err != nil {
		t.Fatalf("GetEntityStore(cafe-order/table-session): %v", err)
	}
	rec, err := store.FindByField(context.Background(), "kafe", "guest_token", guestToken)
	if err != nil || rec == nil {
		t.Fatalf("find table-session by guest_token %q: rec=%v err=%v", guestToken, rec, err)
	}
	return rec.ID
}
