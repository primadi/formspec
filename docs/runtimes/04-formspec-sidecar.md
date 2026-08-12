# FormSpec Sidecar — Binary Reference

> ⚠️ **DEPRECATED** — `formspec-sidecar` sebagai binary terpisah sudah digantikan
> oleh `formspec dev --listen local_http`. Semua fungsionalitas sidecar (ctx listener,
> app monitor, child process management) sudah terintegrasi ke dalam `formspec` CLI.
>
> `cmd/formspec-sidecar` masih ada untuk backward compatibility tetapi akan dihapus
> di rilis mendatang. Gunakan `formspec dev --listen local_http` sebagai pengganti.

**Version:** 1.0 (deprecated)
**Status:** Draft
**License:** Creative Commons CC0 (dokumen) — binary-nya sendiri FSL (open source)
**Governed by:** `docs/architecture/01-architecture-overview.md` §2, `docs/runtimes/02-formspec-resource.md`

> `formspec-sidecar` adalah binary yang **meng-embed FormSpec Resource dalam satu proses** (entity engine, permission, REST API generator — sama seperti app Go native yang meng-`import` langsung), ditambah **listener socket/HTTP lokal** yang menjembatani proses tersebut dengan proses aplikasi non-Go (PHP, Python, Node, Java, dst) dalam pod yang sama. Dokumen ini mendesain protokol komunikasi antara `formspec-sidecar` dan `lib-formspec-*` (SDK client tipis di sisi app).

---

## 1. Peran & Topologi Pod

Untuk app non-Go, satu pod berisi **dua proses** (dua container K8s, atau dua proses dalam satu container — lihat §5):

```
Pod
┌────────────────────────────┐      ┌───────────────────────────────┐
│ Container: formspec-sidecar   │      │ Container: app                 │
│  - formspec-resource engine   │      │  - app.php (business logic)    │
│    (compiled-in, sama      │◄────►│  - lib-formspec-php (SDK tipis)   │
│    seperti app Go native)  │ socket│                                │
│  - entity engine, API      │      │                                │
│    generator, permission   │      │                                │
│  - unix socket listener    │      │                                │
└────────────────────────────┘      └───────────────────────────────┘
     ▲ REST API end-user                    Volume: emptyDir (unix socket)
     │ (masuk langsung ke sidecar)           atau localhost (sama network namespace pod)
```

**Yang penting:** traffic REST API end-user masuk **ke `formspec-sidecar`**, bukan ke app.php. `formspec-sidecar` men-generate & serve seluruh route CRUD (persis seperti app Go native). Hanya saat action punya `impl.type: sidecar`, `formspec-sidecar` **memanggil keluar** ke proses app untuk eksekusi business logic — lalu app **memanggil balik** ke `formspec-sidecar` untuk `ctx.*` primitives (db, cache, lock, dst) selama eksekusi itu.

Ini dua arah komunikasi berbeda lewat satu socket yang sama:

| Arah | Kapan | Isi |
|---|---|---|
| **Sidecar → App** | Eksekusi action `impl.type: sidecar` | Invoke handler, kirim `Resource`+`Params`, tunggu hasil |
| **App → Sidecar** | Selama handler app berjalan | `ctx.db.query()`, `ctx.lock.acquire()`, dst — proxy ke engine yang sama yang dipakai script Starlark |

---

## 2. Fitur

| Fitur | Deskripsi |
|---|---|
| **Embed FormSpec Resource penuh** | Entity engine, REST API generator, permission enforcement — identik dengan app Go native (lihat `02-formspec-resource.md`) |
| **Listener socket/HTTP lokal** | Unix domain socket (default) atau localhost TCP, untuk komunikasi dengan proses app |
| **Invoke handler app** | Memanggil business logic app (PHP/Python/dst) untuk action `impl.type: sidecar` |
| **Proxy `ctx.*` ke app** | Meneruskan panggilan `ctx.db`/`ctx.cache`/`ctx.lock`/dst dari app ke engine yang sama dipakai Starlark (`internal/starlark` primitives) |
| **Startup: pull artifact** | Sama seperti resource pod Go native — pull dari Cluster Control, extract source code app (`app.php`), lalu **start proses app** sebagai child atau sebagai container terpisah yang sudah jalan |
| **Health aggregation** | `/health` sidecar mencerminkan kesehatan proses app juga (app tidak merespons ping → sidecar report degraded) |

---

## 3. Desain Internal

### 3.1 Startup Sequence

```
Pod start
  → formspec-sidecar: pull artifact dari Cluster Control (sama seperti formspec-resource)
  → Extract: YAML specs + source code app (app.php) + runtime info (php:8.3)
  → Load entity spec ke engine (sama seperti formspec-resource.OnDeploy)
  → Start listener socket (unix:///tmp/formspec/sidecar.sock)
  → Tunggu proses app terhubung (app container start terpisah, connect ke socket via shared volume)
      ATAU formspec-sidecar sendiri yang exec proses app, tergantung mode (lihat §5)
  → Serve REST API (sama seperti formspec-resource yang sudah lengkap)
```

### 3.2 Package yang Dipakai Ulang dari FormSpec Resource

`formspec-sidecar` tidak mengimplementasikan ulang entity engine — ia meng-compile-in package Go yang **sama persis** dengan yang dipakai app Go native (`internal/entity`, `internal/api`, `internal/permission`, `internal/db`, `internal/auth`). Bagian yang **ditambahkan khusus untuk sidecar**:

| Komponen baru | Fungsi |
|---|---|
| `SidecarExecutor` (implementasi nyata, mengganti stub di `internal/action/sidecar.go`) | Serialisasi `ExecuteParams` → kirim ke app via socket → deserialisasi `ExecuteResult` |
| Socket server (sisi sidecar) | Terima panggilan `ctx.*` dari app, proxy ke `internal/starlark` primitive runner yang sama |
| Proses supervisor (opsional, tergantung mode §5) | Kalau app dijalankan sebagai child process, bukan container terpisah |

---

## 4. Protokol Komunikasi

### 4.1 Transport

Unix domain socket (default, direkomendasikan — latency lebih rendah, tidak ada konflik port): `unix:///tmp/formspec/sidecar.sock`, dipasang lewat `emptyDir` volume yang di-share antar container dalam pod yang sama. Alternatif: HTTP lokal di `localhost:PORT` (dua container dalam satu pod berbagi network namespace, jadi `localhost` valid) — dipakai kalau runtime bahasa tidak punya library unix-socket yang matang.

Format pesan: **HTTP/1.1 di atas socket tersebut** (bukan protokol biner custom) — memudahkan `lib-formspec-*` dibangun di atas HTTP client standar tiap bahasa, dan mudah di-debug dengan tooling umum (`curl --unix-socket`).

### 4.2 Sidecar → App: Invoke Handler

Dipanggil `SidecarExecutor.Execute` saat action punya `impl.type: sidecar`. Mengirim `ExecuteParams` (bentuk sudah ada di `internal/action/dispatcher.go`), menerima `ExecuteResult`:

```http
POST /invoke/{module}/{entity}/{action}
Content-Type: application/json

{
  "resource_id": "inv-001",
  "resource": { "status": "draft", "total": 150000 },
  "params": { "note": "approve for payment" }
}
```

```http
200 OK
Content-Type: application/json

{
  "data": { "approved_at": "2026-07-10T10:00:00Z" },
  "new_state": "approved",
  "events": [ { "name": "invoice.approved", "payload": {...} } ]
}
```

```http
500 Internal Server Error   — handler app error/exception, pesan di body { "error": "..." }
504 Gateway Timeout         — app tidak merespons dalam batas waktu (configurable, default 30s)
```

Bentuk request/response ini adalah serialisasi langsung dari `action.ExecuteParams`/`action.ExecuteResult` yang sudah ada di kode (`internal/action/dispatcher.go:28-69`) — `SidecarExecutor` asli tinggal marshal/unmarshal struct yang sama, tidak perlu skema baru.

### 4.3 App → Sidecar: `ctx.*` Primitive Calls

`lib-formspec-php` (dan sejenisnya) mengekspos `ctx.*` sebagai objek native di bahasa itu, tapi setiap panggilan sebenarnya adalah HTTP call balik ke sidecar — memakai **kontrak primitive yang sama** dengan `internal/starlark/primitive.go` (`query`/`get`/`set`/`delete`/`acquire`/`release`), supaya Starlark dan app non-Go berbagi implementasi backend yang identik:

```http
POST /ctx/db/query
{ "named": "", "filter": {...} }

POST /ctx/lock/acquire
{ "named": "", "key": "workspace:X", "ttl_seconds": 30 }

POST /ctx/cache/get
{ "named": "", "key": "..." }
```

Panggilan ini terjadi **selama** request `/invoke/...` dari §4.2 sedang berlangsung (app memanggil balik ke sidecar dalam rentang satu invoke) — bukan panggilan independen di luar konteks action.

### 4.3a Entity Primitive — `POST /ctx/entity/{op}`

Primitive infrastruktur (§4.3) tidak menyediakan operasi level-entity; handler sidecar butuh padanan `resource.fetch()`/`resource.save()` milik Starlark. Entity primitive mengisinya dengan lima operasi:

| Operasi | Semantik | Atomik? |
|---|---|---|
| `get` | Fetch record penuh by ID | — |
| `set` | Ganti seluruh data | ✅ satu statement |
| `update` | Merge field tertentu saja | ✅ satu statement |
| `increment` | `field = field + amount` | ✅ satu statement |
| `decrement` | `field = field - amount`, dengan guard `>= amount` | ✅ satu statement |

```http
POST /ctx/entity/update
{ "named": "pharmacy/medicine", "key": "0189abcd-...", "fields": { "stock": 96 } }

POST /ctx/entity/decrement
{ "named": "pharmacy/medicine", "key": "0189abcd-...", "field": "stock", "amount": 4 }
```

`named` berformat `"{module}/{entity}"`. `update`/`increment`/`decrement` dieksekusi sebagai satu statement di PersistBackend — tidak ada read-modify-write race; `version` tetap di-increment otomatis. `decrement` mengembalikan nilai baru dan **gagal kalau guard tidak terpenuhi** (saldo/stok tidak cukup) — pola yang tepat untuk counter stok. Kolom generated ber-index dihitung ulang otomatis oleh backend ([`../renderers/jsonb-persist/02-schema-strategies.md`](../renderers/jsonb-persist/02-schema-strategies.md)).

Semua SDK `lib-formspec-*` mengekspos operasi ini lewat `ctx.entity().named("module/entity").get/set/update/increment/decrement(...)`.

Praktik: pilih `decrement`/`increment` alih-alih `get`+modify+`set` untuk field numerik (menghindari TOCTOU); pilih `update` alih-alih `set` untuk perubahan parsial. Catatan: operasi field-level tidak mengecek `version` (bukan full-record CAS) — kalau butuh optimistic concurrency level record, pakai `get` → compare → `set`.

**Multi-operasi dalam satu handler kini BISA satu transaksi** — lewat header
`X-FormSpec-Scope-Id` (baru). Saat `HandleCustomAction` men-dispatch action
bertipe `sidecar`, ia sudah membuka `TxScope` request-scoped untuk seluruh
eksekusi action itu (`renderers/jsonbpersist/txscope.go`); `SidecarExecutor`
(`internal/action/sidecar.go`) menyertakan id scope itu sebagai header
`X-FormSpec-Scope-Id` pada request `POST /invoke/...` (§4.2). **App process
wajib menyimpan header ini selama menangani satu `/invoke/...` dan
mengirimkannya balik sebagai header yang sama di setiap panggilan
`/ctx/...` (termasuk `/ctx/entity/{op}`) yang dilakukan dalam rentang
invoke tersebut** — `CtxHandler` (`internal/sidecar/ctx.go`) me-resolve
header itu balik ke `*TxScope` yang sama (keduanya berjalan di proses OS
yang sama, lihat `cmd/formspec/dev.go`) sehingga operasi entity itu ikut
transaksi yang sama, bukan commit sendiri-sendiri. Tanpa header ini,
perilakunya persis seperti sebelumnya: tiap operasi commit independen.

**Status implementasi SDK (per 2026-07-20): belum ada `lib-formspec-*` yang
mengirim header ini.** Ini kontrak wire baru di sisi Go; menyesuaikan
`sdk/php`, `sdk/python`, `sdk/typescript` (yang sudah punya implementasi
nyata) dan SDK lain untuk menyimpan+mengirim `X-FormSpec-Scope-Id` sepanjang
satu invoke adalah follow-up terpisah, bukan bagian dari perubahan Go ini.
Sebelum SDK diperbarui, panggilan `ctx.entity.*` dari app process tetap
commit independen — SAMA seperti perilaku hari ini, tidak ada regresi.

### 4.4 Peran `lib-formspec-*` (SDK Client Tipis)

`lib-formspec-php`, `lib-formspec-python`, dst **hanya**:
1. Menjalankan listener kecil (menerima `POST /invoke/...` dari sidecar, memanggil fungsi handler yang didaftarkan developer)
2. Menyediakan objek `ctx` yang method-nya melakukan HTTP call ke §4.3 (client, bukan implementasi)
3. Serialize/deserialize tipe data FormSpec (Document, Field types) ke/dari tipe native bahasa tersebut

**Tidak ada logic bisnis FormSpec** (state machine, permission, entity storage) di sisi `lib-formspec-*` — semuanya di `formspec-sidecar` (keputusan desain, lihat diskusi awal dokumen ini).

---

## 5. Mode Eksekusi Proses App

Dua opsi topologi, keduanya valid tergantung kompleksitas runtime bahasa, dipilih lewat flag `--runtime` (§6):

| Mode | `--runtime` | Deskripsi | Cocok untuk |
|---|---|---|---|
| **Container terpisah** | `local` (default) | App berjalan sebagai proses/container terpisah yang **tidak** dikendalikan `formspec-sidecar` — container K8s sibling, atau dijalankan manual saat dev. App hanya perlu menghubungi `--app-endpoint` sendiri. `formspec-sidecar` tidak tahu (dan tidak perlu tahu) bahasa/runtime app — inilah mode default untuk bahasa apa pun, termasuk yang **belum** punya SDK `lib-formspec-*`. | Semua bahasa; pola sidecar K8s standar |
| **Child process (exec)** | `php` \| `python` \| `node` | `formspec-sidecar` sendiri yang meng-exec proses app sebagai child di container yang sama, dari `--app-dir` (entrypoint konvensi `app.php`/`app.py`/`app.js`, atau override `--app-entrypoint`). Env `FORMA_APP_SOCKET`/`FORMA_SIDECAR_SOCKET` di-set otomatis ke socket yang dipakai `--app-endpoint`/`--listen` — cocok dengan default yang dibaca `lib-formspec-*` (`sdk/README.md`), jadi app boot tanpa konfigurasi tambahan. Mensyaratkan `--app-endpoint` dan `--listen` sama-sama `unix://`. | Runtime ringan yang sudah punya SDK, di mana overhead 2 container tidak sepadan |

Nilai `--runtime` boleh membawa suffix versi (`php:8.3`) — bagian setelah `:` murni informational, binary interpreter yang dieksekusi selalu nama bare (`php`, `python3`, `node`) via `PATH`.

---

## 6. Konfigurasi & Flags

```bash
# Mode container terpisah (default) — app dijalankan sendiri (container sibling)
formspec-sidecar \
  --listen unix:///tmp/formspec/sidecar.sock \
  --app-endpoint unix:///tmp/formspec/app.sock \
  --control-cluster-url https://control-cluster.jkt-premium-01.svc \
  --workspace-id bank-mandiri-prod \
  --invoke-timeout 30s

# Mode child-process — formspec-sidecar meng-exec app.php sendiri
formspec-sidecar \
  --runtime php:8.3 \
  --app-dir /srv/app \
  --app-entrypoint app.php \
  --listen unix:///tmp/formspec/sidecar.sock \
  --app-endpoint unix:///tmp/formspec/app.sock \
  --spec ./spec
```

Referensi lengkap ada di `--help` (`cmd/formspec-sidecar/main.go`); ringkasan flag yang relevan dengan §5: `--runtime` (`local` default, atau `php`/`python`/`node`), `--app-dir` (default `{state-dir}/app`), `--app-entrypoint` (default per-runtime: `app.php`/`app.py`/`app.js`).

---

## 7. Health Aggregation

`GET /health` di `formspec-sidecar` merefleksikan dua hal:
1. Kesehatan proses sidecar sendiri (entity engine, DB connection)
2. Kesehatan proses app — sidecar melakukan ping berkala ke `lib-formspec-*` listener; kalau app tidak merespons N kali berturut-turut, `/health` sidecar melaporkan `degraded` (bukan `unhealthy` — sidecar sendiri masih bisa serve request yang tidak butuh handler app, mis. CRUD murni tanpa custom action).

---

## 8. Status Implementasi Hari Ini

**Implementasi awal sudah ada.** `cmd/formspec-sidecar` + `internal/sidecar` mengimplementasikan:

- `SidecarExecutor` nyata di `internal/action/sidecar.go` (§4.2) — HTTP/1.1 over unix socket atau localhost TCP, serialisasi langsung `ExecuteParams`/`ExecuteResult`, timeout configurable (`--invoke-timeout`, default 30s). Executor tanpa endpoint (app Go native) tetap gagal dengan error "not configured" — perilaku lama dipertahankan. Diaktifkan via `resource.Config.SidecarEndpoint`.
- Listener socket sisi sidecar (`internal/sidecar/server.go`) — `POST /ctx/{prim}/{op}` (§4.3) dengan kontrak resolver yang identik dengan `internal/starlark` (`func(primitiveType, name) (any, error)`), plus `GET /health` agregasi (§7, ping app 10s, 3 kegagalan berturut-turut → `degraded`).
- Startup pull artifact (§3.1) — reuse `internal/resource` deployer; artifact envelope memisah file YAML (→ `--state-dir/spec`, jadi input `resource.New`) dari file lain seperti source app (→ `--app-dir`), sesuai desain §3.1 ("YAML specs + source code app + runtime info"). Artifact baru setelah boot ditulis ke disk + log `restart required` (hot-rebuild route belum ada — sama dengan gap `SyncAgent` di `02-formspec-resource.md` §7).
- Mode eksekusi app (§5) — **kedua mode sudah ada**: `--runtime local` (default, container terpisah) dan `--runtime php|python|node` (child-process, `cmd/formspec-sidecar/childprocess.go`) yang meng-exec interpreter dari `--app-dir`/`--app-entrypoint` dengan env `FORMA_APP_SOCKET`/`FORMA_SIDECAR_SOCKET` otomatis, plus shutdown graceful (SIGTERM lalu SIGKILL setelah timeout).

**SDK client (`lib-formspec-*`) sudah ada di `sdk/`** — PHP (`sdk/php`, Composer `formspec/lib-formspec`), Python (`sdk/python`, `lib-formspec`, stdlib-only), dan TypeScript (`sdk/typescript`, `@formspec/lib-formspec`, tanpa dependency runtime). Ketiganya mengimplementasikan persis tiga peran §4.4: listener `/invoke` + `/health`, objek `ctx` yang memanggil balik §4.3, dan serialisasi wire types. Kontrak wire didokumentasikan di `sdk/README.md`; counterpart Go-nya `internal/action/sidecar.go` dan `internal/sidecar/ctx.go`. Ketiganya sudah diverifikasi end-to-end (invoke + `ctx.lock` round-trip lewat unix socket, termasuk PHP lewat `php-cli`/`composer` yang ditambahkan ke `.devcontainer/Dockerfile`).

**Yang masih terbuka:** backend primitive `ctx.*` masih stub di seluruh codebase (operasi `query`/`get`/... balas 501 sampai `datastore.ConnectionPool` punya implementasi op nyata — gap yang sama dengan Starlark); mode child-process belum punya auto-restart kalau child crash di tengah jalan (hanya di-log, sidecar tetap hidup dan health `/health` melapor degraded via app monitor). **Model `--runtime` di §5–§6 juga baru mendukung satu runtime untuk seluruh project** — belum ada satu-proses-per-Module untuk workspace yang Module-nya dimiliki bahasa berbeda-beda (mis. Module A TypeScript, Module B PHP); kontrak target untuk itu ada di [`docs/spec/platform/08-project-layout.md`](../spec/platform/08-project-layout.md) §3–§5, dicatat sebagai desain belum-diimplementasikan.

### 8.1 Urutan Pembangunan yang Disarankan

1. Implementasikan `SidecarExecutor` nyata (§4.2) — mulai dari HTTP client sederhana ke socket, pakai struct `ExecuteParams`/`ExecuteResult` yang sudah ada.
2. Bangun `lib-formspec-php` sebagai proof-of-concept pertama (PHP dipilih karena paling umum untuk kasus penggunaan FormSpec) — listener `/invoke` minimal + client `ctx.db`/`ctx.lock` yang manggil §4.3.
3. Selesaikan dulu gap `ctx.*` di `02-formspec-resource.md` §7 (`SetDatastoreResolver` belum di-wire) — endpoint `/ctx/db/query` di sidecar pada akhirnya delegasi ke kode yang sama, jadi tidak ada gunanya membangun proxy-nya sebelum backend-nya nyata.
4. Tentukan mode eksekusi proses app (§5) — perlu diputuskan sebelum tooling `formspec apply`/image builder untuk app non-Go dibangun.

---

## 9. References

| Dokumen | Isi |
|---|---|
| `docs/runtimes/02-formspec-resource.md` | Engine yang di-embed sidecar; `ctx.*` primitive contract |
| `docs/architecture/01-architecture-overview.md` §2 | Deployment model Go vs non-Go app |
| `docs/spec/backend/01-core-basic.md` | Skema `impl.type: sidecar` di level Document/Action |
