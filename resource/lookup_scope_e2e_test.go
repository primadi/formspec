package formspec

import (
	"net/http"
	"testing"
)

// kafe 10.76 over the REAL HTTP surface: the branch of the price list must come
// from the QR/table session, never from the request.
//
// The gap this closes was measured: `menu-item-price` is granted anonymously so
// the guest catalog can show prices, and the branch used to arrive as a
// CLIENT-computed `?branch_id=` (a client-side filter, interpolated in
// the browser). Any client could drop that filter — or replace the value — and
// read every branch's price list. There is no PII in a price, but it is a real
// multi-outlet leak: a guest could see another outlet's pricing.
//
// The fix has two halves, and this test pins BOTH:
//
//	manifest  the picker's `lookup.scope` names the session reference and the field to
//	          read (`table-session.branch_id`), and the derived public grant for
//	          the price entity inherits it
//	server    the scope is resolved through the referenced RECORD, so the caller
//	          states which session it is — already visible in the URL — and never
//	          which branch it may see
//
// A client-supplied `branch_id` is therefore OVERRIDDEN, not merged: the number
// that arrives is the table's, whatever the query string said.

func TestKafe_PriceListBranchComesFromTheTableSession(t *testing.T) {
	app := bootKafe(t)
	ids := seedKafeTableScenario(t, app)

	// A second branch with its OWN price for the same menu item, so "another
	// branch" is a real row that a leak would actually return. Without it the
	// test would pass on a table that simply has no rows to leak.
	otherBranch := insertRecord(t, app, "cafe-master", "branch", map[string]any{
		"code": "KFE-BDG-01", "name": "Kafe Bandung",
		"tax_percent": "10", "service_charge_percent": "5",
		"apply_service_charge": true, "is_active": true,
	})
	insertRecord(t, app, "cafe-master", "menu-item-price", map[string]any{
		"branch_id": otherBranch, "menu_item_id": ids.nasiGoreng,
		"price": map[string]any{"amount": "99000", "currency": "IDR"}, "is_active": true,
	})

	// A guest session at the table in the seeded branch (KFE-JKT-01).
	sessionID := insertRecord(t, app, "cafe-order", "table-session", map[string]any{
		"transaction_date": "2026-10-06T10:00:00Z",
		"dining_table_id":  ids.tableID,
		"branch_id":        ids.branchID,
		"guest_token":      "SCOPE-PRICE-A",
	})

	const path = "/kafe/_ui/entity/cafe-master/menu-item-price"

	// ── 1. Without the reference the scope cannot resolve → fail closed ──
	status, out := doJSON(t, app, http.MethodGet, path+"?per_page=500", nil)
	if status != http.StatusForbidden {
		t.Fatalf("an anonymous price list without the session reference must fail closed, got %d (%v)", status, out)
	}

	// ── 2. With the reference, only the table's branch is returned ──
	status, out = doJSON(t, app, http.MethodGet,
		path+"?per_page=500&session_id="+sessionID, nil)
	if status != http.StatusOK {
		t.Fatalf("an anonymous price list for the guest's own session must succeed, got %d (%v)", status, out)
	}
	rows := dataRows(t, out)
	if len(rows) == 0 {
		t.Fatal("the guest's own branch returned no prices — the scope is over-narrowing, not enforcing")
	}
	for _, row := range rows {
		if got, _ := row["branch_id"].(string); got != ids.branchID {
			t.Errorf("price row for branch %q leaked into a session at branch %q", got, ids.branchID)
		}
	}

	// ── 3. The caller cannot widen the view through the query string ──
	// This is the exact request that used to work: the client simply states
	// another branch. The scope must win.
	status, out = doJSON(t, app, http.MethodGet,
		path+"?per_page=500&session_id="+sessionID+"&branch_id="+otherBranch, nil)
	if status != http.StatusOK {
		t.Fatalf("a client filter on the scoped field must be overridden, not rejected, got %d (%v)", status, out)
	}
	for _, row := range dataRows(t, out) {
		if got, _ := row["branch_id"].(string); got != ids.branchID {
			t.Errorf("client filter widened the price list to branch %q (want %q)", got, ids.branchID)
		}
	}
}

// TestKafe_PriceListSecondBranchExistsForTheScopeToHaveSomethingToLeak is the
// fixture guard the test above depends on, stated separately so a future edit
// cannot quietly make the leak assertions vacuous.
//
// A guest at branch A must see branch A's prices; for "no leak" to mean
// anything, branch B's price for the same menu item has to exist. If it were
// absent, the first test would pass on an empty table.
func TestKafe_PriceListSecondBranchExistsForTheScopeToHaveSomethingToLeak(t *testing.T) {
	app := bootKafe(t)
	ids := seedKafeTableScenario(t, app)
	admin := seedKafeAdminToken(t, app)

	otherBranch := insertRecord(t, app, "cafe-master", "branch", map[string]any{
		"code": "KFE-BDG-01", "name": "Kafe Bandung",
		"tax_percent": "10", "service_charge_percent": "5",
		"apply_service_charge": true, "is_active": true,
	})
	insertRecord(t, app, "cafe-master", "menu-item-price", map[string]any{
		"branch_id": otherBranch, "menu_item_id": ids.nasiGoreng,
		"price": map[string]any{"amount": "99000", "currency": "IDR"}, "is_active": true,
	})

	// The admin fixture holds `*`, which SATISFIES the documented `read_all`
	// exemption — a cross-branch reader is exempt by design, so this list is
	// deliberately NOT narrowed. That is precisely why the guest test cannot
	// reuse a staff token to prove the scope works; it drives the anonymous path
	// instead, where the grant's scope is the only thing standing between the
	// guest and another outlet's prices.
	status, out := doAuthed(t, app, http.MethodGet,
		"/kafe/_ui/entity/cafe-master/menu-item-price?per_page=500", admin, nil)
	if status != http.StatusOK {
		t.Fatalf("unscoped staff list: expected 200, got %d (%v)", status, out)
	}
	branches := map[string]bool{}
	for _, row := range dataRows(t, out) {
		got, _ := row["branch_id"].(string)
		branches[got] = true
	}
	if !branches[otherBranch] {
		t.Fatalf("branch B's price is missing from the fixture (%v) — the leak assertions would be vacuous", branches)
	}
	if !branches[ids.branchID] {
		t.Fatalf("branch A's price is missing from the fixture (%v)", branches)
	}
	t.Logf("fixture carries prices for both branches: %d distinct", len(branches))
}

// dataRows unwraps a list response's `data` array.
func dataRows(t *testing.T, out map[string]any) []map[string]any {
	t.Helper()
	raw, _ := out["data"].([]any)
	rows := make([]map[string]any, 0, len(raw))
	for _, r := range raw {
		if m, ok := r.(map[string]any); ok {
			rows = append(rows, m)
		}
	}
	return rows
}
