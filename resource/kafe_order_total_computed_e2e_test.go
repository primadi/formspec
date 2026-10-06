package formspec

import (
	"net/http"
	"regexp"
	"testing"
	"time"

	"github.com/primadi/formspec/pkg/spec"
)

// kafe 10.65 (major): `order.total_amount` had NO writer, so every order
// created through the app — QR or POS — stored `total_amount: null`. The GL
// subscription then refused the paid event (`FORMSPEC.GL.NO_AMOUNT`) and the
// outbox retried to `failed`: no journal, no `gl-balance`, and the same field
// left the sales reports, receipts, and the "Total" column empty.
//
// The earlier "scenario 8 ✅" evidence did not catch it because the tests
// HAND-SUPPLIED `total_amount` in their payloads, while the real order form
// never sends a single money value (by design — the server derives subtotal and
// line totals). So the tested shape was not the shape customers produce.
//
// This test drives the REAL shape: an order body with NO money at all, then the
// cashier's payment path, then the journal. It is the regression that would have
// failed before the fix.
func TestKafe_QrOrderDerivesTotalAndJournals(t *testing.T) {
	app := bootKafe(t)
	ids := seedKafeTableScenario(t, app)
	admin := seedKafeAdminToken(t, app)
	// The GL subscription is a co-consumer of on_paid; without the chart of
	// accounts it retries with ACCOUNT_NOT_FOUND and muddies the result.
	seedKafeAccounts(t, app)

	const guestToken = "E2E-DERIVE-GUEST-1"
	createSession(t, app, admin, ids.tableID, ids.branchID, guestToken)
	sessionID := sessionIDByToken(t, app, guestToken)

	// ── The QR order: no money field anywhere in the body ──
	//
	// Exactly what `order-form-qr.yaml` sends: identity, channel, branch, and
	// the frozen line snapshots. `subtotal`/`line_total` are derived, and so —
	// after the fix — is the money chain up to `total_amount`.
	status, out := doAuthed(t, app, http.MethodPost,
		"/kafe/_ui/entity/cafe-order/order", admin, map[string]any{
			"transaction_date": time.Now().UTC().Format(time.RFC3339),
			"branch_id":        ids.branchID,
			"channel":          "qr_table",
			"table_session_id": sessionID,
			"guest_token":      guestToken,
			"dining_table_id":  ids.tableID,
			"lines": []any{
				map[string]any{
					"line_no":             1,
					"menu_item_id":        ids.nasiGoreng,
					"name_snapshot":       "Nasi Goreng Spesial",
					"unit_price_snapshot": map[string]any{"amount": "45000", "currency": "IDR"},
					"quantity":            1,
					"prep_station":        "kitchen",
				},
			},
		})
	if status != http.StatusCreated {
		t.Fatalf("create QR order: expected 201, got %d (%v)", status, out)
	}
	data, _ := out["data"].(map[string]any)
	orderID, _ := data["id"].(string)
	if orderID == "" {
		t.Fatalf("create order: response carried no id: %v", out)
	}

	// ── The derived chain ──
	//
	// The fixture branch is tax 10% / service 5% / apply_service_charge true
	// (seedKafeTableScenario), and the order's `branch_id` snapshot copies those
	// onto the order so the formulas can reach them — `computed` cannot read
	// another entity (evaluateComputed runs before relations resolve).
	//
	//   subtotal            = 45000
	//   service_charge      = 45000 × 5%          = 2250
	//   tax                 = (45000 + 2250) × 10% = 4725
	//   total               = 45000 + 2250 + 4725  = 51975
	const (
		wantSubtotal = 45000.0
		wantService  = 2250.0
		wantTax      = 4725.0
		wantTotal    = 51975.0
	)

	if got := numberOf(data["subtotal"]); got != wantSubtotal {
		t.Errorf("derived subtotal = %v, want %v", got, wantSubtotal)
	}
	if got := numberOf(data["service_charge_amount"]); got != wantService {
		t.Errorf("derived service_charge_amount = %v, want %v (5%% of subtotal)", got, wantService)
	}
	if got := numberOf(data["tax_amount"]); got != wantTax {
		t.Errorf("derived tax_amount = %v, want %v (10%% of subtotal + service)", got, wantTax)
	}
	// The load-bearing one: this used to be absent for EVERY order.
	if data["total_amount"] == nil {
		t.Fatalf("total_amount is absent — the exact 10.65 failure (response: %v)", data)
	}
	if got := numberOf(data["total_amount"]); got != wantTotal {
		t.Errorf("derived total_amount = %v, want %v", got, wantTotal)
	}

	// ── Payment: the cashier path, which emits the durable `on_paid` ──
	patchOrderStatus(t, app, admin, orderID, "awaiting_payment")
	patchOrderStatus(t, app, admin, orderID, "paid")

	// The event payload is built from the HTTP handler's merged resource (the
	// record as read — computed values included — plus the request body), so the
	// derived total travels with `on_paid`. Before the fix it travelled as null
	// and the journal was refused; now a journal must appear and balance.
	waitForJournal(t, app)

	entry := findJournalBySource(t, app, orderID)
	if entry == nil {
		t.Fatalf("no journal for the derived-total order %s — the money chain did not reach gl/journalize", orderID)
	}
	if got, _ := entry.Data["status"].(string); got != "posted" {
		t.Errorf("journal status = %q, want posted", got)
	}

	lines := journalLines(t, app, entry)
	debit, credit := 0.0, 0.0
	for _, ln := range lines {
		debit += numberOf(ln["debit"])
		credit += numberOf(ln["credit"])
	}
	if debit != credit {
		t.Errorf("journal does not balance: debit=%v credit=%v", debit, credit)
	}
	if debit != wantTotal {
		t.Errorf("cash/debit side = %v, want the DERIVED total %v", debit, wantTotal)
	}
}

// A branch that does NOT apply a service charge must yield a zero-valued
// service charge — in the same currency, not a bare number, because money
// arithmetic refuses mixed operands (05-field-types.md §2.1). This pins the
// `money_zero` fallback branch of the formula, which the happy path above never
// reaches.
func TestKafe_OrderDerivesTotalWithoutServiceCharge(t *testing.T) {
	app := bootKafe(t)
	ids := seedKafeTableScenario(t, app)
	admin := seedKafeAdminToken(t, app)

	// Flip the branch flag: `apply_service_charge: false`.
	status, out := doAuthed(t, app, http.MethodPatch,
		"/kafe/_ui/entity/cafe-master/branch/"+ids.branchID, admin, map[string]any{
			"apply_service_charge": false,
		})
	if status != http.StatusOK {
		t.Fatalf("PATCH branch apply_service_charge: %d (%v)", status, out)
	}

	const guestToken = "E2E-DERIVE-GUEST-2"
	createSession(t, app, admin, ids.tableID, ids.branchID, guestToken)
	sessionID := sessionIDByToken(t, app, guestToken)

	status, out = doAuthed(t, app, http.MethodPost,
		"/kafe/_ui/entity/cafe-order/order", admin, map[string]any{
			"transaction_date": time.Now().UTC().Format(time.RFC3339),
			"branch_id":        ids.branchID,
			"channel":          "qr_table",
			"table_session_id": sessionID,
			"guest_token":      guestToken,
			"dining_table_id":  ids.tableID,
			"lines": []any{
				map[string]any{
					"line_no":             1,
					"menu_item_id":        ids.nasiGoreng,
					"name_snapshot":       "Nasi Goreng Spesial",
					"unit_price_snapshot": map[string]any{"amount": "45000", "currency": "IDR"},
					"quantity":            1,
					"prep_station":        "kitchen",
				},
			},
		})
	if status != http.StatusCreated {
		t.Fatalf("create order: expected 201, got %d (%v)", status, out)
	}
	data, _ := out["data"].(map[string]any)

	if got := numberOf(data["service_charge_amount"]); got != 0 {
		t.Errorf("service_charge_amount = %v, want 0 when the branch does not apply it", got)
	}
	// Service charge 0 → tax on subtotal only: 45000 × 10% = 4500; total 49500.
	if got := numberOf(data["tax_amount"]); got != 4500 {
		t.Errorf("tax_amount = %v, want 4500", got)
	}
	if got := numberOf(data["total_amount"]); got != 49500 {
		t.Errorf("total_amount = %v, want 49500", got)
	}
}

// Guard against the failure mode being reintroduced in its original form: a
// computed formula that names an optional field as a BARE identifier is an
// "undefined:" compile error, which evaluateComputed swallows — leaving the
// field silently absent with no error at any layer.
//
// The check is source-level on purpose: the failure is a silent no-op, so a
// behavioural test cannot distinguish "correctly guarded" from "accidentally
// present". Reading the formulas keeps the rule visible at the place a future
// edit would break it.
func TestKafe_OrderMoneyFormulasUseResourceAccess(t *testing.T) {
	app := bootKafe(t)
	formulas := orderMoneyFormulas(t, app)
	if len(formulas) == 0 {
		t.Fatal("no computed formulas found on the order entity — did the manifest move or rename?")
	}
	// Optional operands: reaching them as bare identifiers is the bug.
	optional := []string{"discount_amount", "manual_discount_amount", "points_value"}
	checked := 0
	for _, f := range formulas {
		for _, field := range optional {
			if !containsIdentifier(f.Computed.Formula, field) {
				continue
			}
			checked++
			if bareIdentifierUse(f.Computed.Formula, field) {
				t.Errorf("formula %q names optional field %q as a bare identifier; "+
					"an absent optional field is then a compile error that is swallowed, "+
					"leaving the computed value silently empty — use resource.%s",
					f.Computed.Formula, field, field)
			}
		}
	}
	// Calibration: if the optional fields stop appearing in any formula, this
	// guard would pass vacuously and could no longer catch a regression.
	if checked == 0 {
		t.Fatal("no formula mentions the optional discount fields — the guard would pass vacuously")
	}
}

// orderMoneyFormulas returns the entity's computed fields, read through the
// loaded spec (the same declaration the engine evaluates) rather than by
// parsing YAML text.
func orderMoneyFormulas(t *testing.T, app *App) []spec.Field {
	t.Helper()
	info, ok := app.Registry().GetEntity("cafe-order", "order")
	if !ok || info.EntitySpec == nil {
		t.Fatal("entity cafe-order/order not registered")
	}
	var out []spec.Field
	for _, f := range info.EntitySpec.Fields {
		if f.Computed != nil && f.Computed.Formula != "" {
			out = append(out, f)
		}
	}
	return out
}

var identifierRe = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)

// containsIdentifier reports whether name appears as a whole identifier.
func containsIdentifier(expr, name string) bool {
	for _, loc := range identifierRe.FindAllStringIndex(expr, -1) {
		if expr[loc[0]:loc[1]] == name {
			return true
		}
	}
	return false
}

// bareIdentifierUse reports whether name appears WITHOUT a preceding dot —
// i.e. as a top-level env identifier rather than `resource.name`.
func bareIdentifierUse(expr, name string) bool {
	for _, loc := range identifierRe.FindAllStringIndex(expr, -1) {
		if expr[loc[0]:loc[1]] != name {
			continue
		}
		if loc[0] > 0 && expr[loc[0]-1] == '.' {
			continue // resource.name / data.name — the safe form
		}
		return true
	}
	return false
}
