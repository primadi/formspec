// Package notify provides the framework-owned notification module (todo 7.7.6):
// the bundled `formspec.core.notification` entity and the delivery that fills it.
//
// It exists to close one gap with two symptoms. The `notification` delivery
// channel was declared, accepted by the schema, and never delivered — the
// bridge it named (`formspec/notify`) did not exist. The SAME missing entity is
// what `kind: NotificationCenter` looks for when it is zero-config, so that page
// was permanently empty too. One module supplies both.
package notify

import (
	"embed"
	"fmt"
	"io/fs"

	"github.com/primadi/formspec/internal/entity"
)

// CoreModule is the reserved namespace for framework-owned resources.
const CoreModule = "formspec.core"

// NotificationEntity is the entity the delivery channel writes and
// NotificationCenter reads. Exported so the two cannot drift apart on a string.
const NotificationEntity = "notification"

//go:embed module
var moduleFS embed.FS

// ModuleFS returns the embedded notification module filesystem.
func ModuleFS() fs.FS { return moduleFS }

// RegisterCoreEntities registers the framework-owned notification entity
// (formspec.core.notification) from the bundled module. UIExposed so it is
// listed on the admin/UI surface. Call BEFORE LoadEntities/SyncSchema so the
// table is created.
func RegisterCoreEntities(reg *entity.Registry) error {
	for _, err := range reg.RegisterEmbeddedCoreModule(moduleFS) {
		return fmt.Errorf("register notify core module: %w", err)
	}
	return nil
}
