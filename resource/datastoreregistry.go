package formspec

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	db "github.com/primadi/formspec/renderers/jsonb-persist"
	"github.com/primadi/formspec/renderers/jsonb-persist/datastore"
	"github.com/primadi/formspec/renderers/jsonb-persist/datastore/memory"
	"github.com/primadi/formspec/renderers/jsonb-persist/datastore/rediskv"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
)

// ─── Datastore registry (todo 2.9.4) ───
//
// Module-scoped ctx.* primitive resolution per
// docs/spec/platform/06-datastore.md §1.1:
//
//   - A module may declare `spec.datastore: <name>` (kind: Module). Inside
//     that module's code, `ctx.db()` without arguments ALWAYS resolves to
//     the bound datastore; unbound modules resolve to 'default'.
//   - There is NO escape hatch: `.named(x)` may only reach the module's own
//     binding. Cross-module/cross-datastore interaction must go through
//     events/outbox — never a direct ctx.* handle.
//
// Named datastores come from `kind: Datastore` manifests. Single-server
// supports drivers: sqlite, postgres, memory, fs, plus valkey/redis for the
// KV primitives (cache/kvstore — Plan C batch 2). Remaining cloud drivers
// (s3, minio, nats) fail loudly at resolve time — they require the
// Control Plane snapshot with real credentials.

// dsEntry is one named datastore: its spec plus lazily-opened connections.
type dsEntry struct {
	name   string
	spec   *spec.DatastoreSpec // nil for the 'default' entry
	mainDB db.DB               // 'default' only — the app's primary database

	// shared in-memory backends for 'default' (nil for named entries, which
	// get fresh instances so datastores stay isolated).
	sharedKV    *memory.KV
	sharedLock  *memory.Lock
	sharedQueue *memory.Queue
	sharedPS    *memory.PubSub

	mu    sync.Mutex
	conns map[string]interface{} // primitive type → resolved connection
}

func (e *dsEntry) serves(pt spec.PrimitiveType) bool {
	if e.spec == nil {
		return true // 'default' serves every primitive
	}
	for _, p := range e.spec.Serves {
		if p == pt {
			return true
		}
	}
	return false
}

// conn returns (opening lazily) the connection for one primitive type.
func (e *dsEntry) conn(stateDir string, pt spec.PrimitiveType) (interface{}, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if c, ok := e.conns[string(pt)]; ok {
		return c, nil
	}

	c, err := e.open(stateDir, pt)
	if err != nil {
		return nil, err
	}
	e.conns[string(pt)] = c
	return c, nil
}

func (e *dsEntry) open(stateDir string, pt spec.PrimitiveType) (interface{}, error) {
	// 'default' entry — wired at boot to the app's primary database and the
	// shared in-memory backends (todo 2.9.3 behavior preserved).
	if e.spec == nil {
		switch pt {
		case spec.PrimitiveDB:
			return &datastore.DBQuerier{DB: e.mainDB}, nil
		case spec.PrimitiveKVStore, spec.PrimitiveCache:
			return e.sharedKV, nil
		case spec.PrimitiveLock:
			return e.sharedLock, nil
		case spec.PrimitiveQueue:
			return e.sharedQueue, nil
		case spec.PrimitivePubSub:
			return e.sharedPS, nil
		case spec.PrimitiveStorage:
			s, err := memory.NewStorage(filepath.Join(stateDir, "storage"))
			if err != nil {
				return nil, fmt.Errorf("filesystem backend failed to initialize: %w", err)
			}
			return s, nil
		default:
			return nil, fmt.Errorf("primitive %q is not routed through the datastore resolver", pt)
		}
	}

	// Named entry — open by driver.
	drv := e.spec.Driver
	switch drv {
	case spec.DatastoreDriverSQLite:
		if pt != spec.PrimitiveDB && pt != spec.PrimitiveKVStore {
			return nil, fmt.Errorf("driver %q does not serve primitive %q", drv, pt)
		}
		path := e.spec.Connection.Database
		if path == "" {
			path = filepath.Join(stateDir, "datastores", e.name+".db")
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("datastore %q: mkdir: %w", e.name, err)
		}
		d, err := db.OpenSQLite(path, nil)
		if err != nil {
			return nil, fmt.Errorf("datastore %q: open sqlite: %w", e.name, err)
		}
		return &datastore.DBQuerier{DB: d}, nil
	case spec.DatastoreDriverPostgres:
		if pt != spec.PrimitiveDB && pt != spec.PrimitiveKVStore {
			return nil, fmt.Errorf("driver %q does not serve primitive %q", drv, pt)
		}
		conn := e.spec.Connection
		dsn := fmt.Sprintf("host=%s port=%d dbname=%s sslmode=require", conn.Host, conn.Port, conn.Database)
		for k, v := range conn.Extra {
			dsn += fmt.Sprintf(" %s=%s", k, v)
		}
		d, err := db.OpenPostgres(dsn, "")
		if err != nil {
			return nil, fmt.Errorf("datastore %q: open postgres: %w", e.name, err)
		}
		return &datastore.DBQuerier{DB: d}, nil
	case spec.DatastoreDriverMemory:
		switch pt {
		case spec.PrimitiveKVStore, spec.PrimitiveCache:
			return memory.NewKV(), nil
		case spec.PrimitiveLock:
			return memory.NewLock(), nil
		case spec.PrimitiveQueue:
			return memory.NewQueue(), nil
		case spec.PrimitivePubSub:
			return memory.NewPubSub(), nil
		default:
			return nil, fmt.Errorf("driver %q does not serve primitive %q", drv, pt)
		}
	case spec.DatastoreDriverFS:
		if pt != spec.PrimitiveStorage {
			return nil, fmt.Errorf("driver %q does not serve primitive %q", drv, pt)
		}
		root := e.spec.Connection.Database
		if root == "" {
			root = filepath.Join(stateDir, "datastores", e.name+"-storage")
		}
		s, err := memory.NewStorage(root)
		if err != nil {
			return nil, fmt.Errorf("datastore %q: filesystem backend: %w", e.name, err)
		}
		return s, nil
	case spec.DatastoreDriverValkey, spec.DatastoreDriverRedis:
		// Cloud cache driver (Plan C batch 2, todo 13.5.6): serves the KV
		// primitives (cache/kvstore) over Redis/Valkey. DB/lock/queue/pubsub
		// on Redis are separate backends — not wired here.
		if pt != spec.PrimitiveCache && pt != spec.PrimitiveKVStore {
			return nil, fmt.Errorf("driver %q does not serve primitive %q (supported: cache, kvstore)", drv, pt)
		}
		addr := fmt.Sprintf("%s:%d", e.spec.Connection.Host, e.spec.Connection.Port)
		if e.spec.Connection.Host == "" {
			addr = "localhost:6379"
		}
		kv, err := rediskv.New(addr, "formspec:"+e.name)
		if err != nil {
			return nil, fmt.Errorf("datastore %q: %w", e.name, err)
		}
		return kv, nil
	default:
		return nil, fmt.Errorf("datastore %q: driver %q is not supported in single-server mode yet (supported: sqlite, postgres, memory, fs)", e.name, drv)
	}
}

// DatastoreRegistry resolves ctx.* primitives against named kind: Datastore
// manifests and per-module `spec.datastore` bindings (todo 2.9.4).
type DatastoreRegistry struct {
	mu       sync.RWMutex
	entries  map[string]*dsEntry // datastore name → entry ('default' included)
	bindings map[string]string   // module name → datastore name
	stateDir string
}

// NewDatastoreRegistry creates a registry whose 'default' entry is backed by
// the app's primary database plus shared in-memory backends.
func NewDatastoreRegistry(mainDB db.DB, stateDir string, sharedPubSub *memory.PubSub) *DatastoreRegistry {
	ps := sharedPubSub
	if ps == nil {
		ps = memory.NewPubSub()
	}
	def := &dsEntry{
		name:        "default",
		mainDB:      mainDB,
		sharedKV:    memory.NewKV(),
		sharedLock:  memory.NewLock(),
		sharedQueue: memory.NewQueue(),
		sharedPS:    ps,
		conns:       map[string]interface{}{},
	}
	return &DatastoreRegistry{
		entries:  map[string]*dsEntry{"default": def},
		bindings: map[string]string{},
		stateDir: stateDir,
	}
}

// LoadManifests registers kind: Datastore specs and kind: Module
// `spec.datastore` bindings from the loaded manifests. It fails loudly on:
//
//   - a module binding that points to an unknown datastore
//   - a Datastore driver incompatible with its declared `serves` set
//     (platform/06-datastore.md §2 compatibility table)
func (r *DatastoreRegistry) LoadManifests(manifests []manifest.RawManifest) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, raw := range manifests {
		switch spec.Kind(raw.Kind) {
		case spec.KindDatastore:
			specMap, ok := raw.Spec.(map[string]any)
			if !ok {
				continue
			}
			ds, err := manifest.RawSpecTo[spec.DatastoreSpec](specMap)
			if err != nil {
				return fmt.Errorf("datastore %s/%s: %w", raw.Metadata.Module, raw.Metadata.Name, err)
			}
			if err := validateDatastoreServes(ds); err != nil {
				return fmt.Errorf("datastore %q: %w", raw.Metadata.Name, err)
			}
			r.entries[raw.Metadata.Name] = &dsEntry{
				name:  raw.Metadata.Name,
				spec:  ds,
				conns: map[string]interface{}{},
			}
		case spec.KindModule:
			specMap, ok := raw.Spec.(map[string]any)
			if !ok {
				continue
			}
			ms, err := manifest.RawSpecToModuleSpec(specMap)
			if err != nil || ms.Datastore == "" {
				continue
			}
			r.bindings[raw.Metadata.Name] = ms.Datastore
		}
	}

	// Validate bindings after all entries are registered.
	for mod, dsName := range r.bindings {
		if _, ok := r.entries[dsName]; !ok {
			return fmt.Errorf("module %q binds datastore %q which has no kind: Datastore manifest", mod, dsName)
		}
	}
	return nil
}

// validateDatastoreServes checks driver×serves compatibility per
// platform/06-datastore.md §2.
func validateDatastoreServes(ds *spec.DatastoreSpec) error {
	if len(ds.Serves) == 0 {
		return fmt.Errorf("spec.serves must list at least one primitive")
	}
	compatible := map[spec.PrimitiveType]bool{}
	for _, p := range ds.Driver.Serves() {
		compatible[p] = true
	}
	for _, p := range ds.Serves {
		if !compatible[p] {
			return fmt.Errorf("driver %q cannot serve primitive %q", ds.Driver, p)
		}
	}
	return nil
}

// Resolve resolves one ctx.* primitive to a live connection.
//
//   - Plain call (name ""/"default"): resolves to the calling module's bound
//     datastore when it serves the primitive, else to 'default'.
//   - Explicit .named(x): only the module's own binding (or plain 'default'
//     for unbound modules) is reachable — anything else is rejected with a
//     clear cross-datastore error (platform/06-datastore.md §1.1).
func (r *DatastoreRegistry) Resolve(primitiveType, name, module string) (interface{}, error) {
	pt := spec.PrimitiveType(primitiveType)

	r.mu.RLock()
	defer r.mu.RUnlock()

	bound := r.bindings[module]
	target := name

	if target == "" || target == "default" {
		if bound != "" {
			// Module is bound: plain ctx.db() resolves to its own datastore.
			// An explicit .named("default") would be an escape hatch back to
			// the shared database — rejected (platform/06-datastore.md §1.1).
			if target == "default" {
				return nil, fmt.Errorf("ctx.%s: datastore %q is not accessible from module %q (bound to %q) — cross-datastore interaction must go through events, not direct handles (platform/06-datastore.md §1.1)", primitiveType, target, module, bound)
			}
			e := r.entries[bound]
			if e == nil {
				// Binding validated at load; defensive only.
				return nil, fmt.Errorf("ctx.%s: module %q binds unknown datastore %q", primitiveType, module, bound)
			}
			target = bound
			// Bound datastore exists but doesn't serve this primitive →
			// fall through to 'default' (e.g. module binds a db-only
			// datastore; ctx.cache() still uses the default cache).
			if !e.serves(pt) {
				target = "default"
			}
		} else {
			target = "default"
		}
	} else {
		// Explicit named access — no escape hatch across bindings.
		if bound == "" {
			if target != "default" {
				return nil, fmt.Errorf("ctx.%s: datastore %q is not accessible from module %q — a module may only reach its own spec.datastore binding or 'default' (platform/06-datastore.md §1.1)", primitiveType, target, module)
			}
		} else if target != bound {
			return nil, fmt.Errorf("ctx.%s: datastore %q is not accessible from module %q (bound to %q) — cross-datastore interaction must go through events, not direct handles (platform/06-datastore.md §1.1)", primitiveType, target, module, bound)
		}
	}

	entry := r.entries[target]
	if entry == nil {
		return nil, fmt.Errorf("ctx.%s: datastore %q not found", primitiveType, target)
	}
	if !entry.serves(pt) {
		return nil, fmt.Errorf("ctx.%s: datastore %q does not serve primitive %q", primitiveType, target, primitiveType)
	}
	return entry.conn(r.stateDir, pt)
}

// Binding returns the datastore name a module is bound to ("" = unbound).
func (r *DatastoreRegistry) Binding(module string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.bindings[module]
}

// buildDatastoreRegistry constructs the registry from manifests. Fails the
// boot when a binding or driver/serves declaration is invalid.
func buildDatastoreRegistry(manifests []manifest.RawManifest, database db.DB, stateDir string, sharedPubSub *memory.PubSub) (*DatastoreRegistry, error) {
	reg := NewDatastoreRegistry(database, stateDir, sharedPubSub)
	if err := reg.LoadManifests(manifests); err != nil {
		return nil, err
	}
	return reg, nil
}

// resolverFromRegistry adapts the registry to the ScriptExecutor resolver
// signature (todo 2.9.4).
func resolverFromRegistry(reg *DatastoreRegistry) func(primitiveType, name, module string) (interface{}, error) {
	return reg.Resolve
}
