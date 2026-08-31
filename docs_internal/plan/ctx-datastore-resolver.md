# Plan — Fase 2.9.1: Wire `ctx.*` datastore resolver + real `ctx.db().query()`

**Tanggal**: 2026-08-17 · **Status**: Complete (2026-08-17 — `ctx.db().query()` jalan; 2.9.2–2.9.4 menyusul)
**Referensi**: `docs/plan/todo.md` (2.9.1), `docs/runtimes/02-formspec-resource.md` §7,
`docs/runtimes/04-formspec-sidecar.md` §8, `docs/spec/platform/06-datastore.md` §2

## Tujuan

Saat ini semua `ctx.db()/cache()/lock()/...` dari Starlark error
`"datastore resolver not configured"` karena `CtxAPI.SetDatastoreResolver`
tidak pernah di-wire dari binary manapun, dan `primitiveRunner` di
`internal/starlark/primitive.go` masih stub (`"not yet implemented"`).

Increment ini membuat **`ctx.db().query()` benar-benar jalan** terhadap
database utama app (SQLite di dev, Postgres di prod), dan me-wire resolver
sehingga primitif lain me-resolve ke koneksi (bukan "not configured") —
operasi yang belum ada backend-nya mengembalikan error yang jelas
("not yet implemented for this backend"), bukan "not configured".

Ini fondasi Fase 7 (Service, Hook, Validation L6, sidecar proxy).

## Perubahan

| File                                           | Perubahan                                                                                                                                                                                                              |
| ---------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `internal/starlark/primitive.go`               | Definisikan capability interfaces (`Querier`, `KVGetter`, `KVSetter`, `KVDeleter`, `Locker`); implementasikan operasi `query/get/set/delete/acquire/release` memakai interface tsb; ambil Go context dari thread-local |
| `internal/starlark/executor.go`                | `ScriptExecutor` + field `datastoreResolver` + `SetDatastoreResolver`; `ExecuteScript` menerima `ctx context.Context` dan menyimpannya di thread (`SetLocal`); `Execute` memanggil `ctxObj.SetDatastoreResolver`       |
| `internal/action/script.go`                    | `SetDatastoreResolver` forwarding ke engine                                                                                                                                                                            |
| `renderers/jsonb-persist/datastore/querier.go` | Adapter `dbQuerier` — bungkus `db.DB` jadi `Querier` (Query → `[]map[string]any`)                                                                                                                                      |
| `resource/formspec.go`                         | `newDispatcher` menerima `database db.DB`; wire resolver: `(db, "default")` → `dbQuerier`; primitif lain → error jelas                                                                                                 |
| `cmd/formspec/dev.go`                          | Wire resolver yang sama ke ScriptExecutor (dev mode)                                                                                                                                                                   |
| `internal/starlark/primitive_test.go`          | Test `ctx.db().query()` terhadap SQLite in-memory                                                                                                                                                                      |

## Keputusan

- Capability interfaces didefinisikan di `internal/starlark` (kontrak primitif
  kanonik — sidecar doc bilang wire contract "mirrors internal/starlark's
  primitive contract"). Go interface struktural → satu adapter bisa memenuhi
  interface sidecar & starlark sekaligus.
- Go context di-thread lewat `starlark.Thread.SetLocal` (thread-local), karena
  `primitiveRunner` tidak punya akses langsung ke `context.Context` dari
  pemanggil.
- `db` primitive resolve ke database utama app (bukan lewat `datastore.Registry`
  yang didesain untuk snapshot Control Plane). `datastore.Registry` tetap untuk
  named datastore / multi-datastore (2.9.2–2.9.4).

## Verifikasi

- `go test ./...` hijau (571+ pass).
- Test baru: script `ctx.db().query("SELECT 1")` mengembalikan hasil.
- `make build` hijau.
