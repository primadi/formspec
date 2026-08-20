package db

import (
	"context"
)

// PersistBackend is the storage seam every FormSpec storage implementation
// must satisfy (docs/spec/backend/04-persist-backend.md §2). It is deliberately
// technology-agnostic — no *sql.DB, ExecContext, or QueryContext leak into the
// contract — so a second backend (e.g. fully-relational) can be added without
// retrofitting the core engine.
//
// jsonb-persist is the single official implementation today; the framework
// must talk to this interface, never shortcut to Postgres/SQLite directly.
type PersistBackend interface {
	// SyncSchema applies the structural diff between the desired entity specs
	// and the current storage state (field add/remove/renamed_from, index
	// changes). Field removal is two-phase (deprecate then drop) across two
	// applied versions; data migration (backfill) is a separate versioned
	// script type, not part of the structural diff.
	SyncSchema(ctx context.Context, entities []EntityMigration) (int, error)

	// PlanSchema returns the pending structural diff without applying it.
	PlanSchema(ctx context.Context, entities []EntityMigration) ([]DDLResult, error)

	// NextKey allocates a gap-free, duplicate-free sequence value for a
	// natural_key_rule field. The increment is atomic and happens under the
	// same lock as the insert/update transaction.
	NextKey(ctx context.Context, workspaceID, module, entity, field string) (string, error)

	// UninstallExtension drops an extension column and locks its namespace so
	// it is never reused.
	UninstallExtension(ctx context.Context, tableName, namespace string) error

	// EntityStore returns the store for a module/entity pair.
	EntityStore(module, entity string) (*EntityStore, error)

	// DriverName returns "sqlite" or "postgres".
	DriverName() string
}
