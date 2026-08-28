package formspec

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/primadi/formspec/internal/manifest"
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
	if err == nil || !strings.Contains(err.Error(), "not supported in single-server mode") {
		t.Fatalf("valkey driver: want unsupported error, got %v", err)
	}
}
