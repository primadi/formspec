package entity

import (
	"testing"

	"github.com/primadi/forma/pkg/spec"
)

// TestRegisterStandardPermissions_LifecycleEnabled verifies that an exposed
// entity participating in the document lifecycle (submit not disabled) gets
// submit/cancel/amend registered alongside the standard CRUD permissions
// (2.6.3) — matching what internal/api/generator.go's generateRESTRoutes
// derives for routes, so PermissionExists() doesn't drift from what routes
// actually require.
func TestRegisterStandardPermissions_LifecycleEnabled(t *testing.T) {
	reg, d := setupTestRegistry(t, "registry_fixtures/customer/spec")
	defer d.Close()

	es := &spec.EntitySpec{
		Plural: "invoices",
		Expose: []spec.ExposeConfig{{Type: spec.ProtocolREST}},
	}
	reg.registerStandardPermissions("billing", "invoice", es, "test")

	perms := reg.GetPermissionRegistry()
	for _, want := range []string{
		"billing.invoices.list", "billing.invoices.view", "billing.invoices.create",
		"billing.invoices.update", "billing.invoices.delete",
		"billing.invoices.submit", "billing.invoices.cancel", "billing.invoices.amend",
	} {
		if !perms.PermissionExists(want) {
			t.Errorf("expected permission %q to be registered", want)
		}
	}
}

// TestRegisterStandardPermissions_SubmitDisabled verifies that disabling
// submit transitively removes cancel/amend too (matching
// db.TransitiveDisabled, the same gating generateRESTRoutes applies), so a
// lifecycle-free entity never gets lifecycle permissions registered for
// routes that don't exist.
func TestRegisterStandardPermissions_SubmitDisabled(t *testing.T) {
	reg, d := setupTestRegistry(t, "registry_fixtures/customer/spec")
	defer d.Close()

	es := &spec.EntitySpec{
		Plural: "notes",
		Expose: []spec.ExposeConfig{{Type: spec.ProtocolREST}},
		Actions: []spec.Action{
			{Name: "submit", Disabled: true},
		},
	}
	reg.registerStandardPermissions("crm", "note", es, "test")

	perms := reg.GetPermissionRegistry()
	for _, want := range []string{"crm.notes.list", "crm.notes.view", "crm.notes.create", "crm.notes.update", "crm.notes.delete"} {
		if !perms.PermissionExists(want) {
			t.Errorf("expected permission %q to be registered", want)
		}
	}
	for _, dontWant := range []string{"crm.notes.submit", "crm.notes.cancel", "crm.notes.amend"} {
		if perms.PermissionExists(dontWant) {
			t.Errorf("expected permission %q to NOT be registered (submit disabled)", dontWant)
		}
	}
}

// TestRegisterStandardPermissions_Summary verifies summary entities (§4.1,
// read-only) never get create/update/delete permissions — mirrors
// generateRESTRoutes' isSummary skip, which only excludes those three
// actions (lifecycle gating is independent, via TransitiveDisabled).
func TestRegisterStandardPermissions_Summary(t *testing.T) {
	reg, d := setupTestRegistry(t, "registry_fixtures/customer/spec")
	defer d.Close()

	es := &spec.EntitySpec{
		Plural:         "balances",
		Characteristic: spec.CharSummary,
		Expose:         []spec.ExposeConfig{{Type: spec.ProtocolREST}},
	}
	reg.registerStandardPermissions("gl", "balance", es, "test")

	perms := reg.GetPermissionRegistry()
	for _, want := range []string{"gl.balances.list", "gl.balances.view"} {
		if !perms.PermissionExists(want) {
			t.Errorf("expected permission %q to be registered", want)
		}
	}
	for _, dontWant := range []string{"gl.balances.create", "gl.balances.update", "gl.balances.delete"} {
		if perms.PermissionExists(dontWant) {
			t.Errorf("expected permission %q to NOT be registered (summary entity)", dontWant)
		}
	}
}
