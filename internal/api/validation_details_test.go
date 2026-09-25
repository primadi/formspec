package api

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/primadi/formspec/internal/validation"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// decodeErrorDetails runs a handler that writes a validation error and returns
// the parsed details array from the response envelope.
func decodeErrorDetails(t *testing.T, errs []error) []ErrorDetailItem {
	t.Helper()
	rec := httptest.NewRecorder()
	writeValidationErrors(rec, errs)

	var body struct {
		Error struct {
			Code    string            `json:"code"`
			Details []ErrorDetailItem `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal error envelope: %v (body=%s)", err, rec.Body.String())
	}
	if body.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("expected VALIDATION_ERROR, got %q", body.Error.Code)
	}
	return body.Error.Details
}

// TestWriteValidationErrors_TypedLevelAndField proves that a typed validation
// error's level + field survive into the envelope's details array (todo 7.9.5)
// instead of the previous hardcoded "error" with no field.
func TestWriteValidationErrors_TypedLevelAndField(t *testing.T) {
	errs := []error{
		&validation.ValidationError{
			Level:   validation.LevelField,
			Field:   "billing.invoice.due_date",
			Message: "billing.invoice.due_date: must be in the future",
		},
		&validation.ValidationError{
			Level:   validation.LevelCrossField,
			Field:   "billing.invoice.end_date",
			Message: `"end_date" must be after "start_date" (2026-01-01)`,
		},
	}

	details := decodeErrorDetails(t, errs)
	if len(details) != 2 {
		t.Fatalf("expected 2 details, got %d", len(details))
	}
	if details[0].Level != "field" || details[0].Field != "billing.invoice.due_date" {
		t.Errorf("detail[0] = %+v, want level=field field=billing.invoice.due_date", details[0])
	}
	if details[1].Level != "cross_field" || details[1].Field != "billing.invoice.end_date" {
		t.Errorf("detail[1] = %+v, want level=cross_field field=billing.invoice.end_date", details[1])
	}
}

// TestWriteValidationErrors_UnstructuredDegradesGracefully guards the fallback:
// a plain error still yields a detail entry (level "error", no field) rather
// than an empty or dropped item.
func TestWriteValidationErrors_UnstructuredDegradesGracefully(t *testing.T) {
	details := decodeErrorDetails(t, []error{errors.New("something plainly wrong")})
	if len(details) != 1 {
		t.Fatalf("expected 1 detail, got %d", len(details))
	}
	if details[0].Level != "error" || details[0].Field != "" {
		t.Errorf("detail = %+v, want level=error field=\"\"", details[0])
	}
	if details[0].Message != "something plainly wrong" {
		t.Errorf("message = %q, want the original error text", details[0].Message)
	}
}

// TestStoreValidationDetail_Levels covers the storage-layer extractor: a typed
// error keeps its level+field, one without a level defaults to "field", and a
// bare ErrValidationRule falls back to level-only.
func TestStoreValidationDetail_Levels(t *testing.T) {
	cases := []struct {
		name      string
		err       error
		wantLevel string
		wantField string
	}{
		{
			name:      "typed with level and field",
			err:       &validation.ValidationError{Level: validation.LevelCrossField, Field: "a.b"},
			wantLevel: "cross_field",
			wantField: "a.b",
		},
		{
			name:      "typed without level defaults to field",
			err:       &validation.ValidationError{Field: "a.c"},
			wantLevel: "field",
			wantField: "a.c",
		},
		{
			name:      "wrapped typed error",
			err:       errors.Join(db.ErrValidationRule, &validation.ValidationError{Level: validation.LevelField, Field: "a.d"}),
			wantLevel: "field",
			wantField: "a.d",
		},
		{
			name:      "bare store error falls back to level only",
			err:       db.ErrValidationRule,
			wantLevel: "field",
			wantField: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			level, field := storeValidationDetail(tc.err)
			if level != tc.wantLevel || field != tc.wantField {
				t.Errorf("got (%q, %q), want (%q, %q)", level, field, tc.wantLevel, tc.wantField)
			}
		})
	}
}

// TestWriteStoreValidationError_HasDetails proves the record-level validation
// path now emits a details array — it previously used plain writeError, so
// callers got only a flat message.
func TestWriteStoreValidationError_HasDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	writeStoreValidationError(rec, db.ErrValidationRule)

	var body struct {
		Error struct {
			Details []ErrorDetailItem `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body.Error.Details) != 1 {
		t.Fatalf("expected a details entry, got %+v", body.Error.Details)
	}
	if body.Error.Details[0].Level != "field" {
		t.Errorf("level = %q, want field", body.Error.Details[0].Level)
	}
}
