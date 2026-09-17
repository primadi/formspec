package db

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// Gap #12 at runtime. The referenceability guard used to answer "target not found
// or table doesn't exist" with `continue` — so a relation pointing nowhere, or one
// whose target table was mis-resolved by the naive `{module}_{plural}` fallback,
// passed silently. Both cases are now named errors.
func TestValidateRelationTargets_RefusesDanglingAndUnresolvable(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "rel.db"), nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = d.Close() }()

	meta := spec.Metadata{Name: "payment", Module: "cafe-order"}
	entity := &spec.EntitySpec{
		Version:        "v1",
		Plural:         "payments",
		Characteristic: spec.CharTransaction,
		Fields: []spec.Field{
			{Name: "order_id", Type: spec.FieldRelation,
				Relation: &spec.RelationDecl{Type: "belongs_to", Resource: "cafe-order.order"}},
			{Name: "branch_id", Type: spec.FieldRelation,
				Relation: &spec.RelationDecl{Type: "belongs_to", Resource: "cafe-master.branch"}},
		},
	}
	runner := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := runner.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// A resolver that mirrors the real one: an unknown target entity is an error,
	// and a known one reports the table the runtime would actually query.
	errUnknownTarget := errors.New("entity not registered")
	store := NewEntityStore(d, DriverSQLite, meta, entity)
	store.SetTargetTableResolver(func(module, name string) (string, error) {
		if module == "cafe-master" && name == "branch" {
			return "", errUnknownTarget
		}
		if module == "cafe-order" && name == "order" {
			return "cafe_order_orders", nil // table exists but has no such row
		}
		return "", errUnknownTarget
	})

	// 1. A relation whose target entity does not resolve must be an error, not a
	//    silent skip: this is the shape that let order→branch dangle.
	err = store.ValidateRelationTargets(ctx, d, "kafe", map[string]any{
		"branch_id": "branch-that-does-not-resolve",
	})
	if err == nil {
		t.Fatal("expected an error for a relation whose target entity cannot be resolved")
	}
	if !strings.Contains(err.Error(), "does not resolve to a registered entity") {
		t.Errorf("error should say the target is unresolvable, got: %v", err)
	}

	// 2. A resolvable target with no matching row is a dangling reference.
	if _, err := d.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS cafe_order_orders (id text, tenant_id text, doc_status text, deleted_at text)"); err != nil {
		t.Fatalf("create target table: %v", err)
	}
	err = store.ValidateRelationTargets(ctx, d, "kafe", map[string]any{
		"order_id": "ord-missing",
	})
	if err == nil {
		t.Fatal("expected an error for a relation pointing at a row that does not exist")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error should say the target row is missing, got: %v", err)
	}

	// 3. A relation the caller simply did not set stays optional.
	if err := store.ValidateRelationTargets(ctx, d, "kafe", map[string]any{}); err != nil {
		t.Fatalf("an unset optional relation must pass: %v", err)
	}
}
