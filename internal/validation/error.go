package validation

import "fmt"

// ValidationError is a structured validation failure. It carries the pieces
// the normative error envelope needs — `details: [{level, field?, message}]`
// (01-core-basic.md §8.5, 02-core-extended.md §14) — instead of only a flat
// string.
//
// `Level` names the validation level that failed, using the vocabulary
// introduced by 02-core-extended.md §14 (the rule-level names were later
// harmonised with it, so a client can filter by level without re-parsing the
// message):
//
//	field          L1–L3 — field presence/type/rules (05-field-types.md §3)
//	cross_field    L3 — after/before/exists (same-document field references)
//	business_rules L4 — single-record business constraint via script
//	cross_validate L5 — multi-field / child-record constraint
//	consistency    L6 — cross-entity consistency
//
// `Field` is the namespaced path of the offending field (e.g. "invoice.due_date",
// "invoice.items.quantity") when the failure is attributable to one field; it is
// empty for record-level failures with no single owner.
type ValidationError struct {
	Level   string
	Field   string
	Message string
	// Cause preserves an underlying error (e.g. a lookup failure) so
	// errors.Is/As still reach it; it is not rendered separately — Message
	// already includes its text.
	Cause error
}

// Validation levels. Only the first two are produced today — L4–L6 are still
// contract-only (todo 7.9.1–7.9.4), but the vocabulary is defined here so the
// envelope stays stable when they land.
const (
	LevelField         = "field"
	LevelCrossField    = "cross_field"
	LevelBusinessRules = "business_rules"
	LevelCrossValidate = "cross_validate"
	LevelConsistency   = "consistency"
)

// Error implements error.
func (e *ValidationError) Error() string { return e.Message }

// Unwrap exposes the underlying cause, if any.
func (e *ValidationError) Unwrap() error { return e.Cause }

// fieldError builds a level-"field" error. field is the bare field name; the
// caller supplies any namespace prefix through msg (existing call sites
// already embed the qualified name in their format strings).
func fieldError(field, format string, args ...any) *ValidationError {
	return &ValidationError{
		Level:   LevelField,
		Field:   field,
		Message: fmt.Sprintf(format, args...),
	}
}

// crossFieldError builds a level-"cross_field" error for a same-document rule
// (after/before/exists) that spans two fields.
func crossFieldError(field, format string, args ...any) *ValidationError {
	return &ValidationError{
		Level:   LevelCrossField,
		Field:   field,
		Message: fmt.Sprintf(format, args...),
	}
}
