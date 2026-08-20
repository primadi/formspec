// Package memory provides in-memory implementations of the ctx.* primitive
// backends used for dev-mode auto-provisioning of the 'default' datastore
// (docs/spec/platform/06-datastore.md §5). Each type implements the matching
// capability interface from internal/starlark (Querier, KVGetter, KVSetter,
// KVDeleter, Locker, Queue, PubSub, Storage) so a single backend can satisfy
// both the Starlark contract and the sidecar's identical one.
package memory

import (
	"context"
	"sync"
	"time"
)

// ─── Cache / KVStore — in-memory key-value with optional TTL ───

type kvEntry struct {
	value   any
	expires time.Time // zero = no expiry
}

// KV is an in-memory key-value store with optional per-key TTL. It backs both
// ctx.cache() and ctx.kvstore() in dev mode.
type KV struct {
	mu    sync.RWMutex
	items map[string]kvEntry
	now   func() time.Time
}

// NewKV creates an empty in-memory KV store.
func NewKV() *KV {
	return &KV{items: make(map[string]kvEntry), now: time.Now}
}

// Get returns the value for key, or (nil, nil) when absent or expired.
func (k *KV) Get(_ context.Context, key string) (any, error) {
	k.mu.RLock()
	e, ok := k.items[key]
	k.mu.RUnlock()
	if !ok {
		return nil, nil
	}
	if !e.expires.IsZero() && k.now().After(e.expires) {
		k.mu.Lock()
		delete(k.items, key)
		k.mu.Unlock()
		return nil, nil
	}
	return e.value, nil
}

// Set stores value under key with an optional TTL (0 = no expiry).
func (k *KV) Set(_ context.Context, key string, value any, ttl time.Duration) error {
	var expires time.Time
	if ttl > 0 {
		expires = k.now().Add(ttl)
	}
	k.mu.Lock()
	k.items[key] = kvEntry{value: value, expires: expires}
	k.mu.Unlock()
	return nil
}

// Delete removes key.
func (k *KV) Delete(_ context.Context, key string) error {
	k.mu.Lock()
	delete(k.items, key)
	k.mu.Unlock()
	return nil
}

// ─── Lock — in-memory distributed lock ───

// Lock is an in-memory mutual-exclusion lock keyed by name. It backs
// ctx.lock() in dev mode.
type Lock struct {
	mu    sync.Mutex
	holds map[string]time.Time // key → expiry (zero = held forever)
	now   func() time.Time
}

// NewLock creates an empty in-memory lock.
func NewLock() *Lock {
	return &Lock{holds: make(map[string]time.Time), now: time.Now}
}

// Acquire attempts to acquire the lock for key. Returns true on success.
// A held lock whose TTL has elapsed is considered expired and re-acquirable.
// A lock acquired with ttl=0 is held forever (never auto-expires).
func (l *Lock) Acquire(_ context.Context, key string, ttl time.Duration) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if exp, ok := l.holds[key]; ok {
		if exp.IsZero() {
			return false, nil // held forever
		}
		if l.now().Before(exp) {
			return false, nil // still held
		}
		// expired — fall through and re-acquire
	}
	var expires time.Time
	if ttl > 0 {
		expires = l.now().Add(ttl)
	}
	l.holds[key] = expires
	return true, nil
}

// Release releases the lock for key.
func (l *Lock) Release(_ context.Context, key string) error {
	l.mu.Lock()
	delete(l.holds, key)
	l.mu.Unlock()
	return nil
}

// ─── Queue — in-memory FIFO queue ───

// Queue is an in-memory FIFO queue keyed by name. It backs ctx.queue() in dev
// mode. Dequeue on an empty queue returns (nil, nil).
type Queue struct {
	mu     sync.Mutex
	queues map[string][]any
}

// NewQueue creates an empty in-memory queue.
func NewQueue() *Queue {
	return &Queue{queues: make(map[string][]any)}
}

// Enqueue appends payload to the named queue.
func (q *Queue) Enqueue(_ context.Context, name string, payload any) error {
	q.mu.Lock()
	q.queues[name] = append(q.queues[name], payload)
	q.mu.Unlock()
	return nil
}

// Dequeue pops the front of the named queue. Returns (nil, nil) when empty.
func (q *Queue) Dequeue(_ context.Context, name string) (any, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	items := q.queues[name]
	if len(items) == 0 {
		return nil, nil
	}
	head := items[0]
	if len(items) == 1 {
		delete(q.queues, name)
	} else {
		q.queues[name] = items[1:]
	}
	return head, nil
}

// ─── PubSub — in-memory publish/subscribe ───

// PubSub is an in-memory publish/subscribe bus keyed by channel. It backs
// ctx.pubsub() in dev mode. Subscribers are invoked synchronously on publish.
type PubSub struct {
	mu   sync.Mutex
	subs map[string][]func(any)
	seq  uint64
}

// NewPubSub creates an empty in-memory pub/sub bus.
func NewPubSub() *PubSub {
	return &PubSub{subs: make(map[string][]func(any))}
}

// Publish delivers payload to every subscriber of channel.
func (p *PubSub) Publish(_ context.Context, channel string, payload any) error {
	p.mu.Lock()
	subs := make([]func(any), len(p.subs[channel]))
	copy(subs, p.subs[channel])
	p.mu.Unlock()
	for _, cb := range subs {
		cb(payload)
	}
	return nil
}

// Subscribe registers cb for channel.
func (p *PubSub) Subscribe(_ context.Context, channel string, cb func(any)) error {
	p.mu.Lock()
	p.subs[channel] = append(p.subs[channel], cb)
	p.mu.Unlock()
	return nil
}
