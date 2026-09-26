package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/manifest"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// seedIntoWorkspace seeds the spec tree into a named workspace, so a test can
// reproduce "records exist, but not in the tenant the command reads".
func seedIntoWorkspace(t *testing.T, dir, workspace string) (*entity.Registry, db.DB) {
	t.Helper()
	database, err := db.Open("sqlite::memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	reg := entity.NewRegistry(database, db.DriverSQLite, dir)
	for _, loadErr := range reg.LoadEntities() {
		t.Fatalf("load entity: %v", loadErr)
	}
	if _, err := reg.SyncSchema(context.Background()); err != nil {
		t.Fatalf("sync schema: %v", err)
	}
	loader := manifest.NewLoader(dir)
	res, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("load manifests: %v", err)
	}
	if _, _, failed := seedAll(context.Background(), res, reg, "", workspace); failed != 0 {
		t.Fatalf("seed failed into %s", workspace)
	}
	return reg, database
}

// TestBackupReadsNamedWorkspace pins todo 4.8.7, which was measured as a silent
// failure: `formspec backup create --full --spec examples/kafe/spec --dsn
// sqlite:.formspec/kafe.db` reported "27 table(s), 0 record(s)" while
// cafe_master_branches held 2 rows under tenant_id "kafe". The command read the
// hardcoded tenant "demo", so a real app's backup was empty and looked fine.
//
// The fix applies the same #48 rule `formspec dev` uses. With exactly one
// declared workspace, reading anything else is never what the operator meant.
func TestBackupReadsNamedWorkspace(t *testing.T) {
	dir := t.TempDir()
	writeBackupSpec(t, dir)
	// Declare exactly one workspace so the rule under test has something to
	// adopt, and seed into it (NOT "demo").
	wsPath := filepath.Join(dir, "workspaces", "staging.yaml")
	if err := os.MkdirAll(filepath.Dir(wsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wsPath, []byte(`apiVersion: formspec.dev/v1
kind: Workspace
metadata: { name: staging }
spec: {}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	reg, database := seedIntoWorkspace(t, dir, "staging")
	defer func() { _ = database.Close() }()

	store, err := reg.GetEntityStore("alpha", "customer")
	if err != nil {
		t.Fatal(err)
	}

	// Sanity: the records really are in `staging`, and `demo` really is empty —
	// otherwise the test could pass for the wrong reason.
	staging, err := listAll(context.Background(), store, "staging")
	if err != nil {
		t.Fatal(err)
	}
	if len(staging) != 2 {
		t.Fatalf("staging has %d records, want 2", len(staging))
	}
	demo, err := listAll(context.Background(), store, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(demo) != 0 {
		t.Fatalf("demo has %d records, want 0 — the test is not reproducing the bug", len(demo))
	}

	// The rule under test: no explicit flag, one declared workspace → adopt it.
	if got := activeWorkspaceFor(dir, "demo", false); got != "staging" {
		t.Errorf("activeWorkspaceFor(no flag, 1 declared) = %q, want staging", got)
	}

	// An explicit choice wins even when it is not declared (with a warning) —
	// asking for `demo` must still read `demo`, so the old behaviour stays
	// reachable and its emptiness is the operator's own doing.
	if got := activeWorkspaceFor(dir, "demo", true); got != "demo" {
		t.Errorf("activeWorkspaceFor(explicit demo) = %q, want demo", got)
	}
}
