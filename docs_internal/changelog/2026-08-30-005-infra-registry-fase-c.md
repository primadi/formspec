# 2026-08-30-005 — Infra Registry fase C: named logical primitive resmi via .named()

**Plan:** `docs/plan/infra-registry-3-level.md` (fase C)

## Apa yang diubah

`.named()` berubah status dari "dibatasi binding module" (spec §1.1 lama)
menjadi **fitur resmi App Registry** — jalur sanksi untuk mengakses named
logical primitive:

- `resource/datastoreregistry.go`: `ResolveNamed(primitiveType, alias,
module)` — resolve alias dari `appNamed` (key `primitive/alias` di
  `AppSpec.Datastores` atau `ModuleSpec.Datastores` module pemiliknya,
  di-merge ke App yang menaunginya). Alias bersifat **app-scoped**: dua App
  boleh punya alias `analytics` yang menunjuk service berbeda. Unknown alias
  → `DATASTORE_NOT_FOUND` (spec §6). Module-level named key kini diterima
  (fase B menolaknya) dengan syarat module di-mount oleh `kind: App`.
- `internal/starlark/primitive.go`: `.named(alias)` kini mengirim prefix
  `named:` melalui resolver yang sama.
- `internal/starlark/context.go`: wrapper resolver me-route prefix `named:`
  ke `ResolveNamed` dengan gate baru `checkDatastoreAlias` — akses
  `ctx.db.named("analytics")` wajib dideklarasikan sebagai key
  `db/analytics` di `uses.datastores`, pelanggaran → `DATASTORE_ACCESS_DENIED`.
  `checkDatastoreAccess` (gate base primitive) kini mengizinkan primitive
  bila action mendeklarasikan named key-nya.
- `internal/starlark/executor.go` + `internal/action/script.go` +
  `resource/formspec.go`: wiring `SetDatastoreResolverNamed(dsReg)` dari
  boot hingga CtxAPI.

## Kenapa

Use case masa depan yang butuh akses ke named logical primitive (analytics
DB, reporting DB, cache terpisah per tier) kini punya jalur eksplisit yang
ter-registrasi, ter-gate `uses.datastores`, dan gagal jelas (`DATASTORE_NOT_FOUND`)
saat alias tidak dikenal — bukan silent write ke datastore salah.

## File terdampak

- `resource/datastoreregistry.go` — `ResolveNamed`, module named key merge
- `internal/starlark/primitive.go` — prefix `named:` di `.named()`
- `internal/starlark/context.go` — `checkDatastoreAlias`,
  `checkDatastoreAccess` named-aware, `SetDatastoreResolverNamed`,
  `resolveNamed`
- `internal/starlark/executor.go`, `internal/action/script.go`,
  `resource/formspec.go` — wiring
- Test: `resource/datastoreregistry_test.go` (+2: ResolveNamed,
  ModuleNamedKey; update LoadErrors), `internal/starlark/primitive_test.go`
  (+1: CtxNamedDatastore), `resource/ctx_db_module_scoped_e2e_test.go`
  (kontrak `.named("default")` → `DATASTORE_NOT_FOUND`)

Verifikasi: 977 test Go lulus tanpa regresi.
