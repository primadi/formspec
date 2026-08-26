package db

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// setupSnapshotEnv creates a master `customer` entity and a transaction
// `order` entity whose customer_id relation snapshots tier + discount_rate
// (02-core-extended.md §1.1, todo 7.10).
func setupSnapshotEnv(t *testing.T) (*EntityStore, *EntityStore) {
	t.Helper()
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "snapshot.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	customerMeta := spec.Metadata{Name: "customer", Module: "billing"}
	customerEntity := &spec.EntitySpec{
		Version:   "v1",
		Lifecycle: "plain_crud",
		Actions:   []spec.Action{{Name: "submit", Disabled: true}},
		Fields: []spec.Field{
			{Name: "name", Type: spec.FieldString},
			{Name: "tier", Type: spec.FieldString},
			{Name: "discount_rate", Type: spec.FieldDecimal},
		},
	}

	orderMeta := spec.Metadata{Name: "order", Module: "billing"}
	orderEntity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "customer_id", Type: spec.FieldRelation, Relation: &spec.RelationDecl{
				Type:     "belongs_to",
				Resource: "customer",
				Snapshot: []spec.SnapshotField{
					{From: "tier", As: "customer_tier_at_transaction"},
					{From: "discount_rate", As: "discount_rate_at_transaction"},
				},
			}},
			{Name: "amount", Type: spec.FieldDecimal},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{
		{Metadata: customerMeta, EntitySpec: *customerEntity},
		{Metadata: orderMeta, EntitySpec: *orderEntity},
	}); err != nil {
		t.Fatalf("ApplyMigrations: %v", err)
	}

	customerStore := NewEntityStore(d, DriverSQLite, customerMeta, customerEntity)
	orderStore := NewEntityStore(d, DriverSQLite, orderMeta, orderEntity)
	return customerStore, orderStore
}

func TestFinancialSnapshot_OnCreate(t *testing.T) {
	customerStore, orderStore := setupSnapshotEnv(t)
	ctx := context.Background()

	custID, err := customerStore.Insert(ctx, InsertParams{
		WorkspaceID: "tenant-1",
		CreatedBy:   "user-1",
		Data:        map[string]any{"name": "PT Maju", "tier": "gold", "discount_rate": 10.5},
	})
	if err != nil {
		t.Fatal(err)
	}

	orderID, err := orderStore.Insert(ctx, InsertParams{
		WorkspaceID: "tenant-1",
		CreatedBy:   "user-1",
		Data:        map[string]any{"customer_id": custID, "amount": 100},
	})
	if err != nil {
		t.Fatal(err)
	}

	rec, err := orderStore.GetByID(ctx, GetByIDParams{WorkspaceID: "tenant-1", ID: orderID})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Data["customer_tier_at_transaction"] != "gold" {
		t.Errorf("snapshot tier: want gold, got %v", rec.Data["customer_tier_at_transaction"])
	}
	if rec.Data["discount_rate_at_transaction"] != 10.5 {
		t.Errorf("snapshot discount_rate: want 10.5, got %v", rec.Data["discount_rate_at_transaction"])
	}
}

func TestFinancialSnapshot_OldTransactionUnaffected(t *testing.T) {
	customerStore, orderStore := setupSnapshotEnv(t)
	ctx := context.Background()

	custID, err := customerStore.Insert(ctx, InsertParams{
		WorkspaceID: "tenant-1",
		CreatedBy:   "user-1",
		Data:        map[string]any{"name": "PT Maju", "tier": "gold", "discount_rate": 10.5},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Order 1 created while customer is gold.
	order1ID, err := orderStore.Insert(ctx, InsertParams{
		WorkspaceID: "tenant-1",
		CreatedBy:   "user-1",
		Data:        map[string]any{"customer_id": custID, "amount": 100},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Customer upgrades to platinum — master value changes.
	if _, err := customerStore.Update(ctx, UpdateParams{
		WorkspaceID: "tenant-1",
		ID:          custID,
		Version:     1,
		UpdatedBy:   "user-1",
		Data:        map[string]any{"tier": "platinum", "discount_rate": 20.0},
	}); err != nil {
		t.Fatal(err)
	}

	// Order 2 created after the upgrade — snapshots the NEW values.
	order2ID, err := orderStore.Insert(ctx, InsertParams{
		WorkspaceID: "tenant-1",
		CreatedBy:   "user-1",
		Data:        map[string]any{"customer_id": custID, "amount": 200},
	})
	if err != nil {
		t.Fatal(err)
	}

	rec1, _ := orderStore.GetByID(ctx, GetByIDParams{WorkspaceID: "tenant-1", ID: order1ID})
	rec2, _ := orderStore.GetByID(ctx, GetByIDParams{WorkspaceID: "tenant-1", ID: order2ID})

	if rec1.Data["customer_tier_at_transaction"] != "gold" {
		t.Errorf("order1 tier: want gold (snapshot at create), got %v", rec1.Data["customer_tier_at_transaction"])
	}
	if rec2.Data["customer_tier_at_transaction"] != "platinum" {
		t.Errorf("order2 tier: want platinum (snapshot at create), got %v", rec2.Data["customer_tier_at_transaction"])
	}
	if rec1.Data["discount_rate_at_transaction"] != 10.5 {
		t.Errorf("order1 discount: want 10.5 (snapshot at create), got %v", rec1.Data["discount_rate_at_transaction"])
	}
	if rec2.Data["discount_rate_at_transaction"] != 20.0 {
		t.Errorf("order2 discount: want 20.0 (snapshot at create), got %v", rec2.Data["discount_rate_at_transaction"])
	}
}