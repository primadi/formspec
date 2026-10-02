package ui

import (
	"testing"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
)

// A wizard that BINDS itself to a transition (`spec.entity` + `spec.action`
// matching the transition's `via`) is the UI FOR THAT TRANSITION — it inherits
// reachability from the entity, so a manifest does not have to list it in
// `registered_views` (plan wizard-commit-patch-dan-peluncur.md, todo 5.25.7).
//
// The failure this prevents has no symptom until a user clicks: the launcher
// (DetailPage) reads `bundle.wizards`, finds nothing, and silently falls back to
// the raw state write — so the inputs the wizard exists to collect are skipped.

// shiftEntity carries a state machine with a via-only `close-shift` transition.
func shiftEntity() *spec.EntitySpec {
	return &spec.EntitySpec{
		Plural: "shifts",
		Fields: []spec.Field{
			{Name: "status", Type: spec.FieldEnum, EnumValues: []string{"open", "closed"}},
			{Name: "counted_cash", Type: spec.FieldMoney},
		},
		StateMachine: &spec.StateMachine{
			Field:   "status",
			Initial: "open",
			States:  []spec.StateDecl{{Name: "open", Label: "Open"}, {Name: "closed", Label: "Closed"}},
			Transitions: []spec.TransitionDecl{
				{From: spec.StateList{"open"}, To: "closed", Action: "close-shift", RequirePermission: "shifts.close-shift"},
			},
		},
	}
}

// wizardFixtureYAML declares the wizard that binds to the `close-shift`
// transition on the shift entity. It is NOT listed in any menu.
const wizardFixtureYAML = `
apiVersion: formspec.dev/v1alpha1
kind: Wizard
metadata: { name: close-shift-wizard, module: cafeorder }
spec:
  title: "Tutup Shift"
  entity: cafeorder.shift
  action: close-shift
  steps:
    - title: "Step"
      fields:
        - { field: counted_cash }
---
apiVersion: formspec.dev/v1alpha1
kind: Wizard
metadata: { name: unrelated-wizard, module: cafeorder }
spec:
  title: "Unrelated"
  entity: cafeorder.shift
  action: close-shift
  steps:
    - title: "Step"
      fields: []
`

func wizardEntities() func() []EntityDescriptor {
	return func() []EntityDescriptor {
		return []EntityDescriptor{
			{Module: "cafeorder", Name: "shift", Spec: shiftEntity()},
			{Module: "cafeorder", Name: "order", Spec: pluralEntity("orders")},
		}
	}
}

func wizardBundle(t *testing.T, ctx AppContext) *Bundle {
	t.Helper()
	r := NewRegistry()
	loader := manifest.NewLoader("")
	raws, errs := loader.ParseBytes([]byte(wizardFixtureYAML), "wizard_fixture.yaml")
	if len(errs) > 0 {
		t.Fatalf("parse fixture: %v", errs)
	}
	if loadErrs := r.Load(raws); len(loadErrs) > 0 {
		t.Fatalf("load fixture: %v", loadErrs)
	}
	return r.BuildBundle(wizardEntities(), func(string) bool { return true }, ctx)
}

func wizardNames(b *Bundle) []string {
	names := make([]string, 0, len(b.Wizards))
	for _, w := range b.Wizards {
		names = append(names, w.Name)
	}
	return names
}

func TestBuildBundle_WizardInheritsReachability(t *testing.T) {
	// `shift` is reachable via the menu; the wizard is NOT registered anywhere.
	ctx := AppContext{
		Modules: map[string]bool{"cafeorder": true},
		Menu: []spec.MenuItem{
			{Label: "Shift Kasir", Module: "cafeorder", Route: "/cafeorder/shifts"},
		},
	}
	b := wizardBundle(t, ctx)

	if !containsString(wizardNames(b), "close-shift-wizard") {
		t.Fatalf("wizard bound to a reachable entity's transition must inherit reachability, got %v", wizardNames(b))
	}
}

func TestBuildBundle_WizardNotInheritedWhenEntityUnreachable(t *testing.T) {
	// No menu, no registered_views → `shift` is present but NOT routable, so the
	// wizard must not become reachable through it either (its PATCH target has
	// no route).
	ctx := AppContext{Modules: map[string]bool{"cafeorder": true}}
	b := wizardBundle(t, ctx)

	if routableByRef(t, b, "cafeorder/shift") {
		t.Fatalf("shift must not be routable without a menu/registered_views")
	}
	if len(b.Wizards) != 0 {
		t.Fatalf("wizard must not inherit from a non-routable entity, got %v", wizardNames(b))
	}
}

func TestBuildBundle_WizardStillReachableWhenExplicitlyRegistered(t *testing.T) {
	// The explicit declaration keeps working (and keeps working even when the
	// entity is not routable — e.g. a wizard opened from a Page).
	ctx := AppContext{
		Modules:         map[string]bool{"cafeorder": true},
		RegisteredViews: []spec.RegisteredViewDecl{{View: "cafeorder/close-shift-wizard"}},
	}
	b := wizardBundle(t, ctx)

	if !containsString(wizardNames(b), "close-shift-wizard") {
		t.Fatalf("explicit registered_views entry must still ship the wizard, got %v", wizardNames(b))
	}
}

func TestBuildBundle_WizardInheritanceInactiveWithoutAppScope(t *testing.T) {
	// A zero AppContext (the `_admin` surface) is not gated at all, so every
	// wizard ships — the inheritance rule must not narrow it.
	b := wizardBundle(t, AppContext{})
	if len(b.Wizards) != 2 {
		t.Fatalf("ungated bundle must ship every wizard, got %v", wizardNames(b))
	}
}
