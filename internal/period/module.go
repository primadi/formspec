// Package period provides the period-closing framework module (todo 7.11):
// the bundled formspec.core.period-closing entity, the period guard used to
// enforce FORMSPEC.PERIOD.CLOSED on transaction writes, and the business
// calendar resolution (02-core-extended.md §9.3–§9.4).
package period

import (
	"embed"
	"fmt"
	"io/fs"

	"github.com/primadi/formspec/internal/entity"
)

// CoreModule is the reserved namespace for framework-owned resources.
const CoreModule = "formspec.core"

//go:embed module
var moduleFS embed.FS

// ModuleFS returns the embedded period-closing module filesystem.
func ModuleFS() fs.FS { return moduleFS }

// RegisterCoreEntities registers the framework-owned period-closing entity
// (formspec.core.period-closing) from the bundled module. Marked Internal but
// UIExposed (manageable on the admin/UI surface). Call BEFORE
// LoadEntities/SyncSchema so the table is created.
func RegisterCoreEntities(reg *entity.Registry) error {
	for _, err := range reg.RegisterEmbeddedCoreModule(moduleFS) {
		return fmt.Errorf("register period core module: %w", err)
	}
	return nil
}
