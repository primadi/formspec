package auth

import "context"

// DevValidator returns a synthetic identity with wildcard permissions.
// Used only in development mode — all requests pass through with full access.
//
// The identity maps to user "developer" with permission "*" which matches
// everything (see Identity.HasPermission). WorkspaceID is left empty so the
// identity adopts whatever workspace is in the URL — this lets dev mode
// reach any workspace slug without tripping the cross-workspace 404 check in
// AuthMiddleware (which only compares when both sides are non-empty).
type DevValidator struct{}

// NewDevValidator creates a dev-mode validator that always returns a synthetic identity.
func NewDevValidator() *DevValidator {
	return &DevValidator{}
}

// Validate always returns a synthetic developer identity with wildcard permissions.
// The token parameter is ignored in dev mode.
func (v *DevValidator) Validate(_ context.Context, _ string) (*Identity, error) {
	return &Identity{
		UserID:      "developer",
		WorkspaceID: "",
		Permissions: []string{"*"},
		Roles:       []string{"admin"},
	}, nil
}
