package formspec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/primadi/formspec/internal/artifact"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
	"github.com/primadi/formspec/renderers/jsonb-persist/datastore"
	"github.com/primadi/formspec/renderers/jsonb-persist/datastore/memory"
	"github.com/primadi/formspec/renderers/jsonb-persist/datastore/minio"
	"github.com/primadi/formspec/renderers/jsonb-persist/datastore/rediskv"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
)

// ─── Infra Registry (plan docs/plan/infra-registry-3-level.md fase A) ───
//
// Module-scoped ctx.* primitive resolution per
// docs/spec/platform/06-datastore.md §1.1, restructured as a per-primitive
// multi-service registry:
//
//   - Unit registrasi = SERVICE (instance infra fisik) dengan logical name.
//     Tiap primitive type dapat menampung >1 service (mis. 2 db).
//   - Tiap primitive punya DEFAULT yang menunjuk ke salah satu service
//     teregistrasi — pointer, bukan backend implisit — dan dapat dioverride
//     via SetDefault (fase A; deklarasi manifest menyusul di fase B).
//   - A module may declare `spec.datastore: <name>` (kind: Module). Inside
//     that module's code, `ctx.db()` without arguments ALWAYS resolves to
//     the bound datastore; unbound modules resolve to the per-primitive
//     default.
//   - There is NO escape hatch: `.named(x)` may only reach the module's own
//     binding. Cross-module/cross-datastore interaction must go through
//     events/outbox — never a direct ctx.* handle. (Opening `.named()` as a
//     first-class app-registry feature is fase C.)
//
// Named services come from `kind: Datastore` manifests. Single-server
// supports drivers: sqlite, postgres, memory, fs, valkey/redis (cache,
// kvstore, lock, queue, pubsub), minio/s3 (storage). NATS fails loudly at
// resolve time — it requires the Control Plane snapshot with real
// credentials (fase E).

// serviceEntry is one registered infra service: its spec (nil for the
// built-in 'default' service) plus lazily-opened connections.
type serviceEntry struct {
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

func (e *serviceEntry) serves(pt spec.PrimitiveType) bool {
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
func (e *serviceEntry) conn(stateDir string, pt spec.PrimitiveType) (interface{}, error) {
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

func (e *serviceEntry) open(stateDir string, pt spec.PrimitiveType) (interface{}, error) {
	// Built-in 'default' service — wired at boot to the app's primary
	// database and the shared in-memory backends (todo 2.9.3 behavior
	// preserved; dev-mode convenience yang tetap tercatat di registry
	// sebagai service biasa — bukan jalur tersembunyi).
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
		case spec.PrimitiveConfig:
			// Built-in config: KV-backed (namespaced "config:") — same store
			// as cache/kvstore in dev mode (plan fase D).
			return datastore.NewKVConfig(e.sharedKV), nil
		case spec.PrimitiveLog:
			// Built-in log: in-memory sink (plan fase D) — entries live in
			// the process; the builtin ctx.log remains the fallback path.
			return datastore.NewMemoryLog(), nil
		default:
			return nil, fmt.Errorf("primitive %q is not routed through the datastore resolver", pt)
		}
	}

	// Named entry — open by driver.
	drv := e.spec.Driver
	switch drv {
	case spec.DatastoreDriverSQLite:
		if pt != spec.PrimitiveDB && pt != spec.PrimitiveKVStore && pt != spec.PrimitiveConfig && pt != spec.PrimitiveLog {
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
		if pt == spec.PrimitiveConfig || pt == spec.PrimitiveLog {
			return datastore.NewDBConfigLog(d, pt), nil
		}
		return &datastore.DBQuerier{DB: d}, nil
	case spec.DatastoreDriverPostgres:
		if pt != spec.PrimitiveDB && pt != spec.PrimitiveKVStore && pt != spec.PrimitiveConfig && pt != spec.PrimitiveLog {
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
		case spec.PrimitiveConfig:
			return datastore.NewKVConfig(memory.NewKV()), nil
		case spec.PrimitiveLog:
			return datastore.NewMemoryLog(), nil
		default:
			return nil, fmt.Errorf("driver %q does not serve primitive %q", drv, pt)
		}
	case spec.DatastoreDriverFS:
		if pt != spec.PrimitiveStorage && pt != spec.PrimitiveLog {
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
		if pt == spec.PrimitiveLog {
			fl, err := datastore.NewFileLog(root)
			if err != nil {
				return nil, fmt.Errorf("datastore %q: file log: %w", e.name, err)
			}
			return fl, nil
		}
		return s, nil
	case spec.DatastoreDriverValkey, spec.DatastoreDriverRedis:
		// Cloud driver (plan fase A2): serves the KV primitives
		// (cache/kvstore) plus lock/queue/pubsub over Redis/Valkey — the
		// full compatibility set of platform/06-datastore.md §2.
		addr := fmt.Sprintf("%s:%d", e.spec.Connection.Host, e.spec.Connection.Port)
		if e.spec.Connection.Host == "" {
			addr = "localhost:6379"
		}
		ns := "formspec:" + e.name
		switch pt {
		case spec.PrimitiveCache, spec.PrimitiveKVStore:
			kv, err := rediskv.New(addr, ns)
			if err != nil {
				return nil, fmt.Errorf("datastore %q: %w", e.name, err)
			}
			return kv, nil
		case spec.PrimitiveLock:
			l, err := rediskv.NewLock(addr, ns)
			if err != nil {
				return nil, fmt.Errorf("datastore %q: %w", e.name, err)
			}
			return l, nil
		case spec.PrimitiveQueue:
			q, err := rediskv.NewQueue(addr, ns)
			if err != nil {
				return nil, fmt.Errorf("datastore %q: %w", e.name, err)
			}
			return q, nil
		case spec.PrimitivePubSub:
			ps, err := rediskv.NewPubSub(addr, ns)
			if err != nil {
				return nil, fmt.Errorf("datastore %q: %w", e.name, err)
			}
			return ps, nil
		case spec.PrimitiveConfig, spec.PrimitiveLog:
			kv, err := rediskv.New(addr, ns)
			if err != nil {
				return nil, fmt.Errorf("datastore %q: %w", e.name, err)
			}
			if pt == spec.PrimitiveConfig {
				return datastore.NewKVConfig(kv), nil
			}
			return datastore.NewKVLog(kv), nil
		default:
			return nil, fmt.Errorf("driver %q does not serve primitive %q (supported: cache, kvstore, lock, queue, pubsub, config, log)", drv, pt)
		}
	case spec.DatastoreDriverMinio, spec.DatastoreDriverS3:
		// Object storage driver (plan fase A2): MinIO/S3 via named
		// Datastore — endpoint/bucket dari spec.connection, kredensial dari
		// spec.connection.extra dengan fallback env FORMSPEC_MINIO_*.
		if pt != spec.PrimitiveStorage {
			return nil, fmt.Errorf("driver %q does not serve primitive %q", drv, pt)
		}
		conn := e.spec.Connection
		endpoint := conn.Host
		if endpoint == "" {
			endpoint = "minio:9000"
		} else if conn.Port > 0 {
			endpoint = fmt.Sprintf("%s:%d", conn.Host, conn.Port)
		}
		bucket := conn.Database
		if bucket == "" {
			bucket = "formspec"
		}
		accessKey := conn.Extra["access_key"]
		if accessKey == "" {
			accessKey = os.Getenv("FORMSPEC_MINIO_ACCESS_KEY")
		}
		secretKey := conn.Extra["secret_key"]
		if secretKey == "" {
			secretKey = os.Getenv("FORMSPEC_MINIO_SECRET_KEY")
		}
		useSSL := conn.Extra["use_ssl"] == "true"
		s, err := minio.NewStorage(endpoint, accessKey, secretKey, bucket, useSSL)
		if err != nil {
			return nil, fmt.Errorf("datastore %q: minio backend: %w", e.name, err)
		}
		return s, nil
	default:
		return nil, fmt.Errorf("datastore %q: driver %q is not supported in single-server mode yet (supported: sqlite, postgres, memory, fs, valkey, redis, minio, s3)", e.name, drv)
	}
}

// DatastoreRegistry resolves ctx.* primitives against registered infra
// services (kind: Datastore manifests + the built-in 'default' service) and
// per-module `spec.datastore` bindings (todo 2.9.4). Strukturnya adalah
// InfraRegistry per-primitive: tiap primitive dapat punya banyak service
// dengan satu default yang overridable (plan fase A).
type DatastoreRegistry struct {
	mu       sync.RWMutex
	services map[string]*serviceEntry      // service name → entry ('default' included)
	defaults map[spec.PrimitiveType]string // primitive → default service name (overridable)
	bindings map[string]string             // module name → service name (legacy ModuleSpec.Datastore)
	// App Registry selection (plan fase B):
	moduleApp map[string]string                        // module name → owning App name (first App that mounts it)
	moduleSel map[string]map[spec.PrimitiveType]string // module → primitive → service (ModuleSpec.Datastores)
	appSel    map[string]map[spec.PrimitiveType]string // app → primitive → service (AppSpec.Datastores defaults)
	appNamed  map[string]map[string]string             // app → "primitive/alias" → service (named logical primitives, fase C)
	// Workspace Binding permissions (plan fase B2): service → ceiling from
	// the snapshot binding (overrides the spec's own access.permission).
	permissions map[string]*spec.DatastorePermission
	stateDir    string
}

// NewDatastoreRegistry creates a registry whose built-in 'default' service is
// backed by the app's primary database plus shared in-memory backends. Every
// primitive in the closed set (9 jenis) starts with 'default' as its default
// service — overridable via SetDefault.
func NewDatastoreRegistry(mainDB db.DB, stateDir string, sharedPubSub *memory.PubSub) *DatastoreRegistry {
	ps := sharedPubSub
	if ps == nil {
		ps = memory.NewPubSub()
	}
	def := &serviceEntry{
		name:        "default",
		mainDB:      mainDB,
		sharedKV:    memory.NewKV(),
		sharedLock:  memory.NewLock(),
		sharedQueue: memory.NewQueue(),
		sharedPS:    ps,
		conns:       map[string]interface{}{},
	}
	defaults := make(map[spec.PrimitiveType]string, len(spec.AllPrimitiveTypes()))
	for _, p := range spec.AllPrimitiveTypes() {
		defaults[p] = "default"
	}
	return &DatastoreRegistry{
		services:    map[string]*serviceEntry{"default": def},
		defaults:    defaults,
		bindings:    map[string]string{},
		moduleApp:   map[string]string{},
		moduleSel:   map[string]map[spec.PrimitiveType]string{},
		appSel:      map[string]map[spec.PrimitiveType]string{},
		appNamed:    map[string]map[string]string{},
		permissions: map[string]*spec.DatastorePermission{},
		stateDir:    stateDir,
	}
}

// SetDefault overrides which registered service serves as the default for
// one primitive type (plan fase A: default overridable). The service must
// exist and serve the primitive — registrasi tetap eksplisit; default hanya
// pointer ke service yang sudah teregistrasi.
func (r *DatastoreRegistry) SetDefault(primitiveType, service string) error {
	pt := spec.PrimitiveType(primitiveType)
	if !spec.IsValidPrimitiveType(pt) {
		return fmt.Errorf("datastore registry: unknown primitive type %q", primitiveType)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.services[service]
	if !ok {
		return fmt.Errorf("datastore registry: cannot set default for %q: service %q is not registered", primitiveType, service)
	}
	if !e.serves(pt) {
		return fmt.Errorf("datastore registry: service %q does not serve primitive %q", service, primitiveType)
	}
	r.defaults[pt] = service
	return nil
}

// Default returns the name of the service currently serving as the default
// for one primitive type.
func (r *DatastoreRegistry) Default(primitiveType string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.defaults[spec.PrimitiveType(primitiveType)]
}

// Services returns the names of all registered services that serve the given
// primitive type (multi-service per primitive — mis. 2 db).
func (r *DatastoreRegistry) Services(primitiveType string) []string {
	pt := spec.PrimitiveType(primitiveType)
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []string
	for name, e := range r.services {
		if e.serves(pt) {
			out = append(out, name)
		}
	}
	return out
}

// parseDatastoreKey parses a datastores map key: "db" → (db, "") for a
// per-primitive default, "db/analytics" → (db, "analytics") for a named
// logical primitive (plan fase B).
func parseDatastoreKey(key string) (spec.PrimitiveType, string, error) {
	prim, alias, found := strings.Cut(key, "/")
	if !found {
		alias = ""
	}
	pt := spec.PrimitiveType(prim)
	if !spec.IsValidPrimitiveType(pt) {
		return "", "", fmt.Errorf("unknown primitive type %q in datastore key %q", prim, key)
	}
	if found && alias == "" {
		return "", "", fmt.Errorf("empty alias in datastore key %q — use %q for the per-primitive default", key, prim)
	}
	return pt, alias, nil
}

// validateSelection checks one datastores selection map (App- or
// module-level): every key must parse, and every target service must be
// registered and serve the primitive (fail loudly at load).
func (r *DatastoreRegistry) validateSelection(owner string, sel map[string]string) error {
	for key, service := range sel {
		pt, _, err := parseDatastoreKey(key)
		if err != nil {
			return fmt.Errorf("%s: %w", owner, err)
		}
		e, ok := r.services[service]
		if !ok {
			return fmt.Errorf("%s: datastore key %q targets service %q which has no kind: Datastore manifest", owner, key, service)
		}
		if !e.serves(pt) {
			return fmt.Errorf("%s: datastore key %q targets service %q which does not serve primitive %q", owner, key, service, pt)
		}
	}
	return nil
}

// chainTarget resolves the plain-call target for one primitive through the
// selection chain: module override → App selection → registry default
// (plan fase B). Called with r.mu held.
func (r *DatastoreRegistry) chainTarget(pt spec.PrimitiveType, module string) string {
	if ms := r.moduleSel[module]; ms != nil {
		if s, ok := ms[pt]; ok {
			if e := r.services[s]; e != nil && e.serves(pt) {
				return s
			}
		}
	}
	if app := r.moduleApp[module]; app != "" {
		if as := r.appSel[app]; as != nil {
			if s, ok := as[pt]; ok {
				if e := r.services[s]; e != nil && e.serves(pt) {
					return s
				}
			}
		}
	}
	return r.defaults[pt]
}

// LoadManifests registers kind: Datastore services and kind: Module
// `spec.datastore` bindings from the loaded manifests. It fails loudly on:
//
//   - a module binding that points to an unknown service
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
			r.services[raw.Metadata.Name] = &serviceEntry{
				name:  raw.Metadata.Name,
				spec:  ds,
				conns: map[string]interface{}{},
			}
		case spec.KindApp:
			// App Registry selection (plan fase B): AppSpec.Datastores sets
			// the per-primitive default for every module of this App.
			specMap, ok := raw.Spec.(map[string]any)
			if !ok {
				continue
			}
			as, err := manifest.RawSpecTo[spec.AppSpec](specMap)
			if err != nil {
				return fmt.Errorf("app %s: %w", raw.Metadata.Name, err)
			}
			if len(as.Datastores) > 0 {
				if err := r.validateSelection("app "+raw.Metadata.Name, as.Datastores); err != nil {
					return err
				}
				sel := map[spec.PrimitiveType]string{}
				named := map[string]string{}
				for key, service := range as.Datastores {
					pt, alias, err := parseDatastoreKey(key)
					if err != nil {
						return fmt.Errorf("app %s: %w", raw.Metadata.Name, err)
					}
					if alias == "" {
						sel[pt] = service
					} else {
						named[string(pt)+"/"+alias] = service
					}
				}
				r.appSel[raw.Metadata.Name] = sel
				if len(named) > 0 {
					r.appNamed[raw.Metadata.Name] = named
				}
			}
			// Record module → App ownership (first App that mounts it wins).
			for _, mod := range as.Modules {
				if _, ok := r.moduleApp[mod]; !ok {
					r.moduleApp[mod] = raw.Metadata.Name
				}
			}
		case spec.KindModule:
			specMap, ok := raw.Spec.(map[string]any)
			if !ok {
				continue
			}
			ms, err := manifest.RawSpecToModuleSpec(specMap)
			if err != nil {
				continue
			}
			if ms.Datastore != "" {
				r.bindings[raw.Metadata.Name] = ms.Datastore
			}
			// Module-level selection (plan fase B): overrides the App-level
			// selection per primitive. Named keys ("db/analytics") register a
			// module-scoped named logical primitive (fase C) — merged into
			// appNamed under the module's owning App so resolution is uniform.
			if len(ms.Datastores) > 0 {
				if err := r.validateSelection("module "+raw.Metadata.Name, ms.Datastores); err != nil {
					return err
				}
				sel := map[spec.PrimitiveType]string{}
				for key, service := range ms.Datastores {
					pt, alias, err := parseDatastoreKey(key)
					if err != nil {
						return fmt.Errorf("module %s: %w", raw.Metadata.Name, err)
					}
					if alias != "" {
						app := r.moduleApp[raw.Metadata.Name]
						if app == "" {
							return fmt.Errorf("module %s: named datastore key %q requires the module to be mounted by a kind: App", raw.Metadata.Name, key)
						}
						if r.appNamed[app] == nil {
							r.appNamed[app] = map[string]string{}
						}
						r.appNamed[app][string(pt)+"/"+alias] = service
						continue
					}
					sel[pt] = service
				}
				if len(sel) > 0 {
					r.moduleSel[raw.Metadata.Name] = sel
				}
			}
		}
	}

	// Validate bindings after all services are registered.
	for mod, dsName := range r.bindings {
		if _, ok := r.services[dsName]; !ok {
			return fmt.Errorf("module %q binds datastore %q which has no kind: Datastore manifest", mod, dsName)
		}
	}
	return nil
}

// LoadSnapshotDatastores populates the Infra Registry from a Control Plane
// snapshot's DatastoreBindings (plan fase B2 — Workspace Binding). Each
// binding is a service the Control Plane already authorized for this
// workspace (access.filter evaluated at snapshot time); the permission
// ceiling travels with the binding. This is the Control-Plane-distributed
// path — the manifest-local LoadManifests remains the dev-mode path.
//
// Services from the snapshot are added alongside existing ones (later
// loads win on name collision); the built-in 'default' service is never
// replaced.
func (r *DatastoreRegistry) LoadSnapshotDatastores(bindings []artifact.DatastoreBinding) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, b := range bindings {
		if b.Name == "" || b.Name == "default" {
			continue // built-in default is never replaced from a snapshot
		}
		var ds spec.DatastoreSpec
		if err := json.Unmarshal(b.Spec, &ds); err != nil {
			return fmt.Errorf("datastore %q: decode spec: %w", b.Name, err)
		}
		if err := validateDatastoreServes(&ds); err != nil {
			return fmt.Errorf("datastore %q: %w", b.Name, err)
		}
		r.services[b.Name] = &serviceEntry{
			name:  b.Name,
			spec:  &ds,
			conns: map[string]interface{}{},
		}
		if len(b.Permission) > 0 {
			var perm spec.DatastorePermission
			if err := json.Unmarshal(b.Permission, &perm); err != nil {
				return fmt.Errorf("datastore %q: decode permission: %w", b.Name, err)
			}
			r.permissions[b.Name] = &perm
		}
	}
	return nil
}

// Permission returns the operation ceiling for a named service (nil =
// read_write everywhere). The ceiling travels with the workspace binding
// (plan fase B2) — deklarasi `uses` module tidak bisa melampaui ini.
func (r *DatastoreRegistry) Permission(service string) *spec.DatastorePermission {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if p, ok := r.permissions[service]; ok {
		return p
	}
	if e, ok := r.services[service]; ok && e.spec != nil && e.spec.Access != nil {
		return e.spec.Access.Permission
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
//   - Plain call (name ""/"default"): resolves through the selection chain
//     (plan fase B): module binding (legacy `datastore:`) → module
//     `datastores` override → App selection → per-primitive registry
//     default. A bound module's plain call still resolves to its own
//     binding when it serves the primitive.
//   - Explicit .named(x): resolves a named logical primitive registered in
//     the App Registry (plan fase C): key "primitive/alias" in
//     AppSpec.Datastores or ModuleSpec.Datastores → service. Unknown alias →
//     DATASTORE_NOT_FOUND; the module's own binding and the resolved chain
//     default remain reachable as plain service names. Direct service-name
//     access across bindings is still rejected (platform/06-datastore.md
//     §1.1) — named logical primitives are the sanctioned escape hatch.
func (r *DatastoreRegistry) Resolve(primitiveType, name, module string) (interface{}, error) {
	pt := spec.PrimitiveType(primitiveType)

	r.mu.RLock()
	defer r.mu.RUnlock()

	bound := r.bindings[module]
	target := name

	if target == "" || target == "default" {
		if bound != "" {
			// Module is bound: plain ctx.db() resolves to its own service.
			// An explicit .named("default") would be an escape hatch back to
			// the shared database — rejected (platform/06-datastore.md §1.1).
			if target == "default" {
				return nil, fmt.Errorf("ctx.%s: datastore %q is not accessible from module %q (bound to %q) — cross-datastore interaction must go through events, not direct handles (platform/06-datastore.md §1.1)", primitiveType, target, module, bound)
			}
			e := r.services[bound]
			if e == nil {
				// Binding validated at load; defensive only.
				return nil, fmt.Errorf("ctx.%s: module %q binds unknown datastore %q", primitiveType, module, bound)
			}
			target = bound
			// Bound service exists but doesn't serve this primitive →
			// fall through to the selection chain (module override → App
			// selection → registry default) for that primitive.
			if !e.serves(pt) {
				target = r.chainTarget(pt, module)
			}
		} else {
			target = r.chainTarget(pt, module)
		}
	} else {
		// Explicit named access — no escape hatch across bindings. The
		// resolved chain default is always reachable.
		chain := r.chainTarget(pt, module)
		if bound == "" {
			if target != "default" && target != chain {
				return nil, fmt.Errorf("ctx.%s: datastore %q is not accessible from module %q — a module may only reach its own spec.datastore binding or the default (platform/06-datastore.md §1.1)", primitiveType, target, module)
			}
		} else if target != bound {
			return nil, fmt.Errorf("ctx.%s: datastore %q is not accessible from module %q (bound to %q) — cross-datastore interaction must go through events, not direct handles (platform/06-datastore.md §1.1)", primitiveType, target, module, bound)
		}
	}

	entry := r.services[target]
	if entry == nil {
		return nil, fmt.Errorf("ctx.%s: datastore %q not found", primitiveType, target)
	}
	if !entry.serves(pt) {
		return nil, fmt.Errorf("ctx.%s: datastore %q does not serve primitive %q", primitiveType, target, primitiveType)
	}
	conn, err := entry.conn(r.stateDir, pt)
	if err != nil {
		return nil, err
	}
	return r.wrapWithPermission(target, primitiveType, conn), nil
}

// ResolveNamed resolves a named logical primitive (plan fase C):
// ctx.db.named("analytics") → the service registered under key "db/analytics"
// in the owning App's App Registry (AppSpec.Datastores or the module's own
// ModuleSpec.Datastores, merged into appNamed). Unknown alias →
// DATASTORE_NOT_FOUND (platform/06-datastore.md §6). The alias namespace is
// app-level: two Apps may define "analytics" pointing at different services.
func (r *DatastoreRegistry) ResolveNamed(primitiveType, alias, module string) (interface{}, error) {
	pt := spec.PrimitiveType(primitiveType)

	r.mu.RLock()
	defer r.mu.RUnlock()

	app := r.moduleApp[module]
	if app == "" {
		return nil, fmt.Errorf("ctx.%s.named(%q): DATASTORE_NOT_FOUND — module %q is not mounted by a kind: App, so no named logical primitives are registered (platform/06-datastore.md §6)", primitiveType, alias, module)
	}
	service, ok := r.appNamed[app][string(pt)+"/"+alias]
	if !ok {
		return nil, fmt.Errorf("ctx.%s.named(%q): DATASTORE_NOT_FOUND — no named logical primitive %q registered for app %q (platform/06-datastore.md §6)", primitiveType, alias, alias, app)
	}
	entry := r.services[service]
	if entry == nil {
		// Validated at load; defensive only.
		return nil, fmt.Errorf("ctx.%s.named(%q): datastore %q not found", primitiveType, alias, service)
	}
	if !entry.serves(pt) {
		return nil, fmt.Errorf("ctx.%s.named(%q): datastore %q does not serve primitive %q", primitiveType, alias, service, primitiveType)
	}
	conn, err := entry.conn(r.stateDir, pt)
	if err != nil {
		return nil, err
	}
	return r.wrapWithPermission(service, primitiveType, conn), nil
}

// wrapWithPermission wraps a resolved connection with the workspace
// binding's permission ceiling (plan fase B2 follow-up) when the ceiling is
// restrictive (read or write — read_write/nil needs no guard). Called with
// r.mu held (Resolve/ResolveNamed hold the read lock).
func (r *DatastoreRegistry) wrapWithPermission(service, primitiveType string, conn interface{}) interface{} {
	perm := r.permissions[service]
	if perm == nil {
		if e, ok := r.services[service]; ok && e.spec != nil && e.spec.Access != nil {
			perm = e.spec.Access.Permission
		}
	}
	if perm == nil || perm.Default == spec.AccessReadWrite || perm.Default == "" {
		return conn // no restrictive ceiling
	}
	return &permissionGuard{conn: conn, perm: perm, primType: primitiveType, service: service}
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
