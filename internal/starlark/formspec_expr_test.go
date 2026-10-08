package starlark

import "testing"

// FormSpecExpr is documented as a subset of Starlark, but it spells its three
// literals in lowercase (`true`, `false`, `null`) while Starlark requires
// `True`/`False`/`None`. Every manifest in the tree uses the lowercase forms, so
// the four existing server-side evaluation sites (grant conditions, action
// conditions, subscription transforms) could not evaluate a renderer-legal
// expression — a divergence in a SHARED vocabulary.
//
// These tests pin the normalizer: the literals map, and nothing else changes.

func TestEvalFormSpecExpr_LowercaseLiterals(t *testing.T) {
	env := FieldMapEnv("fields", map[string]any{"amount": 10.0, "note": nil})

	cases := []struct {
		expr string
		want bool
	}{
		// The kafe declaration, verbatim: an absent field reads as null.
		{"fields.note != null", false},
		{"fields.amount != null", true},
		// Lowercase booleans are the other two thirds of the divergence.
		{"true", true},
		{"false", false},
		{"fields.amount > 5 and true", true},
		{"fields.amount > 100 or false", false},
		{"not false", true},
	}
	for _, c := range cases {
		got, err := EvalFormSpecBool(c.expr, env)
		if err != nil {
			t.Errorf("%q: %v", c.expr, err)
			continue
		}
		if got != c.want {
			t.Errorf("%q = %v, want %v", c.expr, got, c.want)
		}
	}
}

// The rewrite must be TOKEN-aware. A blind replacement corrupts identifiers that
// merely contain the words, and rewrites text inside string literals — silently
// changing what a comparison means.
func TestNormalizeFormSpecLiterals_TokenAware(t *testing.T) {
	cases := []struct{ in, want string }{
		// Identifiers containing a literal word must survive untouched.
		{"fields.is_nullable", "fields.is_nullable"},
		{"nullify(fields.x)", "nullify(fields.x)"},
		{"fields.truthy", "fields.truthy"},
		{"fields.nullable == true", "fields.nullable == True"},
		// String literals are opaque, in both quote styles.
		{`fields.code == "null"`, `fields.code == "null"`},
		{`fields.code == 'true'`, `fields.code == 'true'`},
		{`fields.code == "null" and fields.x == null`, `fields.code == "null" and fields.x == None`},
		// Escaped quotes do not end the literal early.
		{`fields.msg == "say \"null\"" or fields.x == null`, `fields.msg == "say \"null\"" or fields.x == None`},
		// Prose, spacing, and operators are preserved.
		{"fields.a != null and (fields.b == false)", "fields.a != None and (fields.b == False)"},
	}
	for _, c := range cases {
		got, err := NormalizeFormSpecLiterals(c.in)
		if err != nil {
			t.Fatalf("%q: %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// A gate must not pass by accident: an expression that does not yield a boolean is
// an error, so a typo'd `required_when` is loud rather than silently true.
func TestEvalFormSpecBool_RefusesNonBoolean(t *testing.T) {
	env := FieldMapEnv("fields", map[string]any{"amount": 10.0})

	if _, err := EvalFormSpecBool("fields.amount", env); err == nil {
		t.Error("a non-boolean result must be refused, not treated as truthy")
	}
	if _, err := EvalFormSpecBool("fields.amount >", env); err == nil {
		t.Error("an unparsable expression must be an error")
	}
	// A field the record does not carry reads as None — matching the client, where
	// an unknown identifier defaults to null rather than raising.
	if got, err := EvalFormSpecBool("fields.missing == null", env); err != nil || !got {
		t.Errorf("an absent field must compare equal to null (got %v, err %v)", got, err)
	}
}
