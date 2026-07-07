package api

import (
	"net/http"

	"github.com/forma/forma/internal/action"
	"github.com/forma/forma/internal/entity"
	"github.com/forma/forma/pkg/spec"
	"github.com/go-chi/chi/v5"
)

// RouterBuilder constructs a Forma API router.
// It wires middleware, route generation, and handler dispatch together.
type RouterBuilder struct {
	registry   *entity.Registry
	routes     []RouteDescriptor
	factory    *HandlerFactory
	dispatcher *action.Dispatcher
}

// NewRouterBuilder creates a new router builder backed by the entity registry.
func NewRouterBuilder(registry *entity.Registry) *RouterBuilder {
	return &RouterBuilder{
		registry: registry,
		factory:  NewHandlerFactory(registry),
	}
}

// SetDispatcher sets the action dispatcher used for custom action execution.
// Call this before BuildHTTP to wire the action execution engine.
func (b *RouterBuilder) SetDispatcher(d *action.Dispatcher) {
	b.dispatcher = d
	b.factory.SetDispatcher(d)
}

// BuildRoutes generates route descriptors and stores them in the builder.
// Includes both standard CRUD routes and custom action routes.
func (b *RouterBuilder) BuildRoutes() {
	restRoutes := GenerateRoutes(b.registry)
	customRoutes := GenerateCustomActionRoutes(b.registry)
	b.routes = mergeRoutes(restRoutes, customRoutes)
}

// BuildHTTP constructs the chi router with all middleware and route registration.
// Routes are prefixed with workspace slug (D50): /{workspace}/api/v1/...
func (b *RouterBuilder) BuildHTTP() http.Handler {
	r := chi.NewRouter()

	// Global middleware stack
	r.Use(RecoveryMiddleware)
	r.Use(LoggingMiddleware)
	r.Use(CORSMiddleware)
	r.Use(RequestIDMiddleware)
	r.Use(TenantMiddleware)
	r.Use(AuthMiddleware)

	// Workspace-prefixed API routes
	r.Route("/{workspace}", func(r chi.Router) {
		r.Route("/api/v1", func(r chi.Router) {
			for _, rd := range b.routes {
				if rd.Protocol != ProtocolREST {
					continue
				}
				b.registerRoute(r, rd)
			}
		})
	})

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	return r
}

// registerRoute registers a single route descriptor on the chi router.
// If RequiredPermission is set, the route is wrapped with RequirePermission
// middleware via chi's r.With() pattern.
func (b *RouterBuilder) registerRoute(r chi.Router, rd RouteDescriptor) {
	// Route pattern: /{module}/{plural}[/{id}]
	pattern := "/" + rd.Module + "/" + rd.Plural + rd.PathSuffix()

	// Resolve the handler for this action
	var handler http.HandlerFunc
	switch rd.Handler {
	case "auto":
		switch rd.Action {
		case "list":
			handler = b.factory.HandleList(rd.Module, rd.Entity)
		case "find":
			handler = b.factory.HandleFind(rd.Module, rd.Entity)
		case "create":
			handler = b.factory.HandleCreate(rd.Module, rd.Entity)
		case "update":
			handler = b.factory.HandleUpdate(rd.Module, rd.Entity)
		case "delete":
			handler = b.factory.HandleDelete(rd.Module, rd.Entity)
		default:
			return // unknown auto action, skip
		}
	case "custom":
		// Look up entity spec to find the action definition
		specInfo, ok := b.registry.GetEntity(rd.Module, rd.Entity)
		if !ok || specInfo.EntitySpec == nil {
			handler = func(w http.ResponseWriter, r *http.Request) {
				writeError(w, http.StatusNotFound, "NOT_FOUND",
					"entity not found: "+rd.Module+"/"+rd.Entity)
			}
			break
		}

		// Find the action by name
		var actionSpec *spec.Action
		for i, a := range specInfo.EntitySpec.Actions {
			if a.Name == rd.Action {
				actionSpec = &specInfo.EntitySpec.Actions[i]
				break
			}
		}

		if actionSpec == nil {
			handler = func(w http.ResponseWriter, r *http.Request) {
				writeError(w, http.StatusNotFound, "NOT_FOUND",
					"action not found: "+rd.Action)
			}
			break
		}

		handler = b.factory.HandleCustomAction(rd.Module, rd.Entity, rd.Action, *actionSpec)
	default:
		return // unknown handler type, skip
	}

	// Select the chi sub-router: with or without permission middleware
	sub := r
	if rd.RequiredPermission != "" {
		sub = r.With(RequirePermission(rd.RequiredPermission))
	}

	// Register on the sub-router
	switch rd.Method {
	case "GET":
		sub.Get(pattern, handler)
	case "POST":
		sub.Post(pattern, handler)
	case "PATCH":
		sub.Patch(pattern, handler)
	case "DELETE":
		sub.Delete(pattern, handler)
	default:
		sub.Get(pattern, handler)
	}
}

// PathSuffix returns the ID suffix part of the path if needed.
func (rd RouteDescriptor) PathSuffix() string {
	switch rd.Action {
	case "find", "update", "delete":
		return "/{id}"
	default:
		return ""
	}
}

// Routes returns the generated route descriptors.
func (b *RouterBuilder) Routes() []RouteDescriptor {
	return b.routes
}

// RouteCount returns the number of registered routes.
func (b *RouterBuilder) RouteCount() int {
	return len(b.routes)
}
