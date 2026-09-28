package formspec

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/primadi/formspec/internal/api"
	"github.com/primadi/formspec/internal/auth"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// In-process end-to-end harness for the VOID path (todo 5.24.4).
//
// `TestKafe_TableLifecycle_CancelReturnsToAvailable` deliberately drove the
// pre-payment `cancel-order` instead of `void-order`, because void was
// unreachable end to end: it is approval-gated (D5), and it also requires a
// reason that no surface could collect. The action-input contract
// (docs_internal/plan/action-input-contract.md, 2026-09-28-004) closed the
// second half — `params.inputs` on the transition, enforced on the PATCH path
// and carried across the approval boundary. This file drives the REAL chain on
// the REAL kafe spec, which is the only way to know those three fixes hold
// together:
//
//	void tanpa alasan      → 422 (dulu lolos: conditions tidak dievaluasi di PATCH)
//	void dengan alasan     → 202, TIDAK ada yang tertulis (dulu: alasan hilang)
//	pemohon menyetujui     → 403 (requester exclusion, 7.4.5)
//	supervisor menyetujui  → 200, order `cancelled` + `void_reason` TERSIMPAN
//
// A note on who does what: the order is created with the CASHIER's token so
// `created_by` names them, which is what makes the self-approval step above a
// real check rather than a coincidence of the fixture.

// seedKafeRoleToken creates a user holding the given roles and returns an access
// token for it.
//
// Roles ride in the JWT's `roles` claim straight from the user record
// (`IssueAccessToken`: `Roles: u.Roles`), which is what `CanApprove` reads — so
// the workflow's `roles: [cafe-order.supervisor]` step is satisfied by the
// supervisor seeded here and by nobody else.
func seedKafeRoleToken(t *testing.T, app *App, username string, roles []string) string {
	t.Helper()
	api.ResetAuthRateLimiters()
	reg := app.Registry()
	userStore, err := reg.GetEntityStore("formspec.core", "user")
	if err != nil {
		t.Fatalf("user store: %v", err)
	}
	hash, err := auth.HashPassword("kafe123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if _, err := userStore.Insert(context.Background(), db.InsertParams{
		WorkspaceID: "kafe", CreatedBy: "test",
		Data: map[string]any{
			"username": username, "password_hash": hash,
			"roles": roles, "permissions": []string{"*"}, "active": true,
		},
	}); err != nil {
		t.Fatalf("insert user %s: %v", username, err)
	}
	status, body := doJSON(t, app, http.MethodPost, "/kafe/_ui/auth/login", map[string]any{
		"username": username, "password": "kafe123",
	})
	if status != http.StatusOK {
		t.Fatalf("login %s: status %d body %v", username, status, body)
	}
	data, _ := body["data"].(map[string]any)
	tok, _ := data["access_token"].(string)
	if tok == "" {
		t.Fatalf("login %s: no access_token in %v", username, body)
	}
	return tok
}

// orderField reads one field of an order straight from the store the HTTP
// surface writes to, so an assertion cannot pass on a response body alone.
func orderField(t *testing.T, app *App, orderID, field string) any {
	t.Helper()
	store, err := app.GetEntityStore("cafe-order", "order")
	if err != nil {
		t.Fatalf("GetEntityStore(cafe-order/order): %v", err)
	}
	rec, err := store.GetByID(context.Background(), db.GetByIDParams{
		WorkspaceID: "kafe", ID: orderID,
	})
	if err != nil || rec == nil {
		t.Fatalf("GetByID(order %s): rec=%v err=%v", orderID, rec, err)
	}
	return rec.Data[field]
}

func orderStatus(t *testing.T, app *App, orderID string) string {
	t.Helper()
	s, _ := orderField(t, app, orderID, "status").(string)
	return s
}

// patchOrder is the raw PATCH the derived UI issues for a transition without an
// `impl` — the shape the input contract has to be enforced on.
func patchOrder(t *testing.T, app *App, token, orderID string, body map[string]any) (int, map[string]any) {
	t.Helper()
	return doAuthed(t, app, http.MethodPatch,
		"/kafe/_ui/entity/cafe-order/order/"+orderID, token, body)
}

// TestKafe_VoidOrder_ReasonRequiredThenSupervisorApproval is the reported gap,
// closed: the full void chain on the real kafe spec.
func TestKafe_VoidOrder_ReasonRequiredThenSupervisorApproval(t *testing.T) {
	app := bootKafe(t)
	ids := seedKafeTableScenario(t, app)
	admin := seedKafeAdminToken(t, app)
	seedKafeAccounts(t, app)

	cashier := seedKafeRoleToken(t, app, "kasir", []string{})
	supervisor := seedKafeRoleToken(t, app, "supervisor", []string{"cafe-order.supervisor"})

	const guestToken = "E2E-GUEST-TOKEN-VOID"
	createSession(t, app, admin, ids.tableID, ids.branchID, guestToken)
	sessionID := sessionIDByToken(t, app, guestToken)

	// The cashier takes the order, so `created_by` is theirs — which makes the
	// self-approval check below meaningful.
	order := createOrder(t, app, cashier, ids, sessionID, guestToken,
		ids.nasiGoreng, "Nasi Goreng Spesial", 45000, 1)
	patchOrderStatus(t, app, cashier, order, "awaiting_payment")
	patchOrderStatus(t, app, cashier, order, "paid")
	waitForTableStatus(t, app, ids.tableID, "occupied")

	// ── 1. Void WITHOUT a reason is refused ──
	//
	// This is the headline fix. `PATCH` is the only path a transition without
	// an `impl` can take, and it used to evaluate only `guard` — never the
	// transition's `conditions`. `void-order` declares
	// `conditions: len(params.get('void_reason','')) > 0` and looked perfectly
	// declarative while accepting a bare `{"status":"cancelled"}`.
	status, out := patchOrder(t, app, cashier, order, map[string]any{"status": "cancelled"})
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("void tanpa alasan = %d, want 422 — the transition's own condition "+
			"must run on the path that applies it; body: %v", status, out)
	}
	if got := orderStatus(t, app, order); got != "paid" {
		t.Fatalf("a refused void must leave the order alone, status=%q", got)
	}

	// ── 2. Void WITH a reason starts the approval ──
	const reason = "tamu komplain — pesanan salah"
	status, out = patchOrder(t, app, cashier, order, map[string]any{
		"status": "cancelled", "void_reason": reason,
	})
	if status != http.StatusAccepted {
		t.Fatalf("void dengan alasan = %d, want 202 (approval-gated, D5); body: %v", status, out)
	}
	if got := orderStatus(t, app, order); got != "paid" {
		t.Fatalf("202 must write NOTHING to the record, status=%q", got)
	}
	if got, _ := orderField(t, app, order, "void_reason").(string); got != "" {
		t.Fatalf("the 202 call must not persist the reason yet, got %q", got)
	}

	// ── 3. The requester cannot approve their own request ──
	//
	// 7.4.5, and it only holds because `requesterIDFor` reads the record's
	// `created_by` COLUMN (a framework column, absent from the Data map) — read
	// from `Data` it was always empty, so this exclusion never matched.
	status, out = patchOrder(t, app, cashier, order, map[string]any{
		"status": "cancelled", "decision": "approve",
	})
	if status != http.StatusForbidden {
		t.Fatalf("self-approve = %d, want 403 (requester exclusion); body: %v", status, out)
	}

	// ── 4. The supervisor approves, WITHOUT repeating the reason ──
	//
	// The whole point of carrying the requester's inputs on the approval row:
	// the approver is a different person on a different request, and asking
	// them to retype someone else's reason would make a correct approval fail.
	status, out = patchOrder(t, app, supervisor, order, map[string]any{
		"status": "cancelled", "decision": "approve",
	})
	if status != http.StatusOK {
		t.Fatalf("approve = %d, want 200; body: %v", status, out)
	}

	// ── 5. State AND reason land together ──
	if got := orderStatus(t, app, order); got != "cancelled" {
		t.Fatalf("after approval the order must be `cancelled`, got %q", got)
	}
	if got, _ := orderField(t, app, order, "void_reason").(string); got != reason {
		t.Fatalf("void_reason = %q, want %q — an input collected before an approval "+
			"must survive it; the requester's value was dropped here before "+
			"2026-09-28", got, reason)
	}
	// The approval verb is not an entity field.
	if _, stored := out["data"]; stored {
		if _, leaked := orderField(t, app, order, "decision").(string); leaked {
			t.Error("`decision` must never be stored as a record field")
		}
	}
}

// TestKafe_VoidOrder_EmitsOnCancel is the second half of the void contract: the
// transition declares `emit: on_cancel`, and the table bridge is what listens
// for it. A voided order that leaves its table `occupied` forever is the same
// class of defect as 10.40b (the missing `on_paid` bridge).
//
// Kept separate from the test above so a failure here cannot be mistaken for a
// failure of the input/approval chain.
func TestKafe_VoidOrder_EmitsOnCancel(t *testing.T) {
	app := bootKafe(t)
	ids := seedKafeTableScenario(t, app)
	admin := seedKafeAdminToken(t, app)
	seedKafeAccounts(t, app)

	cashier := seedKafeRoleToken(t, app, "kasir-emit", []string{})
	supervisor := seedKafeRoleToken(t, app, "supervisor-emit", []string{"cafe-order.supervisor"})

	const guestToken = "E2E-GUEST-TOKEN-VOID-EMIT"
	createSession(t, app, admin, ids.tableID, ids.branchID, guestToken)
	sessionID := sessionIDByToken(t, app, guestToken)

	order := createOrder(t, app, cashier, ids, sessionID, guestToken,
		ids.nasiGoreng, "Nasi Goreng Spesial", 45000, 1)
	patchOrderStatus(t, app, cashier, order, "awaiting_payment")
	patchOrderStatus(t, app, cashier, order, "paid")
	waitForTableStatus(t, app, ids.tableID, "occupied")

	if status, out := patchOrder(t, app, cashier, order, map[string]any{
		"status": "cancelled", "void_reason": "salah pesan",
	}); status != http.StatusAccepted {
		t.Fatalf("start approval = %d, want 202: %v", status, out)
	}
	if status, out := patchOrder(t, app, supervisor, order, map[string]any{
		"status": "cancelled", "decision": "approve",
	}); status != http.StatusOK {
		t.Fatalf("approve = %d, want 200: %v", status, out)
	}
	if got := orderStatus(t, app, order); got != "cancelled" {
		t.Fatalf("order must be cancelled, got %q", got)
	}

	// The bridge is a SUBSCRIPTION, so the status changes after the HTTP
	// response — polling states that instead of hiding it behind a sleep.
	if got := waitForTableStatus(t, app, ids.tableID, "available"); got != "available" {
		t.Fatalf("a voided order must return its table to `available` (on_cancel → "+
			"bridge), got %q — the order is cancelled, so the table cannot stay "+
			"occupied: the next guest would be unable to check in", got)
	}
	_ = time.Second // keep the time import meaningful if the poll is adjusted
}
