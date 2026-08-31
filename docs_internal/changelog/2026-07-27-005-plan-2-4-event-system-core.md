# Plan 2.4 — Event System Core

**Date**: 2026-07-27  
**Referensi**: `docs/plan/todo.md` §2.4, `docs/spec/backend/01-core-basic.md` §7/§12

## Perubahan

### 2.4.1 Event Naming Enforcement
- **`pkg/spec/entity.go`**: `ValidateEventNaming()` sudah ada — validasi before_* → sync, on_* → async, custom → type wajib eksplisit

### 2.4.2 Event Priority Ordering
- **`internal/action/hooks.go`**: `SelectHooks()` sudah sorting by priority (lower=first); default 10; kelipatan 10 convention

### 2.4.3 Durability Contract Validation
- **`pkg/spec/entity.go`**: `ValidateEventDurability()` baru — jika channel `reliable_event`/`queue` tapi `publish.durable: false` → error
- Dipanggil dari `ValidateDocumentSpec()` sebagai bagian dari event validation

### 2.4.4 Outbox Worker Enhancement
- **`renderers/jsonbpersist/migrate.go`**: outbox table DDL tambah kolom `backoff` + `initial_delay_ms`
- **`renderers/jsonbpersist/outbox.go`**: 
  - `OutboxRecord` tambah field `Backoff` + `InitialDelayMs`
  - `MarkFailed()` diperbarui dengan 3 backoff strategy: exponential (default), linear, fixed
  - `initial_delay_ms` sebagai base delay untuk retry pertama
  - `enqueueOutboxWithParams()` baru untuk retry config
  - Dequeue scan diperbarui untuk kolom baru

### 2.4.5 Error Codes
- **`pkg/spec/entity.go`**: `ValidateEventNaming()` error messages include `[FORMSPEC.EVENT.TYPE_MISMATCH]` dan `[FORMSPEC.EVENT.TYPE_MISSING]`

## File yang terkena
- `pkg/spec/entity.go`
- `renderers/jsonbpersist/outbox.go`
- `renderers/jsonbpersist/migrate.go`
