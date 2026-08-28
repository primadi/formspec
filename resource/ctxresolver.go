package formspec

import (
	"path/filepath"
	"strings"

	db "github.com/primadi/formspec/renderers/jsonb-persist"
	"github.com/primadi/formspec/renderers/jsonb-persist/datastore/memory"
)

// NewCtxPrimitiveResolver builds the ctx.* primitive resolver for
// single-server mode (todo 2.9.2/2.9.3). It auto-provisions the 'default'
// datastore for the closed set of primitives routed through the resolver per
// docs/spec/platform/06-datastore.md §5:
//
//	db → SQLite (dev) / Postgres (prod)
//	cache/lock/queue/pubsub/kvstore → in-memory
//	storage → filesystem lokal
//
// `config` and `log` are NOT routed through the resolver — they are separate
// ctx builtins (ctx.config / ctx.log) exposed directly by the CtxAPI.
//
// The returned resolver is module-aware (todo 2.9.4): with no kind: Datastore
// manifests loaded, every module resolves to 'default'. Use
// buildDatastoreRegistry + resolverFromRegistry to add named datastores and
// per-module `spec.datastore` bindings.
func NewCtxPrimitiveResolver(database db.DB, stateDir string, sharedPubSub ...*memory.PubSub) func(primitiveType, name, module string) (interface{}, error) {
	var ps *memory.PubSub
	if len(sharedPubSub) > 0 {
		ps = sharedPubSub[0]
	}
	reg := NewDatastoreRegistry(database, stateDir, ps)
	return reg.Resolve
}

// StateDirFromDSN derives a state directory from a DSN like
// "sqlite:.formspec/data.db" → ".formspec". Falls back to ".formspec".
func StateDirFromDSN(dsn string) string {
	if strings.HasPrefix(dsn, "sqlite:") {
		path := strings.TrimPrefix(dsn, "sqlite:")
		dir := filepath.Dir(path)
		if dir != "." && dir != "" {
			return dir
		}
	}
	return ".formspec"
}

// stateDirFromDSN derives a state directory from a DSN like
// "sqlite:.formspec/data.db" → ".formspec". Falls back to ".formspec".
func stateDirFromDSN(dsn string) string {
	return StateDirFromDSN(dsn)
}
