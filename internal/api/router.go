package api

import (
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/primadi/formspec/internal/action"
	formspec_app "github.com/primadi/formspec/internal/app"
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/job"
	"github.com/primadi/formspec/internal/observability"
	"github.com/primadi/formspec/internal/service"
	"github.com/primadi/formspec/internal/ui"
	"github.com/primadi/formspec/internal/webhook"
	"github.com/primadi/formspec/internal/workflow"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// RouterBuilder constructs a FormSpec API router.
// It wires middleware, route generation, and handler dispatch together.
type RouterBuilder struct {
	registry      *entity.Registry
	svcRegistry   *service.Registry // kind: Service manifests (todo 7.1)
	whRegistry    *webhook.Registry // kind: Webhook manifests (todo 7.6)
	routes        []RouteDescriptor
	factory       *HandlerFactory
	dispatcher    *action.Dispatcher
	uiRegistry    *ui.Registry
	webDir        string // static SPA root (renderers/react-shadcn/dist); empty = no static serving
	webFS         fs.FS  // embedded SPA (embed.FS); empty = no static serving
	hub           *WSHub
	apps          map[string]*formspec_app.ResolvedApp // resolved kind: App manifests, keyed by name (Core §4.4)
	settings      *spec.Settings                       // resolved global settings namespace (spec §10)
	specVersionFn func() int64                         // returns the current spec version (for Meta API polling)
	// enableAPIAuth mounts /api/v1/auth/* (login/refresh) on the external
	// surface. Default false — auth lives on the UI surface (/_ui/auth/*),
	// which is always available; /api/v1 is deny-by-default for external
	// services (01-core-basic.md §8.2). Opt-in for programmatic clients.
	enableAPIAuth bool

	// Observability wiring (todo 8.2, spec platform/09-observability.md).
	corsOrigins []string               // CORS allow-list (todo 8.1.5); empty = permissive dev
	logger      *observability.Logger  // structured JSON-lines logger (todo 8.2.1)
	metrics     *observability.Metrics // Prometheus metric set (todo 8.2.4)
	health      *observability.Health  // machine-readable health registry (todo 8.2.6)
}

// NewRouterBuilder creates a new router builder backed by the entity registry.
func NewRouterBuilder(registry *entity.Registry) *RouterBuilder {
	b := &RouterBuilder{
		registry: registry,
		factory:  NewHandlerFactory(registry),
		hub:      NewWSHub(registry),
	}
	// Per-resource/per-action rate limiting (todo 7.12).
	b.factory.SetResourceRateLimiter(NewResourceRateLimiter())
	// Entity-spec lookup enables sort/filter validation on list endpoints.
	b.factory.SetSpecLookup(func(module, name string) (*spec.EntitySpec, bool) {
		info, ok := registry.GetEntity(module, name)
		if !ok || info.EntitySpec == nil {
			return nil, false
		}
		return info.EntitySpec, true
	})
	// Entity-spec-directory lookup enables hook scripts on create/update to
	// resolve refs relative to the entity's own YAML directory (same as
	// HandleCustomAction's router-passed specDir).
	b.factory.SetSpecDirLookup(func(module, name string) (string, bool) {
		info, ok := registry.GetEntity(module, name)
		if !ok || info.Source == "" {
			return "", false
		}
		return filepath.Dir(strings.SplitN(info.Source, "#", 2)[0]), true
	})
	return b
}

// SetDispatcher sets the action dispatcher used for custom action execution.
// Call this before BuildHTTP to wire the action execution engine.
func (b *RouterBuilder) SetDispatcher(d *action.Dispatcher) {
	b.dispatcher = d
	b.factory.SetDispatcher(d)
}

// SetServiceRegistry sets the kind: Service registry used to generate
// stateless Service action routes (todo 7.1).
func (b *RouterBuilder) SetServiceRegistry(s *service.Registry) {
	b.svcRegistry = s
	b.factory.SetServiceRegistry(s)
}

// SetWebhookRegistry sets the kind: Webhook registry used to generate
// verified inbound webhook routes (todo 7.6).
func (b *RouterBuilder) SetWebhookRegistry(w *webhook.Registry) {
	b.whRegistry = w
	b.factory.SetWebhookRegistry(w)
}

// SetWebhookKeyResolver wires the config-backed key resolver used to look up
// webhook HMAC secrets / static tokens (todo 7.6).
func (b *RouterBuilder) SetWebhookKeyResolver(k webhook.KeyResolver) {
	b.factory.SetWebhookKeyResolver(k)
}

// SetWorkflowRegistry sets the kind: Workflow registry used to intercept
// state-machine transitions for approval (todo 7.4).
func (b *RouterBuilder) SetWorkflowRegistry(w *workflow.Registry) {
	b.factory.SetWorkflowRegistry(w)
}

// SetWorkflowApprovalStore wires the approval store used to persist
// in-flight approval requests (todo 7.4).
func (b *RouterBuilder) SetWorkflowApprovalStore(s *db.WorkflowApprovalStore) {
	b.factory.SetWorkflowApprovalStore(s)
}

// SetAuditWriter wires the audit writer used to record workflow approval
// decisions as signed statements in the audit trail (todo 7.4.6).
func (b *RouterBuilder) SetAuditWriter(w AuditWriter) {
	b.factory.SetAuditWriter(w)
}

// SetEntityCache wires the optional read-through find-by-id cache (Fase 14).
func (b *RouterBuilder) SetEntityCache(c *EntityCache) {
	b.factory.SetEntityCache(c)
}

// SetDeliveryDeps wires the event-delivery dependencies (hub, outbox, event
// log) used by HandleCreate/HandleUpdate/HandleCustomAction to fan out
// declared events after a successful action.
func (b *RouterBuilder) SetDeliveryDeps(deps action.DeliveryDeps) {
	b.factory.SetDeliveryDeps(deps)
}

// SetIdempotencyStore wires the idempotency-key store used to enforce
// idempotent actions and serve the prepare endpoint (todo 2.7).
func (b *RouterBuilder) SetIdempotencyStore(store *db.IdempotencyStore) {
	b.factory.SetIdempotencyStore(store)
}

// SetJobTracker wires the async job tracker (todo 7.13) into the handler
// factory so tracked async actions return 202 + job_id and the job status
// polling route is served.
func (b *RouterBuilder) SetJobTracker(t *job.Tracker) {
	b.factory.SetJobTracker(t)
}

// SetStorageResolver wires the object-store resolver used by the file
// upload/download routes (todo 7.17.1). When nil, those routes return 503.
func (b *RouterBuilder) SetStorageResolver(fn func() (Storage, error)) {
	b.factory.SetStorageResolver(fn)
}

// SetUploadLimitMB wires the global upload size limit in MB
// (todo 7.17.7). Per-field max_size_mb lowers it further.
func (b *RouterBuilder) SetUploadLimitMB(mb int) {
	b.factory.SetUploadLimitMB(mb)
}

// SetDownloadLimitMB wires the global download size limit in MB
// (todo 7.17.7). Per-field max_download_mb lowers it further.
func (b *RouterBuilder) SetDownloadLimitMB(mb int) {
	b.factory.SetDownloadLimitMB(mb)
}

// SetLinkStore wires the storage-link store used by the link issue/consume
// routes (todo 7.17.6). When nil, link routes return 503.
func (b *RouterBuilder) SetLinkStore(s *db.StorageLinkStore) {
	b.factory.SetLinkStore(s)
}

// SetAssetRoots wires the manifest roots used to resolve module asset files
// (todo 5.9.1).
func (b *RouterBuilder) SetAssetRoots(roots []string) {
	b.factory.SetAssetRoots(roots)
}

// SetUIRegistry wires the frontend UI registry; enables the Meta API
// (/{ws}/_ui/_meta/...). Call before BuildHTTP.
func (b *RouterBuilder) SetUIRegistry(r *ui.Registry) {
	b.uiRegistry = r
}

// SetApps wires the resolved kind: App manifests (internal/app.Resolve) —
// enables /_meta/apps and app-scoped /_meta/ui bundles. A workspace MAY
// resolve to more than one App; they all serve simultaneously, distinguished
// by the `app` query param / their own root_url (Core §4.4).
func (b *RouterBuilder) SetApps(apps map[string]*formspec_app.ResolvedApp) {
	b.apps = apps
}

// SetSettings wires the resolved global settings namespace (spec §10). It is
// exposed on every /_meta/ui bundle so renderers read formatting/presentation
// defaults instead of guessing per component.
func (b *RouterBuilder) SetSettings(s *spec.Settings) {
	b.settings = s
	b.factory.SetSettings(s)
}

// publicEntities returns the set of "module/entity" keys mounted by any
// `access: public` App (frontend/05-app-kinds.md §1). A public App's surface
// is served anonymously, so the entities it mounts get anonymous read +
// create on the UI surface.
func (b *RouterBuilder) publicEntities() map[string]bool {
	out := map[string]bool{}
	for _, app := range b.apps {
		if app.Spec.Access != spec.AppAccessPublic {
			continue
		}
		for module := range app.Modules {
			// Mark the whole module public — the App author chose
			// `access: public` knowing the surface is anonymous.
			out[module+"/*"] = true
		}
	}
	return out
}

// isPublicEntity reports whether module/entity is mounted by a public App
// (see publicEntities).
func (b *RouterBuilder) isPublicEntity(module, entity string) bool {
	pub := b.publicEntities()
	if pub[module+"/*"] {
		return true
	}
	return pub[module+"/"+entity]
}

// SetWebDir enables static SPA serving from dir (typically renderers/react-shadcn/dist) at
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

// Hub returns the websocket hub backing /_ws, so callers (resource/formspec.go)
// can wire it into action.DeliveryDeps for event delivery.
func (b *RouterBuilder) Hub() *WSHub {
	return b.hub
}

// SetHub replaces the websocket hub. Used during spec reload (ReloadSpec)
// to preserve existing WebSocket connections across a router rebuild.
func (b *RouterBuilder) SetHub(h *WSHub) {
	b.hub = h
}

// SetSpecVersionFn wires the spec version function. Called by the Meta API
// version handler so the frontend can detect when the meta bundle changed.
func (b *RouterBuilder) SetSpecVersionFn(fn func() int64) {
	b.specVersionFn = fn
}

// SetEnableAPIAuth mounts /api/v1/auth/* (login/refresh) on the external
// surface. Default false — auth lives on the always-available UI surface
// (/_ui/auth/*); /api/v1 is deny-by-default for external services.
func (b *RouterBuilder) SetEnableAPIAuth(enabled bool) {
	b.enableAPIAuth = enabled
}

// SetCORSOrigins sets the CORS origin allow-list (todo 8.1.5). Empty list
// keeps the permissive dev behavior (`*`); production must pass explicit
// origins.
func (b *RouterBuilder) SetCORSOrigins(origins []string) {
	b.corsOrigins = origins
}

// SetLogger wires the structured JSON-lines logger (todo 8.2.1). When nil,
// the legacy text logging is used (dev convenience).
func (b *RouterBuilder) SetLogger(l *observability.Logger) {
	b.logger = l
}

// SetMetrics wires the Prometheus metric set (todo 8.2.4). When non-nil,
// requests are instrumented and the admin listener can expose /metrics.
func (b *RouterBuilder) SetMetrics(m *observability.Metrics) {
	b.metrics = m
}

// SetHealth wires the machine-readable health registry (todo 8.2.6).
// When non-nil, GET /health returns {status, reasons, checked_at} instead
// of the static dev response.
func (b *RouterBuilder) SetHealth(h *observability.Health) {
	b.health = h
}

// Metrics returns the wired metric set (may be nil).
func (b *RouterBuilder) Metrics() *observability.Metrics { return b.metrics }

// Health returns the wired health registry (may be nil).
func (b *RouterBuilder) Health() *observability.Health { return b.health }

// BuildRoutes generates route descriptors and stores them in the builder.
// Includes both external API (/api/v1/) and UI (/ _ui/entity/) routes,
// plus custom action routes for both surfaces.
func (b *RouterBuilder) BuildRoutes() {
	restRoutes := GenerateRoutes(b.registry)
	uiRoutes := GenerateUIRoutes(b.registry)
	customRoutes := GenerateCustomActionRoutes(b.registry)
	uiCustomRoutes := GenerateUICustomActionRoutes(b.registry)
	svcRoutes := GenerateServiceRoutes(b.svcRegistry)
	uiSvcRoutes := GenerateUIServiceRoutes(b.svcRegistry)
	whRoutes := GenerateWebhookRoutes(b.whRegistry)
	b.routes = mergeRoutes(restRoutes, uiRoutes, customRoutes, uiCustomRoutes, svcRoutes, uiSvcRoutes, whRoutes)
}

// BuildHTTP constructs the chi router with all middleware and route registration.
// Routes are prefixed with workspace slug (D50): /{workspace}/api/v1/...
func (b *RouterBuilder) BuildHTTP() http.Handler {
	r := chi.NewRouter()

	// Global middleware stack
	r.Use(RecoveryMiddleware)
	if b.logger != nil {
		r.Use(LoggingMiddleware(b.logger))
	} else {
		r.Use(legacyLoggingMiddleware)
	}
	r.Use(NewCORSMiddleware(b.corsOrigins))
	r.Use(RequestIDMiddleware)
	if b.metrics != nil {
		r.Use(MetricsMiddleware(b.metrics))
	}
	r.Use(WorkspaceMiddleware)
	r.Use(AuthMiddleware)

	// Workspace-prefixed API routes — two surfaces (§8):
	//   /{ws}/_ui/...        → UI (always available, session auth)
	//   /{ws}/api/v1/...     → external (deny-by-default, spec.expose)
	r.Route("/{workspace}", func(r chi.Router) {
		// JSON 404 for unmatched API routes within this workspace
		r.NotFound(func(w http.ResponseWriter, r *http.Request) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "endpoint not found")
		})
		// ─── UI surface (§8.1): /_ui/ ───
		r.Route("/_ui", func(r chi.Router) {
			// Auth endpoints — public (no auth required). Login issues the
			// initial token pair; refresh rotates it (todo 6.1.1/6.1.3).
			// Auth lives on the always-available UI surface (login is a UI
			// concern); /api/v1/auth is opt-in via EnableAPIAuth.
			r.Post("/auth/login", b.HandleLogin())
			r.Post("/auth/refresh", b.HandleRefresh())
			r.Post("/auth/register", b.HandleRegister())
			// Email verification (account pre-hijacking protection): verify
			// (public, token) + resend (authenticated).
			r.Post("/auth/verify-email", b.HandleVerifyEmail())
			r.Post("/auth/resend-verification", b.HandleResendVerification())
			// Self-service password management: change (authenticated) and
			// email-based reset (public, rate-limited).
			r.Post("/auth/change-password", b.HandleChangePassword())
			r.Post("/auth/forgot-password", b.HandleForgotPassword())
			r.Post("/auth/reset-password", b.HandleResetPassword())
			// Approve a pending user (approval registration policy) — admin
			// only (formspec.core.users.update).
			r.With(RequirePermission("formspec.core.users.update")).
				Post("/auth/approve", b.HandleApproveUser())
			// External auth (OAuth/OIDC, auth redesign Fase 5) — public.
			r.Get("/auth/oauth/{provider}/authorize", b.HandleOAuthAuthorize())
			r.Get("/auth/oauth/{provider}/callback", b.HandleOAuthCallback())
			// Explicit account linking (authenticated) — the signed-in user
			// links an external identity to their account.
			r.Post("/auth/oauth/{provider}/link", b.HandleOAuthLink())
			// Explicit account unlinking (authenticated) — the signed-in user
			// removes an external identity from their account.
			r.Post("/auth/oauth/{provider}/unlink", b.HandleOAuthUnlink())

			// First-run setup — public (no auth). GET reports whether the
			// workspace needs bootstrap (no users); POST creates the first
			// admin (self-hosted prod, no formspec-ctl needed).
			r.Get("/setup", b.HandleSetupStatus())
			r.Post("/setup", b.HandleSetup())

			// Meta API — read-only UI manifests + identity (Frontend §1.1).
			r.Route("/_meta", func(r chi.Router) {
				r.Get("/apps", b.HandleMetaApps())
				r.Get("/ui", b.HandleMetaUI())
				r.Get("/me", b.HandleMetaMe())
				r.Get("/version", b.HandleMetaVersion())
				r.Get("/entities/{module}/{name}", b.HandleMetaEntity())
			})

			// Realtime event push (Frontend kanban/board realtime: true).
			r.Get("/_ws", b.HandleWS())

			// Module asset files (custom UI components, todo 5.9.1).
			// Asset path is spec-root-relative
			// ("modules/{module}/assets/x.js") — the wildcard carries the
			// full path, read via chi.URLParam(r, "*").
			r.Get("/assets/*", b.factory.HandleAsset())

			// Server-side Print PDF generation (todo 5.13.2) — renders a
			// kind: Print manifest + record to PDF without a browser.
			r.Get("/print/{module}/{name}/{id}", b.HandlePrint())

			// Entity CRUD — all entities, regardless of spec.expose.
			// No SPA fallback here: /_ui/entity/* is a REST API surface that
			// must return JSON errors, not HTML. Client-side routing for the
			// SPA lives under /_admin and /app, not under /_ui/entity.
			r.Route("/entity", func(r chi.Router) {
				for _, rd := range b.routes {
					if rd.Protocol != ProtocolREST {
						continue
					}
					if !strings.HasPrefix(rd.Path, "/_ui/entity") {
						continue
					}
					pattern := strings.TrimPrefix(rd.Path, "/_ui/entity")
					// Public App entities: anonymous read + create on
					// the UI surface (frontend/05-app-kinds.md §1). Update/
					// delete stay permission-gated — they're admin ops that
					// live in a private App.
					if b.isPublicEntity(rd.Module, rd.Entity) &&
						(rd.Action == "list" || rd.Action == "find" || rd.Action == "create") {
						rd.RequiredPermission = "public"
					}
					b.registerRouteWithPattern(r, rd, pattern)
				}
				// File upload/download (todo 7.17.1) — generic per-entity routes;
				// the handler resolves module/entity from the path and enforces
				// permission dynamically.
				r.Post("/{module}/{entity}/{id}/{field}", b.factory.HandleFileUpload())
				r.Get("/{module}/{entity}/{id}/{field}", b.factory.HandleFileDownload())
				// Download-link issue (todo 7.17.6) + chunked upload (7.17.5).
				r.Post("/{module}/{entity}/{id}/{field}/link", b.factory.HandleLinkIssue())
				r.Post("/{module}/{entity}/{id}/{field}/upload/init", b.factory.HandleChunkInit())
				r.Post("/{module}/{entity}/{id}/{field}/upload/{uid}/part/{part}", b.factory.HandleChunkPart())
				r.Post("/{module}/{entity}/{id}/{field}/upload/{uid}/complete", b.factory.HandleChunkComplete())
			})

			// Link consume (todo 7.17.6) — token is the credential, anonymous
			// allowed. Registered on the UI surface (session-auth compatible).
			r.Get("/storage/link/{token}", b.factory.HandleLinkConsume())

			// Stateless Service actions on the UI surface (render-context
			// `source: api`, todo 7.1). Session-authenticated callers may
			// invoke services without the deny-by-default /api/v1 gate.
			r.Route("/service", func(r chi.Router) {
				for _, rd := range b.routes {
					if rd.Protocol != ProtocolREST {
						continue
					}
					if !strings.HasPrefix(rd.Path, "/_ui/service") {
						continue
					}
					pattern := strings.TrimPrefix(rd.Path, "/_ui/service")
					b.registerRouteWithPattern(r, rd, pattern)
				}
			})
		})

		// ─── External API surface (§8.2): /api/v1/ ───
		r.Route("/api/v1", func(r chi.Router) {
			// Auth endpoints on the external surface are OPT-IN (EnableAPIAuth)
			// — /api/v1 is deny-by-default for external services; the UI login
			// lives on /_ui/auth/* (always available).
			if b.enableAPIAuth {
				r.Post("/auth/login", b.HandleLogin())
				r.Post("/auth/refresh", b.HandleRefresh())
				r.Post("/auth/register", b.HandleRegister())
			}

			for _, rd := range b.routes {
				if rd.Protocol != ProtocolREST {
					continue
				}
				if !strings.HasPrefix(rd.Path, "/api/v1") {
					continue
				}
				b.registerRoute(r, rd)
			}

			// File upload/download (todo 7.17.1).
			r.Post("/{module}/{entity}/{id}/{field}", b.factory.HandleFileUpload())
			r.Get("/{module}/{entity}/{id}/{field}", b.factory.HandleFileDownload())
			// Download-link issue (todo 7.17.6) + chunked upload (7.17.5).
			r.Post("/{module}/{entity}/{id}/{field}/link", b.factory.HandleLinkIssue())
			r.Post("/{module}/{entity}/{id}/{field}/upload/init", b.factory.HandleChunkInit())
			r.Post("/{module}/{entity}/{id}/{field}/upload/{uid}/part/{part}", b.factory.HandleChunkPart())
			r.Post("/{module}/{entity}/{id}/{field}/upload/{uid}/complete", b.factory.HandleChunkComplete())
			// Link consume (todo 7.17.6) — token is the credential.
			r.Get("/storage/link/{token}", b.factory.HandleLinkConsume())
		})

		// Static SPA (renderer). Fixed mounts: /{ws}/_admin (admin surface,
		// not App-scoped) and /{ws}/app (legacy convention). Plus a dynamic
		// mount at every App's root_url (docs/plan/flexible-root-url.md) —
		// root_url is a free-form prefix inside the workspace, so a
		// single-App workspace can mount at "/" or "/barbershop".
		// Priority: webFS (embed) > webDir (file system) > none.
		var spa http.HandlerFunc
		switch {
		case b.webFS != nil:
			spa = spaHandlerFS(b.webFS)
		case b.webDir != "":
			spa = spaHandler(b.webDir)
		}
		if spa != nil {
			mounts := map[string]bool{"/_admin": true, "/app": true}
			for _, a := range b.apps {
				mounts[a.Spec.RootURL] = true
			}
			for _, m := range sortedStrings(mounts) {
				if m == "/" {
					// A root App owns the whole workspace subtree: serve the
					// SPA for every unmatched GET (API routes stay more
					// specific and win; non-GET still 404s as JSON).
					r.Get("/", spa)
					r.Get("/*", spa)
					continue
				}
				r.Get(m, spa)
				r.Get(m+"/*", spa)
			}
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

	// Health check — machine-readable when a health registry is wired
	// (todo 8.2.6, spec platform/09-observability.md §5); static dev
	// response otherwise.
	if b.health != nil {
		r.Handle("/health", b.health.Handler())
	} else {
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		})
	}

	return r
}

// legacyLoggingMiddleware keeps the pre-8.2 text logging for dev mode
// (no structured logger wired).
func legacyLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := observability.RequestIDFromContext(r.Context())
		fmt.Printf("[api] %s %s [%s]\n", r.Method, r.URL.Path, id)
		next.ServeHTTP(w, r)
	})
}

// NewAdminMux builds the administrative listener (todo 8.2.4): a separate
// mux from business traffic serving GET /metrics (Prometheus text
// exposition) and GET /health (machine-readable vocabulary). It never
// carries business data.
func NewAdminMux(m *observability.Metrics, h *observability.Health) http.Handler {
	mux := http.NewServeMux()
	if m != nil {
		mux.Handle("/metrics", m.Handler())
	}
	if h != nil {
		mux.Handle("/health", h.Handler())
	}
	return mux
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
		case "submit":
			handler = b.factory.HandleSubmit(rd.Module, rd.Entity)
		case "cancel":
			handler = b.factory.HandleCancel(rd.Module, rd.Entity)
		case "amend":
			handler = b.factory.HandleAmend(rd.Module, rd.Entity)
		case "deactivate":
			handler = b.factory.HandleDeactivate(rd.Module, rd.Entity)
		case "reactivate":
			handler = b.factory.HandleReactivate(rd.Module, rd.Entity)
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

		// Extract entity spec directory from source path
		specDir := ""
		if src := specInfo.Source; src != "" {
			specDir = filepath.Dir(strings.SplitN(src, "#", 2)[0])
		}

		handler = b.factory.HandleCustomAction(rd.Module, rd.Entity, rd.Action, *actionSpec, specDir)
	case "prepare":
		// Two-step idempotency prepare (todo 2.7.1): issue a fresh key for
		// a server-sourced idempotent action.
		specInfo, ok := b.registry.GetEntity(rd.Module, rd.Entity)
		if !ok || specInfo.EntitySpec == nil {
			handler = func(w http.ResponseWriter, r *http.Request) {
				writeError(w, http.StatusNotFound, "NOT_FOUND",
					"entity not found: "+rd.Module+"/"+rd.Entity)
			}
			break
		}
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
		handler = b.factory.HandlePrepare(rd.Module, rd.Entity, rd.Action, *actionSpec)
	case "service":
		// Stateless Service action (todo 7.1). Resolve the action spec from
		// the service registry; no persisted record, so no resourceID.
		actionSpec, ok := b.svcRegistry.GetAction(rd.Module, rd.Entity, rd.Action)
		if !ok {
			handler = func(w http.ResponseWriter, r *http.Request) {
				writeError(w, http.StatusNotFound, "NOT_FOUND",
					"service action not found: "+rd.Module+"/"+rd.Entity+"/"+rd.Action)
			}
			break
		}
		handler = b.factory.HandleServiceAction(rd.Module, rd.Entity, rd.Action, *actionSpec)
	case "job-status":
		// Tracked async job status polling (todo 7.13).
		handler = b.factory.HandleJobStatus(rd.Module, rd.Entity)
	case "webhook":
		// Verified inbound webhook (todo 7.6). Resolve the WebhookSpec from
		// the webhook registry; verification happens inside the handler
		// before dispatch.
		wh, ok := b.whRegistry.Get(rd.Module, rd.Entity)
		if !ok {
			handler = func(w http.ResponseWriter, r *http.Request) {
				writeError(w, http.StatusNotFound, "NOT_FOUND",
					"webhook not found: "+rd.Module+"/"+rd.Entity)
			}
			break
		}
		handler = b.factory.HandleWebhook(rd.Module, rd.Entity, wh)
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

// registerRouteWithPattern is like registerRoute but with an explicit path
// pattern (used by UI surface routes which have a different path prefix).
func (b *RouterBuilder) registerRouteWithPattern(r chi.Router, rd RouteDescriptor, pattern string) {
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
		case "submit":
			handler = b.factory.HandleSubmit(rd.Module, rd.Entity)
		case "cancel":
			handler = b.factory.HandleCancel(rd.Module, rd.Entity)
		case "amend":
			handler = b.factory.HandleAmend(rd.Module, rd.Entity)
		case "deactivate":
			handler = b.factory.HandleDeactivate(rd.Module, rd.Entity)
		case "reactivate":
			handler = b.factory.HandleReactivate(rd.Module, rd.Entity)
		default:
			return
		}
	case "custom":
		specInfo, ok := b.registry.GetEntity(rd.Module, rd.Entity)
		if !ok || specInfo.EntitySpec == nil {
			return
		}
		var actionSpec *spec.Action
		for i, a := range specInfo.EntitySpec.Actions {
			if a.Name == rd.Action {
				actionSpec = &specInfo.EntitySpec.Actions[i]
				break
			}
		}
		if actionSpec == nil {
			return
		}
		specDir := ""
		if src := specInfo.Source; src != "" {
			specDir = filepath.Dir(strings.SplitN(src, "#", 2)[0])
		}
		handler = b.factory.HandleCustomAction(rd.Module, rd.Entity, rd.Action, *actionSpec, specDir)
	case "prepare":
		specInfo, ok := b.registry.GetEntity(rd.Module, rd.Entity)
		if !ok || specInfo.EntitySpec == nil {
			return
		}
		var actionSpec *spec.Action
		for i, a := range specInfo.EntitySpec.Actions {
			if a.Name == rd.Action {
				actionSpec = &specInfo.EntitySpec.Actions[i]
				break
			}
		}
		if actionSpec == nil {
			return
		}
		handler = b.factory.HandlePrepare(rd.Module, rd.Entity, rd.Action, *actionSpec)
	case "service":
		// Stateless Service action on the UI surface (render-context
		// `source: api`). Resolve the action spec from the service registry.
		actionSpec, ok := b.svcRegistry.GetAction(rd.Module, rd.Entity, rd.Action)
		if !ok {
			return
		}
		handler = b.factory.HandleServiceAction(rd.Module, rd.Entity, rd.Action, *actionSpec)
	default:
		return
	}

	sub := r
	if rd.RequiredPermission != "" {
		sub = r.With(RequirePermission(rd.RequiredPermission))
	}

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

// sortedStrings returns the map keys in deterministic order (SPA mounts are
// registered in a stable order so router rebuilds are reproducible).
func sortedStrings(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
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
func serveFileFS(w http.ResponseWriter, _ *http.Request, spaFS fs.FS, name string) {
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
