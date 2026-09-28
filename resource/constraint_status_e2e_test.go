package formspec

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// A uniqueness violation is a CLIENT-VISIBLE CONFLICT, not a server fault.
//
// Measured 2026-09-28 (changelog `2026-09-28-001`): all three shapes below
// answered **500 INTERNAL_ERROR**, because `writeStoreError` had no branch for a
// driver constraint error and `isConflictError` only matched
// "version conflict"/"not found". The cost was not cosmetic — 500 is the class
// that pages an operator, so every duplicate value raised a false alarm and real
// faults hid among them, and a client could not tell "send a different value"
// from "retry later".
//
// These tests pin the status AND the field name, because the useful half of the
// fix is that the caller learns WHICH value collided.
func TestConstraintViolation_Returns409WithField(t *testing.T) {
	app := bootKafe(t)
	ids := seedKafeTableScenario(t, app)
	admin := seedKafeAdminToken(t, app)

	cases := []struct {
		name      string
		post      func() (int, map[string]any)
		wantField string
	}{
		{
			// Partial unique index — kafe 10.34c. This is the shape that made
			// the bug user-facing: the QR check-in page answers it.
			name: "partial unique index (one open session per table)",
			post: func() (int, map[string]any) {
				createSession(t, app, admin, ids.tableID, ids.branchID, "CONSTRAINT-TOKEN-1")
				return doAuthed(t, app, http.MethodPost,
					"/kafe/_ui/entity/cafe-order/table-session", admin, map[string]any{
						"transaction_date": time.Now().UTC().Format(time.RFC3339),
						"branch_id":        ids.branchID,
						"dining_table_id":  ids.tableID,
						"guest_token":      "CONSTRAINT-TOKEN-2",
					})
			},
			wantField: "dining_table_id",
		},
		{
			// Composite unique index → "branch_id, code".
			name: "composite unique index (branch + table code)",
			post: func() (int, map[string]any) {
				return doAuthed(t, app, http.MethodPost,
					"/kafe/_ui/entity/cafe-master/dining-table", admin, map[string]any{
						"branch_id": ids.branchID,
						"code":      "A-01", // seeded for this branch
						"qr_token":  "CONSTRAINT-UNIQUE-TOKEN",
					})
			},
			wantField: "branch_id",
		},
		{
			// Field `unique: true` + natural_key. Note this one has NO
			// rule-based pre-check (`validateUnique` only runs for an explicit
			// `rules: [{name: unique}]`), so the DB constraint is the sole
			// enforcement — and it used to be the sole reason for a 500.
			name: "field natural_key uniqueness (duplicate qr_token)",
			post: func() (int, map[string]any) {
				return doAuthed(t, app, http.MethodPost,
					"/kafe/_ui/entity/cafe-master/dining-table", admin, map[string]any{
						"branch_id": ids.branchID,
						"code":      "CONSTRAINT-NEW-CODE",
						"qr_token":  "JKT-A01-DEMO", // the seeded table's token
					})
			},
			wantField: "qr_token",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, out := tc.post()
			if status != http.StatusConflict {
				t.Fatalf("a duplicate value must answer 409 CONFLICT, got %d (%v)", status, out)
			}
			errObj, _ := out["error"].(map[string]any)
			if code, _ := errObj["code"].(string); code != "CONFLICT" {
				t.Errorf("expected code CONFLICT, got %q", code)
			}
			msg, _ := errObj["message"].(string)
			if !strings.Contains(msg, tc.wantField) {
				t.Errorf("the message must name the colliding field %q so the client can act on it; got %q",
					tc.wantField, msg)
			}
			// The tenant scope column is framework-owned: naming it would point
			// the caller at a field it never supplied.
			if strings.Contains(msg, "tenant_id") {
				t.Errorf("the message must not name the framework's tenant scope column; got %q", msg)
			}
		})
	}
}

// A non-uniqueness failure must NOT be reclassified as a conflict. Without this,
// a fix that widened the classifier could turn every storage error into a 409
// and hide real faults — the opposite failure of the one being fixed.
func TestConstraintViolation_NonUniqueStays500(t *testing.T) {
	app := bootKafe(t)
	ids := seedKafeTableScenario(t, app)
	admin := seedKafeAdminToken(t, app)

	// A relation pointing at a non-existent target is a validation failure, not
	// a uniqueness conflict.
	status, out := doAuthed(t, app, http.MethodPost,
		"/kafe/_ui/entity/cafe-order/table-session", admin, map[string]any{
			"transaction_date": time.Now().UTC().Format(time.RFC3339),
			"branch_id":        ids.branchID,
			"dining_table_id":  "00000000-0000-0000-0000-000000000000",
			"guest_token":      "CONSTRAINT-BAD-RELATION",
		})
	if status == http.StatusConflict {
		t.Fatalf("a broken relation must not be reported as a uniqueness conflict, got 409 (%v)", out)
	}
}
