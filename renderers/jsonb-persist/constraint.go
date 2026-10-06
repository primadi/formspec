package db

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ─── Constraint-violation classification ───
//
// A uniqueness violation is a CLIENT-VISIBLE CONFLICT, not a server fault: the
// value the caller sent is well-formed, it simply already exists. Before this
// file existed the platform had no way to say that — the raw driver error
// bubbled up, `internal/api`'s isConflictError only recognized "version
// conflict"/"not found", and `writeStoreError` fell through to
// **500 INTERNAL_ERROR**.
//
// Measured 2026-09-28 against the kafe app (all three shapes answered 500
// INTERNAL_ERROR):
//
//	second OPEN table-session on one table  (partial unique index, kafe 10.34c)
//	duplicate dining-table (branch_id, code) (composite unique index)
//	duplicate qr_token                       (field `unique: true` + natural_key)
//
// The cost was not cosmetic. 500 is the class that pages an operator, so every
// duplicate value in production raised a false alarm AND buried real faults
// among them; and a client could not tell "send a different value" from "retry
// later", so it had no message it could show.
//
// Classification lives HERE, at the storage boundary, rather than in the HTTP
// layer, so every writer gets the same error class — the API, a Starlark
// handler, a seed, an operator script — instead of only the HTTP path.
//
// ─── Second class: enum CHECK (added 2026-10-02, kafe 10.61) ───
//
// A value outside a field's declared `enum_values` is also a CLIENT-VISIBLE
// rejection, and it was answering 500 for the same reason (the raw text
// `CHECK constraint failed: json_extract(data, '$.reason') IN (…)` reached the
// client verbatim). `docs/spec/backend/05-field-types.md` §enum already
// mandates `VALIDATION_ERROR` for it, so the platform was violating a written
// contract, not merely being unhelpful. Classified here by the same argument:
// whoever writes the row gets the same class.

// ErrUniqueViolation is the sentinel for a uniqueness-constraint violation.
// Callers match it with errors.Is; the concrete error also carries the
// constraint detail (see UniqueViolationError).
var ErrUniqueViolation = fmt.Errorf("unique constraint violated")

// UniqueViolationError reports WHICH uniqueness constraint was violated.
//
// The detail matters: the raw driver text names physical columns
// (`cafe_order_table_sessions._dining_table_id`), which a caller handling the
// error should not have to parse, and a user should never see. Detail is
// normalized to the logical field names (`dining_table_id`).
type UniqueViolationError struct {
	// Detail is the normalized constraint detail, e.g. "dining_table_id" or
	// "branch_id, code", or the PostgreSQL constraint name when that is all
	// the driver provides.
	Detail string
	// Err is the original driver error, kept for logs.
	Err error
}

func (e *UniqueViolationError) Error() string {
	if e.Detail != "" {
		return "unique constraint violated: " + e.Detail
	}
	if e.Err != nil {
		return "unique constraint violated: " + e.Err.Error()
	}
	return "unique constraint violated"
}

// Unwrap makes errors.Is(err, ErrUniqueViolation) work.
func (e *UniqueViolationError) Unwrap() error { return ErrUniqueViolation }

// classifyConstraintError returns a UniqueViolationError or an
// EnumViolationError when err is one of those constraint violations, and err
// unchanged otherwise (including nil).
//
// It is idempotent: an already-classified error carries a message that no
// longer matches the driver patterns, so a second pass is a no-op.
func classifyConstraintError(err error) error {
	if err == nil {
		return nil
	}
	// Already classified — return as-is rather than nesting a second wrapper.
	// The message no longer matches the driver patterns, so the guards below
	// would be no-ops anyway; being explicit keeps the guarantee local.
	if errors.Is(err, ErrUniqueViolation) || errors.Is(err, ErrInvalidEnumValue) {
		return err
	}
	if detail, ok := uniqueViolationDetail(err.Error()); ok {
		return &UniqueViolationError{Detail: detail, Err: err}
	}
	if ev, ok := enumViolationFrom(err.Error()); ok {
		return ev
	}
	return err
}

// uniqueViolationDetail recognizes the uniqueness-violation text of the drivers
// this package supports and extracts the violated constraint.
//
// Text matching rather than a typed check, because database/sql exposes no
// portable constraint code and the two drivers differ:
//
//	SQLite   "UNIQUE constraint failed: <table>.<col>[, <table>.<col>...]"
//	         (the driver may append " (2067)", the extended result code)
//	Postgres "duplicate key value violates unique constraint \"<name>\""
//	         with "DETAIL: Key (<cols>)=(<values>) already exists."
//
// This mirrors the existing convention in this package (counter.go matches
// "UNIQUE"/"PRIMARY KEY" the same way). The difference is that the outcome here
// is a first-class error class rather than a branch inside one caller.
func uniqueViolationDetail(msg string) (string, bool) {
	// ── SQLite ──
	if i := strings.Index(msg, "UNIQUE constraint failed:"); i >= 0 {
		detail := strings.TrimSpace(msg[i+len("UNIQUE constraint failed:"):])
		// Drop the driver's extended-code suffix, e.g. " (2067)".
		if j := strings.LastIndex(detail, " ("); j > 0 {
			detail = detail[:j]
		}
		return normalizeConstraintDetail(detail), true
	}

	// ── PostgreSQL (pgx) ──
	if strings.Contains(msg, "duplicate key value violates unique constraint") {
		// Prefer the logical columns from DETAIL when present, since the
		// constraint NAME (e.g. "cafe_tables_branch_id_code_key") is a
		// physical artifact the caller cannot use directly.
		if detail := pgDetailColumns(msg); detail != "" {
			return detail, true
		}
		if name := quotedAfter(msg, "unique constraint "); name != "" {
			return name, true
		}
		return "", true
	}
	// A pgx error whose text was rewritten upstream but kept its SQLSTATE.
	if strings.Contains(msg, "23505") {
		return "", true
	}

	return "", false
}

// pgDetailColumns extracts the column list from a PostgreSQL
// `DETAIL: Key (a, b)=(...) already exists.` line and normalizes it.
func pgDetailColumns(msg string) string {
	i := strings.Index(msg, "Key (")
	if i < 0 {
		return ""
	}
	rest := msg[i+len("Key ("):]
	j := strings.Index(rest, ")")
	if j < 0 {
		return ""
	}
	return normalizeConstraintDetail(rest[:j])
}

// quotedAfter returns the first double-quoted token following marker.
func quotedAfter(msg, marker string) string {
	i := strings.Index(msg, marker)
	if i < 0 {
		return ""
	}
	rest := msg[i+len(marker):]
	start := strings.Index(rest, "\"")
	if start < 0 {
		return ""
	}
	rest = rest[start+1:]
	end := strings.Index(rest, "\"")
	if end < 0 {
		return ""
	}
	return rest[:end]
}

// normalizeConstraintDetail turns physical constraint text into logical field
// names: "cafe_order_table_sessions._dining_table_id" → "dining_table_id", and
// "cafe_tables.branch_id, cafe_tables.code" → "branch_id, code".
//
// The table prefix is dropped because the entity is already known to the caller
// (it is the entity being written), and the leading underscore is dropped
// because it is this package's own generated-column marker, not part of the
// field's name.
//
// Framework-owned scope columns (tenant_id) are dropped too: every uniqueness
// constraint is tenant-scoped, so it appears in nearly every composite index
// while being impossible for a caller to act on. Reporting
// "a record with this value already exists: tenant_id, qr_token" would point at
// a field the user never supplied.
func normalizeConstraintDetail(detail string) string {
	parts := strings.Split(detail, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// Drop "table." prefix when present.
		if dot := strings.LastIndex(p, "."); dot >= 0 {
			p = p[dot+1:]
		}
		// Drop the generated-column underscore marker.
		p = strings.TrimPrefix(p, "_")
		if frameworkScopeColumns[p] {
			continue
		}
		out = append(out, p)
	}
	return strings.Join(out, ", ")
}

// frameworkScopeColumns are storage columns the framework owns rather than the
// entity declaring: they are part of the physical constraint but are never
// something a caller sent or can change.
var frameworkScopeColumns = map[string]bool{
	"tenant_id": true,
}

// ─── Enum CHECK violation ───
//
// The second client-visible class this file names: a value outside a field's
// declared `enum_values`. `GenerateDDL` emits exactly one CHECK per enum field
// (`CHECK (json_extract(data, '$.status') IN (…))` on SQLite,
// `CHECK ((data->>'status') IN (…))` on PostgreSQL), and nothing else in the
// generated schema emits a CHECK — so a CHECK rejection on these tables IS an
// enum rejection; there is no other constraint to confuse it with.
//
// Measured 2026-10-02 (kafe 10.61): a bad enum answered **500 INTERNAL_ERROR**
// carrying the raw driver text
// (`insert row: constraint failed: CHECK constraint failed: json_extract(data,
// '$.reason') IN (…) (275)`), while `docs/spec/backend/05-field-types.md` §enum
// mandates `VALIDATION_ERROR` (422) — "nilai di luar himpunan →
// `VALIDATION_ERROR`". The CHECK text also named the field, so the client could
// have shown something actionable and instead showed "Internal server error".

// ErrInvalidEnumValue is the sentinel for a value rejected by the generated
// enum CHECK constraint. Callers match it with errors.Is.
var ErrInvalidEnumValue = fmt.Errorf("value outside declared enum")

// EnumViolationError reports which enum field was rejected and which values the
// manifest allows, so a caller can show a message instead of a driver string.
//
// Field is empty when the driver text does not carry it — PostgreSQL reports
// only the constraint NAME (`violates check constraint "…"`), not the
// expression, so the rejection is still classified (422, not 500) but cannot
// name the field. That is a limit of the driver, not of the classification.
// Allowed is empty for the same reason.
type EnumViolationError struct {
	// Field is the logical field name, e.g. "reason".
	Field string
	// Allowed is the declared `enum_values` set, in manifest order.
	Allowed []string
	// Err is the original driver error, kept for logs.
	Err error
}

func (e *EnumViolationError) Error() string {
	if e.Field == "" {
		return "value outside the declared enum"
	}
	if len(e.Allowed) == 0 {
		return fmt.Sprintf("field %q has a value outside the declared enum", e.Field)
	}
	return fmt.Sprintf("field %q must be one of: %s", e.Field, strings.Join(e.Allowed, ", "))
}

// Unwrap makes errors.Is(err, ErrInvalidEnumValue) work.
func (e *EnumViolationError) Unwrap() error { return ErrInvalidEnumValue }

// enumViolationFrom recognizes an enum CHECK rejection and extracts the field
// and allowed set when the driver text carries them.
//
//	SQLite   "CHECK constraint failed: json_extract(data, '$.status') IN ('a', 'b') (275)"
//	Postgres `new row for relation "…" violates check constraint "…"` (no expression)
func enumViolationFrom(msg string) (*EnumViolationError, bool) {
	if i := strings.Index(msg, "CHECK constraint failed:"); i >= 0 {
		expr := strings.TrimSpace(msg[i+len("CHECK constraint failed:"):])
		// Drop the driver's extended result code, e.g. " (275)".
		expr = enumExtendedCodeSuffix.ReplaceAllString(expr, "")
		field, allowed := parseEnumCheckExpression(expr)
		return &EnumViolationError{Field: field, Allowed: allowed}, true
	}
	if strings.Contains(msg, "violates check constraint") {
		return &EnumViolationError{}, true
	}
	return nil, false
}

// enumExtendedCodeSuffix matches SQLite's trailing extended result code, e.g.
// " (275)". Only a NUMERIC parenthesized suffix is stripped: the expression
// itself contains parentheses ("IN ('a', 'b')"), so a plain LastIndex(" (")
// would truncate the allowed values when no code is appended.
var enumExtendedCodeSuffix = regexp.MustCompile(`\s*\(\d+\)\s*$`)

// jsonExtractPath matches the SQLite payload expression's field path,
// `json_extract(data, '$.status')` → "status".
var jsonExtractPath = regexp.MustCompile(`\$\.([A-Za-z0-9_]+)`)

// arrowPath matches the PostgreSQL payload expression's field name,
// `(data->>'status')` → "status".
var arrowPath = regexp.MustCompile(`->>\s*'([A-Za-z0-9_]+)'`)

// parseEnumCheckExpression splits "<payload-expr> IN ('a', 'b')" into the field
// name and the allowed set. The split is on the LAST " IN (" so a field whose
// name contains " in " cannot shift the boundary.
//
// Values are unquoted by trimming single quotes; a value containing a comma or
// an escaped quote would be split wrongly, which is acceptable because the set
// is used for a human-facing message only — the CHECK constraint itself is what
// enforces the rule.
func parseEnumCheckExpression(expr string) (field string, allowed []string) {
	i := strings.LastIndex(expr, " IN (")
	if i < 0 {
		return "", nil
	}
	lhs := strings.TrimSpace(expr[:i])
	rhs := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(expr[i+len(" IN ("):]), ")"))

	if m := jsonExtractPath.FindStringSubmatch(lhs); m != nil {
		field = m[1]
	} else if m := arrowPath.FindStringSubmatch(lhs); m != nil {
		field = m[1]
	}

	for _, part := range strings.Split(rhs, ",") {
		if v := strings.Trim(strings.TrimSpace(part), "'"); v != "" {
			allowed = append(allowed, v)
		}
	}
	return field, allowed
}
