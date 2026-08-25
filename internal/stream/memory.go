package stream

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// memoryEntry is one entry in an in-memory stream.
type memoryEntry struct {
	id   string
	data map[string]any
	ts   time.Time
}

// groupState tracks a consumer group's position within a stream.
type groupState struct {
	// cursor is the index of the next NEW entry to deliver. Entries before
	// the cursor have already been delivered as "new" at least once.
	cursor int
	// pending maps entry id → delivery attempts for entries claimed but not
	// yet acked (at-least-once: they are redelivered until acked).
	pending map[string]int
}

// Memory is an in-memory Stream implementation (dev default). It models the
// same semantics as Redis Streams consumer groups: one ordered entry list per
// stream, an independent cursor + pending set per consumer group, at-least-once
// delivery with redelivery of unacked entries.
type Memory struct {
	mu      sync.Mutex
	streams map[string][]*memoryEntry
	groups  map[string]map[string]*groupState // stream → group → state
	seq     uint64
	now     func() time.Time
}

// NewMemory creates an empty in-memory stream backend.
func NewMemory() *Memory {
	return &Memory{
		streams: make(map[string][]*memoryEntry),
		groups:  make(map[string]map[string]*groupState),
		now:     time.Now,
	}
}

// Append appends an entry to the named stream. IDs are "<unixms>-<seq>" so
// they are unique and sortable (positioned replay).
func (m *Memory) Append(_ context.Context, stream string, data map[string]any) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seq++
	id := fmt.Sprintf("%d-%d", m.now().UnixMilli(), m.seq)
	m.streams[stream] = append(m.streams[stream], &memoryEntry{id: id, data: data, ts: m.now()})
	return id, nil
}

// Read claims up to count entries for the consumer group: pending (unacked)
// entries first, then new entries from the group's cursor. See Stream.Read.
func (m *Memory) Read(_ context.Context, stream, group, consumer, position string, count int) ([]Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entries := m.streams[stream]
	gs := m.group(stream, group, position, len(entries))
	if count <= 0 {
		count = 1
	}

	out := make([]Entry, 0, count)
	// 1. Pending (previously claimed, unacked) entries — redelivered with an
	// incremented attempt count.
	for _, e := range entries {
		if len(out) >= count {
			break
		}
		if attempts, ok := gs.pending[e.id]; ok {
			attempts++
			gs.pending[e.id] = attempts
			out = append(out, Entry{ID: e.id, Data: e.data, Timestamp: e.ts, Attempts: attempts})
		}
	}
	// 2. New entries from the cursor.
	for i := gs.cursor; i < len(entries) && len(out) < count; i++ {
		e := entries[i]
		gs.pending[e.id] = 1
		gs.cursor = i + 1
		out = append(out, Entry{ID: e.id, Data: e.data, Timestamp: e.ts, Attempts: 1})
	}
	return out, nil
}

// Ack removes an entry from the group's pending set (marks it processed).
func (m *Memory) Ack(_ context.Context, stream, group, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	gs := m.groups[stream][group]
	if gs == nil {
		return nil
	}
	delete(gs.pending, id)
	return nil
}

// Trim enforces retention on the named stream (max-age duration or max-length
// count). Group cursors are clamped and pending entries that were trimmed are
// dropped.
func (m *Memory) Trim(_ context.Context, stream, retention string) error {
	maxAge, maxLen, ok := ParseRetention(retention)
	if !ok {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	entries := m.streams[stream]
	if len(entries) == 0 {
		return nil
	}
	now := m.now()
	keepFrom := 0
	if maxAge > 0 {
		cutoff := now.Add(-maxAge)
		for keepFrom < len(entries) && entries[keepFrom].ts.Before(cutoff) {
			keepFrom++
		}
	} else if maxLen > 0 {
		if int64(len(entries)) > maxLen {
			keepFrom = len(entries) - int(maxLen)
		}
	}
	if keepFrom == 0 {
		return nil
	}
	removed := entries[:keepFrom]
	m.streams[stream] = entries[keepFrom:]
	for _, gs := range m.groups[stream] {
		gs.cursor -= keepFrom
		if gs.cursor < 0 {
			gs.cursor = 0
		}
		for _, e := range removed {
			delete(gs.pending, e.id)
		}
	}
	return nil
}

// Close is a no-op for the in-memory backend.
func (m *Memory) Close() error { return nil }

// group returns the group state for (stream, group), creating it with the
// given position on first use.
func (m *Memory) group(stream, group, position string, total int) *groupState {
	byGroup := m.groups[stream]
	if byGroup == nil {
		byGroup = make(map[string]*groupState)
		m.groups[stream] = byGroup
	}
	if gs, ok := byGroup[group]; ok {
		return gs
	}
	gs := &groupState{pending: make(map[string]int)}
	switch position {
	case "latest":
		gs.cursor = total
	case "earliest", "":
		gs.cursor = 0
	default:
		// Concrete ID — start at that entry.
		gs.cursor = total
		for i, e := range m.streams[stream] {
			if e.id == position {
				gs.cursor = i
				break
			}
		}
	}
	byGroup[group] = gs
	return gs
}