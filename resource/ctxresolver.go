package formspec

import (
	"fmt"
	"path/filepath"
	"strings"

	db "github.com/primadi/formspec/renderers/jsonb-persist"
	"github.com/primadi/formspec/renderers/jsonb-persist/datastore"
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
// Named datastores other than 'default' are not yet backed by a live
// connection in single-server mode (they come from the Control Plane
// snapshot, todo 2.9.4) — the resolver returns a clear error so scripts fail
// loudly instead of silently.
func NewCtxPrimitiveResolver(database db.DB, stateDir string) func(primitiveType, name string) (interface{}, error) {
	kv := memory.NewKV()         // ctx.kvstore
	cache := memory.NewKV()      // ctx.cache (TTL honored by KV)
	lock := memory.NewLock()     // ctx.lock
	queue := memory.NewQueue()   // ctx.queue
	pubsub := memory.NewPubSub() // ctx.pubsub

	storage, err := memory.NewStorage(filepath.Join(stateDir, "storage"))
	if err != nil {
		// Fall back to a nil backend; surface the error at resolve time.
		storage = nil
	}

	return func(primitiveType, name string) (interface{}, error) {
		if name != "" && name != "default" {
			return nil, fmt.Errorf("ctx.%s: no live datastore for %q in single-server mode (only 'default' is auto-provisioned in dev; named datastores come from the Control Plane snapshot, todo 2.9.4)", primitiveType, name)
		}
		switch primitiveType {
		case "db":
			return &datastore.DBQuerier{DB: database}, nil
		case "kvstore":
			return kv, nil
		case "cache":
			return cache, nil
		case "lock":
			return lock, nil
		case "queue":
			return queue, nil
		case "pubsub":
			return pubsub, nil
		case "storage":
			if storage == nil {
				return nil, fmt.Errorf("ctx.storage: filesystem backend failed to initialize")
			}
			return storage, nil
		default:
			return nil, fmt.Errorf("ctx.%s: unknown primitive (closed set: db, cache, lock, queue, pubsub, storage, config, kvstore, log)", primitiveType)
		}
	}
}

// ctxPrimitiveResolver builds the ctx.* primitive resolver for single-server
// mode (todo 2.9.2/2.9.3). It auto-provisions the 'default' datastore for the
// closed set of primitives routed through the resolver per
// docs/spec/platform/06-datastore.md §5:
//
//	db → SQLite (dev) / Postgres (prod)
//	cache/lock/queue/pubsub/kvstore → in-memory
//	storage → filesystem lokal
//
// `config` and `log` are NOT routed through the resolver — they are separate
// ctx builtins (ctx.config / ctx.log) exposed directly by the CtxAPI.
//
// Named datastores other than 'default' are not yet backed by a live
// connection in single-server mode (they come from the Control Plane
// snapshot, todo 2.9.4) — the resolver returns a clear error so scripts fail
// loudly instead of silently.
func ctxPrimitiveResolver(database db.DB, stateDir string) func(primitiveType, name string) (interface{}, error) {
	return NewCtxPrimitiveResolver(database, stateDir)
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
