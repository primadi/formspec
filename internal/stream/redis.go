package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis is a Stream implementation backed by Redis Streams (also works with
// Valkey, the Redis-compatible server used in the dev container). It uses
// consumer groups for at-least-once delivery: XREADGROUP for new entries,
// XPENDING + XAUTOCLAIM for redelivery of unacked (failed) entries, XACK to
// acknowledge, XTRIM for retention.
type Redis struct {
	client *redis.Client
}

// NewRedis connects to a Redis/Valkey server at addr (e.g. "valkey:6379").
// It fails fast if the server is unreachable.
func NewRedis(addr string) (*Redis, error) {
	client := redis.NewClient(&redis.Options{Addr: addr})
	if err := client.Ping(context.Background()).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("stream redis: connect %s: %w", addr, err)
	}
	return &Redis{client: client}, nil
}

// Append appends an entry to the named stream (XADD). The data map is
// JSON-serialized into a single "data" field — Redis Streams store
// field-value pairs as strings, so nested maps must be encoded.
func (r *Redis) Append(ctx context.Context, stream string, data map[string]any) (string, error) {
	encoded, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("stream redis: encode entry: %w", err)
	}
	id, err := r.client.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: map[string]any{"data": string(encoded)},
	}).Result()
	if err != nil {
		return "", fmt.Errorf("stream redis: xadd %s: %w", stream, err)
	}
	return id, nil
}

// Read claims up to count entries for the consumer group: new entries via
// XREADGROUP, then pending (unacked) entries via XPENDING + XAUTOCLAIM with
// their delivery counts. See Stream.Read.
func (r *Redis) Read(ctx context.Context, stream, group, consumer, position string, count int) ([]Entry, error) {
	if err := r.ensureGroup(ctx, stream, group, position); err != nil {
		return nil, err
	}
	if count <= 0 {
		count = 1
	}

	var out []Entry

	// 1. New entries (XREADGROUP with ">" = only entries never delivered).
	// Block: -1 = do not block (the worker polls on an interval).
	streams, err := r.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{stream, ">"},
		Count:    int64(count),
		Block:    -1,
	}).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("stream redis: xreadgroup %s: %w", stream, err)
	}
	newIDs := make(map[string]bool)
	for _, xs := range streams {
		for _, msg := range xs.Messages {
			newIDs[msg.ID] = true
			out = append(out, Entry{
				ID:        msg.ID,
				Data:      decodeEntry(msg.Values),
				Timestamp: redisIDTime(msg.ID),
				Attempts:  1,
			})
		}
	}

	// 2. Pending (previously claimed, unacked) entries — redelivered with
	// their delivery count (XPENDING extended) and data (XAUTOCLAIM). Entries
	// just claimed as new above are skipped (they are already pending).
	pending, err := r.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: stream,
		Group:  group,
		Start:  "-",
		End:    "+",
		Count:  int64(count),
	}).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("stream redis: xpending %s: %w", stream, err)
	}
	if len(pending) > 0 {
		counts := make(map[string]int64, len(pending))
		for _, p := range pending {
			counts[p.ID] = p.RetryCount
		}
		claimed, _, err := r.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
			Stream:   stream,
			Group:    group,
			Consumer: consumer,
			MinIdle:  0,
			Start:    "0",
			Count:    int64(len(pending)),
		}).Result()
		if err != nil && err != redis.Nil {
			return nil, fmt.Errorf("stream redis: xautoclaim %s: %w", stream, err)
		}
		for _, msg := range claimed {
			if newIDs[msg.ID] {
				continue
			}
			attempts := int(counts[msg.ID])
			if attempts < 1 {
				attempts = 1
			}
			out = append(out, Entry{
				ID:        msg.ID,
				Data:      decodeEntry(msg.Values),
				Timestamp: redisIDTime(msg.ID),
				Attempts:  attempts,
			})
		}
	}
	return out, nil
}

// Ack acknowledges an entry for the consumer group (XACK).
func (r *Redis) Ack(ctx context.Context, stream, group, id string) error {
	if err := r.client.XAck(ctx, stream, group, id).Err(); err != nil {
		return fmt.Errorf("stream redis: xack %s: %w", stream, err)
	}
	return nil
}

// Trim enforces retention on the named stream: max-age → XTRIM MINID,
// max-length → XTRIM MAXLEN.
func (r *Redis) Trim(ctx context.Context, stream, retention string) error {
	maxAge, maxLen, ok := ParseRetention(retention)
	if !ok {
		return nil
	}
	var err error
	if maxAge > 0 {
		minID := strconv.FormatInt(time.Now().Add(-maxAge).UnixMilli(), 10)
		err = r.client.XTrimMinID(ctx, stream, minID).Err()
	} else if maxLen > 0 {
		err = r.client.XTrimMaxLen(ctx, stream, maxLen).Err()
	}
	if err != nil {
		return fmt.Errorf("stream redis: xtrim %s: %w", stream, err)
	}
	return nil
}

// Close closes the underlying Redis client.
func (r *Redis) Close() error {
	return r.client.Close()
}

// ensureGroup creates the consumer group if it does not exist, using the
// subscription's position for a brand-new group ("$" = latest, "0" =
// earliest, or a concrete ID). A BUSYGROUP error means the group already
// exists — its own cursor is kept.
func (r *Redis) ensureGroup(ctx context.Context, stream, group, position string) error {
	start := "0"
	switch position {
	case "latest":
		start = "$"
	case "earliest", "":
		start = "0"
	default:
		start = position
	}
	err := r.client.XGroupCreateMkStream(ctx, stream, group, start).Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return fmt.Errorf("stream redis: xgroup create %s %s: %w", stream, group, err)
	}
	return nil
}

// redisIDTime parses the millisecond timestamp from a Redis stream ID
// ("<ms>-<seq>"). Falls back to zero time on parse failure.
func redisIDTime(id string) time.Time {
	msStr, _, _ := strings.Cut(id, "-")
	ms, err := strconv.ParseInt(msStr, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}

// decodeEntry reconstructs the original data map from a Redis stream message.
// Append stores the JSON-encoded map under the "data" field; anything else
// (e.g. a message written by another tool) is returned as-is.
func decodeEntry(values map[string]any) map[string]any {
	raw, ok := values["data"].(string)
	if !ok {
		return values
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return values
	}
	return data
}