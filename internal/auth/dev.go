package auth

import "context"

// DevValidator returns a synthetic identity with wildcard permissions.
// Used only in development mode — all requests pass through with full access.
//
// The identity maps to workspace "demo" with user "developer" and permission "*"
// which matches everything (see Identity.HasPermission).
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
		WorkspaceID: "demo",
		Permissions: []string{"*"},
		Roles:       []string{"admin"},
	}, nil
}
