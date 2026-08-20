package db

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// TestEntityStore_ExtensionReadWrite verifies the full extension read/write
// path (4.3.1/4.3.2): extension data is written to its ext_{namespace}
// column (isolated from the base data JSONB), merged back into reads under
// the namespace key, and updated in place on Update.
func TestEntityStore_ExtensionReadWrite(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "ext_rw.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	ctx := context.Background()

	// Base entity.
	baseMeta := spec.Metadata{Name: "customer", Module: "billing"}
	baseEntity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "name", Type: spec.FieldString},
			{Name: "email", Type: spec.FieldString, Unique: true},
		},
	}

	// Extension entity targeting billing/customer with namespace "custext".
	extMeta := spec.Metadata{Name: "custext", Module: "billing"}
	extEntity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "loyalty_tier", Type: spec.FieldString, Default: "bronze"},
			{Name: "referral_code", Type: spec.FieldString},
		},
		ExtendStorage: &spec.ExtendStorage{
			Target:    "billing/customer",
			Namespace: "custext",
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{
		{Metadata: baseMeta, EntitySpec: *baseEntity},
		{Metadata: extMeta, EntitySpec: *extEntity},
	}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, baseMeta, baseEntity)
	store.SetExtensions(map[string]string{"custext": "ext_custext"})

	// Insert with extension data under the namespace key.
	id, err := store.Insert(ctx, InsertParams{
		WorkspaceID: "tenant-1",
		CreatedBy:   "user-1",
		Data: map[string]any{
			"name":  "John Doe",
			"email": "john@example.com",
			"custext": map[string]any{
				"loyalty_tier":  "gold",
				"referral_code": "REF-001",
			},
		},
	})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Verify the base data column does NOT contain the extension payload.
	var baseData string
	err = d.QueryRowContext(ctx,
		"SELECT data FROM billing_customers WHERE id = ?", id).Scan(&baseData)
	if err != nil {
		t.Fatalf("query base data failed: %v", err)
	}
	var baseMap map[string]any
	if err := json.Unmarshal([]byte(baseData), &baseMap); err != nil {
		t.Fatalf("unmarshal base data: %v", err)
	}
	if _, ok := baseMap["custext"]; ok {
		t.Error("extension payload leaked into base data JSONB")
	}
	if baseMap["name"] != "John Doe" {
		t.Errorf("expected base field name preserved, got %v", baseMap["name"])
	}

	// Verify the ext_custext column holds the payload.
	var extRaw string
	err = d.QueryRowContext(ctx,
		"SELECT ext_custext FROM billing_customers WHERE id = ?", id).Scan(&extRaw)
	if err != nil {
		t.Fatalf("query ext column failed: %v", err)
	}
	var extMap map[string]any
	if err := json.Unmarshal([]byte(extRaw), &extMap); err != nil {
		t.Fatalf("unmarshal ext data: %v", err)
	}
	if extMap["loyalty_tier"] != "gold" {
		t.Errorf("expected loyalty_tier gold in ext column, got %v", extMap["loyalty_tier"])
	}

	// GetByID merges the extension back under the namespace key.
	rec, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: "tenant-1", ID: id})
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	ext, ok := rec.Data["custext"].(map[string]any)
	if !ok {
		t.Fatalf("expected merged extension data under 'custext', got %#v", rec.Data["custext"])
	}
	if ext["loyalty_tier"] != "gold" {
		t.Errorf("expected loyalty_tier gold on read, got %v", ext["loyalty_tier"])
	}

	// Update with new extension data updates the ext column, not base data.
	newVersion, err := store.Update(ctx, UpdateParams{
		WorkspaceID: "tenant-1",
		ID:          id,
		Version:     rec.Version,
		UpdatedBy:   "user-1",
		Data: map[string]any{
			"name": "John Doe Updated",
			"custext": map[string]any{
				"loyalty_tier":  "platinum",
				"referral_code": "REF-002",
			},
		},
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if newVersion != rec.Version+1 {
		t.Errorf("expected version %d, got %d", rec.Version+1, newVersion)
	}

	// Re-read: extension merged, base field updated.
	rec2, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: "tenant-1", ID: id})
	if err != nil {
		t.Fatalf("GetByID after update failed: %v", err)
	}
	if rec2.Data["name"] != "John Doe Updated" {
		t.Errorf("expected updated base name, got %v", rec2.Data["name"])
	}
	ext2, ok := rec2.Data["custext"].(map[string]any)
	if !ok {
		t.Fatalf("expected merged extension after update, got %#v", rec2.Data["custext"])
	}
	if ext2["loyalty_tier"] != "platinum" {
		t.Errorf("expected loyalty_tier platinum after update, got %v", ext2["loyalty_tier"])
	}
}
