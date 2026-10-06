package formspec

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Three classes of CALLER FAULT used to answer 500 INTERNAL_ERROR (measured
// 2026-10-02 against the kafe dev server, kafe 10.61):
//
//	unknown field        POST with {"zzz":1}   → 500 "unknown field: \"zzz\""
//	unparseable value    transaction_date:"kemarin" → 500 "cannot parse … as date"
//	value outside an enum CHECK (area:"kolong") → 500 "CHECK constraint failed: …"
//
// Each is the caller's mistake, and each already violated a written contract:
// `docs/spec/backend/01-core-basic.md` makes the unknown-field rejection
// normative with `VALIDATION_ERROR` (422), and
// `docs/spec/backend/05-field-types.md` §enum does the same for a value outside
// `enum_values`.
//
// The cost was not cosmetic. 500 is the class that pages an operator (so every
// user typo raised a false alarm and buried real faults among them), a client
// cannot tell "fix your request" from "retry later", and the SPA rendered
// "Internal server error" while the storage error already named the field.
//
// These tests pin the status AND the field name, because the useful half of the
// fix is that the caller learns WHICH field was rejected — without that, the
// message is actionable only to someone reading server logs.
func TestKafe_ClientFaults_Answer422WithField(t *testing.T) {
	app := bootKafe(t)
	ids := seedKafeTableScenario(t, app)
	admin := seedKafeAdminToken(t, app)

	cases := []struct {
		name      string
		path      string
		body      map[string]any
		wantField string
	}{
		{
			// Normative per 01-core-basic.md: unknown fields must be rejected
			// with VALIDATION_ERROR, never silently accepted.
			name: "unknown field",
			path: "/kafe/_ui/entity/cafe-master/dining-table",
			body: map[string]any{
				"branch_id": ids.branchID, "code": "FAULT-NEW-1", "qr_token": "FAULT-TOKEN-1",
				"zzz": 1,
			},
			wantField: "zzz",
		},
		{
			// The CHECK constraint the DDL generator emits for every enum field
			// is the only CHECK in the schema, so a CHECK rejection IS an enum
			// rejection. `area` is declared [indoor, outdoor, smoking, vip].
			name: "value outside enum_values",
			path: "/kafe/_ui/entity/cafe-master/dining-table",
			body: map[string]any{
				"branch_id": ids.branchID, "code": "FAULT-NEW-2", "qr_token": "FAULT-TOKEN-2",
				"area": "kolong",
			},
			wantField: "area",
		},
		{
			// A date that cannot be interpreted at all. `table-session` carries
			// `transaction_date`, and the parse failure happens before any write
			// (so no unique index gets a chance to answer first).
			name: "unparseable transaction_date",
			path: "/kafe/_ui/entity/cafe-order/table-session",
			body: map[string]any{
				"transaction_date": "kemarin",
				"branch_id":        ids.branchID,
				"dining_table_id":  ids.tableID,
				"guest_token":      "FAULT-GUEST-TOKEN",
			},
			wantField: "transaction_date",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, out := doAuthed(t, app, http.MethodPost, tc.path, admin, tc.body)
			if status != http.StatusUnprocessableEntity {
				t.Fatalf("a caller fault must answer 422 VALIDATION_ERROR, got %d (%v)", status, out)
			}
			errObj, _ := out["error"].(map[string]any)
			if code, _ := errObj["code"].(string); code != "VALIDATION_ERROR" {
				t.Errorf("expected code VALIDATION_ERROR, got %q", code)
			}
			// The field must reach the client structurally, not only as prose.
			details, _ := errObj["details"].([]any)
			if len(details) == 0 {
				t.Fatalf("expected a details array naming the field, got %v", errObj)
			}
			first, _ := details[0].(map[string]any)
			if field, _ := first["field"].(string); field != tc.wantField {
				t.Errorf("details[0].field = %q, want %q", field, tc.wantField)
			}
			// Driver noise must not leak: the SQLite CHECK text and the
			// json_extract expression are not things a user can act on.
			msg, _ := errObj["message"].(string)
			for _, leak := range []string{"json_extract", "constraint failed", "(275)"} {
				if strings.Contains(msg, leak) {
					t.Errorf("message must not carry driver text (%q): %q", leak, msg)
				}
			}
		})
	}
}

// The negative control for the same change: a VALID request must still be
// accepted. Without this, "make everything 422" would pass the test above while
// breaking writes — the opposite failure of the one being fixed.
func TestKafe_ClientFaults_ValidRequestStillCreates(t *testing.T) {
	app := bootKafe(t)
	ids := seedKafeTableScenario(t, app)
	admin := seedKafeAdminToken(t, app)

	status, out := doAuthed(t, app, http.MethodPost,
		"/kafe/_ui/entity/cafe-master/dining-table", admin, map[string]any{
			"branch_id": ids.branchID, "code": "FAULT-OK-1", "qr_token": "FAULT-OK-TOKEN",
			"area": "outdoor", "seats": 4,
		})
	if status != http.StatusCreated {
		t.Fatalf("a valid create must still answer 201, got %d (%v)", status, out)
	}
}

// Which unknown field gets named must not depend on Go's randomized map
// iteration order. Before this was pinned, a payload with two unknown keys
// could report either one on the same input — so a client assertion (or a bug
// report pasted into an issue) was not reproducible.
//
// The whole set is reported, sorted, so the caller learns about every typo at
// once instead of one per round-trip.
func TestKafe_ClientFaults_UnknownFieldsAreDeterministic(t *testing.T) {
	app := bootKafe(t)
	ids := seedKafeTableScenario(t, app)
	admin := seedKafeAdminToken(t, app)

	wantFields := []string{"aaa_typo", "zzz_typo"}
	for attempt := 0; attempt < 8; attempt++ {
		status, out := doAuthed(t, app, http.MethodPost,
			"/kafe/_ui/entity/cafe-master/dining-table", admin, map[string]any{
				"branch_id": ids.branchID,
				"code":      "FAULT-DET-" + strconv.Itoa(attempt),
				"qr_token":  "FAULT-DET-TOKEN-" + strconv.Itoa(attempt),
				// Two unknown keys: map order decides which a naive
				// implementation would report first.
				"zzz_typo": 1,
				"aaa_typo": 2,
			})
		if status != http.StatusUnprocessableEntity {
			t.Fatalf("attempt %d: expected 422, got %d (%v)", attempt, status, out)
		}
		errObj, _ := out["error"].(map[string]any)
		details, _ := errObj["details"].([]any)
		var got []string
		for _, d := range details {
			m, _ := d.(map[string]any)
			if f, _ := m["field"].(string); f != "" {
				got = append(got, f)
			}
		}
		if strings.Join(got, ",") != strings.Join(wantFields, ",") {
			t.Fatalf("attempt %d: reported fields = %v, want %v (stable, sorted, complete)",
				attempt, got, wantFields)
		}
	}
}

// A relation pointing at a non-existent target is a validation failure, not a
// client-fault class introduced here — it must keep answering 422 and must NOT
// be mistaken for a uniqueness conflict or an enum violation.
func TestKafe_ClientFaults_BrokenRelationIsStillValidationError(t *testing.T) {
	app := bootKafe(t)
	ids := seedKafeTableScenario(t, app)
	admin := seedKafeAdminToken(t, app)

	status, out := doAuthed(t, app, http.MethodPost,
		"/kafe/_ui/entity/cafe-order/table-session", admin, map[string]any{
			"transaction_date": time.Now().UTC().Format(time.RFC3339),
			"branch_id":        ids.branchID,
			"dining_table_id":  "00000000-0000-0000-0000-000000000000",
			"guest_token":      "FAULT-BAD-RELATION",
		})
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("a broken relation must answer 422, got %d (%v)", status, out)
	}
	errObj, _ := out["error"].(map[string]any)
	if code, _ := errObj["code"].(string); code != "VALIDATION_ERROR" {
		t.Errorf("expected VALIDATION_ERROR, got %q (%v)", code, out)
	}
}
