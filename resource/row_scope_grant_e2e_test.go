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

// Kafe 10.67 measured the gap this closes. Rule #1 of the kafe domain —
// "hanya pesanan LUNAS yang masuk dapur" — is written in docs as applying
// WITHOUT exception, but enforcement lived in ONE place: the Kanban declared
// three columns (`paid`/`in_kitchen`/`ready`). A barista's own token could
// `GET .../order/{id}` on a `draft` order and read it — including the guest's
// `guest_token`, the access key to their own order on the public surface.
//
// GAP-08 is the same shape on a different axis: the KDS branch filter was a kind
// `fixed_filters` entry, merged by the BROWSER, so `curl` dropped it.
//
// Both are now boundary rules: a role grant carries `row_scope` per action, the
// server resolves it (never the client) and the storage layer ANDs it into every
// read and write. These tests exercise the real kafe tree in-process through the
// real HTTP surface, because every earlier verification of this area was either
// unit-level or a manual walkthrough — nothing failed the build if it regressed.

// seedKafeRole writes a role record with the given grants, exactly as the seed
// manifest does (the grants live as JSON on the role entity).
func seedKafeRole(t *testing.T, app *App, name, appName string, grants []map[string]any) {
	t.Helper()
	store, err := app.Registry().GetEntityStore("formspec.core", "role")
	if err != nil {
		t.Fatalf("role store: %v", err)
	}
	rec := map[string]any{
		"name": name, "app": appName, "description": "row-scope test role",
		"grants": grants,
	}
	if _, err := store.Insert(context.Background(), db.InsertParams{
		WorkspaceID: "kafe", CreatedBy: "test", SystemCaller: true, Data: rec,
	}); err != nil {
		t.Fatalf("insert role %s: %v", name, err)
	}
}

// seedKafeRoleUser creates an account bound to one role at one branch, the shape
// the seed file uses (`assignments: [{role, dimension: branch_id, value}]`).
func seedKafeRoleUser(t *testing.T, app *App, username, role, appName, branchID, password string) string {
	t.Helper()
	api.ResetAuthRateLimiters()
	store, err := app.Registry().GetEntityStore("formspec.core", "user")
	if err != nil {
		t.Fatalf("user store: %v", err)
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	data := map[string]any{
		"username": username, "password_hash": hash, "active": true,
		"roles": []string{role}, "permissions": []string{},
	}
	if branchID != "" {
		data["assignments"] = []map[string]any{
			{"role": role, "dimension": "branch_id", "value": branchID},
		}
	}
	if _, err := store.Insert(context.Background(), db.InsertParams{
		WorkspaceID: "kafe", CreatedBy: "test", SystemCaller: true, Data: data,
	}); err != nil {
		t.Fatalf("insert user %s: %v", username, err)
	}
	// The context id is `<role>@<value>` (auth.ContextChoice.ID). With exactly one
	// assignment an empty id would auto-pick it, but naming it explicitly keeps the
	// test honest about WHICH boundary it asserts.
	assignment := ""
	if branchID != "" {
		assignment = role + "@" + branchID
	}
	return loginKafeUser(t, app, username, appName, password, assignment)
}

// loginKafeUser logs in and returns an access token. When the account has one
// assignment the login resolves it automatically; with several, `assignment`
// selects one (`<role>@<value>`).
func loginKafeUser(t *testing.T, app *App, username, appName, password, assignment string) string {
	t.Helper()
	// Login is scoped to one App (10.21): an app-scoped role only contributes
	// its grants when the session names that App.
	body := map[string]any{"username": username, "password": password, "app": appName}
	if assignment != "" {
		body["assignment"] = assignment
	}
	status, resp := doJSON(t, app, http.MethodPost, "/kafe/_ui/auth/login", body)
	if status != http.StatusOK {
		t.Fatalf("login %s: status %d body %v", username, status, resp)
	}
	data, _ := resp["data"].(map[string]any)
	tok, _ := data["access_token"].(string)
	if tok == "" {
		// Several contexts: the caller must pick one explicitly.
		t.Fatalf("login %s: no access_token (choices %v)", username, resp)
	}
	return tok
}

// kdsGrants mirrors the row scope the seed declares for the kitchen roles.
func kdsGrants() []map[string]any {
	kitchen := []map[string]any{
		{"field": "status", "op": "in", "value": "paid,in_kitchen,ready,served"},
		{"field": "branch_id", "op": "eq", "from": "session"},
	}
	act := func(name string, scope []map[string]any) map[string]any {
		m := map[string]any{"name": name}
		if scope != nil {
			m["row_scope"] = scope
		}
		return m
	}
	return []map[string]any{{
		"page": "order-page",
		"actions": []map[string]any{
			act("list", kitchen), act("view", kitchen), act("update", kitchen),
			act("start-preparing", nil), act("mark-ready", nil),
		},
	}}
}

// waiterGrants is the front-of-house half of the same rule.
func waiterGrants() []map[string]any {
	servable := []map[string]any{
		{"field": "status", "op": "in", "value": "ready,served,completed"},
		{"field": "branch_id", "op": "eq", "from": "session"},
	}
	act := func(name string, scope []map[string]any) map[string]any {
		m := map[string]any{"name": name}
		if scope != nil {
			m["row_scope"] = scope
		}
		return m
	}
	return []map[string]any{{
		"page": "order-page",
		"actions": []map[string]any{
			act("list", servable), act("view", servable), act("update", servable),
			act("mark-served", nil),
		},
	}}
}

// cashierGrants grants the POS role `list`/`view` with NO row scope — the point
// of the second assertion below: a cashier must keep seeing the drafts they are
// composing, which a per-ENTITY status filter could never express.
func cashierGrants() []map[string]any {
	return []map[string]any{{
		"page": "order-page",
		"actions": []map[string]any{
			{"name": "list"}, {"name": "view"}, {"name": "create"}, {"name": "update"},
			{"name": "confirm-payment"},
		},
	}}
}

// seedOrderRow writes an order directly through the store (setup, not subject).
//
// It returns the id AND the number the STORE assigned: the entity's natural-key
// rule generates `ORD-<year>-<seq>` per branch, so a literal seeded into
// `number` is overwritten. Asserting on a literal would test nothing.
func seedOrderRow(t *testing.T, app *App, label, branchID, status string) (id, number string) {
	t.Helper()
	store, err := app.Registry().GetEntityStore("cafe-order", "order")
	if err != nil {
		t.Fatalf("order store: %v", err)
	}
	// SystemCaller: seeding post-initial states bypasses the create-state rule
	// (10.72) deliberately — this is fixture data, not a user write.
	id, err = store.Insert(context.Background(), db.InsertParams{
		WorkspaceID: "kafe", CreatedBy: "test", SystemCaller: true,
		Data: map[string]any{
			"branch_id": branchID, "status": status,
			"channel": "cashier", "guest_token": "GUEST-" + label,
			"transaction_date": time.Now().UTC().Format(time.RFC3339),
		},
	})
	if err != nil {
		t.Fatalf("seed order %s (%s): %v", label, status, err)
	}
	rec, err := store.GetByID(context.Background(), db.GetByIDParams{WorkspaceID: "kafe", ID: id})
	if err != nil {
		t.Fatalf("read back %s: %v", label, err)
	}
	number, _ = rec.Data["number"].(string)
	return id, number
}

// orderNumbers runs a list request and returns the `number` of every row seen.
func orderNumbers(t *testing.T, app *App, token, query string) []string {
	t.Helper()
	status, body := doAuthed(t, app, http.MethodGet, "/kafe/_ui/entity/cafe-order/order"+query, token, nil)
	if status != http.StatusOK {
		t.Fatalf("list order%s: status %d body %v", query, status, body)
	}
	rows, _ := body["data"].([]any)
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		row, _ := r.(map[string]any)
		if n, ok := row["number"].(string); ok {
			out = append(out, n)
		}
	}
	return out
}

// orderIDs runs a list request and returns the id of every row seen.
//
// The BRANCH test needs ids, not numbers: the entity numbers orders per branch
// (natural key + `scope_field`, item 3.6), so `ORD-2026-00001` legitimately
// exists in two branches at once and a number cannot identify a row across them.
func orderIDs(t *testing.T, app *App, token, query string) []string {
	t.Helper()
	status, body := doAuthed(t, app, http.MethodGet, "/kafe/_ui/entity/cafe-order/order"+query, token, nil)
	if status != http.StatusOK {
		t.Fatalf("list order%s: status %d body %v", query, status, body)
	}
	rows, _ := body["data"].([]any)
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		row, _ := r.(map[string]any)
		if id, ok := row["id"].(string); ok {
			out = append(out, id)
		}
	}
	return out
}

// hasRow reports whether a list response contained one specific number.
func hasRow(items []string, want string) bool {
	for _, it := range items {
		if it == want {
			return true
		}
	}
	return false
}

// seedBranch creates a branch and returns its id (the value order.branch_id
// stores). Mirrors the harness used by the other kafe e2e tests.
func seedBranch(t *testing.T, app *App, code string) string {
	t.Helper()
	return insertRecord(t, app, "cafe-master", "branch", map[string]any{
		"code": code, "name": "Kafe " + code,
		"tax_percent": "10", "service_charge_percent": "5",
		"apply_service_charge": true, "is_active": true,
	})
}

// orderStatusByNumber reads the RAW stored status, not a read result, so a fix
// cannot pass by changing only what the API reports.
func orderStatusByNumber(t *testing.T, app *App, number string) string {
	t.Helper()
	store, err := app.Registry().GetEntityStore("cafe-order", "order")
	if err != nil {
		t.Fatalf("order store: %v", err)
	}
	rec, err := store.FindByField(context.Background(), "kafe", "number", number)
	if err != nil || rec == nil {
		t.Fatalf("find order %s: %v", number, err)
	}
	st, _ := rec.Data["status"].(string)
	return st
}

func TestKafe_KitchenRowScope_OnlyPaidOrdersReachTheKitchen(t *testing.T) {
	app := bootKafe(t)
	branch := seedBranch(t, app, "KFE-JKT-01")

	seedKafeRole(t, app, "rowscope-barista", "kafe-kds", kdsGrants())
	barista := seedKafeRoleUser(t, app, "rowscope-barista-user", "rowscope-barista", "kafe-kds", branch, "pw-barista")

	// The order the ledger measured: created anonymously and never paid.
	draftID, draftNo := seedOrderRow(t, app, "DRAFT-1", branch, "draft")
	_, paidNo := seedOrderRow(t, app, "PAID-1", branch, "paid")
	_, readyNo := seedOrderRow(t, app, "READY-1", branch, "ready")

	// 1. The plain list no longer carries the draft. Before this change the
	//    restriction lived in the Kanban's column declaration, so `list` — the
	//    very endpoint the board calls with `status[in]=…` — returned it.
	got := orderNumbers(t, app, barista, "?per_page=100")
	if hasRow(got, draftNo) {
		t.Fatalf("a DRAFT order reached the kitchen: %v — rule #1 is not an API boundary", got)
	}
	if !hasRow(got, paidNo) || !hasRow(got, readyNo) {
		t.Fatalf("the kitchen lost its own work: %v", got)
	}

	// 2. A client cannot re-open the hole: asking for drafts explicitly (the
	//    exact query the ledger used) still returns none, because the grant's
	//    predicate is ANDed server-side and the client value is never consulted.
	got = orderNumbers(t, app, barista, "?per_page=100&status[in]=draft")
	if hasRow(got, draftNo) {
		t.Fatalf("a client widened the boundary with an explicit filter: %v", got)
	}

	// 3. `find` is closed too — the ledger's measurement was a direct GET by id,
	//    not the list. The record reads as ABSENT (404), not forbidden, so the
	//    boundary is not an existence oracle either.
	status, body := doAuthed(t, app, http.MethodGet, "/kafe/_ui/entity/cafe-order/order/"+draftID, barista, nil)
	if status != http.StatusNotFound {
		t.Fatalf("GET a draft order as barista: status %d body %v — want 404", status, body)
	}

	// 4. And the write path: the kitchen cannot advance an order it must not see.
	status, _ = doAuthed(t, app, http.MethodPatch, "/kafe/_ui/entity/cafe-order/order/"+draftID,
		barista, map[string]any{"status": "in_kitchen"})
	if status != http.StatusNotFound {
		t.Fatalf("PATCH a draft order as barista: status %d — want 404", status)
	}
	if st := orderStatusByNumber(t, app, draftNo); st != "draft" {
		t.Fatalf("status = %q — a refused write must not have changed the row", st)
	}
}

// The branch half (GAP-08). The filter that used to be a kind `fixed_filters`
// entry — merged by the browser, droppable by curl — is now resolved from the
// session, so another branch's orders are simply not visible.
func TestKafe_KitchenRowScope_ConfinedToItsOwnBranch(t *testing.T) {
	app := bootKafe(t)
	jakarta := seedBranch(t, app, "KFE-JKT-01")
	bandung := seedBranch(t, app, "KFE-BDG-01")

	seedKafeRole(t, app, "rowscope-barista", "kafe-kds", kdsGrants())
	barista := seedKafeRoleUser(t, app, "rowscope-barista-user", "rowscope-barista", "kafe-kds", jakarta, "pw-barista")

	jktID, _ := seedOrderRow(t, app, "JKT-PAID", jakarta, "paid")
	bdgID, _ := seedOrderRow(t, app, "BDG-PAID", bandung, "paid")

	got := orderIDs(t, app, barista, "?per_page=100")
	if hasRow(got, bdgID) {
		t.Fatalf("another branch's order reached the kitchen: %v", got)
	}
	if !hasRow(got, jktID) {
		t.Fatalf("the kitchen lost its own branch's order: %v", got)
	}

	// Attempting to point the filter at the other branch changes nothing: the
	// session value is resolved server-side, and the manifest predicate is what
	// the storage layer applies.
	// Pointing the filter at the OTHER branch does not widen the view: the
	// session's branch is what the storage layer applies, so the caller still
	// sees only their own order (the restriction is not "no rows", it is "the
	// same rows as before" — a client filter may narrow, never redirect).
	got = orderIDs(t, app, barista, "?per_page=100&branch_id[eq]="+bandung)
	if hasRow(got, bdgID) {
		t.Fatalf("client-supplied branch filter widened the view: %v", got)
	}
	if !hasRow(got, jktID) {
		t.Fatalf("the kitchen lost its own order when filtering by another branch: %v", got)
	}
	if status, _ := doAuthed(t, app, http.MethodGet, "/kafe/_ui/entity/cafe-order/order/"+bdgID, barista, nil); status != http.StatusNotFound {
		t.Fatalf("cross-branch read: status %d, want 404", status)
	}
}

// The counterweight, and the reason this had to be per-ROLE rather than a status
// filter on the entity: the cashier composes drafts, so an entity-wide "only
// paid orders" rule would have made the POS unusable. Their grant declares no
// row scope, so they keep seeing everything in their branch.
func TestKafe_OrderRowScope_DoesNotBlindTheCashier(t *testing.T) {
	app := bootKafe(t)
	branch := seedBranch(t, app, "KFE-JKT-01")

	seedKafeRole(t, app, "rowscope-kasir", "kafe-pos", cashierGrants())
	kasir := seedKafeRoleUser(t, app, "rowscope-kasir-user", "rowscope-kasir", "kafe-pos", branch, "pw-kasir")

	_, draftNo := seedOrderRow(t, app, "DRAFT-1", branch, "draft")
	_, paidNo := seedOrderRow(t, app, "PAID-1", branch, "paid")

	got := orderNumbers(t, app, kasir, "?per_page=100")
	if !hasRow(got, draftNo) {
		t.Fatalf("the cashier lost sight of their own draft: %v — a per-entity status filter cannot express this rule", got)
	}
	if !hasRow(got, paidNo) {
		t.Fatalf("the cashier lost the paid order: %v", got)
	}
}

// The waiter sees what is ready to carry, not the drafts being composed.
func TestKafe_WaiterRowScope_OnlyServableOrders(t *testing.T) {
	app := bootKafe(t)
	branch := seedBranch(t, app, "KFE-JKT-01")

	seedKafeRole(t, app, "rowscope-pelayan", "kafe-pos", waiterGrants())
	pelayan := seedKafeRoleUser(t, app, "rowscope-pelayan-user", "rowscope-pelayan", "kafe-pos", branch, "pw-pelayan")

	_, draftNo := seedOrderRow(t, app, "DRAFT-1", branch, "draft")
	_, paidNo := seedOrderRow(t, app, "PAID-1", branch, "paid")
	_, readyNo := seedOrderRow(t, app, "READY-1", branch, "ready")

	got := orderNumbers(t, app, pelayan, "?per_page=100")
	if hasRow(got, draftNo) || hasRow(got, paidNo) {
		t.Fatalf("the waiter saw orders that are not servable yet: %v", got)
	}
	if !hasRow(got, readyNo) {
		t.Fatalf("the waiter lost the ready order: %v", got)
	}
}

// A scoped role whose session cannot supply the dimension FAILS CLOSED rather
// than degrading to "no restriction" — the rule in the manifest is worth nothing
// if an account without an assignment silently reads every row.
func TestKafe_RowScope_FailsClosedWithoutTheSessionAttribute(t *testing.T) {
	app := bootKafe(t)
	branch := seedBranch(t, app, "KFE-JKT-01")

	seedKafeRole(t, app, "rowscope-barista", "kafe-kds", kdsGrants())
	// No branch assignment: the account holds the role but has no dimension value.
	bounded := seedKafeRoleUser(t, app, "rowscope-unassigned", "rowscope-barista", "kafe-kds", "", "pw-x")

	seedOrderRow(t, app, "PAID-1", branch, "paid")

	status, body := doAuthed(t, app, http.MethodGet, "/kafe/_ui/entity/cafe-order/order?per_page=100", bounded, nil)
	if status == http.StatusOK {
		t.Fatalf("a scoped role without a branch assignment listed rows: %v", body)
	}
	if status != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (fail closed, and say so)", status)
	}
}

// typoGrants is kdsGrants with the ONE character the ledger's calibration used:
// `row_scopes` instead of `row_scope`.
//
// It is the most dangerous shape a role can be written in, because the two
// halves fail in opposite directions. `json.Unmarshal` into the typed `Grant`
// struct DROPS the key it does not know, so the grant still materializes — the
// kitchen keeps `list` — and the restriction vanishes. Before this test the
// observable behaviour was a barista reading drafts again (10.67) with every
// gate green: `formspec validate` has no schema for `grants`, `formspec check`
// did not look inside it, and the resolver had nothing to report.
func typoGrants() []map[string]any {
	return []map[string]any{{
		"page": "order-page",
		"actions": []map[string]any{
			{"name": "list", "row_scopes": []map[string]any{
				{"field": "status", "op": "in", "value": "paid,in_kitchen,ready,served"},
			}},
		},
	}}
}

// An unreadable row restriction DENIES the request. The alternative — serve it
// with no predicates — is the leak itself: the role looks confined to the paid
// orders and in fact reads every row, including the `guest_token` of an
// unpaid draft.
func TestKafe_UnreadableRowScope_DeniesInsteadOfReadingEveryRow(t *testing.T) {
	app := bootKafe(t)
	branch := seedBranch(t, app, "KFE-JKT-01")

	seedKafeRole(t, app, "rowscope-typo", "kafe-kds", typoGrants())
	barista := seedKafeRoleUser(t, app, "rowscope-typo-user", "rowscope-typo", "kafe-kds", branch, "pw-barista")

	draftID, _ := seedOrderRow(t, app, "DRAFT-1", branch, "draft")

	// The list must not answer 200 at all: a 200 with the full row set is the
	// regression this asserts against, and a 200 with a partial set would mean
	// the restriction had been silently reinterpreted.
	status, body := doAuthed(t, app, http.MethodGet, "/kafe/_ui/entity/cafe-order/order?per_page=100", barista, nil)
	if status == http.StatusOK {
		t.Fatalf("an unreadable row restriction served a list: %v — the kitchen reads every row while the manifest says otherwise", body)
	}
	if status != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (deny and say so, not a silent unscoped read)", status)
	}

	// The id-addressed paths take the same decision, so a direct GET cannot be
	// used to read the row the list refused to reveal.
	if status, _ := doAuthed(t, app, http.MethodGet, "/kafe/_ui/entity/cafe-order/order/"+draftID, barista, nil); status == http.StatusOK {
		t.Fatal("a direct GET bypassed the denial that blocked the list")
	}
}

// A row restriction on a field the target entity does not declare is the same
// class one level deeper: the predicate cannot be expressed, so the permission
// is refused rather than granted with a filter that does nothing.
//
// The role also holds an unrestricted `view`, and that is deliberate: the
// blocked `list` is what the request asserts on, and without a second grant the
// role would materialize to ZERO permissions and the account would be refused at
// LOGIN (the 0-permission app-access gate) instead — fail-closed too, but it
// would not prove the row-scope denial is what refused the read.
func TestKafe_RowScopeOnUndeclaredField_Denies(t *testing.T) {
	app := bootKafe(t)
	branch := seedBranch(t, app, "KFE-JKT-01")

	seedKafeRole(t, app, "rowscope-badfield", "kafe-kds", []map[string]any{{
		"page": "order-page",
		"actions": []map[string]any{
			{"name": "list", "row_scope": []map[string]any{
				{"field": "tidak_ada_field", "op": "eq", "value": "x"},
			}},
			{"name": "view"},
		},
	}})
	barista := seedKafeRoleUser(t, app, "rowscope-badfield-user", "rowscope-badfield", "kafe-kds", branch, "pw-barista")
	seedOrderRow(t, app, "PAID-1", branch, "paid")

	status, body := doAuthed(t, app, http.MethodGet, "/kafe/_ui/entity/cafe-order/order?per_page=100", barista, nil)
	if status != http.StatusForbidden {
		t.Fatalf("status = %d body %v, want 403 for a row scope on a field the entity does not have", status, body)
	}
}
