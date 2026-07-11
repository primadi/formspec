package datastore

import (
	"github.com/primadi/forma/pkg/spec"
)

// Resolver provides convenience methods for resolving datastores by name or default.
type Resolver struct {
	registry *Registry
}

// NewResolver creates a Resolver backed by the given registry.
func NewResolver(registry *Registry) *Resolver {
	return &Resolver{registry: registry}
}

// DB returns the default db connection pool.
func (r *Resolver) DB() (*ConnectionPool, error) {
	return r.registry.ResolveDefault(spec.PrimitiveDB)
}

// DBNamed returns a named db connection pool.
func (r *Resolver) DBNamed(name string) (*ConnectionPool, error) {
	return r.registry.Resolve(spec.PrimitiveDB, name)
}

// Cache returns the default cache connection pool.
func (r *Resolver) Cache() (*ConnectionPool, error) {
	return r.registry.ResolveDefault(spec.PrimitiveCache)
}

// CacheNamed returns a named cache connection pool.
func (r *Resolver) CacheNamed(name string) (*ConnectionPool, error) {
	return r.registry.Resolve(spec.PrimitiveCache, name)
}

// Lock returns the default lock connection pool.
func (r *Resolver) Lock() (*ConnectionPool, error) {
	return r.registry.ResolveDefault(spec.PrimitiveLock)
}

// LockNamed returns a named lock connection pool.
func (r *Resolver) LockNamed(name string) (*ConnectionPool, error) {
	return r.registry.Resolve(spec.PrimitiveLock, name)
}

// Queue returns the default queue connection pool.
func (r *Resolver) Queue() (*ConnectionPool, error) {
	return r.registry.ResolveDefault(spec.PrimitiveQueue)
}

// QueueNamed returns a named queue connection pool.
func (r *Resolver) QueueNamed(name string) (*ConnectionPool, error) {
	return r.registry.Resolve(spec.PrimitiveQueue, name)
}

// PubSub returns the default pubsub connection pool.
func (r *Resolver) PubSub() (*ConnectionPool, error) {
	return r.registry.ResolveDefault(spec.PrimitivePubSub)
}

// PubSubNamed returns a named pubsub connection pool.
func (r *Resolver) PubSubNamed(name string) (*ConnectionPool, error) {
	return r.registry.Resolve(spec.PrimitivePubSub, name)
}

// Storage returns the default storage connection pool.
func (r *Resolver) Storage() (*ConnectionPool, error) {
	return r.registry.ResolveDefault(spec.PrimitiveStorage)
}

// StorageNamed returns a named storage connection pool.
func (r *Resolver) StorageNamed(name string) (*ConnectionPool, error) {
	return r.registry.Resolve(spec.PrimitiveStorage, name)
}

// KVStore returns the default kvstore connection pool.
func (r *Resolver) KVStore() (*ConnectionPool, error) {
	return r.registry.ResolveDefault(spec.PrimitiveKVStore)
}

// KVStoreNamed returns a named kvstore connection pool.
func (r *Resolver) KVStoreNamed(name string) (*ConnectionPool, error) {
	return r.registry.Resolve(spec.PrimitiveKVStore, name)
}
