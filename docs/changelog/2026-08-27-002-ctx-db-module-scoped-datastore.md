# 2026-08-27-002 — Fase 2.9.4: ctx.db() Module-Scoped Datastore

## Apa yang diubah

Implementasi `ctx.db()` module-scoped (todo 2.9.4, `docs/spec/platform/06-datastore.md` §1.1)
— penutup terakhir Fase 2 (Engine Core).

**`resource/datastoreregistry.go` (baru):**

- `DatastoreRegistry` — load `kind: Datastore` manifests + binding `spec.datastore`
  dari `kind: Module`; `'default'` tetap di-backing database utama app + shared
  in-memory backends (perilaku 2.9.3 dipertahankan).
- Resolusi per-module: plain `ctx.db()` → datastore milik module (fallback
  `'default'` bila datastore terikat tidak melayani primitive tsb, mis. binding
  db-only lalu `ctx.cache()`); `.named(x)` hanya sah untuk binding module sendiri —
  selain itu error jelas yang mengutip §1.1 (termasuk `.named("default")` dari
  module terikat = escape hatch yang ditolak).
- Driver single-server: `sqlite` (file per datastore di `<stateDir>/datastores/`),
  `postgres`, `memory`, `fs`; driver cloud (valkey/redis/s3/minio/nats) gagal
  dengan pesan jelas di resolve time.
- Validasi boot: binding ke datastore tak dikenal + driver×serves mismatch (§2) → error.

**Threading module ke resolver:**

- Signature resolver berubah menjadi `(primitiveType, name, module string)` di
  `internal/starlark/context.go` (`CtxAPI.SetDatastoreResolver` + `SetModule`),
  `internal/starlark/executor.go` (`Execute` memanggil `ctxObj.SetModule(module)`),
  dan `internal/action/script.go` (forwarding).
- `internal/starlark/primitive.go`: plain call meneruskan nama kosong (bukan
  `"default"`) agar registry bisa menerapkan binding; `Attr("named")` kini
  me-resolve langsung ke connection runner sehingga rantai yang benar adalah
  `ctx.db.named("x").query(...)` (sebelumnya `ctx.db().named("x")` menghasilkan
  handle tanpa `.query` — bug laten yang ikut diperbaiki).

**Wiring:**

- `resource/formspec.go` — registry dibangun di `New()` dan `ReloadSpec()`
  (`buildDatastoreRegistry`) lalu di-wire ke dispatcher via `resolverFromRegistry`.
- `cmd/formspec/check.go` — `checkDatastores`: validasi referensi binding +
  kompatibilitas driver×serves saat deploy-time analysis.
- `cmd/formspec/repl.go` / `migrate.go` — adaptasi signature (tanpa manifests,
  semua module → 'default').

**Test:** unit `resource/datastoreregistry_test.go` (6 test: binding, isolasi file,
blokir lintas-datastore, fallback primitive, load errors, unsupported driver) +
e2e `resource/ctx_db_module_scoped_e2e_test.go` (2 module × 2 datastore via HTTP:
tulis masuk DB yang benar, tidak ada kebocoran tabel antar DB; escape hatch
`.named("default")` ditolak dengan error §1.1). Suite penuh: **872 pass, 0 fail**
(sebelumnya 864).

## Kenapa diubah

Kontrak normatif §1.1: pemilihan datastore ada di level Module, bukan pilihan
bebas per kode; interaksi lintas-Module-lintas-Datastore wajib lewat event/outbox
tanpa escape hatch `ctx.db` sekalipun dengan `uses` consent. Ini menutup item
terakhir Fase 2 di todo master.

## File terdampak

- `resource/datastoreregistry.go` (baru), `datastoreregistry_test.go` (baru),
  `ctx_db_module_scoped_e2e_test.go` (baru), `ctxresolver.go`, `formspec.go`
- `internal/starlark/{context,executor,primitive,primitive_test}.go`
- `internal/action/script.go`, `cmd/formspec/{check,repl,migrate}.go`
- `docs/plan/todo.md`

## Referensi

- Plan: `docs/plan/todo.md` §2.9.4 · Spec: `docs/spec/platform/06-datastore.md` §1.1/§2
