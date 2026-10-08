package ui

import (
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
)

// A TableAction may carry `view: "<kind>:<name>"` to navigate to a view
// resource instead of running the entity action — how renderer builtins such as
// `print` (no backing entity action) get a UI trigger. Plan
// docs_internal/plan/print-row-action.md.
//
// A `view` that does not resolve is a link to nowhere: the client registers no
// route for it and the click lands on the SPA's 404. Validate must refuse it.
//
// Run with: go test ./internal/ui/ -run TestValidate_ActionView
func TestValidate_ActionView(t *testing.T) {
	const fixture = `
apiVersion: formspec.dev/v1
kind: Print
metadata: { name: receipt, module: billing }
spec:
  entity: billing.order
  output: { format: html, paper: { size: A5 } }
---
apiVersion: formspec.dev/v1
kind: Table
metadata: { name: order-table, module: billing }
spec:
  entity: billing.order
  columns: [{ field: number }]
  row_actions:
    - { action: view, label: "Lihat" }
    - { action: print, label: "Cetak", view: "print:receipt" }
`
	loader := manifest.NewLoader("")
	raws, perrs := loader.ParseBytes([]byte(fixture), "view.yaml")
	if len(perrs) > 0 {
		t.Fatalf("parse: %v", perrs[0])
	}
	r := NewRegistry()
	if errs := r.Load(raws); len(errs) > 0 {
		t.Fatalf("load: %v", errs)
	}

	resolve := func(module, name string) (*spec.EntitySpec, bool) {
		if module == "billing" && name == "order" {
			return &spec.EntitySpec{
				Plural: "orders",
				Fields: []spec.Field{{Name: "number", Type: spec.FieldString}},
			}, true
		}
		return nil, false
	}

	// A resolvable `print` target must pass — `print` is a builtin with no
	// entity action, so the entity-action check must not reject it.
	if errs := r.Validate(resolve); len(errs) > 0 {
		t.Fatalf("a resolvable view target must validate clean, got: %v", errs)
	}

	// Calibration: an UNRESOLVABLE target must be reported — otherwise the
	// check is indistinguishable from "validation disabled". Two shapes:
	// malformed (no colon) and name-not-registered.
	cases := map[string]string{
		"malformed": `
apiVersion: formspec.dev/v1
kind: Table
metadata: { name: t1, module: billing }
spec:
  entity: billing.order
  columns: [{ field: number }]
  row_actions:
    - { action: print, label: "Cetak", view: "receipt" }
`,
		"unknown-name": `
apiVersion: formspec.dev/v1
kind: Table
metadata: { name: t2, module: billing }
spec:
  entity: billing.order
  columns: [{ field: number }]
  row_actions:
    - { action: print, label: "Cetak", view: "print:missing" }
`,
	}
	for name, fx := range cases {
		raws, _ := loader.ParseBytes([]byte(fx), name+".yaml")
		rr := NewRegistry()
		if errs := rr.Load(raws); len(errs) > 0 {
			t.Fatalf("%s: load: %v", name, errs)
		}
		errs := rr.Validate(resolve)
		if len(errs) == 0 {
			t.Errorf("%s: expected a view-target error, got none", name)
			continue
		}
		found := false
		for _, e := range errs {
			if strings.Contains(e.Error(), "view") {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: expected an error mentioning the view target, got: %v", name, errs)
		}
	}
}
