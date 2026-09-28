package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/manifest"
)

// writeUngatedSpec writes a spec tree exercising every branch of
// checkUngatedActions:
//
//   - pay: required_permission, NO impl, NO transition gate  → WARN (the hole)
//   - ship: required_permission, NO impl, BUT a transition gate → clean
//   - post: required_permission AND impl (route exists)        → clean
//   - cancel-order: reserved name, permission ≠ conventional route name → WARN
//   - cancel (another entity): reserved name, permission == conventional → clean
func writeUngatedSpec(t *testing.T, dir string) {
	t.Helper()
	write := func(rel, content string) {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	write("modules/alpha/master/order/entity.yaml", `apiVersion: formspec.dev/v1
kind: Entity
metadata:
  name: order
  module: alpha
spec:
  version: v1
  characteristic: master
  plural: orders
  fields:
    - name: status
      type: enum
      enum_values: [draft, awaiting_payment, paid, shipped, posted]
      default: draft
  state_machine:
    field: status
    initial: draft
    states:
      - { name: draft }
      - { name: awaiting_payment }
      - { name: paid }
      - { name: shipped }
      - { name: posted }
    transitions:
      - { from: draft, to: awaiting_payment, via: pay }
      # SHIP is gated on the transition → its action needs nothing more.
      - { from: paid, to: shipped, via: ship, require_permission: orders.ship }
      - { from: shipped, to: posted, via: post }
  actions:
    # The hole: declared permission, no impl, no transition gate.
    - { name: pay, required_permission: orders.pay }
    # Clean: the transition above gates it.
    - { name: ship, required_permission: orders.ship }
    # Clean: impl gives it a route, and the route requires the permission.
    - name: post
      required_permission: orders.post
      impl: { type: script_ref, ref: alpha/post }
    # Reserved name whose permission DIFFERS from the conventional (plural)
    # {module}.{plural}.{name} - the entry redirects the generic route.
    - { name: cancel-order, required_permission: order.cancel-order }
`)
}

func TestCheckUngatedActions_FlagsUnenforcedPermission(t *testing.T) {
	dir := t.TempDir()
	writeUngatedSpec(t, dir)

	loader := manifest.NewLoader(dir)
	res, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	result := &checkResult{}
	checkUngatedActions(result, res.Manifests)

	var msgs []string
	for _, i := range result.Issues {
		if i.Kind != "warning" {
			t.Errorf("finding must be a warning (the shape is legitimate), got %q: %s",
				i.Kind, i.Message)
		}
		msgs = append(msgs, i.Message)
	}

	joined := strings.Join(msgs, "\n")

	// The defect: a declared permission nothing enforces.
	if !strings.Contains(joined, `action "pay"`) {
		t.Errorf("`pay` declares required_permission with no impl and no transition "+
			"gate — the check must flag it. Findings:\n%s", joined)
	}
	// And the money-path instance of the same shape.
	if !strings.Contains(joined, `action "cancel-order"`) {
		t.Errorf("`cancel-order` is a reserved name whose permission differs from "+
			"the conventional route name, so the entry redirects it — that must be "+
			"reported. Findings:\n%s", joined)
	}

	// The three clean shapes must NOT be reported. A check that flags correct
	// manifests gets ignored, which is worse than no check.
	for _, clean := range []string{`action "ship"`, `action "post"`} {
		if strings.Contains(joined, clean) {
			t.Errorf("%s is correctly enforced (transition gate / impl route) and "+
				"must not be flagged. Findings:\n%s", clean, joined)
		}
	}
}

// A reserved name whose declared permission EQUALS the conventional
// `{module}.{plural}.{name}` is simply restating the route's own permission —
// no redirect, nothing to report. This pins that the reserved-name exemption is
// not a blanket exemption.
func TestCheckUngatedActions_ReservedNameMatchingConventionIsClean(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "modules/alpha/master/invoice/entity.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`apiVersion: formspec.dev/v1
kind: Entity
metadata:
  name: invoice
  module: alpha
spec:
  version: v1
  characteristic: master
  plural: invoices
  lifecycle: two_step_autosave
  fields:
    - name: code
      type: string
  actions:
    - { name: cancel, required_permission: invoices.cancel }
`), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := manifest.NewLoader(dir)
	res, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	result := &checkResult{}
	checkUngatedActions(result, res.Manifests)

	for _, i := range result.Issues {
		if strings.Contains(i.Message, `action "cancel"`) {
			t.Errorf("`cancel` with the conventional permission just restates the "+
				"generic route's own permission — nothing is redirected, so it must "+
				"not be reported: %s", i.Message)
		}
	}
}
