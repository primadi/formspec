package main

import (
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// TestTsFieldType_ScalarOptionsBecomeLiteralUnion covers todo 5.10.22: a scalar
// field carrying `options` is a closed set on the Entity, so the generated type
// must be the literal union rather than the open scalar type.
//
// Before this, `tsFieldType` read only `EnumValues`, so
//
//	type: integer
//	options:
//	  - { value: 1, label: Senin }
//	  - { value: 2, label: Selasa }
//
// generated `number` — a caller could then write the undeclared `9` and only
// find out from a runtime validation error, which is exactly what `options`
// exists to prevent.
func TestTsFieldType_ScalarOptionsBecomeLiteralUnion(t *testing.T) {
	ints := spec.Field{
		Type: spec.FieldInteger,
		Options: []spec.FieldOption{
			{Value: 1, Label: "Senin"},
			{Value: 2, Label: "Selasa"},
			{Value: 7, Label: "Minggu"},
		},
	}
	if got, want := tsFieldType(ints), "1 | 2 | 7"; got != want {
		t.Errorf("integer with options -> %q, want %q", got, want)
	}

	stringsField := spec.Field{
		Type:    spec.FieldString,
		Options: []spec.FieldOption{{Value: "qris"}, {Value: "cash"}},
	}
	if got, want := tsFieldType(stringsField), `"qris" | "cash"`; got != want {
		t.Errorf("string with options -> %q, want %q", got, want)
	}
}

// TestTsFieldType_OptionCardinality decides the shape. `multiple` is a property
// of the DATA (Field.Multiple), so a set must not generate a single-value type.
func TestTsFieldType_OptionCardinality(t *testing.T) {
	multi := true
	single := false
	opts := []spec.FieldOption{{Value: 1}, {Value: 2}}

	jsonSet := spec.Field{Type: spec.FieldJSON, Options: opts, Multiple: &multi}
	if got, want := tsFieldType(jsonSet), "Array<1 | 2>"; got != want {
		t.Errorf("json set -> %q, want %q", got, want)
	}

	jsonSingle := spec.Field{Type: spec.FieldJSON, Options: opts, Multiple: &single}
	if got, want := tsFieldType(jsonSingle), "1 | 2"; got != want {
		t.Errorf("json single -> %q, want %q", got, want)
	}

	// A comma-separated string set cannot be expressed as a TS string type, so
	// it must stay open rather than claim a union it does not enforce.
	strSet := spec.Field{Type: spec.FieldString, Options: []spec.FieldOption{{Value: "a"}}, Multiple: &multi}
	if got := tsFieldType(strSet); got != "string" {
		t.Errorf("string set -> %q, want string (wire form is comma-separated)", got)
	}
}

// TestTsFieldType_ScalarWithoutOptionsStaysOpen pins that the union is opt-in:
// a plain integer must not become some degenerate type.
func TestTsFieldType_ScalarWithoutOptionsStaysOpen(t *testing.T) {
	if got := tsFieldType(spec.Field{Type: spec.FieldInteger}); got != "number" {
		t.Errorf("plain integer -> %q, want number", got)
	}
}

// TestTsOptionUnion_MixedAndUnrepresentable covers the two ways a union can fail
// to be expressible. Both must fall back to "" so the caller emits the open
// scalar type: returning a partial or malformed union would produce TypeScript
// that does not compile, which is worse than a slightly loose type.
func TestTsOptionUnion_MixedAndUnrepresentable(t *testing.T) {
	if got := tsOptionUnion(nil); got != "" {
		t.Errorf("no options -> %q, want empty", got)
	}
	if got := tsOptionUnion([]spec.FieldOption{{Value: nil}}); got != "" {
		t.Errorf("only-nil options -> %q, want empty (no literals)", got)
	}
	if got := tsOptionUnion([]spec.FieldOption{{Value: map[string]any{"a": 1}}}); got != "" {
		t.Errorf("non-scalar option -> %q, want empty (cannot be a literal)", got)
	}
	// A nil option inside an otherwise usable list is skipped, not fatal.
	mixed := []spec.FieldOption{{Value: 1}, {Value: nil}, {Value: true}}
	if got, want := tsOptionUnion(mixed), "1 | true"; got != want {
		t.Errorf("mixed options -> %q, want %q", got, want)
	}
}
