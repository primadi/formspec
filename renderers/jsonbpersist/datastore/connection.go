package datastore

import (
	"sync"
	"time"
)

// ConnectionConfig holds configuration for a connection pool.
type ConnectionConfig struct {
	Driver  string
	DSN     string
	MaxOpen int
	MaxIdle int
}

// ConnectionPool manages a pool of connections to a datastore backend.
// It provides health checking and lifecycle management.
// The actual connection implementation is driver-specific and
// integrated with the existing renderers/jsonbpersist, cache, lock, etc. packages.
type ConnectionPool struct {
	mu        sync.RWMutex
	config    ConnectionConfig
	healthy   bool
	lastCheck time.Time
	closeCh   chan struct{}
	closeOnce sync.Once
}

// NewConnectionPool creates a new connection pool with the given config.
func NewConnectionPool(config ConnectionConfig) *ConnectionPool {
	return &ConnectionPool{
		config:  config,
		healthy: true, // assume healthy until first health check
		closeCh: make(chan struct{}),
	}
}

// Config returns the pool's configuration.
func (p *ConnectionPool) Config() ConnectionConfig {
	return p.config
}

// DriverName returns the driver identifier.
func (p *ConnectionPool) DriverName() string {
	return p.config.Driver
}

// DSN returns the connection string (may be sanitized).
func (p *ConnectionPool) DSN() string {
	return p.config.DSN
}

// IsHealthy returns whether the pool is currently healthy.
func (p *ConnectionPool) IsHealthy() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.healthy
}

// SetHealth updates the health status.
func (p *ConnectionPool) SetHealth(healthy bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.healthy = healthy
	p.lastCheck = time.Now()
}

// LastHealthCheck returns when the last health check occurred.
func (p *ConnectionPool) LastHealthCheck() time.Time {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastCheck
}

// Close shuts down the connection pool.
func (p *ConnectionPool) Close() error {
	var err error
	p.closeOnce.Do(func() {
		close(p.closeCh)
		p.mu.Lock()
		p.healthy = false
		p.mu.Unlock()
	})
	return err
}

// Closed returns a channel that is closed when the pool is shut down.
func (p *ConnectionPool) Closed() <-chan struct{} {
	return p.closeCh
}
