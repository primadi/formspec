package ui

import (
	"sort"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// TestBuildBundle_AuthorizedActions pins the fix for kafe 10.23/10.19.
//
// Why this exists: every derived CRUD route and every action button was built
// from the entity's LIFECYCLE, never from the caller's authorization. Measured
// on the public `kafe-qr` App (grant `{entity: cafe-master.menu-category,
// actions: [list, find]}`), the SPA rendered a working-looking "Create Menu
// Category" modal whose submit answered 401 — and a cashier without `create`
// was offered a "New" button that opened a form with no Save button.
//
// The bundle is the ONLY place that can settle this, because it is the only
// place that knows both the caller (identity or public grant) and the entity.
// Shipping the resolved set is what lets the renderer stop guessing.
func TestBuildBundle_AuthorizedActions(t *testing.T) {
	entities := func() []EntityDescriptor {
		return []EntityDescriptor{
			{Module: "billing", Name: "order", Spec: orderEntity()},
		}
	}
	perms := func(list ...string) PermissionChecker {
		allowed := map[string]bool{}
		for _, p := range list {
			allowed[p] = true
		}
		return func(p string) bool { return allowed[p] }
	}
	schemaOf := func(t *testing.T, can PermissionChecker) EntitySchema {
		t.Helper()
		b := loadFixture(t).BuildBundle(entities, can, AppContext{Modules: map[string]bool{"billing": true}})
		if len(b.Entities) != 1 {
			t.Fatalf("bundle entities = %d, want 1", len(b.Entities))
		}
		return b.Entities[0]
	}

	t.Run("read-only caller gets no write action", func(t *testing.T) {
		got := schemaOf(t, perms("billing.orders.list", "billing.orders.view")).AuthorizedActions
		sort.Strings(got)
		want := []string{"find", "list"}
		if !equalStrings(got, want) {
			t.Fatalf("authorized_actions = %v, want %v — a caller who may only read "+
				"must not be told it may create/delete (kafe 10.23)", got, want)
		}
	})

	t.Run("view permission satisfies the find action", func(t *testing.T) {
		// The bundle asks with the RESOURCE spelling. A caller holding only
		// `.view` must still get `find` — the UI row action is "View" but the
		// route demands `view`, and spelling it from the UI word produced
		// `.edit`/`.view`-style names that never exist (kafe: `manajer` lost the
		// Edit button while holding `.update`).
		got := schemaOf(t, perms("billing.orders.view")).AuthorizedActions
		if !containsString(got, "find") {
			t.Fatalf("authorized_actions = %v, want it to contain find", got)
		}
	})

	t.Run("full CRUD is reported faithfully", func(t *testing.T) {
		got := schemaOf(t, perms(
			"billing.orders.list", "billing.orders.view", "billing.orders.create",
			"billing.orders.update", "billing.orders.delete",
		)).AuthorizedActions
		sort.Strings(got)
		// `delete` is declared `disabled: true` on the fixture entity, so it must
		// NOT appear even though the permission is held — the router registers no
		// such route either.
		want := []string{"create", "find", "list", "update"}
		if !equalStrings(got, want) {
			t.Fatalf("authorized_actions = %v, want %v", got, want)
		}
	})

	t.Run("lifecycle-free entity exposes no submit", func(t *testing.T) {
		// A `master` entity with no explicit lifecycle IS lifecycle-free
		// (spec.EntitySpec.LifecycleFree), and the router registers no
		// submit/cancel/amend route for it (internal/api/generator.go
		// disabledActions folds LifecycleFree into the disabled set, then
		// transitive gating kills cancel/amend). Holding the permission must
		// therefore NOT report them.
		master := &spec.EntitySpec{
			Plural:         "categories",
			Characteristic: spec.CharMaster,
			Fields:         []spec.Field{{Name: "name", Type: spec.FieldString}},
		}
		got := authorizedActions(
			EntityDescriptor{Module: "billing", Name: "category", Spec: master},
			buildEntitySchema(EntityDescriptor{Module: "billing", Name: "category", Spec: master}),
			perms(
				"billing.categories.list", "billing.categories.submit",
				"billing.categories.cancel",
			),
		)
		if !containsString(got, "list") {
			t.Fatalf("authorized_actions = %v, want it to contain list", got)
		}
		for _, a := range []string{"submit", "cancel", "amend"} {
			if containsString(got, a) {
				t.Errorf("authorized_actions = %v, must not contain %q for a "+
					"lifecycle-free entity (no such route is registered)", got, a)
			}
		}
	})

	t.Run("no permission at all yields nothing", func(t *testing.T) {
		// The entity would not even ship (no list/view), so assert through the
		// helper instead — it must not invent actions for a caller with none.
		got := authorizedActions(
			EntityDescriptor{Module: "billing", Name: "order", Spec: orderEntity()},
			buildEntitySchema(EntityDescriptor{Module: "billing", Name: "order", Spec: orderEntity()}),
			perms(),
		)
		if len(got) != 0 {
			t.Fatalf("authorized_actions = %v, want empty", got)
		}
	})
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
