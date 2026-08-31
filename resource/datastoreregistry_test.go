package formspec

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/primadi/formspec/internal/artifact"
	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
	"github.com/primadi/formspec/renderers/jsonb-persist/datastore"
)

// dsTestManifests builds raw manifests: one named sqlite datastore
// (ds-alpha) plus module bindings — alpha bound, beta unbound.
func dsTestManifests() []manifest.RawManifest {
	return []manifest.RawManifest{
		{
			Kind: "Datastore",
			Metadata: manifest.RawMetadata{
				Name:        "ds-alpha",
				Module:      "alpha",
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			Spec: map[string]any{
				"serves": []any{"db"},
				"driver": "sqlite",
			},
			Source: "datastores/alpha.yaml",
		},
		{
			Kind: "Module",
			Metadata: manifest.RawMetadata{
				Name:        "alpha",
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			Spec: map[string]any{
				"version":   "1.0.0",
				"datastore": "ds-alpha",
			},
			Source: "modules/alpha/module.yaml",
		},
		{
			Kind: "Module",
			Metadata: manifest.RawMetadata{
				Name:        "beta",
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			Spec:   map[string]any{"version": "1.0.0"},
			Source: "modules/beta/module.yaml",
		},
	}
}

func newDSRegistryForTest(t *testing.T, manifests []manifest.RawManifest) *DatastoreRegistry {
	t.Helper()
	stateDir := t.TempDir()
	mainDB, err := db.OpenSQLite(filepath.Join(stateDir, "main.db"), nil)
	if err != nil {
		t.Fatalf("open main db: %v", err)
	}
	t.Cleanup(func() { _ = mainDB.Close() })
	reg := NewDatastoreRegistry(mainDB, stateDir, nil)
	if err := reg.LoadManifests(manifests); err != nil {
		t.Fatalf("LoadManifests: %v", err)
	}
	return reg
}

// TestDatastoreRegistry_PlainCall_ModuleBound proves ctx.db() without
// arguments resolves to the module's bound datastore and that the resolved
// connection is backed by the named SQLite file, not the main database
// (todo 2.9.4, platform/06-datastore.md §1.1).
func TestDatastoreRegistry_PlainCall_ModuleBound(t *testing.T) {
	reg := newDSRegistryForTest(t, dsTestManifests())

	conn, err := reg.Resolve("db", "", "alpha")
	if err != nil {
		t.Fatalf("resolve alpha plain: %v", err)
	}
	dq, ok := conn.(*datastore.DBQuerier)
	if !ok {
		t.Fatalf("resolved connection type %T, want *datastore.DBQuerier", conn)
	}

	ctx := context.Background()
	if _, err := dq.Query(ctx, "CREATE TABLE IF NOT EXISTS probe_alpha (id INTEGER)"); err != nil {
		t.Fatalf("create probe table via ds-alpha conn: %v", err)
	}

	alphaPath := filepath.Join(reg.stateDir, "datastores", "ds-alpha.db")
	if _, err := os.Stat(alphaPath); err != nil {
		t.Fatalf("ds-alpha sqlite file not created at %s: %v", alphaPath, err)
	}
	alphaDB, err := db.OpenSQLite(alphaPath, nil)
	if err != nil {
		t.Fatalf("open %s: %v", alphaPath, err)
	}
	defer alphaDB.Close()
	if ok, _ := alphaDB.HasTable(ctx, "", "probe_alpha"); !ok {
		t.Fatalf("probe_alpha table not found in ds-alpha db — plain call did not resolve to the bound datastore")
	}

	defaultConn, err := reg.Resolve("db", "", "beta")
	if err != nil {
		t.Fatalf("resolve beta plain: %v", err)
	}
	mainDQ := defaultConn.(*datastore.DBQuerier)
	if ok, _ := mainDQ.DB.HasTable(ctx, "", "probe_alpha"); ok {
		t.Fatalf("probe_alpha table leaked into the main database — isolation broken")
	}

	if got := reg.Binding("alpha"); got != "ds-alpha" {
		t.Fatalf("Binding(alpha) = %q, want ds-alpha", got)
	}
	if got := reg.Binding("beta"); got != "" {
		t.Fatalf("Binding(beta) = %q, want empty", got)
	}
}

// TestDatastoreRegistry_CrossDatastoreBlocked proves .named() cannot reach
// another module's datastore — no escape hatch even for a bound module
// targeting 'default', nor an unbound module targeting a named datastore.
func TestDatastoreRegistry_CrossDatastoreBlocked(t *testing.T) {
	reg := newDSRegistryForTest(t, dsTestManifests())

	cases := []struct {
		name    string
		prim    string
		ds      string
		module  string
		wantSub string
	}{
		{"bound → other named", "db", "some-other-db", "alpha", "not accessible"},
		{"bound → escape to default", "db", "default", "alpha", "not accessible"},
		{"unbound → named", "db", "ds-alpha", "beta", "not accessible"},
	}
	for _, tc := range cases {
		_, err := reg.Resolve(tc.prim, tc.ds, tc.module)
		if err == nil || !strings.Contains(err.Error(), tc.wantSub) {
			t.Fatalf("%s: want %q error, got %v", tc.name, tc.wantSub, err)
		}
		if !strings.Contains(err.Error(), "06-datastore.md") {
			t.Fatalf("%s: error should cite platform/06-datastore.md §1.1, got %v", tc.name, err)
		}
	}

	// Allowed explicit references.
	if _, err := reg.Resolve("db", "default", "beta"); err != nil {
		t.Fatalf("unbound module .named(default): unexpected error %v", err)
	}
	if _, err := reg.Resolve("db", "ds-alpha", "alpha"); err != nil {
		t.Fatalf("bound module .named(own binding): unexpected error %v", err)
	}
}

// TestDatastoreRegistry_UnboundFallsBackToDefault proves unbound modules
// resolve plain calls to 'default'.
func TestDatastoreRegistry_UnboundFallsBackToDefault(t *testing.T) {
	reg := newDSRegistryForTest(t, dsTestManifests())

	conn, err := reg.Resolve("db", "", "beta")
	if err != nil {
		t.Fatalf("resolve beta plain: %v", err)
	}
	dq := conn.(*datastore.DBQuerier)
	rows, err := dq.Query(context.Background(), "SELECT 1 AS one")
	if err != nil {
		t.Fatalf("query via default: %v", err)
	}
	if len(rows) != 1 || rows[0]["one"] != int64(1) {
		t.Fatalf("unexpected rows: %v", rows)
	}
}

// TestDatastoreRegistry_LoadErrors proves boot fails loudly on invalid
// declarations: binding to an unknown datastore, and driver×serves mismatch.
func TestDatastoreRegistry_LoadErrors(t *testing.T) {
	stateDir := t.TempDir()
	mainDB, err := db.OpenSQLite(filepath.Join(stateDir, "main.db"), nil)
	if err != nil {
		t.Fatalf("open main db: %v", err)
	}
	t.Cleanup(func() { _ = mainDB.Close() })

	bad := []manifest.RawManifest{
		{
			Kind:     "Module",
			Metadata: manifest.RawMetadata{Name: "alpha"},
			Spec:     map[string]any{"version": "1.0.0", "datastore": "missing"},
		},
	}
	reg := NewDatastoreRegistry(mainDB, stateDir, nil)
	if err := reg.LoadManifests(bad); err == nil || !strings.Contains(err.Error(), "no kind: Datastore manifest") {
		t.Fatalf("unknown binding: want error, got %v", err)
	}

	mismatch := []manifest.RawManifest{
		{
			Kind:     "Datastore",
			Metadata: manifest.RawMetadata{Name: "bad-ds"},
			Spec:     map[string]any{"serves": []any{"db", "cache"}, "driver": "sqlite"},
		},
	}
	reg2 := NewDatastoreRegistry(mainDB, stateDir, nil)
	if err := reg2.LoadManifests(mismatch); err == nil || !strings.Contains(err.Error(), "cannot serve primitive") {
		t.Fatalf("driver×serves mismatch: want error, got %v", err)
	}
}

// TestDatastoreRegistry_PrimitiveFallback proves a bound module whose
// datastore doesn't serve a primitive falls back to 'default' for that
// primitive (e.g. db-only binding; ctx.cache() still works).
func TestDatastoreRegistry_PrimitiveFallback(t *testing.T) {
	reg := newDSRegistryForTest(t, dsTestManifests())

	conn, err := reg.Resolve("cache", "", "alpha")
	if err != nil {
		t.Fatalf("resolve cache for bound module: %v", err)
	}
	if conn == nil {
		t.Fatalf("cache connection is nil")
	}
	setter, ok := conn.(interface {
		Set(ctx context.Context, key string, value any, ttl time.Duration) error
	})
	_ = setter
	if !ok {
		t.Fatalf("cache connection type %T does not look like a KV backend", conn)
	}
}

// TestDatastoreRegistry_UnsupportedDriver proves cloud drivers fail loudly
// at resolve time with a clear single-server message.
func TestDatastoreRegistry_UnsupportedDriver(t *testing.T) {
	stateDir := t.TempDir()
	mainDB, err := db.OpenSQLite(filepath.Join(stateDir, "main.db"), nil)
	if err != nil {
		t.Fatalf("open main db: %v", err)
	}
	t.Cleanup(func() { _ = mainDB.Close() })

	manifests := []manifest.RawManifest{
		{
			Kind:     "Datastore",
			Metadata: manifest.RawMetadata{Name: "cloud-cache"},
			Spec: map[string]any{
				"serves": []any{"cache"},
				"driver": "valkey",
			},
		},
	}
	reg := NewDatastoreRegistry(mainDB, stateDir, nil)
	if err := reg.LoadManifests(manifests); err != nil {
		t.Fatalf("LoadManifests: %v", err)
	}
	// Bind a module so the named datastore is reachable, then resolve.
	bindings := []manifest.RawManifest{
		{
			Kind:     "Module",
			Metadata: manifest.RawMetadata{Name: "cloud-mod"},
			Spec:     map[string]any{"version": "1.0.0", "datastore": "cloud-cache"},
		},
	}
	if err := reg.LoadManifests(bindings); err != nil {
		t.Fatalf("LoadManifests bindings: %v", err)
	}
	_, err = reg.Resolve("cache", "cloud-cache", "cloud-mod")
	// Plan C batch 2: valkey/redis now RESOLVE for KV primitives — the
	// error is a connection failure (no Redis server in the test env), not
	// "unsupported driver". Remaining cloud drivers (s3/minio/nats) still
	// fail as unsupported.
	if err == nil {
		t.Fatalf("valkey driver: want connection error (no redis server), got nil")
	}
	if strings.Contains(err.Error(), "not supported in single-server mode") {
		t.Fatalf("valkey driver: should be supported now, got %v", err)
	}
}

// dsTestManifestsWithBeta extends dsTestManifests with a second sqlite
// datastore (ds-beta) — the multi-service-per-primitive scenario (plan
// docs/plan/infra-registry-3-level.md fase A: 2 db teregistrasi).
func dsTestManifestsWithBeta() []manifest.RawManifest {
	return append(dsTestManifests(), manifest.RawManifest{
		Kind: "Datastore",
		Metadata: manifest.RawMetadata{
			Name:        "ds-beta",
			Module:      "beta",
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
		Spec: map[string]any{
			"serves": []any{"db"},
			"driver": "sqlite",
		},
		Source: "datastores/beta.yaml",
	})
}

// TestDatastoreRegistry_MultiServicePerPrimitive proves one primitive type
// can hold more than one registered service (mis. 2 db) — 'default' plus
// named services — and Services() lists them all.
func TestDatastoreRegistry_MultiServicePerPrimitive(t *testing.T) {
	reg := newDSRegistryForTest(t, dsTestManifestsWithBeta())

	services := reg.Services("db")
	want := map[string]bool{"default": false, "ds-alpha": false, "ds-beta": false}
	for _, s := range services {
		if _, ok := want[s]; ok {
			want[s] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Fatalf("Services(db) missing %q — got %v", name, services)
		}
	}

	// cache only has the built-in default (no cache service registered).
	services = reg.Services("cache")
	if len(services) != 1 || services[0] != "default" {
		t.Fatalf("Services(cache) = %v, want only [default]", services)
	}
}

// TestDatastoreRegistry_SetDefaultOverride proves the per-primitive default
// is a pointer to a registered service and can be overridden: after
// SetDefault("db", "ds-beta"), an unbound module's plain ctx.db() resolves
// to ds-beta (its own sqlite file), while other primitives are unaffected.
func TestDatastoreRegistry_SetDefaultOverride(t *testing.T) {
	reg := newDSRegistryForTest(t, dsTestManifestsWithBeta())

	if got := reg.Default("db"); got != "default" {
		t.Fatalf("Default(db) before override = %q, want %q", got, "default")
	}
	if err := reg.SetDefault("db", "ds-beta"); err != nil {
		t.Fatalf("SetDefault(db, ds-beta): %v", err)
	}
	if got := reg.Default("db"); got != "ds-beta" {
		t.Fatalf("Default(db) after override = %q, want ds-beta", got)
	}

	// Unbound module plain call now resolves to ds-beta.
	conn, err := reg.Resolve("db", "", "beta")
	if err != nil {
		t.Fatalf("resolve beta plain after override: %v", err)
	}
	dq, ok := conn.(*datastore.DBQuerier)
	if !ok {
		t.Fatalf("resolved connection type %T, want *datastore.DBQuerier", conn)
	}
	ctx := context.Background()
	if _, err := dq.Query(ctx, "CREATE TABLE IF NOT EXISTS probe_beta (id INTEGER)"); err != nil {
		t.Fatalf("create probe table via ds-beta conn: %v", err)
	}
	betaPath := filepath.Join(reg.stateDir, "datastores", "ds-beta.db")
	if _, err := os.Stat(betaPath); err != nil {
		t.Fatalf("ds-beta sqlite file not created at %s: %v", betaPath, err)
	}
	betaDB, err := db.OpenSQLite(betaPath, nil)
	if err != nil {
		t.Fatalf("open %s: %v", betaPath, err)
	}
	defer betaDB.Close()
	if ok, _ := betaDB.HasTable(ctx, "", "probe_beta"); !ok {
		t.Fatalf("probe_beta table not found in ds-beta db — default override not applied")
	}

	// Bound module keeps its own binding — override does not leak.
	conn, err = reg.Resolve("db", "", "alpha")
	if err != nil {
		t.Fatalf("resolve alpha plain after override: %v", err)
	}
	if dq, ok = conn.(*datastore.DBQuerier); ok {
		if _, err := os.Stat(filepath.Join(reg.stateDir, "datastores", "ds-alpha.db")); err != nil {
			t.Fatalf("bound module should still resolve to ds-alpha: %v", err)
		}
	}

	// Other primitives unaffected.
	if got := reg.Default("cache"); got != "default" {
		t.Fatalf("Default(cache) = %q, want default (override must be per-primitive)", got)
	}

	// Unbound module may now .named() the per-primitive default explicitly.
	if _, err := reg.Resolve("db", "ds-beta", "beta"); err != nil {
		t.Fatalf("unbound module .named(per-primitive default): unexpected error %v", err)
	}

	// Errors: unknown service, unknown primitive, service not serving it.
	if err := reg.SetDefault("db", "nope"); err == nil || !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("SetDefault unknown service: want error, got %v", err)
	}
	if err := reg.SetDefault("bogus", "ds-beta"); err == nil || !strings.Contains(err.Error(), "unknown primitive") {
		t.Fatalf("SetDefault unknown primitive: want error, got %v", err)
	}
	if err := reg.SetDefault("storage", "ds-beta"); err == nil || !strings.Contains(err.Error(), "does not serve") {
		t.Fatalf("SetDefault non-serving service: want error, got %v", err)
	}
}

// appRegistryManifests builds the fase B scenario: two sqlite datastores
// (pg-main, pg-analytics), an App (shop) selecting db→pg-main, and two
// modules — billing (unbound, inherits the App selection) and reporting
// (module-level override db→pg-analytics).
func appRegistryManifests() []manifest.RawManifest {
	return []manifest.RawManifest{
		{
			Kind:     "Datastore",
			Metadata: manifest.RawMetadata{Name: "pg-main"},
			Spec:     map[string]any{"serves": []any{"db"}, "driver": "sqlite"},
			Source:   "datastores/pg-main.yaml",
		},
		{
			Kind:     "Datastore",
			Metadata: manifest.RawMetadata{Name: "pg-analytics"},
			Spec:     map[string]any{"serves": []any{"db"}, "driver": "sqlite"},
			Source:   "datastores/pg-analytics.yaml",
		},
		{
			Kind:     "App",
			Metadata: manifest.RawMetadata{Name: "shop"},
			Spec: map[string]any{
				"root_url":   "/",
				"modules":    []any{"billing", "reporting"},
				"datastores": map[string]any{"db": "pg-main"},
			},
			Source: "shop.yaml",
		},
		{
			Kind:     "Module",
			Metadata: manifest.RawMetadata{Name: "billing"},
			Spec:     map[string]any{"version": "1.0.0"},
			Source:   "modules/billing/module.yaml",
		},
		{
			Kind:     "Module",
			Metadata: manifest.RawMetadata{Name: "reporting"},
			Spec: map[string]any{
				"version":    "1.0.0",
				"datastores": map[string]any{"db": "pg-analytics"},
			},
			Source: "modules/reporting/module.yaml",
		},
	}
}

// TestDatastoreRegistry_AppSelection proves the App-level datastores map
// sets the per-primitive default for every module of the App (plan fase B):
// billing (no override) resolves plain ctx.db() to pg-main.
func TestDatastoreRegistry_AppSelection(t *testing.T) {
	reg := newDSRegistryForTest(t, appRegistryManifests())

	conn, err := reg.Resolve("db", "", "billing")
	if err != nil {
		t.Fatalf("resolve billing plain: %v", err)
	}
	dq, ok := conn.(*datastore.DBQuerier)
	if !ok {
		t.Fatalf("resolved connection type %T, want *datastore.DBQuerier", conn)
	}
	ctx := context.Background()
	if _, err := dq.Query(ctx, "CREATE TABLE IF NOT EXISTS probe_app (id INTEGER)"); err != nil {
		t.Fatalf("create probe table: %v", err)
	}
	mainPath := filepath.Join(reg.stateDir, "datastores", "pg-main.db")
	if _, err := os.Stat(mainPath); err != nil {
		t.Fatalf("pg-main sqlite file not created at %s — App selection not applied: %v", mainPath, err)
	}
}

// TestDatastoreRegistry_ModuleOverride proves ModuleSpec.Datastores
// overrides the App-level selection per primitive (plan fase B): reporting
// resolves plain ctx.db() to pg-analytics while billing stays on pg-main.
func TestDatastoreRegistry_ModuleOverride(t *testing.T) {
	reg := newDSRegistryForTest(t, appRegistryManifests())

	conn, err := reg.Resolve("db", "", "reporting")
	if err != nil {
		t.Fatalf("resolve reporting plain: %v", err)
	}
	dq, ok := conn.(*datastore.DBQuerier)
	if !ok {
		t.Fatalf("resolved connection type %T, want *datastore.DBQuerier", conn)
	}
	ctx := context.Background()
	if _, err := dq.Query(ctx, "CREATE TABLE IF NOT EXISTS probe_mod (id INTEGER)"); err != nil {
		t.Fatalf("create probe table: %v", err)
	}
	analyticsPath := filepath.Join(reg.stateDir, "datastores", "pg-analytics.db")
	if _, err := os.Stat(analyticsPath); err != nil {
		t.Fatalf("pg-analytics sqlite file not created at %s — module override not applied: %v", analyticsPath, err)
	}
	// The App-level service must NOT have received the write.
	mainPath := filepath.Join(reg.stateDir, "datastores", "pg-main.db")
	if _, err := os.Stat(mainPath); err == nil {
		mainDB, err := db.OpenSQLite(mainPath, nil)
		if err != nil {
			t.Fatalf("open pg-main: %v", err)
		}
		defer mainDB.Close()
		if ok, _ := mainDB.HasTable(ctx, "", "probe_mod"); ok {
			t.Fatalf("probe_mod leaked into pg-main — module override ignored")
		}
	}
}

// TestDatastoreRegistry_LoadErrors_AppSelection proves boot fails loudly on
// invalid App/Module datastores selections: unknown service, unknown
// primitive key, and a named key at module level (fase C).
func TestDatastoreRegistry_LoadErrors_AppSelection(t *testing.T) {
	stateDir := t.TempDir()
	mainDB, err := db.OpenSQLite(filepath.Join(stateDir, "main.db"), nil)
	if err != nil {
		t.Fatalf("open main db: %v", err)
	}
	t.Cleanup(func() { _ = mainDB.Close() })

	cases := []struct {
		name      string
		manifests []manifest.RawManifest
		wantSub   string
	}{
		{
			name: "app targets unknown service",
			manifests: []manifest.RawManifest{
				{
					Kind:     "App",
					Metadata: manifest.RawMetadata{Name: "shop"},
					Spec: map[string]any{
						"root_url":   "/",
						"modules":    []any{"billing"},
						"datastores": map[string]any{"db": "missing"},
					},
				},
			},
			wantSub: "no kind: Datastore manifest",
		},
		{
			name: "app unknown primitive key",
			manifests: []manifest.RawManifest{
				{
					Kind:     "App",
					Metadata: manifest.RawMetadata{Name: "shop"},
					Spec: map[string]any{
						"root_url":   "/",
						"modules":    []any{"billing"},
						"datastores": map[string]any{"graph": "pg-main"},
					},
				},
			},
			wantSub: "unknown primitive type",
		},
		{
			name: "module named key without owning App",
			manifests: []manifest.RawManifest{
				{
					Kind:     "Datastore",
					Metadata: manifest.RawMetadata{Name: "pg-main"},
					Spec:     map[string]any{"serves": []any{"db"}, "driver": "sqlite"},
				},
				{
					Kind:     "Module",
					Metadata: manifest.RawMetadata{Name: "billing"},
					Spec: map[string]any{
						"version":    "1.0.0",
						"datastores": map[string]any{"db/analytics": "pg-main"},
					},
				},
			},
			wantSub: "requires the module to be mounted by a kind: App",
		},
	}
	for _, tc := range cases {
		reg := NewDatastoreRegistry(mainDB, stateDir, nil)
		err := reg.LoadManifests(tc.manifests)
		if err == nil || !strings.Contains(err.Error(), tc.wantSub) {
			t.Fatalf("%s: want error containing %q, got %v", tc.name, tc.wantSub, err)
		}
	}
}

// namedManifests builds the fase C scenario: two sqlite datastores, an App
// (shop) registering the named logical primitive "db/analytics" →
// pg-analytics at App level, and a module (reporting) adding its own named
// "db/rollup" → pg-main.
func namedManifests() []manifest.RawManifest {
	return []manifest.RawManifest{
		{
			Kind:     "Datastore",
			Metadata: manifest.RawMetadata{Name: "pg-main"},
			Spec:     map[string]any{"serves": []any{"db"}, "driver": "sqlite"},
			Source:   "datastores/pg-main.yaml",
		},
		{
			Kind:     "Datastore",
			Metadata: manifest.RawMetadata{Name: "pg-analytics"},
			Spec:     map[string]any{"serves": []any{"db"}, "driver": "sqlite"},
			Source:   "datastores/pg-analytics.yaml",
		},
		{
			Kind:     "App",
			Metadata: manifest.RawMetadata{Name: "shop"},
			Spec: map[string]any{
				"root_url":   "/",
				"modules":    []any{"billing", "reporting"},
				"datastores": map[string]any{"db": "pg-main", "db/analytics": "pg-analytics"},
			},
			Source: "shop.yaml",
		},
		{
			Kind:     "Module",
			Metadata: manifest.RawMetadata{Name: "billing"},
			Spec:     map[string]any{"version": "1.0.0"},
			Source:   "modules/billing/module.yaml",
		},
		{
			Kind:     "Module",
			Metadata: manifest.RawMetadata{Name: "reporting"},
			Spec: map[string]any{
				"version":    "1.0.0",
				"datastores": map[string]any{"db/rollup": "pg-main"},
			},
			Source: "modules/reporting/module.yaml",
		},
	}
}

// TestDatastoreRegistry_ResolveNamed proves named logical primitives (plan
// fase C): ctx.db.named("analytics") resolves to the service registered
// under "db/analytics" in the owning App's registry; a module-level named
// key ("db/rollup") is reachable from its own module; unknown aliases fail
// with DATASTORE_NOT_FOUND; modules outside any App get a clear error.
func TestDatastoreRegistry_ResolveNamed(t *testing.T) {
	reg := newDSRegistryForTest(t, namedManifests())

	ctx := context.Background()

	// App-level named primitive: billing → analytics → pg-analytics.
	conn, err := reg.ResolveNamed("db", "analytics", "billing")
	if err != nil {
		t.Fatalf("resolve named analytics for billing: %v", err)
	}
	dq, ok := conn.(*datastore.DBQuerier)
	if !ok {
		t.Fatalf("resolved connection type %T, want *datastore.DBQuerier", conn)
	}
	if _, err := dq.Query(ctx, "CREATE TABLE IF NOT EXISTS probe_named (id INTEGER)"); err != nil {
		t.Fatalf("create probe table: %v", err)
	}
	analyticsPath := filepath.Join(reg.stateDir, "datastores", "pg-analytics.db")
	if _, err := os.Stat(analyticsPath); err != nil {
		t.Fatalf("pg-analytics sqlite file not created — named resolution went elsewhere: %v", err)
	}

	// Module-level named primitive: reporting → rollup → pg-main.
	conn, err = reg.ResolveNamed("db", "rollup", "reporting")
	if err != nil {
		t.Fatalf("resolve named rollup for reporting: %v", err)
	}
	if _, ok = conn.(*datastore.DBQuerier); !ok {
		t.Fatalf("rollup connection type %T, want *datastore.DBQuerier", conn)
	}

	// Unknown alias → DATASTORE_NOT_FOUND.
	_, err = reg.ResolveNamed("db", "nope", "billing")
	if err == nil || !strings.Contains(err.Error(), "DATASTORE_NOT_FOUND") {
		t.Fatalf("unknown alias: want DATASTORE_NOT_FOUND, got %v", err)
	}

	// Alias is app-scoped: rollup is registered under shop (reporting's
	// App) — billing is in the same App so it CAN see it; a module from a
	// different App cannot. Use an unmounted module name.
	_, err = reg.ResolveNamed("db", "analytics", "ghost-module")
	if err == nil || !strings.Contains(err.Error(), "DATASTORE_NOT_FOUND") {
		t.Fatalf("unmounted module: want DATASTORE_NOT_FOUND, got %v", err)
	}
}

// TestDatastoreRegistry_ModuleNamedKey proves module-level named keys
// ("db/rollup") register into the owning App's named map (fase C) — the
// fase B rejection is gone.
func TestDatastoreRegistry_ModuleNamedKey(t *testing.T) {
	reg := newDSRegistryForTest(t, namedManifests())

	// LoadManifests succeeded (no error from helper) — the named key was
	// accepted. Verify reachability from the owning module.
	if _, err := reg.ResolveNamed("db", "rollup", "reporting"); err != nil {
		t.Fatalf("module-level named key not registered: %v", err)
	}
}

// TestDatastoreRegistry_ConfigLogRoutable proves the 9-primitive closed set
// is fully routable (plan fase D): config and log resolve for the built-in
// 'default' service (KV-backed config, in-memory log) and for named
// memory-driver services.
func TestDatastoreRegistry_ConfigLogRoutable(t *testing.T) {
	reg := newDSRegistryForTest(t, nil)

	ctx := context.Background()

	// Built-in default: config resolves to a KV-backed store.
	conn, err := reg.Resolve("config", "", "beta")
	if err != nil {
		t.Fatalf("resolve config default: %v", err)
	}
	cfg, ok := conn.(interface {
		Get(ctx context.Context, key string) (any, error)
	})
	if !ok {
		t.Fatalf("config connection type %T does not implement Get", conn)
	}
	if v, err := cfg.Get(ctx, "missing"); err != nil || v != nil {
		t.Fatalf("config get missing: want (nil,nil), got (%v,%v)", v, err)
	}

	// Built-in default: log resolves to an in-memory sink.
	conn, err = reg.Resolve("log", "", "beta")
	if err != nil {
		t.Fatalf("resolve log default: %v", err)
	}
	lg, ok := conn.(interface {
		Log(ctx context.Context, level, event string, meta map[string]any) error
	})
	if !ok {
		t.Fatalf("log connection type %T does not implement Log", conn)
	}
	if err := lg.Log(ctx, "info", "probe.event", map[string]any{"k": "v"}); err != nil {
		t.Fatalf("log write: %v", err)
	}

	// Named memory service serving config+log — bound to a module so the
	// named service is reachable through the module binding.
	manifests := []manifest.RawManifest{
		{
			Kind:     "Datastore",
			Metadata: manifest.RawMetadata{Name: "mem-ops"},
			Spec: map[string]any{
				"serves": []any{"config", "log"},
				"driver": "memory",
			},
		},
		{
			Kind:     "Module",
			Metadata: manifest.RawMetadata{Name: "ops"},
			Spec:     map[string]any{"version": "1.0.0", "datastore": "mem-ops"},
		},
	}
	if err := reg.LoadManifests(manifests); err != nil {
		t.Fatalf("LoadManifests mem-ops: %v", err)
	}
	conn, err = reg.Resolve("config", "", "ops")
	if err != nil {
		t.Fatalf("resolve config mem-ops: %v", err)
	}
	if _, ok := conn.(interface {
		Get(ctx context.Context, key string) (any, error)
	}); !ok {
		t.Fatalf("mem-ops config connection type %T does not implement Get", conn)
	}
	conn, err = reg.Resolve("log", "", "ops")
	if err != nil {
		t.Fatalf("resolve log mem-ops: %v", err)
	}
	if _, ok := conn.(interface {
		Log(ctx context.Context, level, event string, meta map[string]any) error
	}); !ok {
		t.Fatalf("mem-ops log connection type %T does not implement Log", conn)
	}
}

// TestDatastoreRegistry_SnapshotBinding proves the Workspace Binding path
// (plan fase B2): services from a Control Plane snapshot populate the
// registry (with their permission ceiling), the built-in 'default' is never
// replaced, and Resolve works against snapshot-provided services.
func TestDatastoreRegistry_SnapshotBinding(t *testing.T) {
	reg := newDSRegistryForTest(t, nil)

	specJSON := `{"serves": ["db"], "driver": "sqlite"}`
	permJSON := `{"default": "read"}`
	bindings := []artifact.DatastoreBinding{
		{Name: "pg-snapshot", Spec: json.RawMessage(specJSON), Permission: json.RawMessage(permJSON)},
		{Name: "default", Spec: json.RawMessage(`{"serves": ["db"], "driver": "memory"}`)},
	}
	if err := reg.LoadSnapshotDatastores(bindings); err != nil {
		t.Fatalf("LoadSnapshotDatastores: %v", err)
	}

	// Bind a module to the snapshot service (module.yaml path — the binding
	// is validated against the now-populated registry).
	if err := reg.LoadManifests([]manifest.RawManifest{
		{
			Kind:     "Module",
			Metadata: manifest.RawMetadata{Name: "beta"},
			Spec:     map[string]any{"version": "1.0.0", "datastore": "pg-snapshot"},
		},
	}); err != nil {
		t.Fatalf("LoadManifests binding: %v", err)
	}

	// Snapshot service resolves via the module binding (wrapped with its
	// read ceiling — assert the capability, not the concrete type).
	conn, err := reg.Resolve("db", "", "beta")
	if err != nil {
		t.Fatalf("resolve snapshot service via binding: %v", err)
	}
	if _, ok := conn.(interface {
		Query(ctx context.Context, sql string, args ...any) ([]map[string]any, error)
	}); !ok {
		t.Fatalf("snapshot connection type %T does not implement Query", conn)
	}

	// Built-in default NOT replaced (still the main DB, not memory driver) —
	// resolve via an unbound module so the chain falls to 'default'.
	defConn, err := reg.Resolve("db", "", "alpha")
	if err != nil {
		t.Fatalf("resolve default: %v", err)
	}
	if _, ok := defConn.(*datastore.DBQuerier); !ok {
		t.Fatalf("default connection type %T — built-in default was replaced", defConn)
	}

	// Permission ceiling travels with the binding.
	perm := reg.Permission("pg-snapshot")
	if perm == nil || perm.Default != spec.AccessRead {
		t.Fatalf("Permission(pg-snapshot) = %v, want default=read", perm)
	}
	// Unknown service → nil ceiling.
	if p := reg.Permission("nope"); p != nil {
		t.Fatalf("Permission(nope) = %v, want nil", p)
	}

	// Invalid spec in snapshot → fail loudly.
	bad := []artifact.DatastoreBinding{
		{Name: "bad-ds", Spec: json.RawMessage(`{"serves": ["db"], "driver": "valkey"}`)},
	}
	if err := reg.LoadSnapshotDatastores(bad); err == nil || !strings.Contains(err.Error(), "cannot serve primitive") {
		t.Fatalf("invalid snapshot spec: want driver×serves error, got %v", err)
	}
}

// TestDatastoreRegistry_PermissionCeiling proves the workspace binding's
// permission ceiling is enforced per-operation (plan fase B2 follow-up): a
// read-only service allows query/get but rejects write operations with
// DATASTORE_PERMISSION_DENIED; a read_write service is unwrapped (direct
// connection).
func TestDatastoreRegistry_PermissionCeiling(t *testing.T) {
	reg := newDSRegistryForTest(t, nil)

	// Read-only sqlite service via snapshot binding.
	specJSON := `{"serves": ["db", "kvstore"], "driver": "sqlite", "access": {"permission": {"default": "read"}}}`
	if err := reg.LoadSnapshotDatastores([]artifact.DatastoreBinding{
		{Name: "ro-db", Spec: json.RawMessage(specJSON)},
	}); err != nil {
		t.Fatalf("LoadSnapshotDatastores: %v", err)
	}
	if err := reg.LoadManifests([]manifest.RawManifest{
		{
			Kind:     "Module",
			Metadata: manifest.RawMetadata{Name: "ro-mod"},
			Spec:     map[string]any{"version": "1.0.0", "datastore": "ro-db"},
		},
	}); err != nil {
		t.Fatalf("LoadManifests binding: %v", err)
	}

	// Read-only KV service (memory driver — real KV backend) via snapshot
	// binding.
	kvSpecJSON := `{"serves": ["cache", "kvstore"], "driver": "memory", "access": {"permission": {"default": "read"}}}`
	if err := reg.LoadSnapshotDatastores([]artifact.DatastoreBinding{
		{Name: "ro-kv", Spec: json.RawMessage(kvSpecJSON)},
	}); err != nil {
		t.Fatalf("LoadSnapshotDatastores ro-kv: %v", err)
	}
	if err := reg.LoadManifests([]manifest.RawManifest{
		{
			Kind:     "Module",
			Metadata: manifest.RawMetadata{Name: "ro-kv-mod"},
			Spec:     map[string]any{"version": "1.0.0", "datastore": "ro-kv"},
		},
	}); err != nil {
		t.Fatalf("LoadManifests ro-kv binding: %v", err)
	}

	ctx := context.Background()

	// Read operation (query) — allowed, delegates to the real backend.
	conn, err := reg.Resolve("db", "", "ro-mod")
	if err != nil {
		t.Fatalf("resolve ro-db: %v", err)
	}
	dq, ok := conn.(interface {
		Query(ctx context.Context, sql string, args ...any) ([]map[string]any, error)
	})
	if !ok {
		t.Fatalf("connection type %T does not implement Query", conn)
	}
	if _, err := dq.Query(ctx, "SELECT 1 AS one"); err != nil {
		t.Fatalf("read op on read-only service: unexpected error %v", err)
	}

	// Write operation (set on kvstore) — denied.
	kvConn, err := reg.Resolve("kvstore", "", "ro-kv-mod")
	if err != nil {
		t.Fatalf("resolve kvstore ro-kv: %v", err)
	}
	setter, ok := kvConn.(interface {
		Set(ctx context.Context, key string, value any, ttl time.Duration) error
	})
	if !ok {
		t.Fatalf("kvstore connection type %T does not implement Set", kvConn)
	}
	err = setter.Set(ctx, "k", "v", time.Minute)
	if err == nil || !strings.Contains(err.Error(), "DATASTORE_PERMISSION_DENIED") {
		t.Fatalf("write op on read-only service: want DATASTORE_PERMISSION_DENIED, got %v", err)
	}

	// Read operation (get on kvstore) — allowed.
	getter, ok := kvConn.(interface {
		Get(ctx context.Context, key string) (any, error)
	})
	if !ok {
		t.Fatalf("kvstore connection type %T does not implement Get", kvConn)
	}
	if _, err := getter.Get(ctx, "k"); err != nil {
		t.Fatalf("read op (get) on read-only service: unexpected error %v", err)
	}

	// read_write service — unwrapped (direct connection, no guard).
	specRW := `{"serves": ["db"], "driver": "sqlite"}`
	if err := reg.LoadSnapshotDatastores([]artifact.DatastoreBinding{
		{Name: "rw-db", Spec: json.RawMessage(specRW)},
	}); err != nil {
		t.Fatalf("LoadSnapshotDatastores rw: %v", err)
	}
	if err := reg.LoadManifests([]manifest.RawManifest{
		{
			Kind:     "Module",
			Metadata: manifest.RawMetadata{Name: "rw-mod"},
			Spec:     map[string]any{"version": "1.0.0", "datastore": "rw-db"},
		},
	}); err != nil {
		t.Fatalf("LoadManifests rw binding: %v", err)
	}
	rwConn, err := reg.Resolve("db", "", "rw-mod")
	if err != nil {
		t.Fatalf("resolve rw-db: %v", err)
	}
	if _, isGuard := rwConn.(*permissionGuard); isGuard {
		t.Fatalf("read_write service was wrapped with a permission guard — unnecessary")
	}
	if _, ok := rwConn.(*datastore.DBQuerier); !ok {
		t.Fatalf("rw connection type %T, want *datastore.DBQuerier", rwConn)
	}
}
