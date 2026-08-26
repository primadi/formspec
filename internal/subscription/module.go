package subscription

import (
	"embed"
	"fmt"
	"io/fs"

	"github.com/primadi/formspec/internal/entity"
)

// CoreModule is the reserved namespace for framework-owned resources
// (platform/02-workspace-app-module.md §9). Dynamic subscriptions live here
// (todo 7.3.4).
const CoreModule = "formspec.core"

//go:embed module
var moduleFS embed.FS

// ModuleFS returns the embedded subscription module filesystem
// (internal/subscription/module). Exposed for tooling to scaffold a
// customizable copy that stays in sync with the bundled entity.
func ModuleFS() fs.FS { return moduleFS }

// RegisterCoreEntities registers the framework-owned dynamic-subscription
// entity (formspec.core.subscription) from the bundled module
// (internal/subscription/module, embedded via //go:embed). The module is
// loaded through the same manifest loader path as user manifests
// (dogfooding) — the entity is marked Internal (no external API routes) but
// UIExposed (manageable on the admin/UI surface via formspec.dev/ui-exposed).
//
// Call this BEFORE LoadEntities/SyncSchema so the table is created. A
// user-provided override in external/ replaces it (user override wins).
func RegisterCoreEntities(reg *entity.Registry) error {
	for _, err := range reg.RegisterEmbeddedCoreModule(moduleFS) {
		return fmt.Errorf("register subscription core module: %w", err)
	}
	return nil
}
