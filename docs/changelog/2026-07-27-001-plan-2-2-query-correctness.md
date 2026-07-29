# Plan 2.2 — Query Correctness

**Date**: 2026-07-27  
**Referensi**: `docs/plan/todo.md` §2.2  

## Perubahan

### 2.2.1 Filter operators 13/13
- **`renderers/jsonbpersist/crud.go`**: Menambahkan 4 operator filter yang hilang di method `List`:
  - `between` — `col BETWEEN ? AND ?`, nilai adalah array dua elemen `[low, high]`
  - `ilike` — case-insensitive LIKE; PG: `col ILIKE ?`, SQLite: `LOWER(col) LIKE LOWER(?)`
  - `null` — `col IS NULL` (nilai diabaikan)
  - `notnull` — `col IS NOT NULL` (nilai diabaikan)
- **`internal/api/handler.go`**: Menambahkan `between`, `ilike`, `null`, `notnull` ke `filterOps`; `between` value parsing sebagai comma-separated pair

### 2.2.2 JSONB path fallback
- **`renderers/jsonbpersist/crud.go`**: Method baru `EntityStore.columnRefExpr()` — fallback ke `data->>'field'` (PG) atau `json_extract(data, '$.field')` (SQLite) jika field tidak punya generated column; dipakai di `List` dan `Sort`
- **`internal/api/handler.go`**: `checkField` sekarang mengizinkan filtering pada semua field entity (tidak hanya yang `index: true`)

### 2.2.3 Generated column dialect-aware
- **`renderers/jsonbpersist/ddl.go`**: `generateGeneratedColumn` sekarang menerima `DriverType`; PG: `data->>'field'`, SQLite: `json_extract(data, '$.field')`; kedua caller di-update

### 2.2.4 `exists:<resource>` real lookup
- Tidak ada perubahan — sudah di-wire di `resource/forma.go` via `SetEntityLookup` yang memanggil entity registry; diverifikasi berfungsi

### 2.2.5 Cross-module relation resolution
- **`renderers/jsonbpersist/crud.go`**: `ValidateRelationTargets` sekarang parse `f.Relation.Resource` sebagai `{module}.{entity}` atau `{entity}` (same module); menggunakan `targetTableResolver` jika tersedia
- **`internal/entity/registry.go`**: `GetEntityStore` meng-inject `targetTableResolver` yang lookup entity spec di registry untuk mendapatkan table name sebenarnya (menggunakan `Plural` dari spec, bukan naive `+s`)

## File yang terkena
- `renderers/jsonbpersist/crud.go` — FilterOp, columnRefExpr, List filter switch, ValidateRelationTargets
- `renderers/jsonbpersist/ddl.go` — generateGeneratedColumn dialect-aware
- `internal/api/handler.go` — filterOps + checkField + between parsing
- `internal/entity/registry.go` — table resolver wiring
