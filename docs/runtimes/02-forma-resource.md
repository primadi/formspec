# Forma Resource — Library Reference

**Version:** 1.0
**Status:** Draft
**License:** Creative Commons CC0 (dokumen) — kode-nya sendiri FSL (open source)
**Governed by:** `docs/architecture/01-architecture-overview.md` §2, §4.3, `docs/spec/02-core-basic.md`, `docs/spec/03-core-extended.md`

> `forma-resource` **bukan binary** — ia adalah **Go library** (`import "github.com/primadi/forma/resource"`) yang di-compile menjadi satu proses dengan aplikasi Go. Untuk app non-Go, engine yang sama di-embed ke dalam proses `forma-sidecar` (lihat `04-forma-sidecar.md`). Dokumen ini menjelaskan fitur, desain internal, dan permukaan API (`ctx.*` serta REST API yang dihasilkan) dari library ini.

---

## 1. Peran

Forma Resource adalah **mesin bisnis** yang dikompilasi ke dalam app: entity engine, penegakan permission, generator REST API, dan (target) admin panel. Ia:

- **Tidak membaca YAML dari filesystem di production** — memuat manifest yang sudah diverifikasi dari artifact yang di-pull lewat plane protocol (lihat `01-forma-ctl.md` §5)
- Menyediakan `ctx.*` primitives untuk script Starlark (`ctx.db`, `ctx.lock`, `ctx.pubsub`, dst)
- Meng-generate route REST CRUD + custom action dari spec `Document`/`Entity`
- Menegakkan permission model deny-by-default (D20/D49 — lihat `docs/spec/04-control-plane.md`)

Dua binary embed engine yang sama: **app Go native** (`import` langsung) dan **`forma-sidecar`** (untuk app non-Go — lihat `04-forma-sidecar.md`).

---

## 2. Fitur

| Fitur | Package | Status |
|---|---|---|
| **Entity engine** — CRUD, optimistic concurrency (`Version`), soft delete, search+pagination, `Submit`/`Cancel`/`Amend` lifecycle | `internal/db` (`crud.go`) | ✅ Implemented |
| **Schema migration** — generate DDL dari `EntitySpec` (dialect-aware SQLite/Postgres), checksum-tracked migration runner | `internal/db` (`ddl.go`, `migrate.go`) | ✅ Implemented |
| **Manifest loading & validasi** — multi-doc YAML, kind validation, reserved-field rules | `internal/manifest`, `pkg/spec` | ✅ Implemented (untuk kind `Document`/`Entity`) |
| **REST API generator** — CRUD routes + custom action routes, deny-by-default via `Expose` | `internal/api` | ✅ Implemented |
| **Permission enforcement** — explicit required-permission per action, auto-prefix, module footprint | `internal/permission` | ✅ Implemented |
| **Action dispatch** — routing berdasarkan `impl.type` (`script`/`native`/`sidecar`) | `internal/action` | ⚠️ Sebagian — lihat §7 |
| **State machine** — validasi transisi state | `internal/entity` (`state_machine.go`) + `internal/db` | ⚠️ Dua implementasi terpisah, tidak konsisten — lihat §7 |
| **`ctx.*` primitives untuk Starlark** — db/cache/lock/queue/pubsub/storage/kvstore | `internal/starlark` | ⚠️ API-nya ada, tapi semua operasi stub — lihat §7 |
| **Auth** — JWT (HS256/RS256/ES256) + dev token, wildcard permission matching | `internal/auth` | ✅ Implemented |
| **Tenant isolation** — `{workspace}` URL scoping, cross-tenant → 404 | `internal/api` (`middleware.go`) | ✅ Implemented |
| **Idempotency store** | `internal/db` (`idempotency.go`) | ✅ Implemented |
| **Outbox (substrat event delivery)** | `internal/db` (`outbox.go`, `outbox_worker.go`) | ⚠️ Fungsional tapi belum disambungkan ke action dispatch — lihat §7 |
| **Admin panel auto-generated (`/_admin`)** | — | ❌ Belum ada sama sekali — lihat §7 |

---

## 3. Desain Internal

### 3.1 Package Map

| Package | Tanggung jawab |
|---|---|
| `internal/entity` | Registry (`LoadEntities`/`RegisterArtifactManifest`), `SyncSchema`, `GetEntityStore`; `StateMachineEngine` (transisi state, guard via Starlark) |
| `internal/api` | Generator route (`GenerateRoutes`, `GenerateCustomActionRoutes`), router chi (`RouterBuilder`), middleware chain |
| `internal/action` | `Dispatcher` — routing eksekusi by `ImplType`; executor `native`/`script`/`sidecar` |
| `internal/permission` | Registry permission & "uses" declaration, module footprint, deteksi cross-module write |
| `internal/db` | `EntityStore` (CRUD lengkap), DDL generator, migration runner, child-table store, natural-key counter, audit log, idempotency store, outbox |
| `internal/manifest` | Loader multi-doc YAML |
| `internal/starlark` | `CtxAPI` — permukaan `ctx.*` untuk script; evaluator kondisi/guard |
| `internal/datastore` | Registry/resolver/factory koneksi per driver (sqlite/postgres/valkey/redis/s3/...) — dipakai `ctx.*` resolusi datastore |
| `internal/auth` | `TokenValidator` (JWT/dev), `Identity`, permission matching |
| `internal/validation` | Cross-field rules (`after`/`before`/`exists:`), validasi action params |

### 3.2 Middleware Chain (REST API)

```
Recovery → Logging → CORS → RequestID → Tenant ({workspace} dari URL)
  → Auth (JWT/dev token) → per-route RequirePermission → Handler
```

Cross-tenant access → **404** (bukan 403) — mencegah workspace enumeration (`internal/api/middleware.go`).

### 3.3 Alur Registrasi Route

```
Document/Entity spec (Expose: [{type: rest, actions: [list, find, create, ...]}])
  → GenerateRoutes(spec)         — CRUD standar sesuai Expose
  → GenerateCustomActionRoutes   — POST /{module}/{plural}/{id}/{action}
  → chi Router mount di /{workspace}/api/v1/...
```

### 3.4 Alur Eksekusi Action

```
POST /{workspace}/api/v1/{module}/{plural}/{id}/{action}
  → Middleware chain (auth, permission)
  → Dispatcher.Dispatch(impl.type)
      type: native  → NativeExecutor (lookup handler by ref/module/entity/action)
      type: script  → ScriptExecutor (resolve .star file, jalankan via internal/starlark)
      type: sidecar → SidecarExecutor (panggil app process via socket — lihat 04-forma-sidecar.md)
  → ExecuteResult { NewState, Events, ... }
```

---

## 4. `ctx.*` API (untuk Script Starlark)

Permukaan primitive yang dipanggil dari `.star` script (`internal/starlark/primitive.go`):

| Primitive | Method | Fungsi |
|---|---|---|
| `ctx.db` | `.query()`, `.get()`, `.set()`, `.delete()` | Akses datastore relasional/dokumen |
| `ctx.cache` | `.get()`, `.set()`, `.delete()` | Key-value cache (Valkey/Redis) |
| `ctx.lock` | `.acquire()`, `.release()` | Distributed lock (mutual exclusion — lihat `docs/architecture/05-failover.md` §3.4) |
| `ctx.queue` | — | Job queue |
| `ctx.pubsub` | — | Publish/subscribe realtime |
| `ctx.storage` | — | Object storage (S3-compatible) |
| `ctx.kvstore` | — | KV store sederhana |
| `ctx.tenant` / `ctx.user` / `ctx.auth` | `.has()` | Identitas & permission check |
| `ctx.now()`, `ctx.next_key()`, `ctx.log.{info,warn,error}`, `ctx.config.get()` | | Utility |

Semua primitive mendukung `.named("name")` untuk binding ke datastore spesifik (multi-datastore per `kind: Datastore` — lihat `docs/spec/12-datastore.md`).

**Status implementasi:** permukaan API-nya lengkap dan stabil, tapi setiap operasi (`query`/`get`/`set`/`delete`/`acquire`/`release`) **selalu mengembalikan error "not yet implemented"** — `CtxAPI.SetDatastoreResolver` (yang seharusnya menyambungkan ke koneksi nyata) tidak pernah dipanggil dari binary manapun. Lihat §7.

---

## 5. Kontrak Embedding

### 5.1 Untuk App Go Native

Model yang didokumentasikan (`docs/architecture/01-architecture-overview.md` §2) sekarang punya facade publik nyata di folder `resource/` (`resource/forma.go`, `resource/syncagent.go` — package `forma`, bukan lagi di bawah `internal/`, supaya bisa di-`import` dari module lain). Import path berakhiran `/resource` (nama folder), tapi identifier package-nya tetap `forma` — jadi pemanggilannya tetap `forma.New(...)`, bukan `resource.New(...)`:

```go
import "github.com/primadi/forma/resource"

app, err := forma.New(forma.Config{
    SpecPath: "./spec",
    DSN:      "sqlite:data.db",
    Addr:     ":8080",
})
if err != nil {
    log.Fatal(err)
}
log.Fatal(app.ListenAndServe())
```

`forma.New` melakukan persis alur di §3.1–3.4 (load entity, sync schema, wire dispatcher, build route) di balik satu pemanggilan. Contoh penggunaannya ada di `examples/reference-app/main.go`.

Untuk pengkabelan yang **sesuai desain plane protocol** (memuat dari artifact Control Plane, bukan disk), `syncagent.go` menyediakan `forma.NewSyncAgent`/`(*SyncAgent).Run` — mem-port persis apa yang dulu `cmd/forma-resource` lakukan (poll snapshot, fetch+verify artifact, evidence). **Gap-nya belum berubah** (lihat §7 gap #1): `SyncAgent` men-sync entity registry & schema, tapi **tidak** menyambungkan hasilnya ke `forma.App`/`api.RouterBuilder` — dua jalur ini masih terpisah secara sengaja (lihat komentar di `syncagent.go`), karena menyatukannya (hot-reload route saat artifact baru datang) adalah pekerjaan desain tersendiri, bukan sekadar pemindahan kode.

### 5.2 Untuk App Non-Go (via Forma Sidecar)

Model *single-process embed* (lihat `04-forma-sidecar.md`): `forma-sidecar` meng-compile-in package `forma` yang sama seperti di atas (entity engine, API generator, dsb) sebagai satu proses, ditambah listener socket/HTTP untuk komunikasi dengan proses app (PHP/Python/dst).

---

## 6. Referensi Skema Manifest

Skema `Document`/`Entity` (yang di-load engine ini) didefinisikan normatif di `docs/spec/02-core-basic.md` dan `docs/spec/10-entity-extension.md` — dokumen ini tidak mengulang skemanya, hanya perilaku runtime-nya. Struct Go yang relevan ada di `pkg/spec/entity.go`: `DocumentSpec`, `Field`, `Action`, `StateMachine`, `EventDecl`, `UsesDecl`, `ExposeConfig`.

Kind lain yang parse valid tapi **belum dikonsumsi runtime apapun** (lihat §7): `Page`, `Form`, `Table`, `Dashboard`, `Workflow`, `Api`, `Webhook`, `Environment`, `Policy`, `Datastore`, dan kind frontend lain di `pkg/spec/frontend.go`.

---

## 7. Status Implementasi Hari Ini

1. **`SyncAgent` (plane-protocol client, di `syncagent.go`) tidak serve REST API sama sekali.** Ia menjalankan pull-based convergence (poll snapshot, fetch+verify artifact, evidence) dan meng-update entity registry + schema — tapi **tidak pernah** menyambungkannya ke `api.RouterBuilder`/`forma.App`. `forma.App` (di `resource/forma.go`) yang benar-benar serve API dan sudah dipakai `examples/reference-app`, tapi hanya untuk mode filesystem (`SpecPath`), belum untuk mode pull-based. **Menyatukan dua jalur ini — supaya artifact yang di-pull `SyncAgent` benar-benar memicu reload route `forma.App`, bukan cuma sync schema — adalah gap paling penting** yang tersisa untuk membuat model "compile forma-resource ke app Go" benar-benar berjalan sesuai desain plane protocol di production.
2. **Admin panel (`/_admin`) tidak ada sama sekali.** Tidak ada generator/route/renderer untuk kind `Page`/`Form`/`Table`/`Dashboard` di manapun di kode — meski `pkg/spec/frontend.go` mendefinisikan strukturnya secara lengkap.
3. **`ctx.*` primitives seluruhnya stub.** `internal/datastore` (registry/resolver/factory per driver) sudah lengkap secara struktur tapi setiap `Open()` cuma bikin struct pool tanpa membuka koneksi nyata sungguhan (`// TODO: integrate with internal/db`). `CtxAPI.SetDatastoreResolver` tidak dipanggil di binary manapun — setiap script yang menyentuh `ctx.db()`/`ctx.cache()`/dst mendapat error `"datastore resolver not configured"`.
4. **State machine punya dua implementasi yang tidak konsisten.** `entity.StateMachineEngine` (lengkap, dengan guard evaluation) tidak pernah dipanggil dari `internal/api.HandleCustomAction` — enforcement transisi state yang benar-benar jalan justru ada di `db.EntityStore.Update` (`validateStateTransition`), sebuah pengecekan terpisah dan lebih sederhana yang ter-trigger saat field state berubah lewat `Update()`.
5. **`emits:` events tidak tersambung.** `ExecuteResult.Events` didefinisikan tapi tidak pernah diisi executor manapun, dan tidak pernah dikonsumsi handler HTTP. Outbox (`internal/db/outbox.go`) fungsional secara independen tapi tidak ada yang men-enqueue dari action dispatch.
6. **`SidecarExecutor.Execute` selalu return error** ("not implemented ... deferred to Fase 5") — ini persis titik yang perlu diimplementasikan sesuai desain `04-forma-sidecar.md`.
7. **`NativeExecutor` fungsional tapi tak ada handler yang pernah diregistrasi** di binary manapun — setiap action `impl.type: native` akan error "not registered" dalam praktiknya hari ini.
8. **Natural key counter (`db.NaturalKeyCounter`) real tapi tidak dipakai** — `resource/forma.go`'s dispatcher (dan sebelumnya `forma-serve`) men-stub `next_key` dengan `fmt.Sprintf("KEY-%d", time.Now().UnixNano())`, bukan memanggil counter sequence yang sudah ada.
9. **`internal/ctx`, `internal/service`, `internal/tenant`, `internal/events` kosong** — bukan berarti fungsinya hilang; masing-masing tersebar: tenant resolution ada inline di `internal/api/middleware.go`, ctx (untuk Go native, bukan Starlark) belum ada bentuknya sama sekali, events paling dekat diwakili outbox yang belum disambungkan.

### 7.1 Prioritas Perbaikan

1. Satukan `SyncAgent` dan `App` — `SyncAgent.OnDeploy`-equivalent (`loadYAMLIntoRegistry` di `syncagent.go`) HARUS pada akhirnya memicu rebuild `api.RouterBuilder`/atomic-swap handler di `App` yang sedang serve, bukan cuma sync schema.
2. Wire `CtxAPI.SetDatastoreResolver` ke `internal/datastore`, dan selesaikan `internal/datastore`'s `Open()` untuk driver yang paling penting dulu (Postgres, Valkey).
3. Satukan state-machine enforcement — pilih satu (`entity.StateMachineEngine` tampaknya lebih lengkap) dan hapus duplikasi di `db.crud.go`, atau jelaskan pembagian tanggung jawabnya kalau memang disengaja.
4. Implementasikan `SidecarExecutor` sesuai kontrak di `04-forma-sidecar.md`.
5. Admin panel generator — ini pekerjaan besar tersendiri, kemungkinan fase terpisah.

---

## 8. References

| Dokumen | Isi |
|---|---|
| `docs/spec/02-core-basic.md`, `03-core-extended.md` | Skema normatif Document/Entity/Action/StateMachine |
| `docs/spec/12-datastore.md` | Skema `kind: Datastore` dan resolusi `ctx.*` |
| `docs/architecture/01-architecture-overview.md` §2, §4.3 | Model compile-in Go, resource pod |
| `docs/runtimes/01-forma-ctl.md` | Sisi server dari plane protocol |
| `docs/runtimes/04-forma-sidecar.md` | Model embedding untuk app non-Go |
