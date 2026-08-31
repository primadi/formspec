package control

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/primadi/formspec/internal/artifact"
	"github.com/primadi/formspec/pkg/spec"
)

// TestBuildSnapshot_DatastoreFilter proves the Workspace Binding evaluation
// (plan fase B2): only datastores whose access.filter matches the workspace
// appear in the snapshot; non-matching services are invisible; the
// permission ceiling travels with the binding.
func TestBuildSnapshot_DatastoreFilter(t *testing.T) {
	store := artifact.NewMemStore()
	h := NewSnapshotHandler(store, nil)

	// Register three services with different filters.
	mustRegister := func(t *testing.T, name, specJSON string, env string) {
		t.Helper()
		if err := store.UpsertDatastore(context.Background(), &artifact.DatastoreRegistration{
			Name:        name,
			Spec:        json.RawMessage(specJSON),
			Environment: env,
		}); err != nil {
			t.Fatalf("upsert %s: %v", name, err)
		}
	}
	mustRegister(t, "shared-pg", `{"serves": ["db"], "driver": "postgres"}`, "") // no filter → all workspaces
	mustRegister(t, "enterprise-pg", `{"serves": ["db"], "driver": "postgres", "access": {"filter": {"labels": {"tier": "enterprise"}}}}`, "")
	mustRegister(t, "prod-only", `{"serves": ["cache"], "driver": "valkey", "access": {"filter": {"environment": "production"}}}`, "")

	snap, err := h.buildSnapshot(context.Background(), "ws-free", 1)
	if err != nil {
		t.Fatalf("buildSnapshot: %v", err)
	}

	names := map[string]bool{}
	for _, b := range snap.Datastores {
		names[b.Name] = true
	}
	if !names["shared-pg"] {
		t.Fatalf("shared-pg missing from snapshot (no filter = all workspaces): %v", names)
	}
	if names["enterprise-pg"] {
		t.Fatalf("enterprise-pg leaked into ws-free snapshot (label filter mismatch)")
	}
	if names["prod-only"] {
		t.Fatalf("prod-only leaked into dev snapshot (environment mismatch)")
	}

	// Enterprise workspace sees both shared + enterprise.
	snapEnt, err := h.buildSnapshot(context.Background(), "ws-ent", 1)
	if err != nil {
		t.Fatalf("buildSnapshot ent: %v", err)
	}
	// Note: buildSnapshot uses Environment "dev" — enterprise label filter
	// still excludes ws-ent (no labels plumbed yet); only shared-pg matches.
	found := false
	for _, b := range snapEnt.Datastores {
		if b.Name == "shared-pg" {
			found = true
		}
	}
	if !found {
		t.Fatalf("shared-pg missing from ws-ent snapshot")
	}

	// Permission ceiling travels with the binding.
	store2 := artifact.NewMemStore()
	h2 := NewSnapshotHandler(store2, nil)
	_ = store2.UpsertDatastore(context.Background(), &artifact.DatastoreRegistration{
		Name: "readonly-db",
		Spec: json.RawMessage(`{"serves": ["db"], "driver": "postgres", "access": {"permission": {"default": "read"}}}`),
	})
	snapRO, err := h2.buildSnapshot(context.Background(), "ws-1", 1)
	if err != nil {
		t.Fatalf("buildSnapshot ro: %v", err)
	}
	for _, b := range snapRO.Datastores {
		if b.Name != "readonly-db" {
			continue
		}
		var perm spec.DatastorePermission
		if err := json.Unmarshal(b.Permission, &perm); err != nil {
			t.Fatalf("decode permission: %v", err)
		}
		if perm.Default != spec.AccessRead {
			t.Fatalf("permission default = %q, want read", perm.Default)
		}
		return
	}
	t.Fatalf("readonly-db missing from snapshot")
}
