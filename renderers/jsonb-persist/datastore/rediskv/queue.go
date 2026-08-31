// Queue is a Redis/Valkey-backed FIFO queue for the ctx.queue() primitive
// (plan docs/plan/infra-registry-3-level.md fase A2). It implements the
// starlark Queue contract with the same semantics as memory.Queue: Dequeue
// on an empty queue returns (nil, nil) — non-blocking. Payloads are
// JSON-encoded; FIFO order comes from LPUSH (tail) + RPOP (head).
package rediskv

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// Queue is a Redis-backed FIFO queue keyed by name.
type Queue struct {
	client    *redis.Client
	namespace string
}

// NewQueue opens a Redis connection at addr ("host:port") with an optional
// key namespace prefix (empty = "formspec") and verifies connectivity.
func NewQueue(addr, namespace string) (*Queue, error) {
	if namespace == "" {
		namespace = "formspec"
	}
	client, err := dialRedis(addr)
	if err != nil {
		return nil, err
	}
	return &Queue{client: client, namespace: namespace}, nil
}

// Enqueue appends payload to the tail of the named queue.
func (q *Queue) Enqueue(ctx context.Context, name string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("redis queue enqueue %s: encode: %w", name, err)
	}
	if err := q.client.LPush(ctx, q.key(name), data).Err(); err != nil {
		return fmt.Errorf("redis queue enqueue %s: %w", name, err)
	}
	return nil
}

// Dequeue pops the head of the named queue. Returns (nil, nil) when the
// queue is empty — matching memory.Queue (no blocking, no error on empty).
func (q *Queue) Dequeue(ctx context.Context, name string) (any, error) {
	data, err := q.client.RPop(ctx, q.key(name)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis queue dequeue %s: %w", name, err)
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("redis queue dequeue %s: decode: %w", name, err)
	}
	return v, nil
}

// Close closes the underlying connection.
func (q *Queue) Close() error { return q.client.Close() }

func (q *Queue) key(name string) string { return q.namespace + ":queue:" + name }
