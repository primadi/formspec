package spec

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// `Field.options` is a closed set of choices *with captions* for a field that
// holds values from it, and `Field.multiple` says how many of them the field
// holds (`json` and `string` can hold either shape, so the type alone cannot
// say). These tests pin both halves — the shape rules that keep the declaration
// honest, and the cardinality rules that keep "single or multi" a property of
// the data rather than of one Form's `widget:`.
//
// The failure they close is a declaration the renderer cannot honour: a picker
// that offers a choice it cannot store, a value that disappears on save, or a
// field that reads as multi-select in one form and single in another.

func parseEntityWithField(t *testing.T, fieldYAML string) *EntitySpec {
	t.Helper()
	var es EntitySpec
	src := "fields:\n" + fieldYAML
	if err := yaml.Unmarshal([]byte(src), &es); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	return &es
}

func TestValidateFieldOptions_AcceptsDeclaredSet(t *testing.T) {
	es := parseEntityWithField(t, `  - name: days_of_week
    type: json
    multiple: true
    options:
      - { value: 1, label: Senin }
      - { value: 2, label: Selasa }
      - { value: 7, label: Minggu }
`)
	if err := ValidateEntitySpec(es); err != nil {
		t.Fatalf("a declared option set must be valid, got: %v", err)
	}
	// The stored value keeps its scalar type: `value: 1` is the number 1, so a
	// json field stores [1], not ["1"].
	if got := es.Fields[0].Options[0].Value; got != 1 {
		t.Errorf("option value type: want int 1, got %#v (%T)", got, got)
	}
	if got := es.Fields[0].Options[0].Label; got != "Senin" {
		t.Errorf("option label: want Senin, got %q", got)
	}
}

func TestValidateFieldOptions_LabelIsOptional(t *testing.T) {
	es := parseEntityWithField(t, `  - name: tags
    type: json
    multiple: true
    options:
      - { value: urgent }
`)
	if err := ValidateEntitySpec(es); err != nil {
		t.Fatalf("an option without a label must be valid, got: %v", err)
	}
}

func TestValidateFieldOptions_NoOptionsIsFine(t *testing.T) {
	// A free-form json field keeps its JSON editor: no choice set, no
	// cardinality to declare.
	es := parseEntityWithField(t, `  - name: payload
    type: json
`)
	if err := ValidateEntitySpec(es); err != nil {
		t.Fatalf("a field with no options must be valid, got: %v", err)
	}
}

// A single-value choice set on a scalar field is the case this whole contract
// exists to allow: `1` renders as "Senin" without an enum, and the field holds
// one of them.
func TestValidateFieldOptions_ScalarSingleSelectIsValid(t *testing.T) {
	es := parseEntityWithField(t, `  - name: day_of_week
    type: integer
    multiple: false
    options:
      - { value: 1, label: Senin }
      - { value: 2, label: Selasa }
`)
	if err := ValidateEntitySpec(es); err != nil {
		t.Fatalf("a single-select with captions on a scalar field must be valid, got: %v", err)
	}
	if got := es.Fields[0].Options[1].Label; got != "Selasa" {
		t.Errorf("option label: want Selasa, got %q", got)
	}
}

// Absent `multiple` on a scalar field means one value — no declaration needed.
func TestValidateFieldOptions_ScalarAbsentMultipleDefaultsSingle(t *testing.T) {
	es := parseEntityWithField(t, `  - name: day_of_week
    type: integer
    options:
      - { value: 1, label: Senin }
`)
	if err := ValidateEntitySpec(es); err != nil {
		t.Fatalf("an undeclared `multiple` on a scalar must default to single, got: %v", err)
	}
	multi, err := FieldIsMultiple(&es.Fields[0])
	if err != nil {
		t.Fatalf("FieldIsMultiple: %v", err)
	}
	if multi {
		t.Error("a scalar field with no `multiple` must read as single")
	}
}

// `string` holding a comma-separated list is the other container the tag widget
// reads — the same ambiguity as `json`, so it needs the same declaration.
func TestValidateFieldOptions_StringMultiNeedsMultiple(t *testing.T) {
	es := parseEntityWithField(t, `  - name: tags
    type: string
    multiple: true
    options:
      - { value: urgent, label: Urgent }
`)
	if err := ValidateEntitySpec(es); err != nil {
		t.Fatalf("a declared multi-value string set must be valid, got: %v", err)
	}
}

func TestValidateFieldOptions_Rejections(t *testing.T) {
	// Each case names the concrete failure it prevents, so a future edit that
	// relaxes one of these has to consciously drop that guarantee.
	cases := []struct {
		name string
		yaml string
		want string // substring the error must contain
		why  string
	}{
		{
			name: "duplicate value",
			yaml: `  - name: days_of_week
    type: json
    multiple: true
    options:
      - { value: 1, label: Senin }
      - { value: 1, label: Monday }
`,
			want: "duplicates the value",
			why:  "the picker would offer a choice that can never be added twice",
		},
		{
			name: "duplicate across scalar spellings",
			yaml: `  - name: days_of_week
    type: json
    multiple: true
    options:
      - { value: 1 }
      - { value: "1" }
`,
			want: "duplicates the value",
			why:  "`1` and \"1\" are the same chip to the user",
		},
		{
			name: "missing value",
			yaml: `  - name: days_of_week
    type: json
    multiple: true
    options:
      - { label: Senin }
`,
			want: "has no `value`",
			why:  "an option that stores nothing renders a chip with no value",
		},
		{
			name: "non-scalar value",
			yaml: `  - name: days_of_week
    type: json
    multiple: true
    options:
      - { value: [1, 2] }
`,
			want: "must be a string, number, or boolean",
			why:  "a list cannot be drawn as one chip label",
		},
		{
			name: "options without multiple on json",
			yaml: `  - name: days_of_week
    type: json
    options:
      - { value: 1, label: Senin }
`,
			want: "`multiple` is required on a json field with `options`",
			why:  "json holds either shape, so the declaration cannot be interpreted without it",
		},
		{
			name: "options without multiple on string",
			yaml: `  - name: tags
    type: string
    options:
      - { value: urgent }
`,
			want: "`multiple` is required on a string field with `options`",
			why:  "string holds either one value or a comma-separated list",
		},
		{
			name: "multiple true on a scalar type",
			yaml: `  - name: priority
    type: integer
    multiple: true
    options:
      - { value: 1 }
`,
			want: "`multiple: true` is not valid on a",
			why:  "an integer column stores one value — accepting this would be a claim nothing can act on",
		},
		{
			name: "multiple without options",
			yaml: `  - name: payload
    type: json
    multiple: true
`,
			want: "`multiple` only means something with `options`",
			why:  "there is no choice set for the cardinality to describe",
		},
		{
			name: "enum points at enum_values",
			yaml: `  - name: status
    type: enum
    enum_values: [open, closed]
    options:
      - { value: open, label: Open }
`,
			want: "`enum_values`",
			why:  "the enum contract already carries the values (and a CHECK constraint); a second source of truth is how the two drift",
		},
		{
			name: "options on money",
			yaml: `  - name: price
    type: money
    options:
      - { value: 1 }
`,
			want: "renders through its own widget",
			why:  "money renders as an amount, not a choice list",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			es := parseEntityWithField(t, tc.yaml)
			err := ValidateEntitySpec(es)
			if err == nil {
				t.Fatalf("must be rejected (%s)", tc.why)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error must contain %q so the author can act on it; got: %v", tc.want, err)
			}
		})
	}
}

// FieldIsMultiple is the single rule the validator, `formspec check`, and the
// renderer derivation must agree on.
func TestFieldIsMultiple(t *testing.T) {
	trueVal, falseVal := true, false
	cases := []struct {
		name    string
		field   Field
		want    bool
		wantErr bool
	}{
		{name: "scalar absent means single", field: Field{Name: "qty", Type: FieldInteger}, want: false},
		{name: "scalar explicit false", field: Field{Name: "qty", Type: FieldInteger, Multiple: &falseVal}, want: false},
		{name: "json explicit true", field: Field{Name: "days", Type: FieldJSON, Multiple: &trueVal}, want: true},
		{name: "json explicit false", field: Field{Name: "day", Type: FieldJSON, Multiple: &falseVal}, want: false},
		{name: "json absent is an error", field: Field{Name: "days", Type: FieldJSON}, wantErr: true},
		{name: "string absent is an error", field: Field{Name: "tags", Type: FieldString}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FieldIsMultiple(&tc.field)
			if tc.wantErr {
				if err == nil {
					t.Fatal("want an error explaining that `multiple` is required, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("FieldIsMultiple: want %v, got %v", tc.want, got)
			}
		})
	}
}

// The gate that keeps a Form from contradicting the Entity's cardinality.
// Both directions matter: a set declared in the Entity must not be rendered as
// a one-value picker, and a single value must not be rendered as tags.
func TestWidgetCardinalityMismatch(t *testing.T) {
	trueVal, falseVal := true, false
	multiField := &Field{Name: "days_of_week", Type: FieldJSON, Multiple: &trueVal, Options: []FieldOption{{Value: 1}}}
	singleScalar := &Field{Name: "day_of_week", Type: FieldInteger, Multiple: &falseVal, Options: []FieldOption{{Value: 1}}}
	singleJSON := &Field{Name: "day_of_week", Type: FieldJSON, Multiple: &falseVal, Options: []FieldOption{{Value: 1}}}
	plainEnum := &Field{Name: "status", Type: FieldEnum, EnumValues: []string{"open"}}
	noOptions := &Field{Name: "notes", Type: FieldString}

	if err := WidgetCardinalityMismatch(multiField, WidgetSelectMultiTag); err != nil {
		t.Errorf("a set field with the tag widget must pass, got: %v", err)
	}
	if err := WidgetCardinalityMismatch(singleScalar, WidgetSelect); err != nil {
		t.Errorf("a single scalar with a select must pass, got: %v", err)
	}
	if err := WidgetCardinalityMismatch(plainEnum, WidgetSelect); err != nil {
		t.Errorf("an enum select is single by nature and must pass, got: %v", err)
	}
	if err := WidgetCardinalityMismatch(noOptions, WidgetSelectMultiTag); err != nil {
		t.Errorf("no declared set means nothing to be multiple of, got: %v", err)
	}
	// A widget that says nothing about cardinality must never be refused.
	if err := WidgetCardinalityMismatch(multiField, WidgetInput); err != nil {
		t.Errorf("a cardinality-neutral widget must pass, got: %v", err)
	}

	err := WidgetCardinalityMismatch(singleScalar, WidgetSelectMultiTag)
	if err == nil {
		t.Fatal("the tag widget on a single-value field must be refused")
	}
	if !strings.Contains(err.Error(), "needs a field that holds a set") {
		t.Errorf("error must say the field needs a set, got: %v", err)
	}
	if !strings.Contains(err.Error(), "multiple: true") {
		t.Errorf("error must name the fix, got: %v", err)
	}

	err = WidgetCardinalityMismatch(multiField, WidgetSelect)
	if err == nil {
		t.Fatal("a one-value select on a set field must be refused")
	}
	if !strings.Contains(err.Error(), "select-multi-tag") {
		t.Errorf("error must point at the set widget, got: %v", err)
	}

	// An undeclared `multiple` on a json field surfaces as the requirement
	// itself, so the author is told what to add rather than which widget to swap.
	ambiguous := &Field{Name: "days", Type: FieldJSON, Options: []FieldOption{{Value: 1}}}
	err = WidgetCardinalityMismatch(ambiguous, WidgetSelectMultiTag)
	if err == nil || !strings.Contains(err.Error(), "`multiple` is required") {
		t.Errorf("want the `multiple` requirement, got: %v", err)
	}

	err = WidgetCardinalityMismatch(singleJSON, WidgetRadioGroup)
	if err != nil {
		t.Errorf("a single-value json field with radio-group must pass, got: %v", err)
	}
}

func TestOptionValueKey_Canonicalises(t *testing.T) {
	// The key is what dedup and renderer lookups agree on; `1` (int, from YAML)
	// and `1` (float64, from a JSON round-trip) must be the same choice.
	if optionValueKey(1) != "1" {
		t.Errorf("int key: want \"1\", got %q", optionValueKey(1))
	}
	if optionValueKey(float64(1)) != "1" {
		t.Errorf("float key: want \"1\", got %q", optionValueKey(float64(1)))
	}
	if optionValueKey(true) != "true" {
		t.Errorf("bool key: want \"true\", got %q", optionValueKey(true))
	}
	if optionValueKey("Senin") != "Senin" {
		t.Errorf("string key: want \"Senin\", got %q", optionValueKey("Senin"))
	}
}
