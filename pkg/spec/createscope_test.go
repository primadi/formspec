package spec

import "testing"

// create_scope is the write-side counterpart of `row_scope` (plan
// docs_internal/plan/public-scope-enforcement.md). Validation matters more here
// than for a read filter: an entry that names an unknown field, or a `via` that
// cannot be resolved, would pin NOTHING — and a create that looks guarded while
// accepting any dimension value is worse than an obviously unguarded one.

func createScopeSpec(mutate func(*EntitySpec)) *EntitySpec {
	e := &EntitySpec{
		Version: "v1",
		Fields: []Field{
			{Name: "branch_id", Type: FieldRelation},
			{Name: "table_session_id", Type: FieldRelation},
		},
	}
	if mutate != nil {
		mutate(e)
	}
	return e
}

func TestValidateEntitySpec_CreateScope(t *testing.T) {
	good := createScopeSpec(func(e *EntitySpec) {
		e.CreateScope = []CreateScopeSpec{{
			Field: "branch_id", From: "record",
			RefField: "table_session_id",
			Via:      "cafe-order.table-session", ViaField: "branch_id",
		}}
	})
	if err := ValidateEntitySpec(good); err != nil {
		t.Fatalf("a well-formed create_scope must validate, got %v", err)
	}

	// `from` is optional and defaults to `record` — the terse form a manifest
	// will normally use.
	terse := createScopeSpec(func(e *EntitySpec) {
		e.CreateScope = []CreateScopeSpec{{
			Field: "branch_id", RefField: "table_session_id",
			Via: "cafe-order.table-session", ViaField: "branch_id",
		}}
	})
	if err := ValidateEntitySpec(terse); err != nil {
		t.Fatalf("from may be omitted (it means record), got %v", err)
	}

	bad := []struct {
		name string
		cs   CreateScopeSpec
	}{
		{"no field", CreateScopeSpec{RefField: "table_session_id", Via: "a.b", ViaField: "branch_id"}},
		{"unknown field", CreateScopeSpec{Field: "outlet_id", RefField: "table_session_id", Via: "a.b", ViaField: "branch_id"}},
		{"no ref_field", CreateScopeSpec{Field: "branch_id", Via: "a.b", ViaField: "branch_id"}},
		{"unknown ref_field", CreateScopeSpec{Field: "branch_id", RefField: "nope", Via: "a.b", ViaField: "branch_id"}},
		{"no via", CreateScopeSpec{Field: "branch_id", RefField: "table_session_id", ViaField: "branch_id"}},
		{"via without an entity ref", CreateScopeSpec{Field: "branch_id", RefField: "table_session_id", Via: "table-session", ViaField: "branch_id"}},
		{"no via_field", CreateScopeSpec{Field: "branch_id", RefField: "table_session_id", Via: "a.b"}},
		{"unknown source", CreateScopeSpec{Field: "branch_id", From: "session", RefField: "table_session_id", Via: "a.b", ViaField: "branch_id"}},
	}
	for _, c := range bad {
		e := createScopeSpec(func(e *EntitySpec) { e.CreateScope = []CreateScopeSpec{c.cs} })
		if err := ValidateEntitySpec(e); err == nil {
			t.Errorf("%s: expected an error, got none", c.name)
		}
	}

	// A duplicate field is refused: the second check could contradict the first,
	// and which one wins would depend on iteration order.
	dup := createScopeSpec(func(e *EntitySpec) {
		e.CreateScope = []CreateScopeSpec{
			{Field: "branch_id", RefField: "table_session_id", Via: "a.b", ViaField: "branch_id"},
			{Field: "branch_id", RefField: "table_session_id", Via: "a.c", ViaField: "branch_id"},
		}
	})
	if err := ValidateEntitySpec(dup); err == nil {
		t.Error("the same field declared twice must be refused")
	}
}

// TestValidateScopeEnforcement pins the coupling between a `scope` declaration
// and the row restriction it claims to have. Before this, an entity could
// declare a dimension and simply never filter by it, and the manifest could not
// answer "is that deliberate?" — the leak class kafe 10.76 belongs to.
func TestValidateScopeEnforcement(t *testing.T) {
	d := func(enforced string) *ScopeDecl {
		return &ScopeDecl{Dimension: "branch", Field: "branch_id", Enforced: enforced}
	}
	sessionFilter := []FilterSpec{{Field: "branch_id", From: "session"}}
	routeFilter := []FilterSpec{{Field: "branch_id", From: "route", Param: "token"}}

	ok := []struct {
		name    string
		decl    *ScopeDecl
		filters []FilterSpec
	}{
		{"session is the default and is satisfied", d(""), sessionFilter},
		{"explicit session", d("session"), sessionFilter},
		{"route", d("route"), routeFilter},
		{"none is an explicit exemption", d("none"), nil},
		{"external is an explicit exemption", d("external"), nil},
		{"external tolerates a matching row_scope", d("external"), sessionFilter},
	}
	for _, c := range ok {
		if err := ValidateScopeEnforcement(c.decl, c.filters); err != nil {
			t.Errorf("%s: expected no error, got %v", c.name, err)
		}
	}

	bad := []struct {
		name    string
		decl    *ScopeDecl
		filters []FilterSpec
	}{
		{"declared dimension with no enforcement", d(""), nil},
		{"route claimed but the filter is session", d("route"), sessionFilter},
		{"session claimed but the filter is a literal", d("session"), []FilterSpec{{Field: "branch_id", Op: "eq", Value: "B1"}}},
		{"a filter on another field does not count", d("session"), []FilterSpec{{Field: "note", From: "session"}}},
		{"none contradicts a declared filter", d("none"), sessionFilter},
		{"unknown value", d("whoever"), nil},
	}
	for _, c := range bad {
		if err := ValidateScopeEnforcement(c.decl, c.filters); err == nil {
			t.Errorf("%s: expected an error, got none", c.name)
		}
	}
}
