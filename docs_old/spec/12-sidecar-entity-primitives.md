# Sidecar Entity Primitives — Protocol Extension

**Version:** 0.1.0 (draft)
**Status:** Draft
**Applies to:** `docs/runtimes/04-forma-sidecar.md` §4.3

---

## 1. Rationale

The original sidecar protocol (`docs/runtimes/04-forma-sidecar.md`) defines 7
primitive types: `db`, `cache`, `lock`, `queue`, `pubsub`, `storage`, `kvstore`.
These provide infrastructure-level access but **no entity-level operations**.

Starlark scripts have `resource.fetch()` and `resource.save()` — cross-entity
CRUD that the script executor handles via callbacks into the entity engine.
Sidecar handlers need the equivalent.

**The entity primitive** (`/ctx/entity/{op}`) fills this gap. It exposes 5
operations:

| Operation | Semantics | Atomic? |
|---|---|---|
| `get` | Fetch full record by ID | — |
| `set` | Replace entire data | ✅ Single SQL |
| `update` | Merge specific fields (`jsonb_patch`) | ✅ Single SQL |
| `increment` | `field = field + amount` | ✅ Single SQL |
| `decrement` | `field = field - amount` (with `>=` guard) | ✅ Single SQL |

`update`, `increment`, `decrement` are **atomic** — single SQL statement,
no read-modify-write race condition.

---

## 2. Wire Protocol

All operations are `POST /ctx/entity/{operation}`.

### 2.1 `entity/get` — Fetch Record

```
POST /ctx/entity/get
Content-Type: application/json

{ "named": "pharmacy/medicine", "key": "0189abcd-..." }
```

```json
200 OK
{ "data": { "id": "0189abcd-...", "stock": 100, "name": "Vitamin C", ... } }
```

- `named`: `"{module}/{entity}"` — resolved via `app.GetEntityStore(module, name)`
- `key`: record UUID

### 2.2 `entity/set` — Full Replace

```
POST /ctx/entity/set
Content-Type: application/json

{
  "named": "pharmacy/medicine",
  "key": "0189abcd-...",
  "value": { "stock": 96, "name": "Vitamin C 500mg", ... }
}
```

```json
200 OK
{ "ok": true }
```

Replaces the entire `data` JSONB column. Use `update` for partial updates.

### 2.3 `entity/update` — Atomic Field Merge

```
POST /ctx/entity/update
Content-Type: application/json

{
  "named": "pharmacy/medicine",
  "key": "0189abcd-...",
  "fields": { "stock": 96 }
}
```

```json
200 OK
{ "ok": true }
```

**SQL (PostgreSQL):**
```sql
UPDATE pharmacy_medicines
SET data = data || '{"stock":96}'::jsonb,
    version = version + 1,
    updated_at = now()
WHERE id = ? AND tenant_id = ?
```

**SQL (SQLite):**
```sql
UPDATE pharmacy_medicines
SET data = json_patch(data, '{"stock":96}'),
    version = version + 1,
    updated_at = (datetime('now'))
WHERE id = ? AND tenant_id = ?
```

Only the specified fields are changed; other fields are untouched. Version is
incremented automatically.

### 2.4 `entity/increment` — Atomic Increment

```
POST /ctx/entity/increment
Content-Type: application/json

{
  "named": "pharmacy/medicine",
  "key": "0189abcd-...",
  "field": "stock",
  "amount": 5
}
```

```json
200 OK
{ "ok": true }
```

**SQL (PostgreSQL):**
```sql
UPDATE pharmacy_medicines
SET data = jsonb_set(data, '{stock}',
        to_jsonb(COALESCE((data->>'stock')::numeric, 0) + 5)),
    version = version + 1,
    updated_at = now()
WHERE id = ? AND tenant_id = ?
```

### 2.5 `entity/decrement` — Atomic Decrement with Guard

```
POST /ctx/entity/decrement
Content-Type: application/json

{
  "named": "pharmacy/medicine",
  "key": "0189abcd-...",
  "field": "stock",
  "amount": 4
}
```

```json
200 OK
{ "data": 96, "ok": true }
```

Returns the **new field value** after decrement.

**SQL (PostgreSQL):**
```sql
UPDATE pharmacy_medicines
SET data = jsonb_set(data, '{stock}',
        to_jsonb(COALESCE((data->>'stock')::numeric, 0) - 4)),
    version = version + 1,
    updated_at = now()
WHERE id = ? AND tenant_id = ?
  AND COALESCE((data->>'stock')::numeric, 0) >= 4
RETURNING COALESCE((data->>'stock')::numeric, 0)
```

The `WHERE ... >= amount` guard prevents negative values. Returns error with
`not found` if guard fails (caller should treat as "insufficient balance").

---

## 3. Generated Columns

Indexed/unique fields create `STORED GENERATED` columns:
```sql
_stock bigint GENERATED ALWAYS AS (json_extract(data, '$.stock')) STORED
```

Atomic operations modify the `data` column via `jsonb_set`/`json_set`/`||`.
The database **automatically recomputes** stored generated columns when their
source column changes — no manual sync needed.

---

## 4. SDK Usage by Language

Semua SDK (`sdk/typescript`, `sdk/dotnet`, `sdk/java`, `sdk/php`, `sdk/python`,
`sdk/ruby`) memiliki method yang sama pada `CtxPrimitive` setelah dipanggil
via `ctx.entity().named("module/entity")`:

| Method | HTTP Call | Entity operation |
|---|---|---|
| `get(key)` | `POST /ctx/entity/get` | Fetch full record |
| `set(key, value)` | `POST /ctx/entity/set` | Full data replace |
| `update(id, fields)` | `POST /ctx/entity/update` | Atomic field merge |
| `increment(id, field, amount)` | `POST /ctx/entity/increment` | Atomic increment |
| `decrement(id, field, amount)` | `POST /ctx/entity/decrement` | Atomic decrement with guard |

Semua method menerima parameter `tenantId` (optional) untuk scoping.

### TypeScript

```typescript
import { App } from "@forma/lib-forma";

const app = new App();

app.handle("pharmacy.otc-sale.sell", async (inv, ctx) => {
  // Atomic decrement — single SQL, no race condition
  await ctx
    .entity()
    .named("pharmacy/medicine")
    .decrement(item.medicine_id, "stock", item.quantity);

  // Atomic field merge
  await ctx
    .entity()
    .named("pharmacy/medicine")
    .update(item.medicine_id, { price: 1500 });

  // Full record fetch
  const med = await ctx
    .entity()
    .named("pharmacy/medicine")
    .get(item.medicine_id);

  return new ActionResult({ total: 5000 }, "completed");
});

app.run();
```

### SDK Method Reference

| Method | HTTP Call | Description |
|---|---|---|
| `get(key)` | `POST /ctx/entity/get` | Fetch full record |
| `set(key, value)` | `POST /ctx/entity/set` | Full data replace |
| `update(id, fields)` | `POST /ctx/entity/update` | Atomic field merge |
| `increment(id, field, amount)` | `POST /ctx/entity/increment` | Atomic increment |
| `decrement(id, field, amount)` | `POST /ctx/entity/decrement` | Atomic decrement with guard |

---

## 5. Best Practices

1. **Prefer `decrement` over `get`+modify+`set`** for counter/numeric fields —
   eliminates TOCTOU race conditions entirely.

2. **Prefer `update` over `set`** for partial updates — `set` replaces the
   entire data blob; `update` merges only the specified fields.

3. **Generated columns** (`_field`) are auto-maintained — no extra work needed.

4. **Version conflict**: EntityStore.IncrementField and DecrementField do NOT
   check version (they're field-level operations, not full-record updates).
   The version is still incremented automatically. If you need optimistic
   concurrency at the record level, use `get` → compare → `set` with CAS.

5. **Transaction boundary**: Each operation is an independent SQL statement.
   Multi-step handlers (e.g., decrement stock + update invoice + publish
   event) are NOT wrapped in a single database transaction. This is a
   known gap — see §6.

---

## 6. Known Gaps

| Gap | Severity | Status |
|---|---|---|
| No transaction wrapping across multiple entity operations | High | Deferred to separate plan |
| Decrement guard error message is generic ("not found") | Low | Acceptable for MVP |
| SQLite's `json_patch` requires SQLite ≥ 3.45.1 | Medium | Dev uses SQLite; prod uses PostgreSQL |
