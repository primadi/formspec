package spec

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// `Field.options` is a closed set of choices *with captions* for a multi-value
// field (the `select-multi-tag` widget). These tests pin the rules that keep the
// declaration honest — the failure they close is a picker that offers a choice
// it cannot store, or a value that silently disappears on save.

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
    options:
      - { value: urgent }
`)
	if err := ValidateEntitySpec(es); err != nil {
		t.Fatalf("an option without a label must be valid, got: %v", err)
	}
}

func TestValidateFieldOptions_NoOptionsIsFine(t *testing.T) {
	es := parseEntityWithField(t, `  - name: payload
    type: json
`)
	if err := ValidateEntitySpec(es); err != nil {
		t.Fatalf("a field with no options must be valid, got: %v", err)
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
    options:
      - { value: [1, 2] }
`,
			want: "must be a string, number, or boolean",
			why:  "a list cannot be drawn as one chip label",
		},
		{
			name: "single-value field type",
			yaml: `  - name: priority
    type: integer
    options:
      - { value: 1 }
`,
			want: "only valid on a field that holds a set of values",
			why:  "`options` describes a set — on a scalar field it acts on nothing",
		},
		{
			name: "enum points at enum_values",
			yaml: `  - name: status
    type: enum
    enum_values: [open, closed]
    options:
      - { value: open, label: Open }
`,
			want: "single-value enum uses `enum_values`",
			why:  "the author most likely wanted the existing contract",
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
