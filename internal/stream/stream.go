// Package stream provides the durable event-stream abstraction for Tier 2
// subscriptions (todo 7.3.2, docs/spec/backend/02-core-extended.md §3).
//
// It is the KVStore-like seam for streaming: subscription code never touches
// a backend (Redis/Kafka/in-memory) directly — it only talks to the Stream
// interface. Backends are pluggable:
//
//	memory — in-memory (dev default, auto-provisioned)
//	redis  — Redis Streams / Valkey (FORMSPEC_STREAM=redis)
//	kafka  — future
//
// Semantics are at-least-once with positioned replay: an entry read by a
// consumer group is "claimed" (pending) until Acked; a failed entry stays
// pending and is redelivered on the next Read (Attempts increments), so a
// consumer can retry up to max_retry before dead-lettering.
package stream

import (
	"context"
	"strconv"
	"strings"
	"time"
)

// Entry is one event in a stream.
type Entry struct {
	// ID is the backend-specific position ID (Redis stream ID, memory
	// sequence, ...). Opaque to callers — pass it back to Ack.
	ID string
	// Data is the event payload (a map, e.g. the subscription wire payload
	// plus delivery metadata).
	Data map[string]any
	// Timestamp is when the entry was appended.
	Timestamp time.Time
	// Attempts is how many times this entry has been delivered to the
	// consumer group so far (1 = first delivery). Used for max_retry /
	// dead-letter decisions.
	Attempts int
}

// Stream is the durable event-stream abstraction (todo 7.3.2). Implementations
// must be safe for concurrent use.
type Stream interface {
	// Append appends an entry to the named stream and returns its ID.
	Append(ctx context.Context, stream string, data map[string]any) (string, error)

	// Read claims up to count entries for the given consumer group and
	// returns them. Claimed entries are pending until Acked (at-least-once).
	//
	// position controls where a brand-new group starts reading:
	//   "" / "earliest" — from the beginning (replay)
	//   "latest"        — only entries appended after the group is created
	//   "<id>"          — from a concrete entry ID
	// Once the group exists, position is ignored (the group keeps its own
	// cursor). Pending (previously claimed but unacked) entries are returned
	// before new ones, with Attempts reflecting prior deliveries.
	Read(ctx context.Context, stream, group, consumer, position string, count int) ([]Entry, error)

	// Ack marks an entry as processed for the given consumer group.
	Ack(ctx context.Context, stream, group, id string) error

	// Trim enforces retention on the named stream. retention is either a
	// duration ("7d", "24h", "30m") or a bare count ("1000" = keep last N
	// entries). Entries older than the retention are removed.
	Trim(ctx context.Context, stream, retention string) error

	// Close releases backend resources.
	Close() error
}

// ParseRetention parses a retention string into either a max-age duration or
// a max-length count. Returns (duration, count, ok). A bare integer string is
// treated as a count; a suffixed duration ("7d", "24h", "30m", "60s") as a
// max-age ("d" = days, which time.ParseDuration does not support). Empty
// string returns ok=false (no retention).
func ParseRetention(retention string) (maxAge time.Duration, maxLen int64, ok bool) {
	if retention == "" {
		return 0, 0, false
	}
	if n, err := strconv.ParseInt(retention, 10, 64); err == nil {
		return 0, n, true
	}
	if strings.HasSuffix(retention, "d") {
		days, err := strconv.ParseInt(strings.TrimSuffix(retention, "d"), 10, 64)
		if err == nil {
			return time.Duration(days) * 24 * time.Hour, 0, true
		}
	}
	d, err := time.ParseDuration(retention)
	if err != nil {
		return 0, 0, false
	}
	return d, 0, true
}

// NormalizeStreamName returns a safe stream key for an event name. Event names
// are fully-qualified resource events ("billing.invoice.on_submit"); they are
// already safe, but we guard against empty names.
func NormalizeStreamName(eventName string) string {
	name := strings.TrimSpace(eventName)
	if name == "" {
		return "default"
	}
	return name
}