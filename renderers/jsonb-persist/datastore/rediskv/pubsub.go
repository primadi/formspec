// PubSub is a Redis/Valkey-backed publish/subscribe bus for the ctx.pubsub()
// primitive (plan docs/plan/infra-registry-3-level.md fase A2). It implements
// the starlark PubSub contract. Redis pub/sub is fire-and-forget: deliveries
// reach only subscribers connected at publish time — no persistence, no
// replay (durable streaming is the separate internal/stream seam). Payloads
// are JSON-encoded; each subscriber's callback runs on its own goroutine.
package rediskv

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// PubSub is a Redis-backed publish/subscribe bus keyed by channel.
type PubSub struct {
	client    *redis.Client
	namespace string
}

// NewPubSub opens a Redis connection at addr ("host:port") with an optional
// channel namespace prefix (empty = "formspec") and verifies connectivity.
func NewPubSub(addr, namespace string) (*PubSub, error) {
	if namespace == "" {
		namespace = "formspec"
	}
	client, err := dialRedis(addr)
	if err != nil {
		return nil, err
	}
	return &PubSub{client: client, namespace: namespace}, nil
}

// Publish delivers payload (JSON-encoded) to every current subscriber of
// channel. With no subscriber the message is dropped silently — Redis
// pub/sub semantics.
func (p *PubSub) Publish(ctx context.Context, channel string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("redis pubsub publish %s: encode: %w", channel, err)
	}
	if err := p.client.Publish(ctx, p.channel(channel), data).Err(); err != nil {
		return fmt.Errorf("redis pubsub publish %s: %w", channel, err)
	}
	return nil
}

// Subscribe registers cb for channel. Deliveries arrive on a background
// goroutine that exits when ctx is cancelled or the connection closes.
func (p *PubSub) Subscribe(ctx context.Context, channel string, cb func(payload any)) error {
	sub := p.client.Subscribe(ctx, p.channel(channel))
	go func() {
		defer sub.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-sub.Channel():
				if !ok {
					return
				}
				var v any
				if err := json.Unmarshal([]byte(msg.Payload), &v); err != nil {
					continue // non-JSON payload — skip
				}
				cb(v)
			}
		}
	}()
	return nil
}

// Close closes the underlying connection.
func (p *PubSub) Close() error { return p.client.Close() }

func (p *PubSub) channel(channel string) string { return p.namespace + ":pubsub:" + channel }
