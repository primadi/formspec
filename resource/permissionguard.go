package formspec

import (
	"context"
	"fmt"
	"time"

	"github.com/primadi/formspec/pkg/spec"
)

// ─── Permission guard (plan fase B2 follow-up) ───
//
// permissionGuard wraps a resolved ctx.* primitive connection with the
// workspace binding's permission ceiling (platform/06-datastore.md §4/§6).
// Operations exceeding the ceiling fail with DATASTORE_PERMISSION_DENIED;
// allowed operations delegate to the underlying connection. Go interfaces
// are structural, so the guard transparently satisfies every capability
// interface (Querier, KVGetter, KVSetter, KVDeleter, Locker, Queue, PubSub,
// Storage, Config, Logger) the wrapped connection implements.
//
// Operation classification:
//   - read:  query, get (cache/kvstore/config), download, subscribe
//   - write: set, delete, acquire, release, enqueue, dequeue, publish,
//            upload, log
//
// A `read` ceiling permits read operations only; `write` implies read;
// `read_write`/nil permits everything.

// permissionGuard wraps one resolved connection with its ceiling.
type permissionGuard struct {
	conn     interface{}
	perm     *spec.DatastorePermission
	primType string // e.g. "db" — for error messages
	service  string // logical service name — for error messages
}

// denied builds the DATASTORE_PERMISSION_DENIED error for one operation.
func (g *permissionGuard) denied(op string) error {
	return fmt.Errorf("ctx.%s: DATASTORE_PERMISSION_DENIED — %q on service %q exceeds the workspace permission ceiling (platform/06-datastore.md §6)", g.primType, op, g.service)
}

// check returns nil when the operation kind is permitted.
func (g *permissionGuard) check(op string) error {
	if !g.perm.Allows(op) {
		return g.denied(op)
	}
	return nil
}

// Query serves ctx.db().query — read.
func (g *permissionGuard) Query(ctx context.Context, sql string, args ...any) ([]map[string]any, error) {
	if err := g.check("read"); err != nil {
		return nil, err
	}
	q, ok := g.conn.(interface {
		Query(ctx context.Context, sql string, args ...any) ([]map[string]any, error)
	})
	if !ok {
		return nil, fmt.Errorf("ctx.%s: backend does not implement query", g.primType)
	}
	return q.Query(ctx, sql, args...)
}

// Get serves ctx.cache/kvstore/config .get — read.
func (g *permissionGuard) Get(ctx context.Context, key string) (any, error) {
	if err := g.check("read"); err != nil {
		return nil, err
	}
	gt, ok := g.conn.(interface {
		Get(ctx context.Context, key string) (any, error)
	})
	if !ok {
		return nil, fmt.Errorf("ctx.%s: backend does not implement get", g.primType)
	}
	return gt.Get(ctx, key)
}

// Set serves ctx.cache/kvstore .set — write.
func (g *permissionGuard) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	if err := g.check("write"); err != nil {
		return err
	}
	st, ok := g.conn.(interface {
		Set(ctx context.Context, key string, value any, ttl time.Duration) error
	})
	if !ok {
		return fmt.Errorf("ctx.%s: backend does not implement set", g.primType)
	}
	return st.Set(ctx, key, value, ttl)
}

// Delete serves ctx.cache/kvstore .delete — write.
func (g *permissionGuard) Delete(ctx context.Context, key string) error {
	if err := g.check("write"); err != nil {
		return err
	}
	del, ok := g.conn.(interface {
		Delete(ctx context.Context, key string) error
	})
	if !ok {
		return fmt.Errorf("ctx.%s: backend does not implement delete", g.primType)
	}
	return del.Delete(ctx, key)
}

// Acquire serves ctx.lock().acquire — write.
func (g *permissionGuard) Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if err := g.check("write"); err != nil {
		return false, err
	}
	l, ok := g.conn.(interface {
		Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error)
	})
	if !ok {
		return false, fmt.Errorf("ctx.%s: backend does not implement acquire", g.primType)
	}
	return l.Acquire(ctx, key, ttl)
}

// Release serves ctx.lock().release — write.
func (g *permissionGuard) Release(ctx context.Context, key string) error {
	if err := g.check("write"); err != nil {
		return err
	}
	l, ok := g.conn.(interface {
		Release(ctx context.Context, key string) error
	})
	if !ok {
		return fmt.Errorf("ctx.%s: backend does not implement release", g.primType)
	}
	return l.Release(ctx, key)
}

// Enqueue serves ctx.queue().enqueue — write.
func (g *permissionGuard) Enqueue(ctx context.Context, name string, payload any) error {
	if err := g.check("write"); err != nil {
		return err
	}
	q, ok := g.conn.(interface {
		Enqueue(ctx context.Context, name string, payload any) error
	})
	if !ok {
		return fmt.Errorf("ctx.%s: backend does not implement enqueue", g.primType)
	}
	return q.Enqueue(ctx, name, payload)
}

// Dequeue serves ctx.queue().dequeue — write (removes from the queue).
func (g *permissionGuard) Dequeue(ctx context.Context, name string) (any, error) {
	if err := g.check("write"); err != nil {
		return nil, err
	}
	q, ok := g.conn.(interface {
		Dequeue(ctx context.Context, name string) (any, error)
	})
	if !ok {
		return nil, fmt.Errorf("ctx.%s: backend does not implement dequeue", g.primType)
	}
	return q.Dequeue(ctx, name)
}

// Publish serves ctx.pubsub().publish — write.
func (g *permissionGuard) Publish(ctx context.Context, channel string, payload any) error {
	if err := g.check("write"); err != nil {
		return err
	}
	p, ok := g.conn.(interface {
		Publish(ctx context.Context, channel string, payload any) error
	})
	if !ok {
		return fmt.Errorf("ctx.%s: backend does not implement publish", g.primType)
	}
	return p.Publish(ctx, channel, payload)
}

// Subscribe serves ctx.pubsub().subscribe — read.
func (g *permissionGuard) Subscribe(ctx context.Context, channel string, cb func(payload any)) error {
	if err := g.check("read"); err != nil {
		return err
	}
	p, ok := g.conn.(interface {
		Subscribe(ctx context.Context, channel string, cb func(payload any)) error
	})
	if !ok {
		return fmt.Errorf("ctx.%s: backend does not implement subscribe", g.primType)
	}
	return p.Subscribe(ctx, channel, cb)
}

// Upload serves ctx.storage().upload — write.
func (g *permissionGuard) Upload(ctx context.Context, path string, data []byte) error {
	if err := g.check("write"); err != nil {
		return err
	}
	s, ok := g.conn.(interface {
		Upload(ctx context.Context, path string, data []byte) error
	})
	if !ok {
		return fmt.Errorf("ctx.%s: backend does not implement upload", g.primType)
	}
	return s.Upload(ctx, path, data)
}

// Download serves ctx.storage().download — read.
func (g *permissionGuard) Download(ctx context.Context, path string) ([]byte, error) {
	if err := g.check("read"); err != nil {
		return nil, err
	}
	s, ok := g.conn.(interface {
		Download(ctx context.Context, path string) ([]byte, error)
	})
	if !ok {
		return nil, fmt.Errorf("ctx.%s: backend does not implement download", g.primType)
	}
	return s.Download(ctx, path)
}

// Log serves the ctx.log primitive — write.
func (g *permissionGuard) Log(ctx context.Context, level, event string, meta map[string]any) error {
	if err := g.check("write"); err != nil {
		return err
	}
	l, ok := g.conn.(interface {
		Log(ctx context.Context, level, event string, meta map[string]any) error
	})
	if !ok {
		return fmt.Errorf("ctx.%s: backend does not implement log", g.primType)
	}
	return l.Log(ctx, level, event, meta)
}
