# Forma Database Layer — `internal/db`

**Category:** Implementation Documentation  
**Status:** Implemented (Fase 1.1 — 84 tests)  
**Package:** `github.com/forma/forma/internal/db`  
**Last Updated:** 2026-07-05

---

## 1. Overview

`internal/db` is the **database abstraction layer** for Forma. Semua akses data — entity CRUD, schema migration, idempotency keys, outbox events, natural key counters — melewati package ini.

### Architecture

```
┌─────────────────────────────────────────────────────┐
│                   EntityStore                       │
│            (CRUD per entity, tenant-aware)          │
├────────────┬────────────┬─────────────┬─────────────┤
│ ChildStore │ NaturalKey │ Idempotency │ OutboxStore │
│            │ Counter    │ Store       │             │
├────────────┴────────────┴─────────────┴─────────────┤
│                  DB Interface                       │
│         ExecContext / QueryContext / BeginTx        │
├──────────────────────┬──────────────────────────────┤
│   SQLite (dev)       │   PostgreSQL (production)    │
│   modernc.org/sqlite │   pgx/v5 via database/sql    │
└──────────────────────┴──────────────────────────────┘
```

### Prinsip

1. **Interface-based** — semua operasi lewat `DB` interface → mudah di-test dan di-mock
2. **Dialect-aware** — SQL yang di-generate otomatis menyesuaikan SQLite vs PostgreSQL
3. **Tenant isolation built-in** — setiap query include `WHERE tenant_id = ?`
4. **Soft delete by default** — `deleted_at IS NULL` otomatis, bisa di-disable
5. **Optimistic concurrency** — `version` column untuk CAS (Compare-And-Swap)

---

## 2. DB Interface

```go
type DB interface {
    ExecContext(ctx, query, args...) (sql.Result, error)
    QueryContext(ctx, query, args...) (*sql.Rows, error)
    QueryRowContext(ctx, query, args...) *sql.Row
    BeginTx(ctx, opts) (Tx, error)
    Close() error
    Ping(ctx) error
    DSN() string
    DriverName() string
    HasTable(ctx, schema, table) (bool, error)
    Driver() *sql.DB
}
```

Factory function:

```go
func Open(dsn string) (DB, error)
```

`Open()` otomatis parse DSN dan pilih driver yang sesuai.

---

## 3. DSN Configuration

### Format

| Skema | Contoh | Driver |
|---|---|---|
| (no scheme) | `.forma/data.db` | SQLite |
| `sqlite:` | `sqlite:data.db` | SQLite |
| `sqlite:` rel | `sqlite:./data/app.db` | SQLite |
| `sqlite:` abs | `sqlite:///abs/path/db.sqlite` | SQLite |
| `postgres:` | `postgres://user:pass@host:5432/db?sslmode=disable` | PostgreSQL |

### Default Development DSN

```go
const DefaultDevDSN = "sqlite:.forma/data.db"
```

### PostgreSQL Query Parameters

- `sslmode` — `disable`, `require`, `verify-full`
- `schema` — search path (e.g. `operational`)

### SQLite Pragmas

Semua koneksi SQLite mengaktifkan:
- `_pragma=journal_mode(WAL)` — concurrent read selama write
- `_pragma=foreign_keys(ON)` — referential integrity
- `_pragma=busy_timeout(5000)` — wait 5s jika locked
- `_pragma=cache_size(-32000)` — 32MB page cache

---

## 4. Supported Drivers

### SQLite (`sqlite_db.go`)

```go
func OpenSQLite(path string, pragmas map[string]string) (DB, error)
```

- **Library:** `modernc.org/sqlite` (pure Go, no CGo)
- **Digunakan untuk:** development, testing, embedded/single-node
- **Storage:** file-based, WAL mode
- **UUID PK:** `integer PRIMARY KEY AUTOINCREMENT`
- **Timestamp:** `text` dengan format ISO 8601
- **JSONB:** `text` (SQLite tidak punya native JSONB)
- **Generated columns:** `json_extract(data, '$.field')`

### PostgreSQL (`postgres_db.go`)

```go
func OpenPostgres(connStr string, poolSize int) (DB, error)
```

- **Library:** `pgx/v5` via `database/sql` bridge
- **Digunakan untuk:** production, multi-node
- **Connection pool:** configurable (default max 25, idle 10)
- **UUID PK:** `uuid PRIMARY KEY DEFAULT gen_uuid_v7()`
- **Timestamp:** `timestamptz`
- **JSONB:** `jsonb` (native binary JSON)
- **Generated columns:** `data->>'field'`
- **Extension:** `pgcrypto` (gen_random_uuid), auto-create `gen_uuid_v7()`

---

## 5. DDL Generation (`ddl.go`)

Entity manifest → DDL statement (dialect-aware).

### Normative Columns

| Column | SQLite | PostgreSQL |
|---|---|---|
| `id` | `integer PRIMARY KEY AUTOINCREMENT` | `uuid PRIMARY KEY DEFAULT gen_uuid_v7()` |
| `tenant_id` | `text NOT NULL` | `uuid NOT NULL` |
| `version` | `integer NOT NULL DEFAULT 1` | `bigint NOT NULL DEFAULT 1` |
| `created_at` | `text NOT NULL DEFAULT (datetime('now'))` | `timestamptz NOT NULL DEFAULT now()` |
| `updated_at` | `text NOT NULL DEFAULT (datetime('now'))` | `timestamptz NOT NULL DEFAULT now()` |
| `deleted_at` | `text` (optional) | `timestamptz` (optional) |
| `created_by` | `text` | `uuid` |
| `updated_by` | `text` | `uuid` |
| `data` | `text NOT NULL DEFAULT '{}'` | `jsonb NOT NULL DEFAULT '{}'` |

### Generated Columns

Field yang di-index (`index: true`, `unique: true`, `natural_key: true`) auto-generate kolom generated:

```sql
-- SQLite
_email text GENERATED ALWAYS AS (json_extract(data, '$.email')) STORED

-- PostgreSQL
_email text GENERATED ALWAYS AS (data->>'email') STORED
```

### Child Tables

Child dengan `storage: table` auto-generate child table:

```sql
CREATE TABLE billing_invoices__lines (
    id          integer PRIMARY KEY AUTOINCREMENT,
    parent_id   integer NOT NULL REFERENCES billing_invoices(id) ON DELETE CASCADE,
    line_number integer NOT NULL,                    -- jika SequenceField di-set
    created_at  text NOT NULL DEFAULT (datetime('now')),
    data        text NOT NULL DEFAULT '{}'
);
```

### Schema Mapping (PostgreSQL)

| Category | Schema |
|---|---|
| (default) | `operational` |
| `master` | `master` |
| `financial` | `financial` |
| `compliance` | `compliance` |
| `analytics` | `analytics` |
| `archive` | `archive` |

---

## 6. Migration Runner (`migrate.go`)

### System Tables (auto-created)

| Table | Purpose |
|---|---|
| `forma_schema_migrations` | Migration ledger — version, description, checksum, applied_at |
| `forma_natural_key_counters` | Atomic sequencer untuk natural key generation |
| `forma_idempotency_keys` | Idempotency key store (deduplikasi action) |
| `forma_outbox` | Outbox pattern untuk reliable event delivery |

### Key Functions

```go
// Pastikan 4 system tables exist
func (r *MigrationRunner) EnsureSystemTables(ctx) error

// Bandingkan desired vs actual schema
func (r *MigrationRunner) PlanMigrations(ctx, entities) ([]DDLResult, error)

// Eksekusi DDL + record migration
func (r *MigrationRunner) ApplyMigrations(ctx, entities) ([]DDLResult, error)
```

### Idempotency

Setiap DDL di-hash (SHA256). Hash disimpan di `forma_schema_migrations`. Jika hash berubah untuk entity yang sama, migration runner akan mendeteksi dan menolak (perubahan schema harus explicit migration).

---

## 7. CRUD Operations (`crud.go`)

### EntityStore

```go
store := NewEntityStore(db, driver, metadata, entitySpec)
```

| Method | Deskripsi |
|---|---|
| `Insert(ctx, params)` | Create record, return ID |
| `GetByID(ctx, params)` | Read record by ID (include hydrated children) |
| `Update(ctx, params)` | Update dengan optimistic concurrency (version check) |
| `SoftDelete(ctx, tenantID, id)` | Set `deleted_at` (atau hard delete jika soft delete disabled) |
| `List(ctx, params)` | Paginated list dengan filter, sort, search |
| `FindByField(ctx, tenantID, field, value)` | Find single by field |

### ListParams

```go
type ListParams struct {
    TenantID string
    Page     int              // 1-based
    PerPage  int              // default 20
    Sort     string           // field name, prefix - untuk DESC
    Filters  map[string]FilterOp  // eq, neq, gt, gte, lt, lte, like, in
    Search   string           // full-text search
}
```

### EntityRecord

```go
type EntityRecord struct {
    ID        string
    TenantID  string
    Version   int
    CreatedAt string
    UpdatedAt string
    CreatedBy string
    UpdatedBy string
    Data      map[string]any   // user-defined fields (JSONB)
}
```

---

## 8. Child Storage (`child.go`)

Dua mode storage untuk child fields:

### `storage: jsonb` (default)

Child data inline di parent `data` JSONB. Tidak ada child table terpisah.

```yaml
fields:
  - name: line_items
    type: child
    child:
      storage: jsonb
```

### `storage: table`

Child data di tabel terpisah (`parent__child`), dengan FK `ON DELETE CASCADE`.

```yaml
fields:
  - name: line_items
    type: child
    child:
      storage: table
      sequence_field: line_no   # optional: auto-increment per parent
```

### Key Behaviors

- **Insert:** children di-extract dari data → disimpan di child table → removed from parent JSONB
- **GetByID:** children di-hydrate dari child table → dimasukkan kembali ke `Data["child_name"]`
- **Update:** replace-all strategy (delete all + insert new)
- **Delete:** cascade delete (ON DELETE CASCADE untuk hard delete, explicit untuk soft delete)

---

## 9. Natural Key Counter (`counter.go`)

Atomic sequence generator untuk natural keys (seperti nomor invoice, nomor PO).

### Reset Strategies

| Strategy | Period Format | Contoh |
|---|---|---|
| `never` | `""` | Selalu increment |
| `yearly` | `"2026"` | Reset tiap tahun |
| `monthly` | `"2026-07"` | Reset tiap bulan |
| `daily` | `"2026-07-05"` | Reset tiap hari |

### Key Functions

```go
counter := NewNaturalKeyCounter(db, driver)

// Atomic increment, return (counter, period)
counter.NextSequence(ctx, tenantID, resource, field, scope, reset)

// Generate formatted key
counter.GenerateNaturalKey(ctx, tenantID, resource, field, scope, reset, format)

// Peek tanpa increment
counter.PeekCounter(ctx, tenantID, resource, field, scope, reset)
```

### Format Placeholders

| Placeholder | Contoh Output |
|---|---|
| `{counter}` | `42` |
| `{counter:05d}` | `00042` |
| `{period}` | `2026-07` |
| `{year}` | `2026` |
| `{month}` | `07` |
| `{day}` | `05` |
| `{resource}` | `invoice` |
| `{field}` | `number` |

---

## 10. Idempotency Store (`idempotency.go`)

Mencegah duplikasi action dengan idempotency key.

### State Machine

```
            TryClaim → key baru: [pending]
                                     ↓
            eksekusi sukses → [completed]  → replay (return cached)
            eksekusi gagal → [failed]       → retry allowed
            expired        → di-cleanup     → same as new
```

### Key Functions

```go
store := NewIdempotencyStore(db, driver).WithTTL(24 * time.Hour)

// Claim key untuk eksekusi
claimed, existing, err := store.TryClaim(ctx, tenantID, action, key)

// Record hasil
store.RecordCompleted(ctx, tenantID, action, key, responseJSON)
store.RecordFailed(ctx, tenantID, action, key, responseJSON)

// Get cached result
result := store.GetResult(ctx, tenantID, action, key)

// Cleanup expired keys
deleted := store.CleanupExpired(ctx)
```

---

## 11. Outbox Table (`outbox.go`)

Reliable event delivery via outbox pattern: event ditulis dalam 1 transaksi dengan data bisnis, lalu background worker mengirim ke event bus.

### Status Flow

```
[pending] → [delivering] → [completed]
    ↑                          |
    └── (retry with backoff) ←─┘
    ↓
[failed] (max retries exceeded)
```

### Key Functions

```go
store := NewOutboxStore(db, driver)

// Enqueue event
id := store.Enqueue(ctx, tenantID, eventName, resource, payload)

// Dequeue untuk processing
records := store.Dequeue(ctx, batchSize)

// Mark hasil
store.MarkCompleted(ctx, id)
store.MarkFailed(ctx, id, maxRetries)  // exponential backoff

// Monitoring
counts := store.CountByStatus(ctx)
recent := store.Peek(ctx, limit)

// Maintenance
deleted := store.Cleanup(ctx, olderThan)
```

### Exponential Backoff

Retry delay = `min(2^retry_count seconds, 3600s)`. Contoh:
- Retry 1: 2s
- Retry 2: 4s
- Retry 3: 8s
- ...
- Retry 10+: 3600s (1 jam)

---

## 12. Error Handling

```go
var ErrNotFound = fmt.Errorf("not found")

// Pattern: wrap error dengan entity name + operation
// Contoh: "invoice update: not found (version conflict or not found)"
```

Semua error dibungkus (`fmt.Errorf("%s operation: %w", entity, err)`) untuk konteks yang jelas.

---

## 13. Testing

```bash
go test ./internal/db/... -count=1
```

- **84 tests** (Fase 1.1 — Database Layer)
- Semua test menggunakan SQLite in-memory (`t.TempDir()`)
- Test coverage: DSN parsing, SQLite driver, DDL generation, migration runner, CRUD, child storage, natural key counter, idempotency store, outbox store

---

## 14. Usage Example

```go
package main

import (
    "context"
    "log"
    "github.com/forma/forma/internal/db"
    "github.com/forma/forma/pkg/spec"
)

func main() {
    ctx := context.Background()

    // 1. Open database
    d, err := db.Open("sqlite:./data.db")
    if err != nil {
        log.Fatal(err)
    }
    defer d.Close()

    // 2. Ensure system tables
    runner := db.NewMigrationRunner(d, db.DriverSQLite)
    if err := runner.EnsureSystemTables(ctx); err != nil {
        log.Fatal(err)
    }

    // 3. Define entity
    meta := spec.Metadata{Name: "customer", Module: "billing"}
    entity := &spec.EntitySpec{
        Fields: []spec.Field{
            {Name: "name", Type: spec.FieldString},
            {Name: "email", Type: spec.FieldString, Unique: true},
        },
    }

    // 4. Migrate schema
    runner.ApplyMigrations(ctx, []db.EntityMigration{
        {Metadata: meta, EntitySpec: *entity},
    })

    // 5. CRUD
    store := db.NewEntityStore(d, db.DriverSQLite, meta, entity)
    id, _ := store.Insert(ctx, db.InsertParams{
        TenantID: "tenant-1", CreatedBy: "admin",
        Data: map[string]any{"name": "Alice", "email": "alice@example.com"},
    })

    rec, _ := store.GetByID(ctx, db.GetByIDParams{TenantID: "tenant-1", ID: id})
    log.Printf("Created: %+v", rec.Data)

    // 6. Counter
    counter := db.NewNaturalKeyCounter(d, db.DriverSQLite)
    key, _ := counter.GenerateNaturalKey(ctx, "tenant-1", "invoice", "number",
        "", "yearly", "INV-{period}-{counter:05d}")
    log.Printf("Natural key: %s", key)
}
```

---

## 15. File Reference

| File | Lines | Purpose |
|---|---|---|
| `interface.go` | 60 | DB + Tx interfaces |
| `db.go` | 50 | Factory `Open(dsn)` |
| `config.go` | 130 | DSN parser |
| `sqlite_db.go` | 140 | SQLite driver |
| `postgres_db.go` | 110 | PostgreSQL driver |
| `ddl.go` | 350 | Entity → DDL generator |
| `migrate.go` | 260 | Schema migration runner |
| `crud.go` | 500 | CRUD query builder |
| `child.go` | 210 | Child storage (jsonb + table) |
| `counter.go` | 160 | Natural key counter |
| `idempotency.go` | 200 | Idempotency store |
| `outbox.go` | 250 | Outbox pattern |
| `*_test.go` | ~700 | 84 unit tests |
