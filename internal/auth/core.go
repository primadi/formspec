package auth

import (
	"embed"
	"fmt"
	"io/fs"

	"github.com/primadi/formspec/internal/entity"
)

// CoreModule is the reserved namespace for framework-owned resources
// (platform/02-workspace-app-module.md §9). It is always present in every
// workspace and cannot be declared by any user Module.
const CoreModule = "formspec.core"

// Role names for the logical auth roles resolved by RoleResolver.
const (
	RoleUser    = "user"
	RoleSession = "session"
	RoleRole    = "role"
	RoleApiKey  = "api-key"
)

// Owner role names (todo 6.3.4) — the 4 symmetric owner roles. Recognized by
// the permission resolver to grant broad (wildcard) access within their scope.
const (
	RoleWorkspaceOwner = "workspace-owner"
	RoleAppOwner       = "app-owner"
	RoleModuleOwner    = "module-owner"
	RoleCloudOwner     = "cloud-owner"
)

//go:embed module
var moduleFS embed.FS

// ModuleFS returns the embedded auth module filesystem (internal/auth/module).
// Exposed for tooling (formspec generate auth) to scaffold a customizable copy
// that stays in sync with the bundled entities.
func ModuleFS() fs.FS { return moduleFS }

// RegisterCoreEntities registers the framework-owned auth entities
// (formspec.core.user, formspec.core.session, formspec.core.role,
// formspec.core.role-assignment) from the bundled auth module
// (internal/auth/module, embedded via //go:embed). The module is loaded
// through the same manifest loader path as user manifests (dogfooding) —
// entities are marked Internal (no API routes, no meta-bundle exposure) and
// are only reachable by framework code via GetEntityStore.
//
// Call this BEFORE SyncSchema so the tables are created. A user-provided
// override in external/ (or via auth_config_ref) replaces these at the
// RoleResolver level; the defaults remain registered as a safe fallback.
// If an entity with the same key is already registered (e.g. loaded from
// external/), the default is skipped — user override wins.
func RegisterCoreEntities(reg *entity.Registry) error {
	for _, err := range reg.RegisterEmbeddedCoreModule(moduleFS) {
		return fmt.Errorf("register core module: %w", err)
	}
	return nil
}
