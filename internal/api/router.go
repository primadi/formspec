package api

import (
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/primadi/forma/internal/action"
	"github.com/primadi/forma/internal/entity"
	"github.com/primadi/forma/internal/ui"
	"github.com/primadi/forma/pkg/spec"
)

// RouterBuilder constructs a Forma API router.
// It wires middleware, route generation, and handler dispatch together.
type RouterBuilder struct {
	registry   *entity.Registry
	routes     []RouteDescriptor
	factory    *HandlerFactory
	dispatcher *action.Dispatcher
	uiRegistry *ui.Registry
	webDir     string // static SPA root (web/dist); empty = no static serving
	webFS      fs.FS  // embedded SPA (embed.FS); empty = no static serving
	hub        *WSHub
}

// NewRouterBuilder creates a new router builder backed by the entity registry.
func NewRouterBuilder(registry *entity.Registry) *RouterBuilder {
	b := &RouterBuilder{
		registry: registry,
		factory:  NewHandlerFactory(registry),
		hub:      NewWSHub(),
	}
	// Entity-spec lookup enables sort/filter validation on list endpoints.
	b.factory.SetSpecLookup(func(module, name string) (*spec.EntitySpec, bool) {
		info, ok := registry.GetEntity(module, name)
		if !ok || info.EntitySpec == nil {
			return nil, false
		}
		return info.EntitySpec, true
	})
	return b
}

// SetDispatcher sets the action dispatcher used for custom action execution.
// Call this before BuildHTTP to wire the action execution engine.
func (b *RouterBuilder) SetDispatcher(d *action.Dispatcher) {
	b.dispatcher = d
	b.factory.SetDispatcher(d)
}

// SetDeliveryDeps wires the event-delivery dependencies (hub, outbox, event
// log) used by HandleCreate/HandleUpdate/HandleCustomAction to fan out
// declared events after a successful action.
func (b *RouterBuilder) SetDeliveryDeps(deps action.DeliveryDeps) {
	b.factory.SetDeliveryDeps(deps)
}

// SetUIRegistry wires the frontend UI registry; enables the Meta API
// (/{ws}/api/v1/_meta/...). Call before BuildHTTP.
func (b *RouterBuilder) SetUIRegistry(r *ui.Registry) {
	b.uiRegistry = r
}

// SetWebDir enables static SPA serving from dir (typically web/dist) at
// /{ws}/_admin and /{ws}/app with an index.html fallback for client-side
// routes. Call before BuildHTTP.
func (b *RouterBuilder) SetWebDir(dir string) {
	b.webDir = dir
}

// SetWebFS enables static SPA serving from an embed.FS (or any fs.FS) at
// /{ws}/_admin and /{ws}/app with index.html fallback. Takes precedence
// over SetWebDir. Call before BuildHTTP.
func (b *RouterBuilder) SetWebFS(spaFS fs.FS) {
	b.webFS = spaFS
}

// Hub returns the websocket hub backing /_ws, so callers (resource/forma.go)
// can wire it into action.DeliveryDeps for event delivery.
func (b *RouterBuilder) Hub() *WSHub {
	return b.hub
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
	r.Use(WorkspaceMiddleware)
	r.Use(AuthMiddleware)

	// Workspace-prefixed API routes
	r.Route("/{workspace}", func(r chi.Router) {
		r.Route("/api/v1", func(r chi.Router) {
			// Meta API — read-only UI manifests + identity (Frontend §1.1).
			// No RequirePermission wrapper: the bundle itself is filtered per
			// caller, and /_meta/me is by definition caller-scoped.
			r.Route("/_meta", func(r chi.Router) {
				r.Get("/ui", b.HandleMetaUI())
				r.Get("/me", b.HandleMetaMe())
				r.Get("/entities/{module}/{name}", b.HandleMetaEntity())
			})

			// Realtime event push (Frontend kanban/board realtime: true).
			r.Get("/_ws", b.HandleWS())

			for _, rd := range b.routes {
				if rd.Protocol != ProtocolREST {
					continue
				}
				b.registerRoute(r, rd)
			}
		})

		// Static SPA (renderer) — /{ws}/_admin and /{ws}/app.
		// Priority: webFS (embed) > webDir (file system) > none.
		var spa http.HandlerFunc
		switch {
		case b.webFS != nil:
			spa = spaHandlerFS(b.webFS)
		case b.webDir != "":
			spa = spaHandler(b.webDir)
		}
		if spa != nil {
			r.Get("/_admin", spa)
			r.Get("/_admin/*", spa)
			r.Get("/app", spa)
			r.Get("/app/*", spa)
		}
	})

	// Root-level static assets (Vite generates /assets/... absolute paths).
	// These need to be accessible at the root so the SPA index.html can find them.
	if b.webFS != nil {
		assetHandler := spaAssetHandler(b.webFS)
		r.Get("/assets/*", assetHandler)
		r.Get("/favicon.svg", assetHandler)
		r.Get("/icons.svg", assetHandler)
		r.Get("/manifest.json", assetHandler)
	} else if b.webDir != "" {
		assetHandler := spaAssetHandlerDir(b.webDir)
		r.Get("/assets/*", assetHandler)
		r.Get("/favicon.svg", assetHandler)
		r.Get("/icons.svg", assetHandler)
		r.Get("/manifest.json", assetHandler)
	}

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
	// rd.Path is already the correct, fully-qualified path for every route
	// kind (standard CRUD and custom actions alike — see generator.go). This
	// router is mounted under /{workspace}/api/v1, so strip that prefix to
	// get the pattern relative to this sub-router.
	pattern := strings.TrimPrefix(rd.Path, "/api/v1")

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

// Routes returns the generated route descriptors.
func (b *RouterBuilder) Routes() []RouteDescriptor {
	return b.routes
}

// RouteCount returns the number of registered routes.
func (b *RouterBuilder) RouteCount() int {
	return len(b.routes)
}

// spaHandler serves static renderer assets from dir with an index.html
// fallback: any path that doesn't match a real file gets index.html, so
// client-side routes (/{ws}/app/orders/42) survive a hard refresh.
func spaHandler(dir string) http.HandlerFunc {
	fs := http.FileServer(http.Dir(dir))
	return func(w http.ResponseWriter, r *http.Request) {
		// Strip /{workspace}/_admin or /{workspace}/app → asset path.
		path := chi.URLParam(r, "*")
		if path == "" {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}

		clean := filepath.Clean("/" + path) // confine to dir (no ..)
		full := filepath.Join(dir, clean)
		if info, err := os.Stat(full); err != nil || info.IsDir() {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}

		// Serve the real asset with the request path rewritten for FileServer.
		r2 := new(http.Request)
		*r2 = *r
		r2.URL = new(url.URL)
		*r2.URL = *r.URL
		r2.URL.Path = clean
		fs.ServeHTTP(w, r2)
	}
}

// spaHandlerFS serves static renderer assets from an embed.FS with an
// index.html fallback for client-side routing.
func spaHandlerFS(spaFS fs.FS) http.HandlerFunc {
	fsrv := http.FileServer(http.FS(spaFS))
	return func(w http.ResponseWriter, r *http.Request) {
		path := chi.URLParam(r, "*")
		if path == "" {
			serveFileFS(w, r, spaFS, "index.html")
			return
		}

		clean := filepath.Clean("/" + path)
		clean = strings.TrimPrefix(clean, "/")
		if _, err := fs.Stat(spaFS, clean); err != nil {
			serveFileFS(w, r, spaFS, "index.html")
			return
		}

		r2 := new(http.Request)
		*r2 = *r
		r2.URL = new(url.URL)
		*r2.URL = *r.URL
		r2.URL.Path = "/" + clean
		fsrv.ServeHTTP(w, r2)
	}
}

// serveFileFS serves a single file from an fs.FS (like http.ServeFile for embed).
func serveFileFS(w http.ResponseWriter, r *http.Request, spaFS fs.FS, name string) {
	data, err := fs.ReadFile(spaFS, name)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	// Guess content type based on extension
	ct := mimeTypeByExtension(name)
	w.Header().Set("Content-Type", ct)
	w.Write(data)
}

// mimeTypeByExtension returns a MIME type for common web file extensions.
func mimeTypeByExtension(name string) string {
	switch {
	case strings.HasSuffix(name, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(name, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(name, ".js"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(name, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(name, ".png"):
		return "image/png"
	case strings.HasSuffix(name, ".woff2"):
		return "font/woff2"
	case strings.HasSuffix(name, ".json"):
		return "application/json"
	default:
		return "application/octet-stream"
	}
}

// spaAssetHandler serves static assets from an embed.FS at root level
// (/assets/*, /favicon.svg, etc.) for Vite-generated absolute paths.
// Uses serveFileFS to ensure correct Content-Type headers.
func spaAssetHandler(spaFS fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := chi.URLParam(r, "*")
		if path == "" {
			// /favicon.svg or /icons.svg — serve from root of FS
			name := strings.TrimPrefix(r.URL.Path, "/")
			serveFileFS(w, r, spaFS, name)
			return
		}
		// /assets/index-xxx.js — chi strips /assets/, so path is the filename
		serveFileFS(w, r, spaFS, "assets/"+path)
	}
}

// spaAssetHandlerDir serves static assets from a file system directory at root level.
func spaAssetHandlerDir(dir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := chi.URLParam(r, "*")
		if path == "" {
			// /favicon.svg or /icons.svg — serve from root of directory
			http.ServeFile(w, r, filepath.Join(dir, filepath.Base(r.URL.Path)))
			return
		}
		// /assets/index-xxx.js
		http.ServeFile(w, r, filepath.Join(dir, "assets", path))
	}
}
