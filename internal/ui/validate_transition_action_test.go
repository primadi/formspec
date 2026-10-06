package ui

import (
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
)

// A Form/Table action may name a state-machine transition's `via` — `via` IS an
// action (plan docs_internal/plan/via-sebagai-action-penuh.md, L2/L3), and since
// L4 a manifest declaring it a second time under `actions:` is REJECTED, so
// via-only is the intended shape.
//
// Regression: `actionExists` read `es.Actions` directly, so every via-only
// transition used as a button warned `action "…" not on entity …` at boot —
// while a sibling action that still happened to have an `actions:` entry
// (confirm-payment / void-order in kafe) passed. This test pins the union read
// on BOTH surfaces (Form and Table) from one fixture.
//
// Run with: go test ./internal/ui/ -run TestValidate_TransitionViaIsAnAction
func TestValidate_TransitionViaIsAnAction(t *testing.T) {
	const fixture = `
apiVersion: formspec.dev/v1alpha1
kind: Form
metadata: { name: order-form, module: billing }
spec:
  entity: order
  sections:
    - title: T
      fields:
        - { field: number }
  actions:
    - { action: mark-ready, label: "Siap" }
---
apiVersion: formspec.dev/v1alpha1
kind: Table
metadata: { name: order-table, module: billing }
spec:
  entity: order
  columns: [{ field: number }]
  row_actions:
    - { action: verify, label: "Verifikasi" }
`
	loader := manifest.NewLoader("")
	raws, perrs := loader.ParseBytes([]byte(fixture), "vias.yaml")
	if len(perrs) > 0 {
		t.Fatalf("parse: %v", perrs[0])
	}
	r := NewRegistry()
	if errs := r.Load(raws); len(errs) > 0 {
		t.Fatalf("load: %v", errs)
	}

	// order declares NO `actions:` at all — both names exist only as a
	// transition `via`, which must still resolve as an action.
	resolve := func(module, name string) (*spec.EntitySpec, bool) {
		if module == "billing" && name == "order" {
			return &spec.EntitySpec{
				Plural: "orders",
				Fields: []spec.Field{{Name: "number", Type: spec.FieldString}},
				StateMachine: &spec.StateMachine{
					Field:   "status",
					Initial: "paid",
					States:  []spec.StateDecl{{Name: "paid"}, {Name: "ready"}, {Name: "verified"}},
					Transitions: []spec.TransitionDecl{
						{From: spec.StateList{"paid"}, To: "ready", Action: "mark-ready"},
						{From: spec.StateList{"ready"}, To: "verified", Action: "verify"},
					},
				},
			}, true
		}
		return nil, false
	}

	errs := r.Validate(resolve)

	// Calibration: an unknown name must STILL be reported — otherwise the fix
	// would be indistinguishable from "validation disabled".
	unknown := `
apiVersion: formspec.dev/v1alpha1
kind: Table
metadata: { name: bogus-table, module: billing }
spec:
  entity: order
  columns: [{ field: number }]
  row_actions:
    - { action: nope, label: "Nope" }
`
	raws2, _ := loader.ParseBytes([]byte(unknown), "bogus.yaml")
	r2 := NewRegistry()
	if loadErrs := r2.Load(raws2); len(loadErrs) > 0 {
		t.Fatalf("load bogus: %v", loadErrs)
	}
	bogusErrs := r2.Validate(resolve)

	for _, e := range errs {
		if strings.Contains(e.Error(), "not on entity") {
			t.Errorf("a transition `via` used as a button must resolve, got: %s", e)
		}
	}
	foundUnknown := false
	for _, e := range bogusErrs {
		if strings.Contains(e.Error(), `action "nope" not on entity`) {
			foundUnknown = true
		}
	}
	if !foundUnknown {
		t.Fatalf("unknown action must still be reported, got: %v", bogusErrs)
	}
}
