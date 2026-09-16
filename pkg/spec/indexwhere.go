package spec

import (
	"fmt"
	"strconv"
	"strings"
)

// IndexDecl.Where grammar (S8, kafe 1.6)
//
// A partial index predicate is written against **field names**:
//
//	indexes:
//	  - fields: [branch_id, cashier_id]
//	    unique: true
//	    where: "status = 'open'"
//
// The grammar is deliberately closed rather than "any SQL":
//
//	predicate := term (AND term)*
//	term      := <field> <op> <literal> | <field> IS NULL | <field> IS NOT NULL
//	op        := = | != | <> | > | >= | < | <=
//	literal   := 'text' | number | true | false
//
// Two reasons it is not a free-form string. First, the text lands in DDL, so an
// unvalidated string is injection through a manifest. Second, a closed grammar
// can name the offending field in its error, which a raw SQL passthrough cannot.
// Anything richer (OR, functions, subqueries) stays the job of `kind: Migration`
// — and needs a portable form there anyway (GAP-35).

// IndexWhereTerm is one comparison in a partial-index predicate.
type IndexWhereTerm struct {
	// Field is the entity field name (translated to the derived column by the
	// persist backend, exactly like IndexDecl.Fields).
	Field string
	// Op is one of =, !=, <>, >, >=, <, <=. Empty when Null is set.
	Op string
	// Value is the literal as written in the manifest (already unquoted for
	// strings): "open", "7", "true".
	Value string
	// Null / NotNull select the IS NULL / IS NOT NULL forms; literal
	// comparisons leave both false.
	Null    bool
	NotNull bool
	// Quoted records that the author wrote the literal as a quoted string
	// (`status = '007'`), so it is rendered as text rather than as the number 7.
	Quoted bool
}

// indexWhereOps is the closed operator set.
var indexWhereOps = map[string]bool{
	"=": true, "!=": true, "<>": true,
	">": true, ">=": true, "<": true, "<=": true,
}

// ParseIndexWhere parses a partial-index predicate.
//
// fieldNames is the set of fields the predicate may reference — the entity's own
// fields. Columns that are not entity fields (e.g. `deleted_at`, `tenant_id`)
// are also accepted, since they exist as real columns; anything that is neither
// a field nor a known system column is rejected so a typo cannot silently
// produce an index that never matches.
func ParseIndexWhere(where string, fieldNames map[string]bool) ([]IndexWhereTerm, error) {
	trimmed := strings.TrimSpace(where)
	if trimmed == "" {
		return nil, nil
	}

	parts := splitTopLevelAnd(trimmed)
	terms := make([]IndexWhereTerm, 0, len(parts))
	for _, part := range parts {
		term, err := parseIndexWhereTerm(part, fieldNames)
		if err != nil {
			return nil, err
		}
		terms = append(terms, term)
	}
	return terms, nil
}

// splitTopLevelAnd splits on the keyword AND outside string literals.
func splitTopLevelAnd(s string) []string {
	var parts []string
	var current strings.Builder
	inString := false

	upper := strings.ToUpper(s)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\'' {
			// '' inside a literal is an escaped quote, not a terminator.
			if inString && i+1 < len(s) && s[i+1] == '\'' {
				current.WriteString("''")
				i++
				continue
			}
			inString = !inString
			current.WriteByte(c)
			continue
		}
		if !inString && i+4 <= len(upper) && upper[i:i+4] == " AND" && isBoundary(s, i+4) {
			parts = append(parts, strings.TrimSpace(current.String()))
			current.Reset()
			i += 3
			continue
		}
		current.WriteByte(c)
	}
	parts = append(parts, strings.TrimSpace(current.String()))
	return parts
}

// isBoundary reports whether s[i] starts a new token (end of string, space, or
// an operator/paren).
func isBoundary(s string, i int) bool {
	if i >= len(s) {
		return true
	}
	return s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '(' || s[i] == ')'
}

// parseIndexWhereTerm parses one comparison.
func parseIndexWhereTerm(term string, fieldNames map[string]bool) (IndexWhereTerm, error) {
	upper := strings.ToUpper(term)

	// IS NULL / IS NOT NULL — no operator precedence to worry about.
	if idx := strings.Index(upper, " IS NOT NULL"); idx > 0 && idx+len(" IS NOT NULL") == len(term) {
		field := strings.TrimSpace(term[:idx])
		if err := checkWhereField(field, fieldNames); err != nil {
			return IndexWhereTerm{}, err
		}
		return IndexWhereTerm{Field: field, NotNull: true}, nil
	}
	if idx := strings.Index(upper, " IS NULL"); idx > 0 && idx+len(" IS NULL") == len(term) {
		field := strings.TrimSpace(term[:idx])
		if err := checkWhereField(field, fieldNames); err != nil {
			return IndexWhereTerm{}, err
		}
		return IndexWhereTerm{Field: field, Null: true}, nil
	}

	// <field> <op> <literal> — longest operator first so ">=" is not read as ">".
	for _, op := range []string{"!=", "<>", ">=", "<=", "=", ">", "<"} {
		idx := strings.Index(term, op)
		if idx <= 0 {
			continue
		}
		field := strings.TrimSpace(term[:idx])
		raw := strings.TrimSpace(term[idx+len(op):])
		if err := checkWhereField(field, fieldNames); err != nil {
			return IndexWhereTerm{}, err
		}
		value, quoted, err := parseWhereLiteral(raw)
		if err != nil {
			return IndexWhereTerm{}, fmt.Errorf("index where %q: %w", term, err)
		}
		if !indexWhereOps[op] {
			return IndexWhereTerm{}, fmt.Errorf("index where %q: operator %q is not supported", term, op)
		}
		return IndexWhereTerm{Field: field, Op: op, Value: value, Quoted: quoted}, nil
	}

	return IndexWhereTerm{}, fmt.Errorf("unsupported index where predicate %q — supported forms: \"<field> <op> <literal>\", \"<field> IS NULL\", \"<field> IS NOT NULL\", joined by AND (put free-form SQL in a kind: Migration instead)", term)
}

// checkWhereField rejects unknown identifiers so a typo cannot produce an index
// that silently never applies.
func checkWhereField(field string, fieldNames map[string]bool) error {
	if field == "" {
		return fmt.Errorf("index where: missing field name")
	}
	if !isIdentifier(field) {
		return fmt.Errorf("index where: %q is not a valid field name", field)
	}
	if fieldNames[field] || systemColumns[field] {
		return nil
	}
	return fmt.Errorf("index where: unknown field %q — the predicate may reference entity fields or system columns", field)
}

// systemColumns are real columns on every entity table that a predicate may
// legitimately reference without being a declared field.
var systemColumns = map[string]bool{
	"deleted_at":       true,
	"doc_status":       true,
	"is_active":        true,
	"id":               true,
	"tenant_id":        true,
	"version":          true,
	"created_at":       true,
	"created_by":       true,
	"updated_at":       true,
	"updated_by":       true,
	"transaction_date": true,
}

// isIdentifier reports whether s looks like a column identifier.
func isIdentifier(s string) bool {
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_':
		case r >= '0' && r <= '9':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return len(s) > 0
}

// parseWhereLiteral validates a literal, returning its manifest form and whether
// the author wrote it as a quoted string.
//
// The scan for a quoted literal is strict: the closing quote must be the last
// character. Accepting "first quote … last quote" instead would swallow a
// trailing ` OR …` into the literal, turning `status = 'open' OR 1=1` into a
// comparison against the string "open' OR 1=1" — silently not the predicate the
// author wrote, which is exactly how an unenforced rule looks enforced.
func parseWhereLiteral(raw string) (value string, quoted bool, err error) {
	if raw == "" {
		return "", false, fmt.Errorf("missing comparison value")
	}

	if raw[0] == '\'' {
		for i := 1; i < len(raw); i++ {
			if raw[i] != '\'' {
				continue
			}
			if i+1 < len(raw) && raw[i+1] == '\'' {
				i++ // doubled quote — an escaped quote inside the literal
				continue
			}
			if i != len(raw)-1 {
				return "", false, fmt.Errorf("unexpected text after the string literal in %s", raw)
			}
			return raw[1:i], true, nil
		}
		return "", false, fmt.Errorf("unterminated string literal %s", raw)
	}

	switch strings.ToLower(raw) {
	case "true", "false", "null":
		return strings.ToLower(raw), false, nil
	}
	if _, err := strconv.ParseFloat(raw, 64); err == nil {
		return raw, false, nil
	}
	// A bare word is the enum-like value the author means: `status = open` reads
	// naturally after seeing `enum_values: [open, closed]`. It is quoted when
	// rendered, so it can never become an identifier.
	if isBareLiteral(raw) {
		return raw, false, nil
	}
	return "", false, fmt.Errorf("unsupported literal %s — use a quoted string, a number, true/false, or IS NULL", raw)
}

// isBareLiteral reports whether s is a single unquoted word that can be treated
// as a string value. Anything containing syntax (`=`, `(`, quotes, whitespace)
// is not a value — it is a fragment of a predicate the grammar does not support.
func isBareLiteral(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '_', r == '-', r == '.', r == ':', r == '/':
		default:
			return false
		}
	}
	return true
}

// RenderIndexWhereTerms renders parsed terms to a predicate over the given
// column mapping. Used by persist backends that store fields in JSONB and
// therefore index derived columns rather than the field names.
//
// Literal rendering lives here, next to the grammar, so the two can never
// disagree about what a literal means. columnOf is supplied by the persist
// backend: it maps a field name to its physical column, and names with no
// mapping (system columns) are emitted as-is.
func RenderIndexWhereTerms(terms []IndexWhereTerm, columnOf func(string) string) string {
	rendered := make([]string, 0, len(terms))
	for _, t := range terms {
		col := columnOf(t.Field)
		switch {
		case t.Null:
			rendered = append(rendered, col+" IS NULL")
		case t.NotNull:
			rendered = append(rendered, col+" IS NOT NULL")
		default:
			rendered = append(rendered, col+" "+t.Op+" "+renderWhereLiteral(t))
		}
	}
	return strings.Join(rendered, " AND ")
}

// renderWhereLiteral emits a SQL literal for one comparison term. SQLite and
// PostgreSQL escape single quotes identically (`”`).
func renderWhereLiteral(t IndexWhereTerm) string {
	if t.Quoted {
		return "'" + strings.ReplaceAll(t.Value, "'", "''") + "'"
	}
	switch strings.ToLower(t.Value) {
	case "true", "false", "null":
		return strings.ToLower(t.Value)
	}
	if _, err := strconv.ParseFloat(t.Value, 64); err == nil {
		return t.Value
	}
	if strings.HasPrefix(t.Value, "{") || strings.HasPrefix(t.Value, "[") {
		return t.Value
	}
	return "'" + strings.ReplaceAll(t.Value, "'", "''") + "'"
}

// IndexWhereFieldNames collects the set of field names a predicate may
// reference from an entity spec.
func IndexWhereFieldNames(d *EntitySpec) map[string]bool {
	names := make(map[string]bool, len(d.Fields))
	for _, f := range d.Fields {
		names[f.Name] = true
	}
	return names
}

// validateIndexDecls validates the `where` predicate of every index at one
// declaration site. label names the site ("indexes", "persist.indexes") so the
// error points at the right place in the manifest.
func validateIndexDecls(label string, indexes []IndexDecl, fieldNames map[string]bool) error {
	for i, idx := range indexes {
		if strings.TrimSpace(idx.Where) == "" {
			continue
		}
		if len(idx.Fields) == 0 {
			return fmt.Errorf("%s[%d]: where is only meaningful on an index with fields", label, i)
		}
		if _, err := ParseIndexWhere(idx.Where, fieldNames); err != nil {
			return fmt.Errorf("%s[%d]: %w", label, i, err)
		}
	}
	return nil
}
