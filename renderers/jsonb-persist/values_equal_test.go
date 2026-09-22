package db

import "testing"

// computeChanges/diff the old and new record data for the audit log. The values
// come straight from entity data, so any shape a field can hold reaches
// valuesEqual — and a bare `==` on an uncomparable type PANICS the process
// rather than returning false. A child collection stored in its own table comes
// back as []map[string]any while the write path carries []any, which no case
// matched: the outbox worker crashed mid-delivery with
// "comparing uncomparable type []map[string]interface {}".
func TestValuesEqual_UncomparableShapes(t *testing.T) {
	tableChild := []map[string]any{
		{"account_id": "a", "debit": 143750.0, "credit": 0.0},
		{"account_id": "b", "debit": 0.0, "credit": 125000.0},
	}
	sameChild := []map[string]any{
		{"account_id": "a", "debit": 143750.0, "credit": 0.0},
		{"account_id": "b", "debit": 0.0, "credit": 125000.0},
	}
	differentChild := []map[string]any{
		{"account_id": "a", "debit": 1.0, "credit": 0.0},
	}

	cases := []struct {
		name string
		a, b any
		want bool
	}{
		{"equal table-stored children", tableChild, sameChild, true},
		{"different table-stored children", tableChild, differentChild, false},
		{"different lengths", tableChild, tableChild[:1], false},
		{"child not a slice at all", tableChild, "not-a-slice", false},
		{"equal scalars", 143750.0, 143750.0, true},
		{"different scalars", 143750.0, 143749.0, false},
		{"equal strings", "posted", "posted", true},
		{"different types", "1", 1, false},
		{"both nil", nil, nil, true},
		{"nil vs value", nil, 1.0, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// The point of the test: these must return, not panic.
			if got := valuesEqual(tc.a, tc.b); got != tc.want {
				t.Errorf("valuesEqual(%v, %v) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

// A hydrated child collection ([]map[string]any) compared against the same
// collection as the write path carries it ([]any) must report equal — that is
// what makes an update that only touches a scalar produce a diff that mentions
// just that scalar, instead of rewriting every line of the journal.
func TestValuesEqual_SliceShapeCrossComparison(t *testing.T) {
	asMaps := []map[string]any{{"account_id": "a", "debit": 1.0}}
	asAnys := []any{map[string]any{"account_id": "a", "debit": 1.0}}

	if !valuesEqual(asMaps, asAnys) {
		t.Error("valuesEqual([]map[string]any, []any) with equal content = false, want true")
	}
	if !valuesEqual(asAnys, asMaps) {
		t.Error("valuesEqual([]any, []map[string]any) with equal content = false, want true")
	}
}

// The nested-map case must reach the same conclusion as the slice case, since a
// child row is just a map.
func TestValuesEqual_NestedMaps(t *testing.T) {
	a := map[string]any{"lines": []map[string]any{{"debit": 1.0}}}
	b := map[string]any{"lines": []any{map[string]any{"debit": 1.0}}}
	if !valuesEqual(a, b) {
		t.Error("nested child collections in different slice shapes should compare equal")
	}
}

// computeChanges must survive an uncomparable child collection on both sides —
// this is the exact path the outbox worker died on.
func TestComputeChanges_WithTableStoredChildren(t *testing.T) {
	old := map[string]any{
		"status": "draft",
		"lines":  []map[string]any{{"account_id": "a", "debit": 143750.0}},
	}
	new := map[string]any{
		"status": "posted",
		"lines":  []map[string]any{{"account_id": "a", "debit": 143750.0}},
	}

	changes := computeChanges(old, new)

	if _, ok := changes["status"]; !ok {
		t.Errorf("status change missing from diff: %+v", changes)
	}
	if _, ok := changes["lines"]; ok {
		t.Errorf("unchanged lines wrongly reported as changed: %+v", changes["lines"])
	}
}
