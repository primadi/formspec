// Package rediskv provides a Redis/Valkey-backed KV connection for the
// ctx.cache()/ctx.kvstore() primitives (Plan C batch 2 — todo 13.5.6).
// It implements the starlark KVGetter/KVSetter/KVDeleter contract with the
// same semantics as memory.KV: values are JSON-encoded, missing keys return
// (nil, nil), TTL 0 = no expiry.
package rediskv

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// KV is a Redis-backed key-value connection.
type KV struct {
	client    *redis.Client
	namespace string // key prefix for multi-tenant/multi-app isolation
}

// invalidateChannel is the pub/sub channel for multi-instance cache
// invalidation (Fase 14 v2): mutators publish the deleted key; every
// instance subscribed deletes it locally (no re-broadcast — no loop).
const invalidateChannel = "formspec:cache:invalidate"

// New opens a Redis/Valkey connection at addr ("host:port") with an optional
// key namespace prefix (empty = "formspec:"). A background goroutine
// subscribes to the invalidation channel and deletes published keys locally,
// making cross-instance invalidation automatic.
func New(addr, namespace string) (*KV, error) {
	if namespace == "" {
		namespace = "formspec"
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connect %s: %w", addr, err)
	}
	kv := &KV{client: client, namespace: namespace}
	go kv.subscribeInvalidate()
	return kv, nil
}

// subscribeInvalidate listens on the invalidation channel and deletes
// published keys locally. Runs for the lifetime of the connection; exits
// quietly when the client is closed.
func (k *KV) subscribeInvalidate() {
	sub := k.client.Subscribe(context.Background(), invalidateChannel)
	defer sub.Close()
	for msg := range sub.Channel() {
		if msg.Payload == "" {
			continue
		}
		_ = k.client.Del(context.Background(), msg.Payload)
	}
}

// BroadcastInvalidate deletes the key locally AND publishes it to the
// invalidation channel so other instances delete their copy too (Fase 14 v2).
// Implements the api.CacheInvalidator optional contract.
func (k *KV) BroadcastInvalidate(ctx context.Context, key string) error {
	if err := k.client.Del(ctx, k.key(key)).Err(); err != nil {
		return fmt.Errorf("redis invalidate %s: %w", key, err)
	}
	// Publish the FULL (namespaced) key — subscribers delete verbatim.
	if err := k.client.Publish(ctx, invalidateChannel, k.key(key)).Err(); err != nil {
		return fmt.Errorf("redis invalidate publish %s: %w", key, err)
	}
	return nil
}

// Close closes the underlying connection.
func (k *KV) Close() error { return k.client.Close() }

func (k *KV) key(key string) string { return k.namespace + ":" + key }

// Get returns the value stored at key, or (nil, nil) when absent — matching
// memory.KV semantics (no error on miss).
func (k *KV) Get(ctx context.Context, key string) (any, error) {
	data, err := k.client.Get(ctx, k.key(key)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis get %s: %w", key, err)
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("redis get %s: decode: %w", key, err)
	}
	return v, nil
}

// Set stores value (JSON-encoded) with an optional TTL (0 = no expiry).
func (k *KV) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("redis set %s: encode: %w", key, err)
	}
	if err := k.client.Set(ctx, k.key(key), data, ttl).Err(); err != nil {
		return fmt.Errorf("redis set %s: %w", key, err)
	}
	return nil
}

// Delete removes key (no-op when absent).
func (k *KV) Delete(ctx context.Context, key string) error {
	if err := k.client.Del(ctx, k.key(key)).Err(); err != nil {
		return fmt.Errorf("redis delete %s: %w", key, err)
	}
	return nil
}
