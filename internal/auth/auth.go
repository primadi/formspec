// Package auth provides authentication and identity primitives for FormSpec.
//
// It defines the Identity struct (user ID, workspace, permissions, roles),
// the TokenValidator interface, and the AuthMode that switches between
// dev (synthetic identity) and prod (JWT validation) behavior.
//
// Conforms to:
//   - Core §15.1 (Auth required by default, anonymous via public permission)
//   - Core §15.2 (Workspace isolation — identity carries workspace scope)
//   - D20 (Explicit permission model — permissions as {module}.{key})
package auth

import (
	"context"
	"fmt"
	"strings"
)

// contextKey is a private key type for context values to avoid collisions.
type contextKey int

const ctxPermissions contextKey = iota

// WithPermissions stores caller permissions in the context.
func WithPermissions(ctx context.Context, permissions []string) context.Context {
	return context.WithValue(ctx, ctxPermissions, permissions)
}

// PermissionsFromContext extracts caller permissions from the context.
// Returns nil if none were stored.
func PermissionsFromContext(ctx context.Context) []string {
	v, _ := ctx.Value(ctxPermissions).([]string)
	return v
}

// Identity represents an authenticated caller.
//
// It carries the minimal set of claims needed for authorization decisions:
// user identity, workspace scope, permission grants, and role assignments.
// All fields are immutable after construction.
type Identity struct {
	UserID      string   // authenticated user ID (sub claim in JWT)
	WorkspaceID string   // workspace scope
	App         string   // app scope (empty = workspace-level, e.g. _admin)
	Permissions []string // granted permissions, e.g. ["billing.invoices.*", "billing.customers.list"]
	Roles       []string // assigned roles, e.g. ["billing-admin"]
}

// HasPermission checks whether this identity holds the required permission.
//
// Matching rules:
//   - Exact match: "billing.invoices.list" == "billing.invoices.list"
//   - Wildcard: "billing.invoices.*" matches "billing.invoices.list"
//   - Super-wildcard: "*" matches everything (dev mode)
//   - "public" always returns true (anonymous access)
func (id *Identity) HasPermission(required string) bool {
	if required == "" || required == "public" {
		return true
	}

	for _, p := range id.Permissions {
		if p == "*" {
			return true
		}
		if p == required {
			return true
		}
		// Wildcard match: "billing.invoices.*" → prefix before "*"
		if before, ok := strings.CutSuffix(p, ".*"); ok {
			// Required must be under the same prefix AND have at least one
			// more segment after the wildcard.
			if strings.HasPrefix(required, before) {
				rest := strings.TrimPrefix(required, before)
				if strings.HasPrefix(rest, ".") {
					// Module-level wildcard "billing.*" → any entity.action
					// under the module (1+ segments).
					if !strings.Contains(before, ".") {
						return true
					}
					// Entity-level wildcard "billing.invoices.*" → exactly
					// one more segment (the action).
					if !strings.Contains(rest[1:], ".") {
						return true
					}
				}
			}
		}
	}
	return false
}

// IsAuthenticated returns true if this identity has a non-empty UserID.
func (id *Identity) IsAuthenticated() bool {
	return id != nil && id.UserID != ""
}

// DisplayName returns a human-readable name for logging.
func (id *Identity) DisplayName() string {
	if id == nil {
		return "anonymous"
	}
	return id.UserID + "@" + id.WorkspaceID
}

// TokenValidator validates an authentication token and returns an Identity.
//
// Implementations:
//   - JWTValidator: validates JWT (HS256/RS256) and extracts claims
//   - DevValidator:   returns a synthetic identity for development
type TokenValidator interface {
	Validate(ctx context.Context, token string) (*Identity, error)
}

// AuthMode selects the authentication strategy.
type AuthMode string

const (
	ModeDev  AuthMode = "dev"
	ModeProd AuthMode = "prod"
)

// PermissionChecker checks whether an identity has a specific permission.
// This is the interface for ctx.auth.has() — it validates that the permission
// exists in the module's declared footprint before checking the identity.
type PermissionChecker interface {
	// PermissionExists checks if a permission key is registered in any module.
	PermissionExists(key string) bool
	// HasPermission checks if the identity holds the permission.
	// identity must implement HasPermission(string) bool.
	HasPermission(identity interface{ HasPermission(string) bool }, permission string) error
}

// defaultPermissionChecker is the standard implementation.
// It checks identity.HasPermission() with no additional validation.
type defaultPermissionChecker struct{}

func (d *defaultPermissionChecker) PermissionExists(_ string) bool { return true }

func (d *defaultPermissionChecker) HasPermission(id interface{ HasPermission(string) bool }, permission string) error {
	if !id.HasPermission(permission) {
		return fmt.Errorf("missing permission: %s", permission)
	}
	return nil
}

// globalPermissionChecker is the package-level checker used by ctx.auth.has().
var globalPermissionChecker PermissionChecker = &defaultPermissionChecker{}

// SetPermissionChecker replaces the global permission checker.
// Called during server startup to wire in the real permission registry.
func SetPermissionChecker(pc PermissionChecker) {
	globalPermissionChecker = pc
}

// CtxAuthHas implements ctx.auth.has() functionality.
//
// In Starlark/Go code: auth.CtxAuthHas(identity, "billing.invoices.list")
// Returns true if the identity has the permission AND the permission is declared.
// In dev mode with wildcard "*", always returns true.
func CtxAuthHas(id *Identity, permission string) bool {
	if id == nil {
		return false
	}
	// Dev mode short-circuit
	if id.HasPermission("*") || permission == "public" {
		return true
	}
	// Validate permission exists in registry
	if !globalPermissionChecker.PermissionExists(permission) {
		return false
	}
	return id.HasPermission(permission)
}
