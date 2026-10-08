package formspec

import (
	"net/http"
	"testing"

	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// create_scope over the REAL HTTP surface (plan
// docs_internal/plan/public-scope-enforcement.md).
//
// The unit tests in internal/api call `enforceCreateScope` directly, which
// proves the RULE but not that the rule is reachable: a guard wired into the
// wrong handler, or after the write, would still pass those. This test drives
// `POST /_ui/entity/...` so both halves are covered — the decision AND the
// wiring that feeds it.
//
// The gap being closed: a create has no row to filter, so `row_scope` cannot
// constrain it. Before this, an anonymous QR guest could POST a table session or
// an order claiming ANY branch — `branch_id` declares no `required_permission`,
// so `denyForbiddenFieldWrites` never looked at it.

// TestKafe_CreateScope_SessionCannotClaimAnotherBranch drives the first step of
// the QR flow: opening a table session. The branch of a session is the branch of
// its table, so a session claiming a different branch must be refused BEFORE the
// write.
func TestKafe_CreateScope_SessionCannotClaimAnotherBranch(t *testing.T) {
	app := bootKafe(t)
	ids := seedKafeTableScenario(t, app)
	admin := seedKafeAdminToken(t, app)

	// A second branch, so "another branch" is a real, existing target rather than
	// a value that would fail for other reasons (an unknown id would be refused
	// by the relation check instead, which would make this test prove nothing).
	otherBranch := insertRecord(t, app, "cafe-master", "branch", map[string]any{
		"code": "KFE-BDG-01", "name": "Kafe Bandung",
		"tax_percent": "10", "service_charge_percent": "5",
		"apply_service_charge": true, "is_active": true,
	})

	status, out := doAuthed(t, app, http.MethodPost,
		"/kafe/_ui/entity/cafe-order/table-session", admin, map[string]any{
			"transaction_date": "2026-10-06T10:00:00Z",
			"dining_table_id":  ids.tableID, // table lives in KFE-JKT-01
			"branch_id":        otherBranch, // claiming KFE-BDG-01
			"guest_token":      "SCOPE-CLAIM-A",
		})
	if status != http.StatusForbidden {
		t.Fatalf("a session claiming another branch must be refused with 403, got %d (%v)", status, out)
	}

	// It must NOT exist: the refusal has to happen before the write, not be
	// reported after one (the same rule the initial-state guard is held to).
	assertNoTableSessionWithToken(t, app, "SCOPE-CLAIM-A")

	// And the honest payload still works — a guard that blocks the happy path
	// gets disabled.
	status, out = doAuthed(t, app, http.MethodPost,
		"/kafe/_ui/entity/cafe-order/table-session", admin, map[string]any{
			"transaction_date": "2026-10-06T10:00:00Z",
			"dining_table_id":  ids.tableID,
			"branch_id":        ids.branchID, // the table's own branch
			"guest_token":      "SCOPE-CLAIM-OK",
		})
	if status != http.StatusCreated {
		t.Fatalf("the table's own branch must be accepted, got %d (%v)", status, out)
	}
}

// TestKafe_CreateScope_OrderCannotClaimAnotherBranch drives the second step: the
// order must inherit the branch of the table session it references.
func TestKafe_CreateScope_OrderCannotClaimAnotherBranch(t *testing.T) {
	app := bootKafe(t)
	ids := seedKafeTableScenario(t, app)
	admin := seedKafeAdminToken(t, app)

	otherBranch := insertRecord(t, app, "cafe-master", "branch", map[string]any{
		"code": "KFE-BDG-01", "name": "Kafe Bandung",
		"tax_percent": "10", "service_charge_percent": "5",
		"apply_service_charge": true, "is_active": true,
	})

	// A session in the table's own branch, which the order will reference.
	sessionID := insertRecord(t, app, "cafe-order", "table-session", map[string]any{
		"transaction_date": "2026-10-06T10:00:00Z",
		"dining_table_id":  ids.tableID,
		"branch_id":        ids.branchID,
		"guest_token":      "SCOPE-ORDER-A",
	})

	orderBody := func(branchID string) map[string]any {
		return map[string]any{
			"transaction_date": recentDate(),
			"channel":          "qr_table",
			"table_session_id": sessionID,
			"guest_token":      "SCOPE-ORDER-A",
			"branch_id":        branchID,
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
		}
	}

	status, out := doAuthed(t, app, http.MethodPost,
		"/kafe/_ui/entity/cafe-order/order", admin, orderBody(otherBranch))
	if status != http.StatusForbidden {
		t.Fatalf("an order claiming another branch must be refused with 403, got %d (%v)", status, out)
	}

	status, out = doAuthed(t, app, http.MethodPost,
		"/kafe/_ui/entity/cafe-order/order", admin, orderBody(ids.branchID))
	if status != http.StatusCreated {
		t.Fatalf("an order in the session's own branch must be accepted, got %d (%v)", status, out)
	}
}

// TestKafe_CreateScope_DanglingReferenceStaysValidationError pins the
// CLASSIFICATION on the real path, because getting it wrong was a measured
// regression: the check once answered 403 for an unresolvable reference, which
// relabelled a client fault as a permission problem and broke the 422 that
// TestKafe_ClientFaults_BrokenRelationIsStillValidationError had always seen.
//
// The distinction is the point: a MISMATCH is an authorization refusal (403),
// while a reference that does not exist is bad INPUT (422).
func TestKafe_CreateScope_DanglingReferenceStaysValidationError(t *testing.T) {
	app := bootKafe(t)
	ids := seedKafeTableScenario(t, app)
	admin := seedKafeAdminToken(t, app)

	status, out := doAuthed(t, app, http.MethodPost,
		"/kafe/_ui/entity/cafe-order/table-session", admin, map[string]any{
			"transaction_date": "2026-10-06T10:00:00Z",
			"dining_table_id":  "00000000-0000-0000-0000-000000000000",
			"branch_id":        ids.branchID,
			"guest_token":      "SCOPE-DANGLING",
		})
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("a dangling reference must answer 422 (bad input, not a permission problem), got %d (%v)", status, out)
	}
}

// assertNoTableSessionWithToken fails when a session carrying the token exists,
// which is how a "refused" response is held to "and nothing was written".
func assertNoTableSessionWithToken(t *testing.T, app *App, token string) {
	t.Helper()
	store, err := app.GetEntityStore("cafe-order", "table-session")
	if err != nil {
		t.Fatalf("GetEntityStore(cafe-order/table-session): %v", err)
	}
	res, err := store.List(t.Context(), db.ListParams{
		WorkspaceID: "kafe", Page: 1, PerPage: 500,
	})
	if err != nil {
		t.Fatalf("list table-sessions: %v", err)
	}
	for _, rec := range res.Data {
		if got, _ := rec.Data["guest_token"].(string); got == token {
			t.Errorf("a session was written despite the refusal (guest_token=%q)", token)
		}
	}
}
