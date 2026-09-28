package db

import (
	"fmt"
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

// classifyConstraintError returns a UniqueViolationError when err is a
// uniqueness violation, and err unchanged otherwise (including nil).
//
// It is idempotent: an already-classified error carries a message that no
// longer matches the driver patterns, so a second pass is a no-op.
func classifyConstraintError(err error) error {
	if err == nil {
		return nil
	}
	if detail, ok := uniqueViolationDetail(err.Error()); ok {
		return &UniqueViolationError{Detail: detail, Err: err}
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
