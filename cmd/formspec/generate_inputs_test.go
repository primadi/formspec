package main

import (
	"strings"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

func specFieldOfType(t string) spec.Field {
	return spec.Field{Name: "f", Type: spec.FieldType(t)}
}

// TestGenerateTypeScript_TypedActionInputs pins the gain from the action-input
// contract: a parameter described by `params.inputs` gets its type from the
// declaration — either its own (ad-hoc) or the Entity field it refers to —
// instead of the blanket `unknown` a validate-only action still produces.
//
// The action is declared on the TRANSITION (`via: void-order` + `impl`), with no
// `actions:` entry — the shape plan L4 pushes manifests toward, by deleting the
// duplicated entry. `formspec generate` mirrors the public REST surface, so this
// also locks that `GenerateCustomActionRoutes` reads the action-source union: it
// used to build from `es.Actions` alone, which made a `via`-only transition serve
// `/_ui/entity/…` while being absent from the generated client entirely (todo
// 5.24.3).
func TestGenerateTypeScript_TypedActionInputs(t *testing.T) {
	dir := t.TempDir()
	writeSpecFile(t, dir, "order.yaml", `
apiVersion: formspec.dev/v1
kind: Entity
metadata: { name: order, module: cafe }
spec:
  version: v1
  characteristic: master
  expose:
    - { type: rest, actions: [list, find] }
  fields:
    - { name: status, type: enum, enum_values: [paid, cancelled], rules: [required] }
    - { name: void_reason, type: text }
    - { name: reason_code, type: string, enum_values: [spoil, mistake] }
  input_sets:
    - name: reason
      inputs:
        - { name: reason_code, rules: [required] }
  state_machine:
    field: status
    initial: paid
    states: [{ name: paid }, { name: cancelled }]
    transitions:
      - from: paid
        to: cancelled
        via: void-order
        impl: { type: script_ref, ref: cafe/void_order }
        params:
          inputs:
            - { name: void_reason, widget: textarea, required: true }
            - { name: approver_note, type: text }
          inputs_from: [reason]
`)
	reg, err := loadRegistryForCodegen(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ts, err := generateTypeScript(reg)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	checks := []string{
		`export interface CafeOrderVoidOrderParams {`,
		// Referring input: the field is `text`, required via the input's own flag.
		`"void_reason": string;`,
		// Ad-hoc input: typed from its own declaration, optional.
		`"approver_note"?: string;`,
		// Resolved through `inputs_from` → the set's input refers to the field
		// `reason_code`, so it is typed from the FIELD (a `string` — `enum_values`
		// is not a closed set for a `string` field; only `options` is), and the
		// set's `required` rule makes it non-optional.
		`"reason_code": string;`,
	}
	for _, want := range checks {
		if !strings.Contains(ts, want) {
			t.Errorf("generated output missing %q\n--- full output ---\n%s", want, ts)
		}
	}
}

// A validate-only action must keep emitting `unknown` — the manifest names the
// parameter but declares no type, and asserting one would be unverifiable.
func TestGenerateTypeScript_ValidateOnlyStaysUnknown(t *testing.T) {
	dir := t.TempDir()
	writeSpecFile(t, dir, "invoice.yaml", `
apiVersion: formspec.dev/v1
kind: Entity
metadata: { name: invoice, module: billing }
spec:
  version: v1
  characteristic: master
  expose:
    - { type: rest, actions: [list, find] }
  fields:
    - { name: note, type: string }
  actions:
    - name: approve
      params:
        validate:
          - { field: note, rules: [] }
      impl: { type: script_ref, ref: billing/approve }
`)
	reg, err := loadRegistryForCodegen(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ts, err := generateTypeScript(reg)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if !strings.Contains(ts, `"note"?: unknown;`) {
		t.Errorf("validate-only params must stay `unknown`, got:\n%s", ts)
	}
}

// TestTsFieldType_TextIsString covers the field-type gap the input contract
// exposed: `text` and `richtext` fell through to `unknown`, so an input referring
// to a plain `text` field was untyped in the generated client.
func TestTsFieldType_TextIsString(t *testing.T) {
	for _, ft := range []string{"text", "richtext"} {
		if got := tsFieldType(specFieldOfType(ft)); got != "string" {
			t.Errorf("tsFieldType(%s) = %q, want \"string\"", ft, got)
		}
	}
}
