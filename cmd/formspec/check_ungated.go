package main

import (
	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
)

// checkUngatedActions warns about actions that declare `required_permission`
// but have NOTHING enforcing it — a permission that exists, can be granted, and
// is listed in the grant editor, yet never gates the operation it names.
//
// Why this check exists (measured 2026-09-27, kafe):
//
// `cafe-order.order` declares:
//
//   - name: confirm-payment
//     required_permission: orders.confirm-payment
//     # no `impl` — so no route is generated
//
// The state machine reaches `paid` through `{from: awaiting_payment, to: paid,
// via: confirm-payment}`. Applying that transition is `PATCH {status: "paid"}`,
// which the framework authorizes with `{module}.{plural}.update` — ONE
// permission for every transition of the entity (§8.6). Measured:
//
//	cashier token perms: [create, list, update, view]   (no confirm-payment)
//	PATCH {status: "paid"} → 200, status = paid
//
// So a cashier could mark an order PAID without holding the permission the
// manifest says is required — the money path, not a hypothetical. The declared
// permission was enforced NOWHERE: not on a route (none exists without `impl`),
// not by the transition gate (`require_permission` is a different field that
// this manifest does not set).
//
// A warning rather than an error on purpose: the historical shape is legitimate
// when `required_permission` is the action's DECLARED IDENTITY (so it can be
// granted and appear in footprints) and the actual gate lives on the
// transition. Rejecting it outright would break ~50 manifests at once. Naming
// each one is what makes the class fixable — the remedy is either
// `state_machine.transitions[].require_permission` (gate the transition) or
// `impl` (give the action a route that enforces it).
func checkUngatedActions(result *checkResult, manifests []manifest.RawManifest) {
	for _, m := range manifests {
		if !spec.IsEntityKind(spec.Kind(m.Kind)) {
			continue
		}
		specMap, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		es, err := manifest.RawSpecToEntitySpec(specMap)
		if err != nil || es == nil {
			continue
		}

		module := m.Metadata.Module
		plural := es.Plural
		if plural == "" {
			plural = m.Metadata.Name + "s"
		}

		// Which transitions already gate on a permission? Those names are
		// enforced on the PATCH path, so their action needs nothing more.
		gated := map[string]bool{}
		if es.StateMachine != nil {
			for _, t := range es.StateMachine.Transitions {
				if t.RequirePermission != "" && t.Action != "" {
					gated[t.Action] = true
				}
			}
		}

		for _, a := range es.Actions {
			if a.RequiredPermission == "" {
				continue // nothing declared → nothing to enforce
			}
			if a.Impl != nil {
				continue // a route exists, and it requires this permission
			}
			if gated[a.Name] {
				continue // the transition gate enforces it on PATCH
			}
			// Reserved lifecycle names DO have a generic route; its permission is
			// normally the conventional one, so a redirect here is suspicious but
			// not the same defect — reported only when it differs from the
			// conventional name.
			conventional := module + "." + plural + "." + a.Name
			declared := spec.QualifyPermission(a.RequiredPermission, module)
			if spec.IsReservedAction(a.Name) {
				if declared == conventional {
					continue // equals the route's own permission — no redirect
				}
			}
			result.add(m.Source, "warning",
				"action %q declares required_permission %q, but nothing enforces it: "+
					"the action has no `impl` (so it gets no route) and no transition "+
					"with `via: %s` sets `require_permission`. Anyone holding "+
					"%s.update can apply the transition this action names. Either gate "+
					"the transition (`state_machine.transitions[].require_permission`) "+
					"or give the action an `impl`.",
				a.Name, declared, a.Name, module)
		}
	}
}
