package db

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// An enum CHECK rejection is a CLIENT-VISIBLE validation failure, not a server
// fault. Measured 2026-10-02 (kafe 10.61): a value outside `enum_values`
// answered **500 INTERNAL_ERROR**, and the response carried the raw driver text
// `insert row: constraint failed: CHECK constraint failed: json_extract(data,
// '$.reason') IN (…) (275)`. `docs/spec/backend/05-field-types.md` §enum
// mandates `VALIDATION_ERROR` (422) for exactly this case.
//
// The field and the allowed set are extracted from the driver text so the
// caller can show something actionable: the driver already named the field,
// the platform was the thing throwing it away.
func TestClassifyConstraintError_EnumCheck(t *testing.T) {
	cases := []struct {
		name       string
		msg        string
		wantField  string
		wantAllowl []string
		enum       bool
	}{
		{
			name: "sqlite enum check names the field and the allowed set",
			msg: "insert row: constraint failed: CHECK constraint failed: " +
				"json_extract(data, '$.reason') IN ('drop_to_safe', 'buy_supplies', 'petty_cash') (275)",
			wantField:  "reason",
			wantAllowl: []string{"drop_to_safe", "buy_supplies", "petty_cash"},
			enum:       true,
		},
		{
			// No extended result code appended — the expression's own
			// parentheses must not be mistaken for one and truncate the set.
			name:       "sqlite enum check without an extended code",
			msg:        "CHECK constraint failed: json_extract(data, '$.status') IN ('open', 'closed')",
			wantField:  "status",
			wantAllowl: []string{"open", "closed"},
			enum:       true,
		},
		{
			name:       "postgres enum check (constraint name only)",
			msg:        `pq: new row for relation "cafe_order_cash_movements" violates check constraint "cafe_order_cash_movements_reason_check"`,
			wantField:  "",
			wantAllowl: nil,
			enum:       true,
		},
		{
			name: "uniqueness is still uniqueness, not an enum violation",
			msg:  "insert row: constraint failed: UNIQUE constraint failed: t._code",
			enum: false,
		},
		{
			name: "a foreign key failure is neither class",
			msg:  "insert row: constraint failed: FOREIGN KEY constraint failed (787)",
			enum: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := classifyConstraintError(errText(tc.msg))
			gotEnum := errors.Is(err, ErrInvalidEnumValue)
			if gotEnum != tc.enum {
				t.Fatalf("errors.Is(ErrInvalidEnumValue) = %v, want %v (err=%v)", gotEnum, tc.enum, err)
			}
			if !tc.enum {
				if errors.Is(err, ErrUniqueViolation) != strings.Contains(tc.msg, "UNIQUE constraint failed") {
					t.Errorf("uniqueness classification changed for %q: %v", tc.msg, err)
				}
				return
			}
			var ev *EnumViolationError
			if !errors.As(err, &ev) {
				t.Fatalf("expected an *EnumViolationError, got %T", err)
			}
			if ev.Field != tc.wantField {
				t.Errorf("Field = %q, want %q", ev.Field, tc.wantField)
			}
			if strings.Join(ev.Allowed, ",") != strings.Join(tc.wantAllowl, ",") {
				t.Errorf("Allowed = %v, want %v", ev.Allowed, tc.wantAllowl)
			}
		})
	}
}

// The enum message must name the field and the allowed values when the driver
// gave them — that is the whole point of classifying rather than re-wrapping.
func TestEnumViolationError_MessageNamesFieldAndAllowed(t *testing.T) {
	err := classifyConstraintError(errText(
		"CHECK constraint failed: json_extract(data, '$.area') IN ('indoor', 'outdoor') (275)"))

	msg := err.Error()
	for _, want := range []string{"area", "indoor", "outdoor"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q must mention %q", msg, want)
		}
	}
	// The driver's own noise must not leak through.
	if strings.Contains(msg, "json_extract") || strings.Contains(msg, "(275)") {
		t.Errorf("message must not carry driver text, got %q", msg)
	}
}

// Re-classifying an already-classified error must be a no-op — the same
// guarantee uniqueness has (TestClassifyConstraintError_Idempotent), because
// the write path can pass an error through more than one classify call.
func TestClassifyConstraintError_EnumIdempotent(t *testing.T) {
	once := classifyConstraintError(errText(
		"CHECK constraint failed: json_extract(data, '$.status') IN ('a', 'b')"))
	twice := classifyConstraintError(once)
	if twice != once {
		t.Errorf("second classification must be a no-op, got a new error: %v", twice)
	}
	var ev *EnumViolationError
	if !errors.As(twice, &ev) || ev.Field != "status" {
		t.Errorf("classification lost on the second pass: %v", twice)
	}
}

// A non-enum message must pass through byte-for-byte: wrapping everything would
// make every storage error look like a validation failure, hiding real faults.
func TestClassifyConstraintError_UnrelatedPassesThroughUnchanged(t *testing.T) {
	const msg = "database is locked"
	if got := classifyConstraintError(errText(msg)).Error(); got != msg {
		t.Errorf("unrelated error must pass through unchanged, got %q", got)
	}
}

// errText builds a plain error carrying driver text, the shape a driver error
// takes by the time classification sees it.
func errText(msg string) error { return fmt.Errorf("%s", msg) }
