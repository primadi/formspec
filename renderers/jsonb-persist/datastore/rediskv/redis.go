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

// New opens a Redis/Valkey connection at addr ("host:port") with an optional
// key namespace prefix (empty = "formspec:").
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
	return &KV{client: client, namespace: namespace}, nil
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
