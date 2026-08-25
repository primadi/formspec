package stream

import (
	"context"
	"testing"
	"time"
)

func TestMemory_AppendReadAck(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()

	id1, err := m.Append(ctx, "demo.products.on_create", map[string]any{"name": "a"})
	if err != nil {
		t.Fatal(err)
	}
	id2, _ := m.Append(ctx, "demo.products.on_create", map[string]any{"name": "b"})
	id3, _ := m.Append(ctx, "demo.products.on_create", map[string]any{"name": "c"})

	// Read 2 — new entries, attempts=1.
	entries, err := m.Read(ctx, "demo.products.on_create", "demo/audit", "w", "earliest", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("Read: want 2, got %d", len(entries))
	}
	if entries[0].ID != id1 || entries[1].ID != id2 {
		t.Errorf("Read order: want %s,%s got %s,%s", id1, id2, entries[0].ID, entries[1].ID)
	}
	if entries[0].Attempts != 1 || entries[1].Attempts != 1 {
		t.Errorf("Attempts: want 1,1 got %d,%d", entries[0].Attempts, entries[1].Attempts)
	}

	// Ack the first, read again — should get the third (new) entry.
	if err := m.Ack(ctx, "demo.products.on_create", "demo/audit", id1); err != nil {
		t.Fatal(err)
	}
	entries, err = m.Read(ctx, "demo.products.on_create", "demo/audit", "w", "earliest", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("Read after ack: want 2 (pending id2 + new id3), got %d", len(entries))
	}
	if entries[0].ID != id2 || entries[1].ID != id3 {
		t.Errorf("Read after ack: want %s,%s got %s,%s", id2, id3, entries[0].ID, entries[1].ID)
	}
}

func TestMemory_AtLeastOnceRetry(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()

	id, _ := m.Append(ctx, "s", map[string]any{"v": 1})

	// First read — attempts=1.
	entries, _ := m.Read(ctx, "s", "g", "c", "earliest", 10)
	if len(entries) != 1 || entries[0].Attempts != 1 {
		t.Fatalf("first read: want 1 entry attempts=1, got %d attempts=%d", len(entries), entries[0].Attempts)
	}
	// Do NOT ack — simulate a failed dispatch. Next read must redeliver the
	// same entry with attempts=2 (at-least-once).
	entries, _ = m.Read(ctx, "s", "g", "c", "earliest", 10)
	if len(entries) != 1 || entries[0].ID != id || entries[0].Attempts != 2 {
		t.Fatalf("redelivery: want id=%s attempts=2, got id=%s attempts=%d", id, entries[0].ID, entries[0].Attempts)
	}
	// Ack — no more pending.
	if err := m.Ack(ctx, "s", "g", id); err != nil {
		t.Fatal(err)
	}
	entries, _ = m.Read(ctx, "s", "g", "c", "earliest", 10)
	if len(entries) != 0 {
		t.Fatalf("after ack: want 0 entries, got %d", len(entries))
	}
}

func TestMemory_Position(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if _, err := m.Append(ctx, "s", map[string]any{"i": i}); err != nil {
			t.Fatal(err)
		}
	}

	// latest — a new group starts after existing entries.
	entries, _ := m.Read(ctx, "s", "g-latest", "c", "latest", 10)
	if len(entries) != 0 {
		t.Fatalf("latest: want 0 existing entries, got %d", len(entries))
	}
	// Append after group creation — the latest group sees it.
	if _, err := m.Append(ctx, "s", map[string]any{"i": 3}); err != nil {
		t.Fatal(err)
	}
	entries, _ = m.Read(ctx, "s", "g-latest", "c", "latest", 10)
	if len(entries) != 1 || entries[0].Data["i"] != 3 {
		t.Fatalf("latest after append: want 1 entry i=3, got %d", len(entries))
	}

	// earliest — a new group replays from the beginning.
	entries, _ = m.Read(ctx, "s", "g-earliest", "c", "earliest", 10)
	if len(entries) != 4 {
		t.Fatalf("earliest: want 4 entries, got %d", len(entries))
	}
}

func TestMemory_MultipleGroups(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_, _ = m.Append(ctx, "s", map[string]any{"i": i})
	}

	// Two independent consumer groups each see all 3 entries.
	for _, g := range []string{"g1", "g2"} {
		entries, _ := m.Read(ctx, "s", g, "c", "earliest", 10)
		if len(entries) != 3 {
			t.Fatalf("group %s: want 3 entries, got %d", g, len(entries))
		}
	}
}

func TestMemory_TrimByCount(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_, _ = m.Append(ctx, "s", map[string]any{"i": i})
	}
	if err := m.Trim(ctx, "s", "3"); err != nil {
		t.Fatal(err)
	}
	entries, _ := m.Read(ctx, "s", "g", "c", "earliest", 10)
	if len(entries) != 3 {
		t.Fatalf("trim by count: want 3 entries, got %d", len(entries))
	}
	if entries[0].Data["i"] != 2 {
		t.Errorf("trim by count: first kept entry should be i=2, got %v", entries[0].Data["i"])
	}
}

func TestMemory_TrimByAge(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()
	now := time.Now()
	m.now = func() time.Time { return now }

	_, _ = m.Append(ctx, "s", map[string]any{"old": true})
	now = now.Add(2 * time.Hour)
	_, _ = m.Append(ctx, "s", map[string]any{"new": true})

	if err := m.Trim(ctx, "s", "1h"); err != nil {
		t.Fatal(err)
	}
	entries, _ := m.Read(ctx, "s", "g", "c", "earliest", 10)
	if len(entries) != 1 {
		t.Fatalf("trim by age: want 1 entry, got %d", len(entries))
	}
	if entries[0].Data["new"] != true {
		t.Errorf("trim by age: kept entry should be the new one")
	}
}

func TestParseRetention(t *testing.T) {
	cases := []struct {
		in     string
		maxAge time.Duration
		maxLen int64
		ok     bool
	}{
		{"", 0, 0, false},
		{"1000", 0, 1000, true},
		{"7d", 7 * 24 * time.Hour, 0, true},
		{"24h", 24 * time.Hour, 0, true},
		{"30m", 30 * time.Minute, 0, true},
		{"bogus", 0, 0, false},
	}
	for _, c := range cases {
		age, n, ok := ParseRetention(c.in)
		if ok != c.ok || age != c.maxAge || n != c.maxLen {
			t.Errorf("ParseRetention(%q): got (%v,%d,%v), want (%v,%d,%v)", c.in, age, n, ok, c.maxAge, c.maxLen, c.ok)
		}
	}
}
