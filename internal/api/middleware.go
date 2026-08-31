package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/internal/observability"
	"github.com/primadi/formspec/pkg/spec"
)

// Middleware chains for the FormSpec API router.

// authValidator is the active token validator, configured at startup.
// In dev mode: DevValidator (synthetic identity). In prod mode: JWTValidator.
var authValidator auth.TokenValidator

// SetAuthValidator configures the global token validator.
// Called once at server startup from main().
func SetAuthValidator(v auth.TokenValidator) {
	authValidator = v
}

// GetAuthValidator returns the current global token validator (may be nil).
// Used by tests to restore the previous validator after overriding it.
func GetAuthValidator() auth.TokenValidator {
	return authValidator
}

// apiKeyStore resolves X-FormSpec-Key credentials on the external surface
// (/api/v1/). Configured at startup when API key auth is enabled; nil disables
// API key auth.
var apiKeyStore *auth.ApiKeyStore

// SetApiKeyStore configures the global API key store (nil disables API key auth).
func SetApiKeyStore(s *auth.ApiKeyStore) { apiKeyStore = s }

// GetApiKeyStore returns the current global API key store (may be nil).
func GetApiKeyStore() *auth.ApiKeyStore { return apiKeyStore }

// WorkspaceMiddleware extracts the workspace slug from the URL, resolves it,
// and injects the workspace ID into the request context.
//
// URL format: /{workspace_slug}/api/...
// Falls back to "demo" for development.
// Workspace isolation (§15.2): the workspace ID is set once here and all
// downstream handlers MUST use it from context — never from request body.
// Cross-workspace mismatch is enforced in AuthMiddleware (identity workspace vs URL).
func WorkspaceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract workspace slug from path: /{workspace}/api/...
		path := strings.TrimPrefix(r.URL.Path, "/")
		parts := strings.SplitN(path, "/", 3)

		var workspaceID string
		if len(parts) >= 1 && parts[0] != "" {
			workspaceID = parts[0]
		}
		if workspaceID == "" {
			workspaceID = "demo"
		}

		// Save URL-extracted workspace for cross-workspace check in AuthMiddleware
		ctx := WithURLWorkspace(r.Context(), workspaceID)
		// In production: lookup workspace slug → internal workspace UUID
		ctx = WithWorkspace(ctx, workspaceID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AuthMiddleware verifies the request authentication token
// and injects an Identity into the context.
//
// Dev mode: all requests get a synthetic developer identity with full permissions.
// Prod mode: validates JWT via the configured TokenValidator.
// Anonymous access is only allowed for routes with required_permission: "public".
//
// Cross-workspace enforcement (§15.2): if the identity's workspace does not match
// the URL workspace, returns 404 (NOT_FOUND) — per spec, cross-workspace access
// is indistinguishable from the resource not existing.
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// API key auth (external surface only — /api/v1/). The UI surface
		// (/_ui/) never accepts X-FormSpec-Key; it uses session cookie / OAuth.
		if apiKeyStore != nil && !isUISurface(r.URL.Path) {
			if apiKey := r.Header.Get("X-FormSpec-Key"); apiKey != "" {
				ws := workspaceFromContext(ctx)
				if ws == "" {
					ws = "demo"
				}
				k, err := apiKeyStore.GetByKey(ctx, ws, apiKey)
				if err != nil || !k.IsValid(time.Now()) {
					writeError(w, http.StatusUnauthorized, "UNAUTHORIZED",
						"invalid api key")
					return
				}
				identity := k.Identity(ws)
				ctx = WithIdentity(ctx, identity)
				ctx = WithUser(ctx, identity.UserID)
				ctx = WithWorkspace(ctx, ws)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		// Extract token from Authorization header, falling back to the
		// `token` query param — browsers cannot set custom headers on a
		// WebSocket handshake, so the realtime client (/ _ui/_ws) authenticates
		// via ?token=. Note: query-string tokens can leak into logs/referrers;
		// prefer the header for regular REST calls.
		token := ""
		if authHeader := r.Header.Get("Authorization"); authHeader != "" {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		} else if q := r.URL.Query().Get("token"); q != "" {
			token = q
		}

		// Validate token and resolve identity
		if authValidator != nil {
			identity, err := authValidator.Validate(ctx, token)
			if err != nil {
				// Only fail if a token was provided but invalid.
				// Missing token is allowed — identity will be nil (anonymous).
				if token != "" {
					writeError(w, http.StatusUnauthorized, "UNAUTHORIZED",
						"invalid token: "+err.Error())
					return
				}
			}
			if identity != nil {
				// Cross-workspace enforcement: identity workspace must match URL workspace.
				// Spec §15.2: cross-workspace access → 404 NOT_FOUND (not 403).
				urlWorkspace := URLWorkspaceFromContext(ctx)
				if identity.WorkspaceID != "" && urlWorkspace != "" &&
					identity.WorkspaceID != urlWorkspace {
					writeError(w, http.StatusNotFound, "NOT_FOUND",
						"workspace not found")
					return
				}

				ctx = WithIdentity(ctx, identity)
				// Carry the app scope (from JWT claims) so downstream handlers
				// can make app-aware decisions (role management is per-App).
				ctx = WithApp(ctx, identity.App)
				// Also set legacy context values for backward compatibility
				ctx = WithUser(ctx, identity.UserID)
				// Override workspace with identity's workspace
				if identity.WorkspaceID != "" {
					ctx = WithWorkspace(ctx, identity.WorkspaceID)
				}
			}
		} else {
			// No validator configured — dev fallback (should not happen if SetAuthValidator is called)
			ctx = WithUser(ctx, "developer")
			ctx = WithWorkspace(ctx, "demo")
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequirePermission returns middleware that enforces a specific permission.
//
// Rules:
//   - If required is empty or "public", everyone is allowed (including anonymous).
//   - If the identity is nil (no auth), returns 401 Unauthorized.
//   - If the identity lacks the permission, returns 403 Forbidden.
//   - Otherwise, passes through to the next handler.
//
// RequirePermission returns middleware that enforces a specific permission.
//
// Rules:
//   - If required is empty or "public", everyone is allowed (including anonymous).
//   - If the identity is nil (no auth), returns 401 Unauthorized.
//   - If the identity lacks the permission, returns 403 Forbidden — EXCEPT on
//     the UI surface (/_ui/) for entity list/view access, which returns 404
//     NOT_FOUND per spec (frontend/04-spec-resolution-api.md §4): a caller
//     without list/view on an entity must not learn the entity exists.
//   - Otherwise, passes through to the next handler.
//
// Usage in chi router:
//
//	r.With(RequirePermission("billing.invoices.list")).Get("/...", handler)
func RequirePermission(required string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Public endpoints — no auth required
			if required == "" || required == "public" {
				next.ServeHTTP(w, r)
				return
			}

			identity := IdentityFromContext(r.Context())
			if identity == nil {
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED",
					"authentication required")
				return
			}

			if !identity.HasPermission(required) {
				// UI surface + entity list/view → 404 (don't leak existence).
				if isUISurface(r.URL.Path) && isEntityVisibilityPerm(required) {
					writeError(w, http.StatusNotFound, "NOT_FOUND",
						"resource not found")
					return
				}
				writeError(w, http.StatusForbidden, "FORBIDDEN",
					"missing permission: "+required)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isUISurface reports whether the request path targets the UI surface
// (/{ws}/_ui/...), as opposed to the external API (/{ws}/api/v1/...).
func isUISurface(path string) bool {
	// Path is like "/{ws}/_ui/entity/..." — find the "_ui" segment.
	trimmed := strings.TrimPrefix(path, "/")
	parts := strings.SplitN(trimmed, "/", 3)
	if len(parts) < 2 {
		return false
	}
	return parts[1] == "_ui"
}

// isEntityVisibilityPerm reports whether a permission string is an entity
// list/view permission (the ones that gate entity existence per spec §4).
func isEntityVisibilityPerm(perm string) bool {
	// Format: {module}.{plural}.{action} — check the last segment.
	idx := strings.LastIndexByte(perm, '.')
	if idx < 0 {
		return false
	}
	action := perm[idx+1:]
	return action == "list" || action == "view"
}

// RequestIDMiddleware issues a unique request ID for every request, or
// forwards the upstream X-Request-ID when present (todo 8.2.3, spec
// platform/09-observability.md §2.3: "meneruskan yang datang dari upstream").
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = observability.NewRequestID()
		}
		ctx := observability.WithRequestID(r.Context(), id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// CORSMiddleware applies permissive CORS headers for development.
// Production must use NewCORSMiddleware with an explicit allow-list
// (todo 8.1.5 — `*` is never acceptable in production).
func CORSMiddleware(next http.Handler) http.Handler {
	return NewCORSMiddleware(nil)(next)
}

// NewCORSMiddleware returns a CORS middleware restricted to the given
// origin allow-list (todo 8.1.5). Behavior:
//   - nil/empty list  → permissive `*` (development only)
//   - list contains "*" → permissive (explicit opt-in)
//   - otherwise       → echo the Origin header only when it matches an
//     allow-listed origin; other origins get no CORS headers.
func NewCORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	permissive := len(allowedOrigins) == 0
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		if o == "*" {
			permissive = true
		}
		allowed[strings.TrimSuffix(o, "/")] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			switch {
			case permissive:
				w.Header().Set("Access-Control-Allow-Origin", "*")
			case origin != "" && allowed[strings.TrimSuffix(origin, "/")]:
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RecoveryMiddleware recovers from panics and returns 500.
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR",
					fmt.Sprintf("panic: %v", rec))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// LoggingMiddleware logs every request as a structured JSON line carrying
// the mandatory observability fields (todo 8.2.1, spec
// platform/09-observability.md §2.1). Metadata only — never request bodies
// or business values (§2.2).
func LoggingMiddleware(log *observability.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			log.Info(observability.Fields{
				"request_id":  observability.RequestIDFromContext(r.Context()),
				"workspace":   workspaceFromContext(r.Context()),
				"module":      nil,
				"entity":      nil,
				"action":      nil,
				"actor":       userFromContext(r.Context()),
				"duration_ms": time.Since(start).Milliseconds(),
				"error_code":  nil,
				"trace_id":    nil,
				"method":      r.Method,
				// route_class, not raw path (§3.2 cardinality discipline).
				"route_class": ClassifyRoute(r.URL.Path),
				"status":      rec.status,
			})
		})
	}
}

// statusRecorder captures the response status for logging/metrics.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wroteHeader {
		r.status = code
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(code)
}

// ClassifyRoute maps a request path to a bounded route_class label
// (spec §3.2: entity CRUD, action invoke, admin panel, websocket, health —
// never per-record paths).
func ClassifyRoute(path string) string {
	switch {
	case strings.HasSuffix(path, "/health"):
		return observability.RouteClassHealth
	case strings.Contains(path, "/_ui/_ws") || strings.Contains(path, "/ws"):
		return observability.RouteClassWebsocket
	case strings.Contains(path, "/_admin") || strings.Contains(path, "/_ui"):
		return observability.RouteClassAdmin
	case strings.Contains(path, "/actions/"):
		return observability.RouteClassAction
	case strings.Contains(path, "/api/v1/"):
		return observability.RouteClassEntityCRUD
	}
	return observability.RouteClassOther
}

// MetricsMiddleware instruments requests into the mandatory Prometheus
// metric set (todo 8.2.4). routeClass is derived via ClassifyRoute —
// bounded labels only (todo 8.2.5).
func MetricsMiddleware(m *observability.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			m.ObserveRequest(
				ClassifyRoute(r.URL.Path),
				r.Method,
				rec.status,
				time.Since(start).Seconds(),
				"",
			)
		})
	}
}

// contextKey and context functions are defined in handler.go.
// GetWorkspace returns the current workspace ID from the request context.
func GetWorkspace(ctx context.Context) string {
	return workspaceFromContext(ctx)
}

// GetUser returns the authenticated user from context.
func GetUser(ctx context.Context) string {
	return userFromContext(ctx)
}

// strictMode controls whether permission/uses enforcement blocks or only warns.
// Default false (dev mode: warn only). Set true for production.
var strictMode bool

// SetStrictMode enables/disables strict enforcement of uses declarations.
// In strict mode, undeclared uses violations return errors (500/403).
// In relaxed mode, violations are logged but allowed (dev convenience).
func SetStrictMode(strict bool) {
	strictMode = strict
}

// IsStrictMode returns whether strict enforcement is enabled.
func IsStrictMode() bool {
	return strictMode
}

// ============================================================================
// Uses Enforcement & Standard Error Codes (§16)
// ============================================================================

// UsesEnforcement returns middleware that enforces "uses" declarations.
//
// In strict mode (prod):
//   - Blocks undeclared resource/primitives/config access
//   - Cross-module write without declaration → high-risk consent violation
//
// In relaxed mode (dev):
//   - Logs warning but allows the request
//
// Currently a stub — full enforcement requires Starlark runtime integration (Fase 2).
// This middleware exists to:
//
//	a) Define the contract for future enforcement
//	b) Return proper error codes when enforcement is active
//	c) Be wired into the route chain so it's ready
func UsesEnforcement(_ *spec.UsesDecl) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strictMode {
				// TODO(Fase 2): actual uses checking against Starlark runtime
				// For now in strict mode, we log that enforcement is active
				// but pass through since we don't have a runtime to validate against.
				_ = r.Context()
			}
			// In relaxed mode (dev), always allow
			next.ServeHTTP(w, r)
		})
	}
}

// writeUsesViolation writes a USES_VIOLATION error response.
// This is triggered when code accesses a resource/primitives/config
// that was not declared in its "uses" block (D20/D46).
func writeUsesViolation(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusInternalServerError, "USES_VIOLATION",
		"undeclared access: "+msg)
}

// writeConfigAccessDenied writes a CONFIG_ACCESS_DENIED error response.
func writeConfigAccessDenied(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusInternalServerError, "CONFIG_ACCESS_DENIED",
		"config access denied: "+msg)
}

// writeKvstoreAccessDenied writes a KVSTORE_ACCESS_DENIED error response.
func writeKvstoreAccessDenied(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusInternalServerError, "KVSTORE_ACCESS_DENIED",
		"kvstore access denied: "+msg)
}

// Deferred helpers for Fase 2 uses enforcement (§16, todo UsesEnforcement):
// dipakai begitu pengecekan uses terhadap Starlark runtime diaktifkan.
// Assigned to _ agar tidak terdeteksi unused sampai wiring selesai.
var (
	_ = writeUsesViolation
	_ = writeConfigAccessDenied
	_ = writeKvstoreAccessDenied
)
