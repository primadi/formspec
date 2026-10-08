package starlark

import (
	"fmt"
	"strings"
)

// EvalFormSpecExpr evaluates a FormSpecExpr — the client-behavior vocabulary used
// by `visible_when` / `readonly_when` / `required_when` / `compute` (frontend
// 08-formspec-expr.md) — with the SAME meaning the renderer gives it.
//
// Why this wrapper exists: FormSpecExpr is documented as a subset of Starlark, but
// it is not quite one. It spells its three literals in lowercase — `true`,
// `false`, `null` — while Starlark requires `True`, `False`, `None`. The client
// lexer emits boolean/null tokens for the lowercase forms and every manifest in
// the tree uses them (e.g. kafe's
// `required_when: "fields.manual_discount_amount != null"`).
//
// The consequence was invisible until a server-side caller needed it: the four
// existing evaluation sites (grant conditions, action conditions, subscription
// transforms) call EvalExpr directly, so an expression written the way the
// renderer accepts it fails there with `undefined: true` — a server/client
// divergence in the shared vocabulary.
//
// Normalising here, in one place, is the point: the alternative is each caller
// remembering which spellings Starlark tolerates, and the copy that forgets is the
// one that silently never matches.
//
// `env` follows the same convention as EvalExpr. Callers that model the record
// should bind `fields` as a FieldMap (see NewFieldMap) so both `fields.x` and
// `fields["x"]` resolve, matching what the renderer supports.
func EvalFormSpecExpr(expr string, env map[string]any) (any, error) {
	normalized, err := normalizeFormSpecLiterals(expr)
	if err != nil {
		return nil, err
	}
	return EvalExpr(normalized, env)
}

// normalizeFormSpecLiterals rewrites the lowercase FormSpecExpr literals into
// their Starlark spellings.
//
// The rewrite is token-aware on purpose. A blind string replacement would corrupt
// any identifier that CONTAINS one of the words — `is_nullable`, `truthy`,
// `nullify` — and would rewrite text inside string literals, silently changing a
// comparison against the word itself (`fields.code == "null"`).
func normalizeFormSpecLiterals(expr string) (string, error) {
	var out strings.Builder
	out.Grow(len(expr))
	runes := []rune(expr)

	for i := 0; i < len(runes); {
		ch := runes[i]

		// String literals are opaque: both quote styles are legal in the subset.
		if ch == '"' || ch == '\'' {
			quote := ch
			out.WriteRune(ch)
			i++
			for i < len(runes) {
				if runes[i] == '\\' && i+1 < len(runes) {
					out.WriteRune(runes[i])
					out.WriteRune(runes[i+1])
					i += 2
					continue
				}
				out.WriteRune(runes[i])
				if runes[i] == quote {
					i++
					break
				}
				i++
			}
			continue
		}

		// An identifier is a maximal run of name characters; only a WHOLE word is
		// a literal. A word starting with a digit is a number, not a literal.
		if isNameStart(ch) {
			start := i
			for i < len(runes) && isNameChar(runes[i]) {
				i++
			}
			word := string(runes[start:i])
			switch word {
			case "true":
				out.WriteString("True")
			case "false":
				out.WriteString("False")
			case "null":
				out.WriteString("None")
			default:
				out.WriteString(word)
			}
			continue
		}

		out.WriteRune(ch)
		i++
	}
	return out.String(), nil
}

func isNameStart(ch rune) bool {
	return ch == '_' || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isNameChar(ch rune) bool {
	return isNameStart(ch) || (ch >= '0' && ch <= '9')
}

// NormalizeFormSpecLiterals is exported for callers that need to store or compare
// the normalized form (and for tests that pin the token-awareness).
func NormalizeFormSpecLiterals(expr string) (string, error) {
	return normalizeFormSpecLiterals(expr)
}

// FieldMapEnv binds a record so FormSpecExpr can reach its fields the way the
// renderer does: `fields.name` and `fields["name"]` both work, and a field the
// record does not carry reads as None (so `fields.x != null` is false), matching
// the client's "unknown identifiers default to null".
func FieldMapEnv(name string, record map[string]any) map[string]any {
	return map[string]any{name: NewFieldMap(record)}
}

// EvalFormSpecBool evaluates a FormSpecExpr expected to answer yes/no.
//
// A non-boolean result is refused rather than coerced: `required_when: "fields.x"`
// silently meaning "truthy" would make a typo'd expression look like it works,
// and the failure mode of a gate is that it must not pass by accident.
func EvalFormSpecBool(expr string, env map[string]any) (bool, error) {
	v, err := EvalFormSpecExpr(expr, env)
	if err != nil {
		return false, err
	}
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("expression %q must evaluate to a boolean, got %T (%v)", expr, v, v)
	}
	return b, nil
}
