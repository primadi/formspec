# 2026-08-17-005 — Wire ctx.* datastore resolver + real ctx.db().query()

**Apa:** Menuntaskan Fase 2.9.1 — `ctx.db()` (dan primitif `ctx.*` lain) kini
me-resolve ke koneksi nyata, dan `ctx.db().query()` benar-benar jalan terhadap
database utama app. Sebelumnya semua `ctx.db()/cache()/lock()/...` error
`"datastore resolver not configured"` karena `CtxAPI.SetDatastoreResolver`
tidak pernah di-wire, dan operasi `primitiveRunner` masih stub
(`"not yet implemented"`). Referensi: `docs/plan/ctx-datastore-resolver.md`.

## Perubahan

1. **`internal/starlark/primitive.go`** — definisikan capability interfaces
   (`Querier`, `KVGetter`, `KVSetter`, `KVDeleter`, `Locker`); implementasikan
   operasi `query/get/set/delete/acquire/release` memakai interface tsb
   (bukan stub). Go context diambil dari thread-local (`threadContext`).
2. **`internal/starlark/executor.go`** — `ScriptExecutor` + field
   `DatastoreResolver` + `SetDatastoreResolver`; `ExecuteScript` menerima
   `ctx context.Context` dan menyimpannya di thread (`SetLocal`); `Execute`
   memanggil `ctxObj.SetDatastoreResolver`.
3. **`internal/action/script.go`** — `SetDatastoreResolver` forwarding ke engine.
4. **`renderers/jsonb-persist/datastore/querier.go`** — adapter `DBQuerier`
   (bungkus `db.DB` jadi `Querier`; `Query` → `[]map[string]any`).
5. **`resource/formspec.go`** — `newDispatcher` menerima `database db.DB`;
   wire resolver: `(db, "default")` → `DBQuerier`; primitif lain / named
   datastore → error jelas ("no live datastore ... only db/default is wired").
6. **`internal/starlark/evaluator.go`** — `toStarlark` handle `[]map[string]any`
   (hasil query) jadi list of dict, bukan string.

## Test

- `internal/starlark/primitive_test.go` — resolver di-wire + query dieksekusi;
  perilaku no-resolver dipertahankan; backend non-Querier → error jelas.
- `renderers/jsonb-persist/datastore/querier_test.go` — `DBQuerier` terhadap
  SQLite nyata (query + error propagation).
- `resource/ctx_db_e2e_test.go` — e2e via HTTP: action script `ctx.db().query`
  mengembalikan baris dari database utama app.

`go test ./...` kini **577 pass, 0 fail** (naik dari 571).

## File terdampak

- `internal/starlark/{primitive,executor,evaluator}.go` + test
- `internal/action/script.go`
- `renderers/jsonb-persist/datastore/{querier.go,querier_test.go}`
- `resource/{formspec.go,ctx_db_e2e_test.go}`
- `docs/plan/{todo.md,ctx-datastore-resolver.md}`

## Sisa (2.9.2–2.9.4)

Primitif lain (`cache/lock/queue/pubsub/storage/kvstore`) + named datastore
masih error jelas (bukan "not configured"); `datastore.Open()` untuk driver
lain (Postgres/Valkey/dst.) dan dev auto-provision `'default'` per primitive
menyusul.
