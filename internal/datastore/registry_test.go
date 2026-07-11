package datastore

import (
	"testing"

	"github.com/primadi/forma/pkg/spec"
)

func TestRegistry_RegisterNamed_And_Resolve(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterFactory(spec.DatastoreDriverMemory, &memoryFactory{})

	ds := spec.DatastoreSpec{
		Serves: []spec.PrimitiveType{spec.PrimitiveCache, spec.PrimitiveLock},
		Driver: spec.DatastoreDriverMemory,
		Connection: spec.DatastoreConnection{
			Lazy: true,
		},
	}

	err := registry.RegisterNamed("session-cache", ds)
	if err != nil {
		t.Fatalf("RegisterNamed failed: %v", err)
	}

	// Resolve by name
	pool, err := registry.Resolve(spec.PrimitiveCache, "session-cache")
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if pool == nil {
		t.Fatal("expected non-nil pool")
	}

	// Resolve wrong primitive
	_, err = registry.Resolve(spec.PrimitiveDB, "session-cache")
	if err == nil {
		t.Error("expected error for wrong primitive type")
	}

	// Resolve unknown name
	_, err = registry.Resolve(spec.PrimitiveCache, "nonexistent")
	if err == nil {
		t.Error("expected error for unknown name")
	}
}

func TestRegistry_ResolveDefault(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterFactory(spec.DatastoreDriverMemory, &memoryFactory{})

	ds := spec.DatastoreSpec{
		Serves: []spec.PrimitiveType{spec.PrimitiveDB, spec.PrimitiveKVStore},
		Driver: spec.DatastoreDriverMemory,
		Connection: spec.DatastoreConnection{
			Lazy: true,
		},
	}

	err := registry.RegisterNamed("default", ds)
	if err != nil {
		t.Fatalf("RegisterNamed failed: %v", err)
	}

	pool, err := registry.ResolveDefault(spec.PrimitiveDB)
	if err != nil {
		t.Fatalf("ResolveDefault failed: %v", err)
	}
	if pool == nil {
		t.Fatal("expected non-nil pool")
	}
}

func TestRegistry_GetPermission(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterFactory(spec.DatastoreDriverMemory, &memoryFactory{})

	// Datastore with explicit permission
	ds := spec.DatastoreSpec{
		Serves: []spec.PrimitiveType{spec.PrimitiveDB},
		Driver: spec.DatastoreDriverMemory,
		Access: &spec.DatastoreAccess{
			Permission: &spec.DatastorePermission{
				Default: spec.AccessRead,
			},
		},
	}

	err := registry.RegisterNamed("readonly-db", ds)
	if err != nil {
		t.Fatalf("RegisterNamed failed: %v", err)
	}

	perm := registry.GetPermission("readonly-db")
	if perm == nil {
		t.Fatal("expected non-nil permission")
	}
	if perm.Default != spec.AccessRead {
		t.Errorf("expected read, got %q", perm.Default)
	}

	// Datastore without explicit permission → default
	ds2 := spec.DatastoreSpec{
		Serves: []spec.PrimitiveType{spec.PrimitiveCache},
		Driver: spec.DatastoreDriverMemory,
	}

	err = registry.RegisterNamed("plain-cache", ds2)
	if err != nil {
		t.Fatalf("RegisterNamed failed: %v", err)
	}

	perm2 := registry.GetPermission("plain-cache")
	if perm2 == nil {
		t.Fatal("expected default permission")
	}
	if perm2.Default != spec.AccessReadWrite {
		t.Errorf("expected read_write default, got %q", perm2.Default)
	}
}

func TestRegistry_List_Remove_Shutdown(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterFactory(spec.DatastoreDriverMemory, &memoryFactory{})

	ds := spec.DatastoreSpec{
		Serves:     []spec.PrimitiveType{spec.PrimitiveCache},
		Driver:     spec.DatastoreDriverMemory,
		Connection: spec.DatastoreConnection{Lazy: true},
	}

	_ = registry.RegisterNamed("cache-1", ds)
	_ = registry.RegisterNamed("cache-2", ds)

	list := registry.List()
	if len(list) != 2 {
		t.Errorf("expected 2, got %d", len(list))
	}

	err := registry.Remove("cache-1")
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	list = registry.List()
	if len(list) != 1 {
		t.Errorf("expected 1 after remove, got %d", len(list))
	}

	err = registry.Shutdown()
	if err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	list = registry.List()
	if len(list) != 0 {
		t.Errorf("expected 0 after shutdown, got %d", len(list))
	}
}
