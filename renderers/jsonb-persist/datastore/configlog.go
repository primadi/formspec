// Package datastore — config backend (plan docs/plan/infra-registry-3-level.md fase D).
//
// KVConfig adapts a KV backend (memory.KV, rediskv.KV) into the starlark
// Config capability (Get — structural interface, no import needed) so a
// kind: Datastore serving `config` can back ctx.config — e.g. a centralized
// Redis config store for multi-instance deployments. Missing keys return
// (nil, nil); the caller's default applies.
package datastore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// KVGetter is the minimal KV contract needed by KVConfig (satisfied by
// memory.KV and rediskv.KV).
type KVGetter interface {
	Get(ctx context.Context, key string) (any, error)
}

// KVConfig serves the starlark Config capability (Get(ctx, key)) over a KV
// backend. Keys are namespaced under "config:" to avoid collisions with
// cache/kvstore keys sharing the same physical service.
type KVConfig struct {
	kv KVGetter
}

// NewKVConfig wraps a KV backend into the Config capability.
func NewKVConfig(kv KVGetter) *KVConfig { return &KVConfig{kv: kv} }

// Get returns the value stored at key, or (nil, nil) when absent — the
// script's default applies (same semantics as the builtin configAPI).
func (c *KVConfig) Get(ctx context.Context, key string) (any, error) {
	return c.kv.Get(ctx, "config:"+key)
}

// ─── log backend ───

// KVLogWriter is the minimal KV write contract for the log backend
// (satisfied by memory.KV and rediskv.KV).
type KVLogWriter interface {
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
}

// KVLog serves the starlark Logger capability (Log(ctx, level, event, meta))
// over a KV backend. Entries are appended under the namespaced key
// "log:<level>:<event>" as a growing slice — a simple centralized sink for
// multi-instance deployments; a production-grade backend (Loki, etc.) would
// replace this adapter without changing the contract.
type KVLog struct {
	kv KVLogWriter
}

// NewKVLog wraps a KV backend into the Logger capability.
func NewKVLog(kv KVLogWriter) *KVLog { return &KVLog{kv: kv} }

// Log appends one entry to the backend. Errors are returned for the caller
// to log — a log failure must never break the business action.
func (l *KVLog) Log(ctx context.Context, level, event string, meta map[string]any) error {
	key := "log:" + level + ":" + event
	existing, _ := l.appendGet(ctx, key)
	entries, _ := existing.([]any)
	entries = append(entries, meta)
	return l.kv.Set(ctx, key, entries, 0)
}

// appendGet reads the current entry list (best-effort — a read failure
// starts a fresh list).
func (l *KVLog) appendGet(ctx context.Context, key string) (any, error) {
	g, ok := l.kv.(interface {
		Get(ctx context.Context, key string) (any, error)
	})
	if !ok {
		return nil, nil
	}
	return g.Get(ctx, key)
}

// ─── in-memory log (built-in 'default' + memory driver) ───

// MemoryLog is an in-memory centralized log sink implementing the starlark
// Logger capability (Log(ctx, level, event, meta)). Entries live in the
// process — suitable for the built-in 'default' service and the memory
// driver; use KVLog (Redis) for multi-instance deployments.
type MemoryLog struct {
	mu      sync.Mutex
	entries []map[string]any
}

// NewMemoryLog creates an empty in-memory log sink.
func NewMemoryLog() *MemoryLog { return &MemoryLog{} }

// Log appends one entry.
func (l *MemoryLog) Log(_ context.Context, level, event string, meta map[string]any) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := map[string]any{"level": level, "event": event}
	for k, v := range meta {
		entry[k] = v
	}
	l.entries = append(l.entries, entry)
	return nil
}

// Entries returns a copy of all recorded entries (diagnostics/tests).
func (l *MemoryLog) Entries() []map[string]any {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]map[string]any, len(l.entries))
	copy(out, l.entries)
	return out
}

// ─── file log (fs driver) ───

// FileLog is a filesystem log sink implementing the starlark Logger
// capability. Entries are appended as JSON lines to <root>/formspec.log.
type FileLog struct {
	path string
	mu   sync.Mutex
}

// NewFileLog creates a file-backed log sink rooted at root (created if
// absent).
func NewFileLog(root string) (*FileLog, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("file log mkdir: %w", err)
	}
	return &FileLog{path: filepath.Join(root, "formspec.log")}, nil
}

// Log appends one JSON line entry.
func (l *FileLog) Log(_ context.Context, level, event string, meta map[string]any) error {
	entry := map[string]any{"level": level, "event": event, "ts": time.Now().UTC().Format(time.RFC3339)}
	for k, v := range meta {
		entry[k] = v
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("file log marshal: %w", err)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("file log open: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("file log write: %w", err)
	}
	return nil
}

// ─── SQL config/log (sqlite/postgres drivers) ───

// DBConfigLog serves the Config and Logger capabilities over a SQL database
// (sqlite/postgres) — a simple key-value table for config and an append-only
// table for log. It reuses the db.DB seam so no driver-specific imports are
// needed here.
type DBConfigLog struct {
	db db.DB
	pt spec.PrimitiveType
}

// NewDBConfigLog wraps a SQL database into the Config (pt=config) or Logger
// (pt=log) capability.
func NewDBConfigLog(d db.DB, pt spec.PrimitiveType) *DBConfigLog {
	return &DBConfigLog{db: d, pt: pt}
}

// Get returns the config value for key from the formspec_config table, or
// (nil, nil) when absent.
func (c *DBConfigLog) Get(ctx context.Context, key string) (any, error) {
	rows, err := c.db.QueryContext(ctx, "SELECT value FROM formspec_config WHERE key = $1", "config:"+key)
	if err != nil {
		return nil, nil // table may not exist yet — treat as miss
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	var value any
	if err := rows.Scan(&value); err != nil {
		return nil, nil
	}
	return value, nil
}

// Log appends one entry to the formspec_log table (best-effort schema).
func (c *DBConfigLog) Log(ctx context.Context, level, event string, meta map[string]any) error {
	metaJSON, _ := json.Marshal(meta)
	_, err := c.db.ExecContext(ctx,
		"INSERT INTO formspec_log (level, event, meta) VALUES ($1, $2, $3)",
		level, event, string(metaJSON))
	return err
}
