package formspec

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/primadi/formspec/internal/api"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// In-process end-to-end harness for the kafe Order-to-Cash chain (todo 9.2.1).
//
// Every other kafe check in this repo is either unit-level (internal/*) or a
// MANUAL walkthrough recorded in examples/kafe/gaps_found/TODO.md. The O2C
// journal chain — order paid → durable `on_paid` → `gl` subscription →
// balanced journal — was verified by hand (ledger scenario 8), which means
// nothing fails the build if it breaks.
//
// These tests boot the REAL kafe app in-process (the same `New(Config{…})` the
// dev server and the native binaries use: manifest loading, schema sync,
// outbox worker, subscription dispatch, Starlark), so the chain is exercised
// rather than simulated. Reuses the harness in auth_e2e_test.go (doJSON,
// doAuthed, seedAdminToken).

// bootKafe boots the kafe example against a temp SQLite database and starts
// its background workers (outbox, streaming, escalation).
func bootKafe(t *testing.T) *App {
	t.Helper()
	api.ResetAuthRateLimiters()

	app, err := New(Config{
		SpecPath:    filepath.Join("..", "examples", "kafe", "spec"),
		DSN:         "sqlite:" + filepath.Join(t.TempDir(), "o2c-e2e.db"),
		WorkspaceID: "kafe",
	})
	if err != nil {
		t.Fatalf("boot kafe in-process: %v", err)
	}
	t.Cleanup(func() { _ = app.Close(context.Background()) })

	app.StartBackgroundWorkers()
	return app
}

// TestKafe_BootsInProcess is the harness's own smoke test. If the kafe tree
// stops booting, everything below would fail for an uninteresting reason — so
// this says so explicitly.
func TestKafe_BootsInProcess(t *testing.T) {
	app := bootKafe(t)

	if app.RouteCount() == 0 {
		t.Fatal("booted app exposes no routes")
	}
	if app.Registry() == nil {
		t.Fatal("booted app has no entity registry")
	}
	// The outbox/subscription path is the transport the whole O2C chain rides.
	if app.Subscriptions() == nil {
		t.Fatal("no subscription registry — on_paid → journal cannot run")
	}

	status, _ := doJSON(t, app, http.MethodGet, "/kafe/_ui/_meta/version", nil)
	if status != http.StatusOK {
		t.Fatalf("GET /_meta/version: expected 200, got %d", status)
	}
}

// TestKafe_OnPaidSubscriptionIsRegistered pins the runtime link the journal
// depends on: a durable `gl` subscription owning the order's `on_paid` event.
// `formspec validate` checks the declaration, not that it resolves at runtime.
func TestKafe_OnPaidSubscriptionIsRegistered(t *testing.T) {
	app := bootKafe(t)

	owned := app.Subscriptions().ForEventOwned("cafe-order.order.on_paid")
	if len(owned) == 0 {
		t.Fatal("no subscription owns cafe-order.order.on_paid — the journal would never be created")
	}

	found := false
	for _, s := range owned {
		if s.Spec.Handler.Ref != "gl/journalize" {
			continue
		}
		found = true
	}
	if !found {
		t.Fatalf("on_paid is owned, but not by gl/journalize: %+v", owned)
	}

	// The durability that matters for accounting is on the EVENT
	// (`publish: {durable: true}`), which is what puts delivery on the outbox
	// with retry + dead-letter. A subscription's own `durability:` is a
	// different knob (Tier 2 stream vs Tier 1 direct) and kafe leaves it
	// unset — so asserting it here would encode the wrong mental model.
	info, ok := app.Registry().GetEntity("cafe-order", "order")
	if !ok || info.EntitySpec == nil {
		t.Fatal("cafe-order.order not registered")
	}
	for _, ev := range info.EntitySpec.Events {
		if ev.Name == "on_paid" && !ev.Publish.Durable {
			t.Error("on_paid must be publish.durable — accounting must not lose the event")
		}
	}
}

// TestKafe_OrderEntityHasPaidTransition asserts the emitting half of the chain
// in the loaded runtime spec: `confirm-payment` moves the order to `paid` and
// emits `on_paid`, and the event is declared durable. Without the emit the
// subscription has nothing to receive — silently.
func TestKafe_OrderEntityHasPaidTransition(t *testing.T) {
	app := bootKafe(t)

	info, ok := app.Registry().GetEntity("cafe-order", "order")
	if !ok || info.EntitySpec == nil {
		t.Fatal("cafe-order.order not registered")
	}
	es := info.EntitySpec
	if es.StateMachine == nil {
		t.Fatal("cafe-order.order has no state machine")
	}

	var emitted string
	for _, tr := range es.StateMachine.Transitions {
		if tr.Action == "confirm-payment" && tr.To == "paid" {
			emitted = tr.Emit
		}
	}
	if emitted != "on_paid" {
		t.Fatalf("confirm-payment → paid must emit \"on_paid\", got %q", emitted)
	}

	var declared bool
	for _, ev := range es.Events {
		if ev.Name == "on_paid" {
			declared = true
			if !ev.Publish.Durable {
				t.Error("on_paid must be durable (outbox + retry + dead-letter)")
			}
		}
	}
	if !declared {
		t.Fatal("on_paid is emitted by a transition but not declared in the entity's events")
	}
}

// TestKafe_OnPaidCreatesBalancedJournal is the automated Order-to-Cash chain
// (todo 9.2.1): a durable `on_paid` event carrying a real order payload goes
// onto the outbox exactly as the HTTP path would enqueue it, the running
// outbox worker delivers it to the `gl` subscription, `journalize.star` builds
// the double-entry rows, and the journal is posted.
//
// The accounting invariant is asserted numerically — debits == credits and the
// cash side equals the order total — because a "journal exists" check would
// pass even for a journal that does not balance.
func TestKafe_OnPaidCreatesBalancedJournal(t *testing.T) {
	app := bootKafe(t)
	seedKafeAccounts(t, app)

	// subtotal 125000 + tax 12500 + service charge 6250 = total 143750, the
	// exact figures the manual walkthrough recorded for ORD-2026-00021.
	payload := map[string]any{
		"id":                     "11111111-1111-1111-1111-111111111111",
		"number":                 "ORD-E2E-00001",
		"transaction_date":       "2026-09-22",
		"total_amount":           map[string]any{"amount": "143750", "currency": "IDR"},
		"subtotal":               map[string]any{"amount": "125000", "currency": "IDR"},
		"tax_amount":             map[string]any{"amount": "12500", "currency": "IDR"},
		"service_charge_amount":  map[string]any{"amount": "6250", "currency": "IDR"},
		"discount_amount":        map[string]any{"amount": "0", "currency": "IDR"},
		"manual_discount_amount": map[string]any{"amount": "0", "currency": "IDR"},
		"points_value":           map[string]any{"amount": "0", "currency": "IDR"},
		"items":                  []any{},
	}

	enqueueOnPaid(t, app, payload)
	waitForJournal(t, app)

	entry := findJournalBySource(t, app, "11111111-1111-1111-1111-111111111111")
	if entry == nil {
		t.Fatal("no journal-entry was created for the paid order")
	}
	if got, _ := entry.Data["status"].(string); got != "posted" {
		t.Errorf("journal status = %q, want posted (the handler posts it itself)", got)
	}

	// Balance the journal from its child lines.
	lines := journalLines(t, app, entry)
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 journal lines, got %d", len(lines))
	}
	debit, credit := 0.0, 0.0
	for _, ln := range lines {
		debit += numberOf(ln["debit"])
		credit += numberOf(ln["credit"])
	}
	if debit != credit {
		t.Errorf("journal does not balance: debit=%v credit=%v (lines=%v)", debit, credit, lines)
	}
	if debit != 143750 {
		t.Errorf("cash/debit side = %v, want the order total 143750", debit)
	}
}

// TestKafe_OnPaidIsIdempotent pins the retry contract: the outbox may deliver
// the same event more than once, and a second delivery must NOT double the
// journal. `journalize.star` guards on source_id; this proves the guard holds
// through the real dispatch path rather than only in the script's own logic.
func TestKafe_OnPaidIsIdempotent(t *testing.T) {
	app := bootKafe(t)
	seedKafeAccounts(t, app)

	payload := map[string]any{
		"id":               "22222222-2222-2222-2222-222222222222",
		"number":           "ORD-E2E-00002",
		"transaction_date": "2026-09-22",
		"total_amount":     map[string]any{"amount": "50000", "currency": "IDR"},
		"subtotal":         map[string]any{"amount": "50000", "currency": "IDR"},
		"tax_amount":       map[string]any{"amount": "0", "currency": "IDR"},
		"items":            []any{},
	}

	enqueueOnPaid(t, app, payload)
	waitForJournal(t, app)
	first := countJournalEntries(t, app)
	if first == 0 {
		t.Fatal("first delivery produced no journal")
	}

	// Redeliver the identical event — exactly what an outbox retry does.
	enqueueOnPaid(t, app, payload)
	time.Sleep(700 * time.Millisecond)

	if second := countJournalEntries(t, app); second != first {
		t.Errorf("redelivery duplicated the journal: before=%d after=%d", first, second)
	}
}

// TestKafe_JournalizeRejectsOrderWithNoAmount drives the handler with a paid
// event carrying no amount and asserts it refuses instead of posting a journal
// that cannot balance. This is the "must not silently write a wrong journal"
// half of the contract.
func TestKafe_JournalizeRejectsOrderWithNoAmount(t *testing.T) {
	app := bootKafe(t)
	seedKafeAccounts(t, app)

	before := countJournalEntries(t, app)

	payload := map[string]any{
		"id":               "33333333-3333-3333-3333-333333333333",
		"number":           "ORD-E2E-NO-AMOUNT",
		"transaction_date": "2026-09-22",
		"total_amount":     map[string]any{"amount": "0", "currency": "IDR"},
		"subtotal":         map[string]any{"amount": "0", "currency": "IDR"},
		"items":            []any{},
	}
	enqueueOnPaid(t, app, payload)
	time.Sleep(700 * time.Millisecond)

	if after := countJournalEntries(t, app); after != before {
		t.Errorf("an order with no amount must not create a journal: before=%d after=%d", before, after)
	}
}

// ── helpers ──

// enqueueOnPaid puts a durable `on_paid` event onto the outbox exactly as the
// HTTP transition path does — the running worker then delivers it to the `gl`
// subscription. Going through the outbox (rather than calling a handler
// directly) is what makes this an end-to-end check of the production transport.
func enqueueOnPaid(t *testing.T, app *App, payload map[string]any) {
	t.Helper()
	store := db.NewOutboxStore(app.Database(), db.DriverSQLite)

	// Mirror the emitting path exactly (internal/api/handler.go: the durable
	// branch): the outbox row carries the SHORT event name and an
	// events.EventMessage envelope — the handler re-resolves the channel
	// declaration from the live registry and derives the fully-qualified name
	// ("module.entity.event") itself. Enqueuing the fully-qualified name here
	// made the runtime report "no channels resolved", which is precisely the
	// mistake this helper now documents.
	envelope, err := json.Marshal(map[string]any{
		"event":    "on_paid",
		"resource": "cafe-order/order",
		"payload":  payload,
		"emitted":  "2026-09-22T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	if _, err := store.Enqueue(context.Background(), "kafe",
		"on_paid", "cafe-order/order", string(envelope)); err != nil {
		t.Fatalf("enqueue on_paid: %v", err)
	}
}

// waitForJournal waits for the outbox worker to deliver and the journal to
// appear. The worker polls on an interval, so a bounded poll is the honest way
// to observe asynchronous delivery.
func waitForJournal(t *testing.T, app *App) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if countJournalEntries(t, app) > 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("timed out waiting for the outbox worker to deliver on_paid and create a journal")
}

// countJournalEntries lists journal entries through the registry's store — the
// same store the HTTP surface reads.
func countJournalEntries(t *testing.T, app *App) int {
	t.Helper()
	store, err := app.GetEntityStore("gl", "journal-entry")
	if err != nil {
		t.Fatalf("GetEntityStore(gl/journal-entry): %v", err)
	}
	res, err := store.List(context.Background(), db.ListParams{
		WorkspaceID: "kafe",
		Page:        1,
		PerPage:     500,
	})
	if err != nil {
		t.Fatalf("list journal entries: %v", err)
	}
	return len(res.Data)
}

// findJournalBySource returns the journal whose `source_id` is the order id, or
// nil when none exists.
func findJournalBySource(t *testing.T, app *App, sourceID string) *db.EntityRecord {
	t.Helper()
	store, err := app.GetEntityStore("gl", "journal-entry")
	if err != nil {
		t.Fatalf("GetEntityStore(gl/journal-entry): %v", err)
	}
	rec, err := store.FindByField(context.Background(), "kafe", "source_id", sourceID)
	if err != nil || rec == nil {
		return nil
	}
	return rec
}

// journalLines returns the journal's child `lines` rows.
//
// It re-reads through GetByID on purpose: child relation data is hydrated by
// GetByID's hydrateAndCompute, not by FindByField — reading line items off the
// FindByField record returned zero rows even though the journal had posted.
func journalLines(t *testing.T, app *App, entry *db.EntityRecord) []map[string]any {
	t.Helper()
	store, err := app.GetEntityStore("gl", "journal-entry")
	if err != nil {
		t.Fatalf("GetEntityStore(gl/journal-entry): %v", err)
	}
	full, err := store.GetByID(context.Background(), db.GetByIDParams{
		WorkspaceID: "kafe",
		ID:          entry.ID,
	})
	if err != nil || full == nil {
		t.Fatalf("GetByID(%s): %v", entry.ID, err)
	}
	// The hydrated value's dynamic type is []map[string]any (not []any), so
	// assert the concrete slice types rather than going through []any.
	switch raw := full.Data["lines"].(type) {
	case []any:
		out := make([]map[string]any, 0, len(raw))
		for _, ln := range raw {
			if m, ok := ln.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	case []map[string]any:
		return raw
	default:
		return nil
	}
}

// numberOf coerces a money/decimal value to float64 for arithmetic assertions.
func numberOf(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case string:
		var f float64
		_, _ = fmt.Sscanf(n, "%g", &f)
		return f
	case map[string]any:
		return numberOf(n["amount"])
	default:
		return 0
	}
}

// seedKafeAccounts inserts the chart of accounts the journal's `exists: account`
// constraint and the gl setting keys resolve against. Production kafe gets these
// from `formspec seed`; the test seeds the same rows through the same store so
// natural-key generation and validation still apply.
func seedKafeAccounts(t *testing.T, app *App) {
	t.Helper()
	store, err := app.GetEntityStore("gl", "account")
	if err != nil {
		t.Fatalf("GetEntityStore(gl/account): %v", err)
	}
	// Codes MUST match the defaults in modules/gl/config/gl.yaml
	// (gl_journal_account_*), because journalize.star looks each one up and
	// fails with FORMSPEC.GL.ACCOUNT_NOT_FOUND otherwise — which is exactly the
	// error the first run of this test surfaced.
	accounts := []map[string]any{
		{"code": "1-1000", "name": "Kas", "type": "asset", "normal_balance": "debit", "is_active": true},
		{"code": "4-1000", "name": "Omzet Penjualan", "type": "revenue", "normal_balance": "credit", "is_active": true},
		{"code": "2-2000", "name": "Utang Pajak", "type": "liability", "normal_balance": "credit", "is_active": true},
		{"code": "2-1000", "name": "Service Charge Diterima Dimuka", "type": "liability", "normal_balance": "credit", "is_active": true},
		{"code": "5-1000", "name": "Diskon Penjualan", "type": "expense", "normal_balance": "debit", "is_active": true},
	}
	for _, a := range accounts {
		if _, err := store.Insert(context.Background(), db.InsertParams{
			WorkspaceID: "kafe",
			CreatedBy:   "e2e-seed",
			Data:        a,
		}); err != nil {
			t.Fatalf("seed account %v: %v", a["code"], err)
		}
	}
}
