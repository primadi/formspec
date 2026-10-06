package auth

import (
	"strings"
	"testing"
)

// `grants` is free JSON on the role entity, so nothing in the pipeline validates
// its SHAPE — not `formspec validate` (no schema), not the typed read (which
// drops unknown keys silently). These tests pin the one reader that does, and
// they are the negative control for kafe 10.67: renaming `row_scope` to
// `row_scopes` must be REPORTED, not absorbed.
func TestValidateGrantListShape_RowScopesTypoIsFailOpenAndReported(t *testing.T) {
	raw := []any{
		map[string]any{
			"page": "order-page",
			"actions": []any{
				map[string]any{
					"name": "list",
					// The exact typo measured on kafe: unreadable, yet the
					// permission still resolves — so the restriction is gone and
					// the role reads every row.
					"row_scopes": []any{map[string]any{"field": "status", "op": "in", "value": "paid"}},
				},
			},
		},
	}

	problems := ValidateGrantListShape(raw)
	if len(problems) != 1 {
		t.Fatalf("problems = %#v, want exactly the row_scopes key", problems)
	}
	if !problems[0].RowScope {
		t.Fatal("a misspelled row_scope must be flagged as a row-scope defect — that is what makes the runtime deny instead of reading unscoped")
	}
	if !strings.Contains(problems[0].Message, "row_scope") {
		t.Errorf("message must name the key to write instead: %q", problems[0].Message)
	}
}

func TestValidateGrantListShape_AcceptsReadableGrants(t *testing.T) {
	raw := []any{
		map[string]any{
			"page": "order-page",
			"actions": []any{
				map[string]any{"name": "list", "row_scope": []any{
					map[string]any{"field": "status", "op": "in", "value": "paid,in_kitchen"},
					map[string]any{"field": "branch_id", "op": "eq", "from": "session"},
				}},
				map[string]any{"name": "view"},
			},
		},
		map[string]any{
			"page": "sales",
			"tabs": []any{
				map[string]any{"tab": "Order", "actions": []any{map[string]any{"name": "list"}}},
			},
		},
	}
	if problems := ValidateGrantListShape(raw); len(problems) != 0 {
		t.Fatalf("readable grants rejected: %#v", problems)
	}
}

// A row scope whose entry declares no value source filters NOTHING if it were
// accepted — the same fail-open class as the typo, one level deeper.
func TestValidateGrantListShape_RowScopeWithoutValueSource(t *testing.T) {
	raw := []any{
		map[string]any{
			"page": "order-page",
			"actions": []any{
				map[string]any{"name": "list", "row_scope": []any{
					map[string]any{"field": "status", "op": "in"},
				}},
			},
		},
	}
	problems := ValidateGrantListShape(raw)
	if len(problems) != 1 || !problems[0].RowScope {
		t.Fatalf("problems = %#v, want one row-scope defect", problems)
	}
	if !strings.Contains(problems[0].Message, "value source") {
		t.Errorf("message must point at the missing value source: %q", problems[0].Message)
	}
}

// An unknown operator is the third fail-open shape: the query builder produces
// no clause for it, so the filter silently disappears.
func TestValidateGrantListShape_UnknownOperatorIsRefused(t *testing.T) {
	raw := []any{
		map[string]any{
			"page": "order-page",
			"actions": []any{
				map[string]any{"name": "list", "row_scope": []any{
					map[string]any{"field": "status", "op": "sounds_like", "value": "paid"},
				}},
			},
		},
	}
	problems := ValidateGrantListShape(raw)
	if len(problems) != 1 || !problems[0].RowScope {
		t.Fatalf("problems = %#v, want one row-scope defect", problems)
	}
	if !strings.Contains(problems[0].Message, "unknown operator") {
		t.Errorf("message must name the operator problem: %q", problems[0].Message)
	}
}

// `from` plus a literal `value` is refused: the manifest would not say which one
// wins, and the losing half is a restriction the operator believes is in force.
func TestValidateGrantListShape_FromAndValueAreMutuallyExclusive(t *testing.T) {
	raw := []any{
		map[string]any{
			"page": "order-page",
			"actions": []any{
				map[string]any{"name": "list", "row_scope": []any{
					map[string]any{"field": "branch_id", "op": "eq", "from": "session", "value": "KFE-JKT-01"},
				}},
			},
		},
	}
	problems := ValidateGrantListShape(raw)
	if len(problems) != 1 || !problems[0].RowScope {
		t.Fatalf("problems = %#v, want one row-scope defect", problems)
	}
}

// A valueless operator (`notnull`) carries no value BY DEFINITION — the
// "exactly one value source" rule must not turn it into a false positive.
func TestValidateGrantListShape_ValuelessOperatorNeedsNoValue(t *testing.T) {
	raw := []any{
		map[string]any{
			"page": "order-page",
			"actions": []any{
				map[string]any{"name": "list", "row_scope": []any{
					map[string]any{"field": "voided_at", "op": "null"},
				}},
			},
		},
	}
	if problems := ValidateGrantListShape(raw); len(problems) != 0 {
		t.Fatalf("valueless operator rejected: %#v", problems)
	}
}

// Defects that only cost a grant (a misspelled page, a nameless action) are NOT
// row-scope defects: they fail closed on their own, so they must not turn an
// unrelated permission into a denial.
func TestValidateGrantListShape_GrantOnlyDefectsAreNotRowScope(t *testing.T) {
	raw := []any{
		map[string]any{"actions": []any{map[string]any{"name": "list"}}},
		map[string]any{"page": "order-page", "actions": []any{map[string]any{"row_scope": []any{}}}},
	}
	problems := ValidateGrantListShape(raw)
	if len(problems) == 0 {
		t.Fatal("expected the missing page and the nameless action to be reported")
	}
	for _, p := range problems {
		if p.RowScope {
			t.Errorf("%s was flagged as a row-scope defect: %s", p.Path, p.Message)
		}
	}
}

func TestValidateGrantListShape_NonListValueIsRejected(t *testing.T) {
	problems := ValidateGrantListShape(map[string]any{"page": "order-page"})
	if len(problems) != 1 || problems[0].RowScope {
		t.Fatalf("problems = %#v, want a single shape defect (not row-scope)", problems)
	}
}
