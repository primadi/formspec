package formspec

import (
	"context"
	"net/http"
	"testing"
	"time"

	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// TestKafe_PurchaseReceivedCreatesJournal pins the purchase→journal chain wired
// in kafe item 10.7.
//
// `cafe-stock/purchase-order` only EMITS `on_po_received`; the `gl` module
// subscribes and builds the entry. Nothing in `cafe-stock` names an account or
// knowns `gl` exists — which is the whole point: the emitting module declares
// what happened, the accounting module decides how to record it.
//
// Two failures this test exists to catch, both of which happened for real while
// wiring it:
//
//  1. `emit:` on the transition without `emits:` on the action. The state
//     machine reads correctly, `formspec validate` is green, and NO event is
//     ever sent — the subscription sits waiting for something that never
//     arrives, and receiving goods silently produces no accounting at all.
//  2. A subscription script calling a function defined in another .star file.
//     Starlark compiles each file as its own unit, so that fails at RUNTIME
//     with `undefined: <fn>` on the first delivered event. `formspec validate`
//     cannot see it, because it compiles each file independently.
func TestKafe_PurchaseReceivedCreatesJournal(t *testing.T) {
	app := bootKafe(t)
	seedKafeAccounts(t, app)
	seedKafePurchaseAccounts(t, app)
	token := seedKafeAdminToken(t, app)

	branchID := insertRecord(t, app, "cafe-master", "branch", map[string]any{
		"code": "KFE-PJ-01", "name": "Cabang Jurnal", "tax_percent": 10,
	})
	supplierID := insertRecord(t, app, "cafe-stock", "supplier", map[string]any{
		"code": "SUP-PJ", "name": "Pemasok Jurnal",
	})
	telurID := insertRecord(t, app, "cafe-stock", "ingredient", map[string]any{
		"code": "BHN-PJ-01", "name": "Telur Jurnal", "unit": "pcs",
		"cost_per_unit": map[string]any{"amount": "2500", "currency": "IDR"},
	})

	// 200 × 2.500 = 500.000 — the figure the journal must carry.
	po := map[string]any{
		"transaction_date": "2026-09-22",
		"branch_id":        branchID,
		"supplier_id":      supplierID,
		"lines": []any{
			map[string]any{
				"line_no": 1, "line_type": "ingredient", "ingredient_id": telurID,
				"quantity": 200, "unit_cost": map[string]any{"amount": "2500", "currency": "IDR"},
				"received_quantity": 200,
			},
		},
	}
	status, body := doAuthed(t, app, http.MethodPost,
		"/kafe/_ui/entity/cafe-stock/purchase-order", token, po)
	if status != http.StatusCreated && status != http.StatusOK {
		t.Fatalf("create PO: status %d body %v", status, body)
	}
	poID, _ := body["data"].(map[string]any)["id"].(string)
	if poID == "" {
		t.Fatalf("create PO: no id in %v", body)
	}

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

	// The journal is built by the outbox worker through the subscription.
	waitForJournalSource(t, app, poID)

	entry := findJournalBySource(t, app, poID)
	if entry == nil {
		t.Fatal("no journal-entry was created for the received purchase — the " +
			"emitted event either never reached the outbox (missing `emits:` on the " +
			"action) or the subscription handler failed")
	}
	if got, _ := entry.Data["status"].(string); got != "posted" {
		t.Errorf("journal status = %q, want posted", got)
	}
	if got, _ := entry.Data["source"].(string); got != "cafe-stock.purchase-order" {
		t.Errorf("journal source = %q, want cafe-stock.purchase-order", got)
	}

	lines := journalLines(t, app, entry)
	if len(lines) < 2 {
		t.Fatalf("expected 2 journal lines, got %d", len(lines))
	}
	debit, credit := 0.0, 0.0
	for _, ln := range lines {
		debit += numberOf(ln["debit"])
		credit += numberOf(ln["credit"])
	}
	if debit != credit {
		t.Errorf("journal does not balance: debit=%v credit=%v", debit, credit)
	}
	if debit != 500000 {
		t.Errorf("journal value = %v, want 500000 (200 x 2500)", debit)
	}

	// Correct accounts, not just a balanced pair: inventory on the debit side,
	// accounts payable on the credit side. A purchase recorded against the
	// wrong accounts still balances, which is exactly why this needs asserting.
	accStore, err := app.Registry().GetEntityStore("gl", "account")
	if err != nil {
		t.Fatalf("account store: %v", err)
	}
	codeOf := func(accountID string) string {
		rec, err := accStore.GetByID(context.Background(), db.GetByIDParams{WorkspaceID: "kafe", ID: accountID})
		if err != nil || rec == nil {
			return ""
		}
		s, _ := rec.Data["code"].(string)
		return s
	}
	var debitCode, creditCode string
	for _, ln := range lines {
		id, _ := ln["account_id"].(string)
		if numberOf(ln["debit"]) > 0 {
			debitCode = codeOf(id)
		}
		if numberOf(ln["credit"]) > 0 {
			creditCode = codeOf(id)
		}
	}
	if debitCode != "1-2000" {
		t.Errorf("debit account = %q, want 1-2000 (Persediaan Bahan)", debitCode)
	}
	if creditCode != "2-3000" {
		t.Errorf("credit account = %q, want 2-3000 (Utang Dagang)", creditCode)
	}
}

// waitForJournalSource polls until the outbox worker has delivered the purchase
// event and the subscription has produced a journal for this specific PO.
//
// A source-specific wait rather than the generic waitForJournal: the sales
// chain's helper returns as soon as ANY journal exists, which would let this
// test pass on a journal created by an unrelated order.
func waitForJournalSource(t *testing.T, app *App, sourceID string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if findJournalBySource(t, app, sourceID) != nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for the outbox worker to deliver on_po_received and create a journal for PO %s", sourceID)
}

// seedKafePurchaseAccounts adds the purchasing accounts the purchase journal
// needs. Kept separate from seedKafeAccounts (which covers the sales side) so
// the two chains can be tested independently.
func seedKafePurchaseAccounts(t *testing.T, app *App) {
	t.Helper()
	store, err := app.Registry().GetEntityStore("gl", "account")
	if err != nil {
		t.Fatalf("account store: %v", err)
	}
	for _, a := range []map[string]any{
		{"code": "1-2000", "name": "Persediaan Bahan", "type": "asset", "normal_balance": "debit", "is_active": true},
		{"code": "2-3000", "name": "Utang Dagang", "type": "liability", "normal_balance": "credit", "is_active": true},
	} {
		if _, err := store.Insert(context.Background(), db.InsertParams{
			WorkspaceID: "kafe", CreatedBy: "e2e-seed", Data: a,
		}); err != nil {
			t.Fatalf("seed account %v: %v", a["code"], err)
		}
	}
}
