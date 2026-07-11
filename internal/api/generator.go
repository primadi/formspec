package api

import (
	"github.com/primadi/forma/internal/entity"
	"github.com/primadi/forma/pkg/spec"
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
	}

	return routes
}

// generateRESTRoutes creates REST route descriptors for one entity.
func generateRESTRoutes(module, name, plural string, exp spec.ExposeConfig, isSummary bool, disabled map[string]bool) []RouteDescriptor {
	var routes []RouteDescriptor

	// Build the set of allowed actions
	allowed := make(map[string]bool)
	for _, a := range exp.Actions {
		allowed[a] = true
	}

	// If no actions filter, default to all except delete
	useAll := len(exp.Actions) == 0

	// Path prefix: /api/{version}/{module}/{plural}
	pathPrefix := "/api/v1/" + module + "/" + plural

	for _, std := range StandardRESTActions {
		// Disabled standard actions never generate a route (§11.1)
		if disabled[std.Action] {
			continue
		}

		// Summary entities: only list + find
		if isSummary && (std.Action == "create" || std.Action == "update" || std.Action == "delete") {
			continue
		}

		// Filter: if actions are specified, only include those
		if !useAll && !allowed[std.Action] {
			continue
		}

		// Default: skip delete unless explicitly enabled
		if useAll && std.Action == "delete" {
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
				// Skip standard CRUD actions — they're handled by generateRESTRoutes
				if isStandardAction(action.Name) {
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

// isStandardAction returns true if the action name is a standard CRUD action.
func isStandardAction(name string) bool {
	for _, std := range StandardRESTActions {
		if std.Action == name {
			return true
		}
	}
	return false
}

// mergeRoutes combines standard REST routes with custom action routes.
func mergeRoutes(restRoutes, customRoutes []RouteDescriptor) []RouteDescriptor {
	// Deduplicate by (module, entity, action)
	seen := make(map[string]bool)
	result := make([]RouteDescriptor, 0, len(restRoutes)+len(customRoutes))

	for _, rd := range restRoutes {
		key := rd.Module + "/" + rd.Entity + "/" + rd.Action
		seen[key] = true
		result = append(result, rd)
	}

	for _, rd := range customRoutes {
		key := rd.Module + "/" + rd.Entity + "/" + rd.Action
		if !seen[key] {
			result = append(result, rd)
		}
	}

	return result
}
