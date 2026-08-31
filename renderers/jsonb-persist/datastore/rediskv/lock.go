// Lock is a Redis/Valkey-backed distributed lock for the ctx.lock()
// primitive (plan docs/plan/infra-registry-3-level.md fase A2). It
// implements the starlark Locker contract with the same semantics as
// memory.Lock: Acquire returns true when the lock was free (or its TTL
// elapsed); ttl=0 holds forever (no expiry). Release only succeeds for the
// acquirer — each Lock instance carries a random token and release is a
// compare-and-delete Lua script, so a stale holder can never release a
// lock re-acquired by someone else.
package rediskv

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Lock is a Redis-backed mutual-exclusion lock.
type Lock struct {
	client    *redis.Client
	namespace string
	token     string // per-instance identity for safe release
}

// NewLock opens a Redis connection at addr ("host:port") with an optional
// key namespace prefix (empty = "formspec") and verifies connectivity.
func NewLock(addr, namespace string) (*Lock, error) {
	if namespace == "" {
		namespace = "formspec"
	}
	client, err := dialRedis(addr)
	if err != nil {
		return nil, err
	}
	tok := make([]byte, 16)
	if _, err := rand.Read(tok); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis lock token: %w", err)
	}
	return &Lock{client: client, namespace: namespace, token: hex.EncodeToString(tok)}, nil
}

// Acquire attempts to acquire the lock for key. Returns true on success,
// false when the lock is currently held by another acquirer (or this one,
// prior to expiry). ttl=0 holds the lock forever — matching memory.Lock.
func (l *Lock) Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	var ttlArg time.Duration
	if ttl > 0 {
		ttlArg = ttl
	}
	ok, err := l.client.SetNX(ctx, l.key(key), l.token, ttlArg).Result()
	if err != nil {
		return false, fmt.Errorf("redis lock acquire %s: %w", key, err)
	}
	return ok, nil
}

// releaseScript deletes the lock key only when its value still equals the
// caller's token — compare-and-delete, atomic via Lua.
const releaseScript = `if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("del", KEYS[1]) else return 0 end`

// Release releases the lock for key. Releasing a lock held by another
// acquirer is a no-op (the key is left intact).
func (l *Lock) Release(ctx context.Context, key string) error {
	if err := l.client.Eval(ctx, releaseScript, []string{l.key(key)}, l.token).Err(); err != nil &&
		err != redis.Nil {
		return fmt.Errorf("redis lock release %s: %w", key, err)
	}
	return nil
}

// Close closes the underlying connection.
func (l *Lock) Close() error { return l.client.Close() }

func (l *Lock) key(key string) string { return l.namespace + ":lock:" + key }
