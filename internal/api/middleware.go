package api

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"strings"

	"github.com/primadi/forma/internal/auth"
	"github.com/primadi/forma/pkg/spec"
)

// Middleware chains for the Forma API router.

// authValidator is the active token validator, configured at startup.
// In dev mode: DevValidator (synthetic identity). In prod mode: JWTValidator.
var authValidator auth.TokenValidator

// SetAuthValidator configures the global token validator.
// Called once at server startup from main().
func SetAuthValidator(v auth.TokenValidator) {
	authValidator = v
}

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

		// Extract token from Authorization header
		token := ""
		if authHeader := r.Header.Get("Authorization"); authHeader != "" {
			token = strings.TrimPrefix(authHeader, "Bearer ")
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
				writeError(w, http.StatusForbidden, "FORBIDDEN",
					"missing permission: "+required)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequestIDMiddleware generates a unique request ID for every request.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := generateRequestID()
		ctx := WithRequestID(r.Context(), id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// CORSMiddleware applies permissive CORS headers for development.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
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

// LoggingMiddleware logs every request with method, path, status.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := requestIDFromContext(r.Context())
		fmt.Printf("[api] %s %s [%s]\n", r.Method, r.URL.Path, start)
		next.ServeHTTP(w, r)
	})
}

// generateRequestID creates a short random request ID.
func generateRequestID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
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
