package api

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// ─── Entity read-through cache (Fase 14, docs/plan/fase14-entity-cache.md) ───
//
// Opt-in per entity via spec.cache.ttl. Scope: find-by-id ONLY — list
// endpoints are never cached (arbitrary filter combinations make
// invalidation intractable). Cached entries hold the RAW EntityRecord
// (pre-sanitize): field-security sanitization stays per-request. Write
// paths always hit the DB (CAS version check) and invalidate the key.

// CacheKV is the minimal backend contract for the entity cache — the same
// shape as the starlark primitive KV interfaces, so memory.KV and
// rediskv.KV plug in directly.
type CacheKV interface {
	Get(ctx context.Context, key string) (any, error)
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

// CacheKey builds the cache key for one entity record. Workspace is part of
// the key — cross-tenant 404 semantics are preserved.
func CacheKey(workspaceID, module, entity, id string) string {
	return workspaceID + ":" + module + ":" + entity + ":id:" + id
}

// EntityCache is the read-through cache for one HandlerFactory. nil backend
// (or nil resolver result) = caching disabled for that entity.
type EntityCache struct {
	// Resolve returns the KV backend for (module, entity), or nil when the
	// entity is not cacheable (no spec.cache) or no backend is available.
	Resolve func(module, entity string) CacheKV
	// TTLFor returns the configured TTL for (module, entity); called only
	// when Resolve returns a backend.
	TTLFor func(module, entity string) time.Duration
}

// cacheEntry is the JSON shape stored in the cache. db.EntityRecord has a
// custom flat MarshalJSON (data merged into the top level) with no matching
// UnmarshalJSON — encoding it directly would not round-trip. This plain
// struct captures the record losslessly.
type cacheEntry struct {
	ID          string         `json:"id"`
	WorkspaceID string         `json:"workspace_id"`
	Version     int            `json:"version"`
	CreatedAt   string         `json:"created_at"`
	UpdatedAt   string         `json:"updated_at"`
	CreatedBy   string         `json:"created_by"`
	UpdatedBy   string         `json:"updated_by"`
	DocStatus   string         `json:"doc_status,omitempty"`
	Data        map[string]any `json:"data"`
}

func toCacheEntry(rec *db.EntityRecord) *cacheEntry {
	return &cacheEntry{
		ID: rec.ID, WorkspaceID: rec.WorkspaceID, Version: rec.Version,
		CreatedAt: rec.CreatedAt, UpdatedAt: rec.UpdatedAt,
		CreatedBy: rec.CreatedBy, UpdatedBy: rec.UpdatedBy,
		DocStatus: rec.DocStatus, Data: rec.Data,
	}
}

func fromCacheEntry(e *cacheEntry) *db.EntityRecord {
	return &db.EntityRecord{
		ID: e.ID, WorkspaceID: e.WorkspaceID, Version: e.Version,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
		CreatedBy: e.CreatedBy, UpdatedBy: e.UpdatedBy,
		DocStatus: e.DocStatus, Data: e.Data,
	}
}

// GetRecord returns the cached record for key, or (nil, nil) on miss.
// Corrupted entries are treated as a miss (and deleted).
func (c *EntityCache) GetRecord(ctx context.Context, backend CacheKV, key string) (*db.EntityRecord, error) {
	v, err := backend.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("cache get: %w", err)
	}
	if v == nil {
		return nil, nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, nil // non-JSON payload — treat as miss
	}
	var e cacheEntry
	if err := json.Unmarshal(raw, &e); err != nil || e.ID == "" || e.Data == nil {
		_ = backend.Delete(ctx, key) // corrupted — self-heal
		return nil, nil
	}
	return fromCacheEntry(&e), nil
}

// SetRecord stores the record under key with the entity's TTL.
func (c *EntityCache) SetRecord(ctx context.Context, backend CacheKV, key string, rec *db.EntityRecord, ttl time.Duration) error {
	if rec == nil || rec.ID == "" {
		return nil
	}
	return backend.Set(ctx, key, toCacheEntry(rec), ttl)
}

// Invalidate deletes the record's cache key. Errors are returned for
// logging by the caller but never block the write path.
func (c *EntityCache) Invalidate(ctx context.Context, backend CacheKV, key string) error {
	return backend.Delete(ctx, key)
}

// CacheInvalidator is an optional backend extension for multi-instance
// invalidation (Fase 14 v2): the mutator deletes locally AND broadcasts the
// key; every instance subscribed deletes its copy (rediskv implements this
// via Redis pub/sub). Backends without it fall back to local-only delete —
// staleness on other instances is then bounded by the TTL.
type CacheInvalidator interface {
	BroadcastInvalidate(ctx context.Context, key string) error
}

// invalidateEntityCache deletes the read-through cache entry for one record
// (Fase 14). Best-effort: a cache error never blocks the write path. When
// the backend supports broadcast (Redis), other instances invalidate too.
func (f *HandlerFactory) invalidateEntityCache(ctx context.Context, workspaceID, module, entity, id string) {
	if f.entityCache == nil || f.entityCache.Resolve == nil {
		return
	}
	backend := f.entityCache.Resolve(module, entity)
	if backend == nil {
		return
	}
	key := CacheKey(workspaceID, module, entity, id)
	if bi, ok := backend.(CacheInvalidator); ok {
		_ = bi.BroadcastInvalidate(ctx, key)
		return
	}
	_ = f.entityCache.Invalidate(ctx, backend, key)
}
