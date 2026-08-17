# Plan — Idempotency Prepare Flow (todo 2.7)

**Status**: In Progress (2026-08-17) · **Referensi**: `docs/plan/todo.md` 2.7,
`docs/spec/backend/01-core-basic.md` §5, `docs/renderers/jsonb-persist/04-query-and-keys.md` §3

## Masalah

- `IdempotencyStore` (`renderers/jsonb-persist/idempotency.go`) sudah lengkap di
  layer store (TryClaim/RecordCompleted/RecordFailed/GetResult/CleanupExpired)
  - test, dan di-expose `App.Idempotency()` — **tapi belum dipakai di jalur
    HTTP**. Todo 2.7.1 (endpoint prepare) dan enforcement idempotency di action
    handler belum ada; docs `04-query-and-keys.md` §3 eksplisit menyebut ini gap.
- Frontend (`renderers/react-shadcn/src/lib/api/client.ts`) sudah mengirim
  header `Idempotency-Key` via `apiPost(..., {idempotencyKey})` — tapi server
  mengabaikannya.

## Kontrak (01-core-basic §5)

- `idempotent: true` mensyaratkan sumber `idempotency_key` (`header` | `param` | `server`).
- Sumber `server` → alur **prepare dua-langkah**:
  `POST /{resource}/{action}/prepare` → return key → panggil action dengan key terlampir.
- Store `(tenant, action, key) → pending | completed | failed + response`.
- Duplikat setelah `completed` → replay response asli.
- Duplikat saat masih `pending` (in-flight) → `409 CONFLICT`.
- Setelah `failed` → retry diizinkan (previous attempt tidak lengkap).
- Entry kedaluwarsa via TTL (default 24 jam), di-cleanup `CleanupExpired`.

## Desain

### 1. Wire store ke HandlerFactory

- `HandlerFactory.SetIdempotencyStore(store *db.IdempotencyStore)`.
- `RouterBuilder.SetIdempotencyStore(store)` → forward ke factory.
- `resource/formspec.go`: panggil `rb.SetIdempotencyStore(idempotencyStore)`
  di **kedua** jalur — `New()` dan `ReloadSpec()`.

### 2. Helper resolusi key

`resolveIdempotencyKey(r, actionSpec) string`:

- `from: header` → `Idempotency-Key` header.
- `from: server` → key hasil prepare dikirim balik via `Idempotency-Key` header.
- `from: param` → query param `field` (fallback `idempotency_key`).
- Action tidak idempotent / tidak ada decl → `""` (skip enforcement).

### 3. Enforcement di action handler

Helper `beginIdempotent(ctx, w, ws, actionName, actionSpec, r) (key string, proceed bool, store *db.IdempotencyStore)`:

1. `!actionSpec.Idempotent || f.idempotency == nil` → proceed tanpa key.
2. Resolve key; kosong untuk action idempotent → `422 VALIDATION_ERROR`.
3. `store.TryClaim(ws, actionName, key)`:
   - `claimed=false`, existing `completed` → **replay**: tulis stored response
     (status + body asli), `proceed=false`.
   - `claimed=true`, existing `pending` → **409 CONFLICT** (in-flight), `proceed=false`.
   - `claimed=true`, existing `failed` → izinkan retry, `proceed=true` (key di-set).
   - `claimed=true`, existing nil (key baru / expired reset) → `proceed=true`.

Setelah eksekusi sukses: `RecordCompleted(ws, action, key, envelope)` di mana
envelope = `{"status":<http status>,"body":<json body>}` agar replay setia
(preserve status + body). Gagal: `RecordFailed` dengan envelope error.

Handler yang memakai enforcement: **`HandleCreate`** (kasus utama spec:
double-submit browser) dan **`HandleCustomAction`**.

### 4. Endpoint prepare (2.7.1)

`HandlePrepare(module, entity, actionName, actionSpec)`:

- Hanya untuk action `idempotent: true` + `idempotency_key.from == "server"`.
- Generate UUID v7 (`db.NewUUIDv7`), return
  `{data: {idempotency_key: "<key>"}}` (status 200).
- Tidak pre-claim — klaim terjadi saat action dipanggil.

Route (kedua surface):

- External: `POST /api/v1/{module}/{plural}/create/prepare` (untuk create
  yang idempotent server) dan `POST /api/v1/{module}/{plural}/{action}/prepare`
  (untuk custom action idempotent server).
- UI: `/_ui/entity/{module}/{entity}/create/prepare` dan
  `/_ui/entity/{module}/{entity}/{action}/prepare`.

Registrasi: pattern prepare memakai segmen statis `{action}/prepare`, sehingga
tidak bertabrakan dengan route custom `{id}/{action}` — chi prioritaskan node
statis di atas param.

### 5. Test

- `internal/api/idempotency_test.go`:
  - prepare create → dapat key; panggil create 2x dengan key sama →
    response kedua = replay (data sama, tidak ada duplikat row).
  - panggil create idempotent tanpa key → 422.
  - in-flight pending → 409 (uji lewat TryClaim manual + handler).
  - custom action idempotent → replay.
- Pastikan `go test ./...` hijau.

## File terdampak

| File                                                | Perubahan                                                                                          |
| --------------------------------------------------- | -------------------------------------------------------------------------------------------------- |
| `internal/api/handler.go`                           | `SetIdempotencyStore`, `HandlePrepare`, enforcement di `HandleCreate`/`HandleCustomAction`, helper |
| `internal/api/router.go`                            | forward `SetIdempotencyStore`, registrasi handler `prepare`                                        |
| `internal/api/generator.go`                         | generate route prepare (external + UI)                                                             |
| `internal/api/descriptor.go`                        | (mungkin) konstanta handler `"prepare"`                                                            |
| `resource/formspec.go`                              | wire store di `New()` + `ReloadSpec()`                                                             |
| `internal/api/idempotency_test.go`                  | test baru                                                                                          |
| `docs/renderers/jsonb-persist/04-query-and-keys.md` | update §3 — jalur HTTP sudah ada                                                                   |
| `docs/plan/todo.md`                                 | tandai 2.7.1 + 2.7.2 selesai; 2.8 selesai (sudah ter-implement)                                    |

## Level of effort

Medium (~2 file besar + 1 file test). Self-contained, tidak ada blocker.
