// Package datastore provides the named datastore registry and connection management.
//
// The Registry maps named datastores (received from the Control Plane via snapshot)
// to connection pools. It is the single point of resolution for all ctx.* primitives.
package datastore

import (
	"fmt"
	"sync"

	"github.com/primadi/forma/pkg/spec"
)

// Registry manages named datastore connections scoped to a single workspace.
// It is populated from the Control Plane snapshot and resolves ctx.* calls.
type Registry struct {
	mu         sync.RWMutex
	datastores map[string]*Entry // name → entry
	factories  map[spec.DatastoreDriver]ConnectionFactory
}

// Entry represents a registered datastore with its spec and active pool.
type Entry struct {
	Spec spec.DatastoreSpec
	Pool *ConnectionPool
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		datastores: make(map[string]*Entry),
		factories:  make(map[spec.DatastoreDriver]ConnectionFactory),
	}
}

// RegisterFactory registers a connection factory for a driver.
// Must be called before Register() for that driver.
func (r *Registry) RegisterFactory(driver spec.DatastoreDriver, factory ConnectionFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[driver] = factory
}

// RegisterNamed registers a datastore with an explicit name (from metadata).
func (r *Registry) RegisterNamed(name string, ds spec.DatastoreSpec) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	factory, ok := r.factories[ds.Driver]
	if !ok {
		return fmt.Errorf("datastore registry: no factory registered for driver %q", ds.Driver)
	}

	entry := &Entry{Spec: ds}

	if !ds.Connection.Lazy {
		pool, err := factory.Open(ds)
		if err != nil {
			return fmt.Errorf("datastore registry: failed to open %q: %w", name, err)
		}
		entry.Pool = pool
	}

	r.datastores[name] = entry
	return nil
}

// Resolve returns the connection pool for a named datastore serving the given primitive type.
// Returns an error if the datastore is not found or does not serve the requested primitive.
func (r *Registry) Resolve(primitiveType spec.PrimitiveType, name string) (*ConnectionPool, error) {
	// Fast path: read lock first
	r.mu.RLock()
	entry, ok := r.datastores[name]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("datastore %q not found in registry", name)
	}

	// Verify this datastore serves the requested primitive type
	serves := false
	for _, p := range entry.Spec.Serves {
		if p == primitiveType {
			serves = true
			break
		}
	}
	if !serves {
		return nil, fmt.Errorf("datastore %q does not serve primitive %q", name, primitiveType)
	}

	// If pool is already initialized, return it
	if entry.Pool != nil {
		return entry.Pool, nil
	}

	// Lazy initialization — acquire write lock
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	entry, ok = r.datastores[name]
	if !ok {
		return nil, fmt.Errorf("datastore %q removed during resolution", name)
	}

	// Double-check pool (another goroutine may have initialized it)
	if entry.Pool != nil {
		return entry.Pool, nil
	}

	factory, ok := r.factories[entry.Spec.Driver]
	if !ok {
		return nil, fmt.Errorf("datastore registry: no factory for driver %q", entry.Spec.Driver)
	}
	pool, err := factory.Open(entry.Spec)
	if err != nil {
		return nil, fmt.Errorf("datastore registry: lazy open %q: %w", name, err)
	}
	entry.Pool = pool
	return entry.Pool, nil
}

// ResolveDefault finds the datastore named "default" that serves the given primitive type.
func (r *Registry) ResolveDefault(primitiveType spec.PrimitiveType) (*ConnectionPool, error) {
	return r.Resolve(primitiveType, "default")
}

// GetPermission returns the permission ceiling for a named datastore.
func (r *Registry) GetPermission(name string) *spec.DatastorePermission {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, ok := r.datastores[name]
	if !ok {
		return nil
	}

	if entry.Spec.Access == nil || entry.Spec.Access.Permission == nil {
		return spec.DefaultDatastorePermission()
	}
	return entry.Spec.Access.Permission
}

// List returns all registered datastore names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.datastores))
	for name := range r.datastores {
		names = append(names, name)
	}
	return names
}

// Remove removes a datastore from the registry and closes its pool.
func (r *Registry) Remove(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.datastores[name]
	if !ok {
		return fmt.Errorf("datastore %q not found", name)
	}

	if entry.Pool != nil {
		if err := entry.Pool.Close(); err != nil {
			return fmt.Errorf("datastore %q close: %w", name, err)
		}
	}

	delete(r.datastores, name)
	return nil
}

// Shutdown closes all connection pools and clears the registry.
func (r *Registry) Shutdown() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []error
	for name, entry := range r.datastores {
		if entry.Pool != nil {
			if err := entry.Pool.Close(); err != nil {
				errs = append(errs, fmt.Errorf("datastore %q: %w", name, err))
			}
		}
	}
	r.datastores = make(map[string]*Entry)

	if len(errs) > 0 {
		return fmt.Errorf("datastore shutdown errors: %v", errs)
	}
	return nil
}
