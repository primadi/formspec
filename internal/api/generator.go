package api

import (
	"strings"

	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// GenerateRoutes produces RouteDescriptors for all registered entities
// that have opted into external API exposure via spec.expose (D49).
func GenerateRoutes(registry *entity.Registry) []RouteDescriptor {
	var routes []RouteDescriptor

	for _, info := range registry.ListEntities() {
		specInfo, ok := registry.GetEntity(info.Module, info.Name)
		if !ok || specInfo.EntitySpec == nil {
			continue
		}

		es := specInfo.EntitySpec

		// No expose config → private entity, skip
		if len(es.Expose) == 0 {
			continue
		}

		plural := es.Plural
		if plural == "" {
			plural = info.Name + "s" // simple default
		}

		isSummary := es.Characteristic == spec.CharSummary

		// Standard actions disabled via `disabled: true` (§11.1) are removed
		// from every surface, equivalent to never existing.
		disabled := make(map[string]bool)
		for _, a := range es.Actions {
			if a.Disabled {
				disabled[a.Name] = true
			}
		}

		for _, exp := range es.Expose {
			if exp.Type == spec.ProtocolREST {
				routes = append(routes, generateRESTRoutes(info.Module, info.Name, plural, exp, isSummary, disabled)...)
			}
			// Future: grpc, ws
		}

		// Two-step idempotency prepare (todo 2.7.1): every server-sourced
		// idempotent action (create and custom) exposes a prepare endpoint
		// on the external surface.
		routes = append(routes, generatePrepareRoutes(info.Module, info.Name, plural, es, isSummary, disabled)...)
	}

	return routes
}

// GenerateUIRoutes produces RouteDescriptors for ALL registered entities on
// the `/_ui/entity/` surface (§8.1). Unlike GenerateRoutes, this does NOT
// check spec.expose — all entities are available on the UI surface regardless
// of their expose configuration. Permission gating still applies.
func GenerateUIRoutes(registry *entity.Registry) []RouteDescriptor {
	var routes []RouteDescriptor

	for _, info := range registry.ListEntities() {
		specInfo, ok := registry.GetEntity(info.Module, info.Name)
		if !ok || specInfo.EntitySpec == nil {
			continue
		}

		es := specInfo.EntitySpec

		plural := es.Plural
		if plural == "" {
			plural = info.Name + "s"
		}

		isSummary := es.Characteristic == spec.CharSummary

		// Standard actions disabled via `disabled: true` (§11.1)
		disabled := make(map[string]bool)
		for _, a := range es.Actions {
			if a.Disabled {
				disabled[a.Name] = true
			}
		}

		// UI surface uses the same internal logic; expose config is
		// ignored. All standard CRUD actions (including delete) are available
		// on the UI surface unless explicitly disabled in entity actions.
		uiActions := []string{"list", "find", "create", "update", "delete"}
		uiExp := spec.ExposeConfig{Type: spec.ProtocolREST, Actions: uiActions}
		routes = append(routes, generateRESTRoutes(info.Module, info.Name, plural, uiExp, isSummary, disabled)...)

		// Two-step idempotency prepare (todo 2.7.1) on the UI surface too —
		// the primary use case is browser double-submit on create.
		routes = append(routes, generatePrepareRoutes(info.Module, info.Name, plural, es, isSummary, disabled)...)
	}

	// Rewrite path prefixes from /api/v1/... to /_ui/entity/...
	for i := range routes {
		routes[i].Path = strings.Replace(routes[i].Path, "/api/v1/"+routes[i].Module+"/"+routes[i].Plural,
			"/_ui/entity/"+routes[i].Module+"/"+routes[i].Entity, 1)
	}

	return routes
}

// generateRESTRoutes creates REST route descriptors for one entity.
// Applies transitive gating (2.3.2) and auto-derives composite actions (2.3.6).
func generateRESTRoutes(module, name, plural string, exp spec.ExposeConfig, isSummary bool, disabled map[string]bool) []RouteDescriptor {
	var routes []RouteDescriptor

	// Apply transitive gating (2.3.2): submit disabled → cancel/amend implicitly disabled;
	// cancel disabled → amend implicitly disabled.
	fullDisabled := db.TransitiveDisabled(disabled)

	// Build the set of explicitly allowed actions from spec.expose.actions
	allowed := make(map[string]bool)
	for _, a := range exp.Actions {
		allowed[a] = true
	}

	// If no actions filter, default to all CRUD except delete (plus lifecycle if submit enabled)
	useAll := len(exp.Actions) == 0

	// Determine if submit is disabled (lifecycle-free entity → skip lifecycle actions)
	submitDisabled := fullDisabled["submit"]

	// Path prefix: /api/{version}/{module}/{plural}
	pathPrefix := "/api/v1/" + module + "/" + plural

	for _, std := range StandardRESTActions {
		// Disabled standard actions never generate a route (§11.1)
		if fullDisabled[std.Action] {
			continue
		}

		// Summary entities: only list + find
		if isSummary && (std.Action == "create" || std.Action == "update" || std.Action == "delete") {
			continue
		}

		// Lifecycle-free entities: skip submit/cancel/amend routes
		if submitDisabled && (std.Action == "submit" || std.Action == "cancel" || std.Action == "amend") {
			continue
		}

		// Filter: if actions are specified, only include those
		if !useAll && !allowed[std.Action] {
			continue
		}

		// Default: skip delete and lifecycle unless explicitly enabled
		if useAll && (std.Action == "delete" || std.Action == "submit" || std.Action == "cancel" || std.Action == "amend") {
			continue
		}

		// Build fully qualified permission: {module}.{plural}.{action}
		perm := module + "." + plural + "." + std.PermissionAction

		routes = append(routes, RouteDescriptor{
			Module:             module,
			Entity:             name,
			Plural:             plural,
			Action:             std.Action,
			Method:             std.Method,
			Path:               pathPrefix + std.PathSuffix,
			Protocol:           spec.ProtocolREST,
			Handler:            "auto",
			RequiredPermission: perm,
		})
	}

	return routes
}

// generatePrepareRoutes creates two-step idempotency prepare routes for
// server-sourced idempotent actions (todo 2.7.1, 01-core-basic §5):
//
//	POST /api/v1/{module}/{plural}/create/prepare     (idempotent create)
//	POST /api/v1/{module}/{plural}/{action}/prepare   (idempotent custom action)
//
// Only actions declared `idempotent: true` with `idempotency_key.from: server`
// expose a prepare endpoint — header/param-sourced actions let the client
// supply its own key and need no prepare step. Lifecycle actions
// (submit/cancel/amend) are excluded: their idempotency is guarded by the
// state machine, not a client key.
func generatePrepareRoutes(module, name, plural string, es *spec.EntitySpec, isSummary bool, disabled map[string]bool) []RouteDescriptor {
	var routes []RouteDescriptor

	if es == nil || isSummary {
		return routes
	}

	pathPrefix := "/api/v1/" + module + "/" + plural

	for _, a := range es.Actions {
		// Only server-sourced idempotent actions (create + custom) get a
		// prepare endpoint.
		if a.Disabled || !a.Idempotent || a.IdempotencyKey == nil || a.IdempotencyKey.From != "server" {
			continue
		}
		switch a.Name {
		case "submit", "cancel", "amend":
			continue
		case "create":
			if disabled["create"] {
				continue
			}
			routes = append(routes, RouteDescriptor{
				Module:             module,
				Entity:             name,
				Plural:             plural,
				Action:             "create",
				Method:             "POST",
				Path:               pathPrefix + "/create/prepare",
				Protocol:           spec.ProtocolREST,
				Handler:            "prepare",
				RequiredPermission: module + "." + plural + ".create",
			})
		default:
			perm := a.RequiredPermission
			if perm == "" {
				perm = module + "." + plural + "." + a.Name
			}
			routes = append(routes, RouteDescriptor{
				Module:             module,
				Entity:             name,
				Plural:             plural,
				Action:             a.Name,
				Method:             "POST",
				// Literal action name in the path so chi's static-segment
				// priority disambiguates /{action}/prepare from the custom
				// action route /{id}/{action}.
				Path:               pathPrefix + "/" + a.Name + "/prepare",
				Protocol:           spec.ProtocolREST,
				Handler:            "prepare",
				RequiredPermission: perm,
			})
		}
	}

	return routes
}

// GenerateCustomActionRoutes creates route descriptors for custom (non-CRUD) actions
// that have an impl type and are exposed via the REST protocol.
//
// Custom actions are POST-only and scoped to a specific entity:
//
//	POST /api/v1/{module}/{plural}/{id}/{action}
func GenerateCustomActionRoutes(registry *entity.Registry) []RouteDescriptor {
	var routes []RouteDescriptor

	for _, info := range registry.ListEntities() {
		specInfo, ok := registry.GetEntity(info.Module, info.Name)
		if !ok || specInfo.EntitySpec == nil {
			continue
		}

		es := specInfo.EntitySpec

		// Only generate custom action routes if the entity is exposed
		if len(es.Expose) == 0 {
			continue
		}

		plural := es.Plural
		if plural == "" {
			plural = info.Name + "s"
		}

		for _, exp := range es.Expose {
			if exp.Type != spec.ProtocolREST {
				continue
			}

			for _, action := range es.Actions {
				// Standard CRUD actions (list/find/create/update/delete) are handled
				// by generateRESTRoutes. Lifecycle actions (submit/cancel/amend) with
				// a custom impl constitute custom state-machine actions, not document
				// lifecycle actions, and are generated here as custom routes.
				if isStandardCrudAction(action.Name) {
					continue
				}

				// Skip disabled or no-impl actions
				if action.Disabled || action.Impl == nil {
					continue
				}

				// Build permission: prefer action.RequiredPermission, else derive
				perm := action.RequiredPermission
				if perm == "" {
					perm = info.Module + "." + plural + "." + action.Name
				}

				routes = append(routes, RouteDescriptor{
					Module:             info.Module,
					Entity:             info.Name,
					Plural:             plural,
					Action:             action.Name,
					Method:             "POST",
					Path:               "/api/v1/" + info.Module + "/" + plural + "/{id}/" + action.Name,
					Protocol:           spec.ProtocolREST,
					Handler:            "custom",
					RequiredPermission: perm,
				})
			}
		}
	}

	return routes
}

// GenerateUICustomActionRoutes creates route descriptors for custom actions
// on the UI surface (/_ui/entity/). Unlike GenerateCustomActionRoutes, this
// includes ALL entities regardless of spec.expose.
func GenerateUICustomActionRoutes(registry *entity.Registry) []RouteDescriptor {
	var routes []RouteDescriptor

	for _, info := range registry.ListEntities() {
		specInfo, ok := registry.GetEntity(info.Module, info.Name)
		if !ok || specInfo.EntitySpec == nil {
			continue
		}

		es := specInfo.EntitySpec
		plural := es.Plural
		if plural == "" {
			plural = info.Name + "s"
		}

		for _, action := range es.Actions {
			// Standard CRUD actions (list/find/create/update/delete) are handled
			// by generateRESTRoutes. Lifecycle actions (submit/cancel/amend) with
			// a custom impl constitute custom actions and are generated here.
			if isStandardCrudAction(action.Name) {
				continue
			}
			if action.Disabled || action.Impl == nil {
				continue
			}

			perm := action.RequiredPermission
			if perm == "" {
				perm = info.Module + "." + plural + "." + action.Name
			}

			routes = append(routes, RouteDescriptor{
				Module:             info.Module,
				Entity:             info.Name,
				Plural:             plural,
				Action:             action.Name,
				Method:             "POST",
				Path:               "/_ui/entity/" + info.Module + "/" + info.Name + "/{id}/" + action.Name,
				Protocol:           spec.ProtocolREST,
				Handler:            "custom",
				RequiredPermission: perm,
			})
		}
	}

	return routes
}

// isStandardCrudAction returns true only for standard CRUD action names
// (list/find/create/update/delete). Lifecycle actions (submit/cancel/amend)
// are excluded because entities may define them as custom state-machine
// actions with a script impl.
func isStandardCrudAction(name string) bool {
	switch name {
	case "list", "find", "create", "update", "delete":
		return true
	}
	return false
}

// mergeRoutes combines multiple route slices, deduplicating by path + method.
// Different path prefixes (e.g. /api/v1/ vs /_ui/entity/) are kept separate.
func mergeRoutes(slices ...[]RouteDescriptor) []RouteDescriptor {
	seen := make(map[string]bool)
	result := make([]RouteDescriptor, 0)

	for _, slice := range slices {
		for _, rd := range slice {
			key := rd.Module + "/" + rd.Entity + "/" + rd.Action + "/" + rd.Path
			if !seen[key] {
				seen[key] = true
				result = append(result, rd)
			}
		}
	}
	return result
}
