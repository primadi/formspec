# Topologi Database

**Version:** 1.0
**Status:** Draft
**License:** Creative Commons CC0
**Governed by:** FormSpec Architecture Overview (D-ARCH-10, D-ARCH-31) · [`spec/platform/06-datastore.md`](../spec/platform/06-datastore.md) · [`renderers/jsonb-persist/`](../renderers/jsonb-persist/README.md)

> Satu backend penyimpanan (jsonb-persist) berjalan di atas **dua engine**: SQLite dan
> PostgreSQL. Dokumen ini menetapkan **kapan masing-masing dipakai** dan batas teknis
> yang tidak boleh dilanggar selama SQLite masih didukung.

---

## 1. Keputusan

| Konteks                                     | Engine                                 | Penegakan                                                  |
| ------------------------------------------- | -------------------------------------- | ---------------------------------------------------------- |
| Development, CI, contoh, embedding library  | **SQLite** (default `formspec dev`)    | `config.go` menerima `sqlite` \| `postgres`                |
| Production single-server (`formspec serve`) | **PostgreSQL — wajib**                 | `serve.go` menolak DSN berawalan `sqlite:` (lihat §1.1)    |
| Shared economy (banyak workspace)           | **PostgreSQL**                         | Wajib untuk replica > 1, scale-to-zero, failover node (§5) |
| Enterprise / regulated                      | **PostgreSQL dedicated per workspace** | `kind: Datastore` + `access.filter.workspaces` (§6.1)      |

**Aturan penentunya:** SQLite sah sebagai engine **development** dan **embedding**, tetapi
bukan sebagai engine **production**. Sebuah workspace membutuhkan PostgreSQL begitu ia harus
bertahan hidup di luar satu proses tunggal — yaitu begitu ia membutuhkan replica kedua,
scale-to-zero, atau restore terarah per-tenant. Perhatikan bahwa aturan ini **sudah ditetapkan
berlaku di kode**, bukan sekadar direkomendasikan:

```go
// cmd/formspec/serve.go — production constraints (todo 8.1.4)
if strings.HasPrefix(*dsn, "sqlite:") {
    fail("SQLite is not allowed in production mode — use a postgres:// DSN (todo 8.1.4)")
}
```

### 1.1 Sudah Ditegakkan Sejak Fase 8

Gate di atas bukan usulan baru. `formspec serve --mode=production` sudah menolak SQLite,
mewajibkan `--dsn postgres://…`, mewajibkan `--jwt-secret`/`--jwt-public-key`, dan mewajibkan
allow-list CORS — semuanya bersama-sama pada satu jalur (Fase 8, `cmd/formspec/serve.go`).
Dokumen ini menjelaskan **alasan teknis di balik gate itu**, sehingga alasan tersebut tidak
hilang saat seseorang tergoda melonggarkan gate demi "menyederhanakan deployment".

Perpindahan engine **tidak pernah mengubah manifest aplikasi** — itu keputusan deployment
([`spec/platform/02-workspace-app-module.md`](../spec/platform/02-workspace-app-module.md) §1),
dinyatakan sebagai binding Datastore, bukan sebagai flag di spec App.

---

## 2. Batas yang Sebenarnya: Satu Koneksi, Bukan Satu Proses

Pembatas SQLite di FormSpec **bukan** "hanya satu proses yang boleh membuka file". Pembatasnya
adalah **satu koneksi**, dan pool-nya dipaksa demikian secara sengaja oleh `OpenSQLite`
(`renderers/jsonb-persist/sqlite_db.go`):

```go
sqldb.SetMaxOpenConns(1)
sqldb.SetMaxIdleConns(1)
```

| Properti           | SQLite                                          | PostgreSQL       |
| ------------------ | ----------------------------------------------- | ---------------- |
| `SetMaxOpenConns`  | `1`                                             | `25`             |
| `SetMaxIdleConns`  | `1`                                             | `10`             |
| Journal / WAL      | `journal_mode(WAL)`                             | bawaan           |
| Foreign keys       | `foreign_keys(ON)`                              | bawaan           |
| Batas tunggu tulis | `busy_timeout(5000)` (5 detik)                  | —                |
| Cache              | `cache_size(-32000)` (≈31 MB)                   | `shared_buffers` |
| Bentuk PK          | UUID v7 sebagai `text`, digenerate di app layer | UUID v7, `uuid`  |

Konsekuensi yang mengikat:

1. **Semua tulis diserialisasi.** Hanya ada satu koneksi, jadi dua penulisan tidak pernah
   berjalan bersamaan — di dalam satu proses sekalipun.
2. **Permintaan koneksi kedua saat koneksi tunggal sedang dipakai akan menggantung.**
   Ini bukan penurunan performa, melainkan kebuntuan: pool tidak punya koneksi untuk
   diberikan. Lihat §4.
3. **Lintas proses, WAL mengizinkan banyak pembaca tetapi hanya satu penulis.** Proses kedua
   tetap bisa membuka file dan membaca; penulisan kedua menunggu sampai `busy_timeout`
   habis lalu gagal dengan `SQLITE_BUSY`. Efeknya adalah _latency cliff_, bukan korupsi data.

`SetMaxOpenConns(1)` dipertahankan justru karena poin 2: `TxScope` bergantung pada satu koneksi
yang stabil untuk menahan transaksi sepanjang satu eksekusi action
([`renderers/jsonb-persist/txscope.go`](../renderers/jsonb-persist/01-architecture.md) §3).

---

## 3. Worker Wajib In-Process

Seluruh pekerjaan latar FormSpec adalah **goroutine di dalam proses server**, berbagi handle
`DB` yang sama dengan jalur HTTP:

| Worker            | Dikonstruksi di                                 | Loop                                |
| ----------------- | ----------------------------------------------- | ----------------------------------- |
| Outbox            | `resource/formspec.go` (`db.NewOutboxWorker`)   | `outbox_worker.go` `go w.runLoop()` |
| Eskalasi approval | `internal/approval/escalation.go`               | `go w.runLoop()`                    |
| Subscription      | `internal/subscription/dynamic.go`, `stream.go` | `go w.runLoop()`                    |
| Job tracker       | `resource/formspec.go`                          | goroutine                           |
| Link sweeper      | `resource/formspec.go`                          | goroutine                           |
| Streaming         | `resource/formspec.go`                          | goroutine                           |

Tidak ada binary worker terpisah di `cmd/` — hanya `formspec`, `formspec-ctl`,
`formspec-gen-kind-docs`, `formspec-gen-schema`, `formspec-operator`, dan `formspec-registry`.

**Aturan:** selama sebuah deployment memakai SQLite, setiap worker harus berbagi handle `DB`
yang sama dengan jalur HTTP. Memisahkan worker ke proses atau binary lain mengubah poin §2.3
dari _latency cliff_ menjadi kegagalan tulis rutin — dan tidak ada perbaikan di level aplikasi
yang bisa menutupnya, karena masalahnya lintas proses, bukan lintas goroutine.

Di PostgreSQL batasan ini hilang: pool 25 koneksi (§2) membuat worker bisa tinggal di proses
terpisah tanpa mematikan jalur HTTP. Yang **tidak** hilang adalah kewajiban in-process saat ini —
memisahkan worker adalah pekerjaan tersendiri (`cmd/` tidak punya binary worker), bukan sesuatu
yang sudah tersedia. Batasan §3 karena itu berbunyi "selama memakai SQLite", bukan "selamanya".

---

## 4. Invarian Baca/Tulis

Hazard §2.2 ditegakkan lewat satu invarian di `renderers/jsonb-persist`:

| Operasi                       | Helper                             | Perilaku saat `TxScope` aktif                    |
| ----------------------------- | ---------------------------------- | ------------------------------------------------ |
| Baca di dalam mutasi          | `txReadDB(ctx, s.db)` / `TxReadDB` | Memakai koneksi transaksi (read-your-own-writes) |
| Tulis multi-statement         | `runTx(ctx, s.db, fn)`             | Bergabung ke transaksi scope                     |
| Tulis satu statement (atomik) | `writeDB(ctx, s.db)`               | Bergabung ke transaksi scope                     |

Alasan keberadaan `txReadDB` dinyatakan langsung di doc comment-nya (`tx.go`): memakai transaksi
scope untuk baca menghindari "a deadlock against a pool with no free connections" pada driver
satu-koneksi. Sebuah baca yang memakai pool dasar (`s.db.QueryContext`) saat `TxScope` aktif
akan meminta koneksi kedua sementara satu-satunya koneksi dipegang transaksi — dan menggantung
tanpa batas.

**Verifikasi invarian ini bisa dijalankan, bukan sekadar dipercaya:**

```bash
# Harus nol hasil: tidak boleh ada baca langsung ke pool dasar di crud.go.
grep -n 's\.db\.\(QueryContext\|ExecContext\)' renderers/jsonb-persist/crud.go

# Harus ada: baca yang sadar-transaksi, termasuk resolusi relasi.
grep -c 'txReadDB(ctx, s\.db)' renderers/jsonb-persist/crud.go
```

Invarian ini berlaku untuk **kedua** engine — di PostgreSQL ia tidak menyelamatkan dari
kebuntuan (pool 25 koneksi menyediakannya), tetapi ia tetap wajib demi _read-your-own-writes_:
sebuah guard di dalam action harus melihat tulisannya sendiri, bukan snapshot sebelum transaksi.

---

## 5. Konsekuensi per Tier — Kenapa Economy Tidak Boleh SQLite

Tier economy menjalankan banyak workspace murah di satu cluster, dengan scale-to-zero sebagai
mekanisme biayanya ([`05-failover.md`](./05-failover.md) §3.2). Lima hal berikut bertabrakan
langsung dengan §2:

1. **Replica > 1 tidak mungkin.** Satu Deployment = satu proses = satu koneksi. Tiga replica
   berarti tiga proses menulis ke satu file; penulis kedua menunggu sampai `busy_timeout` habis
   lalu gagal `SQLITE_BUSY`.
2. **Scale-to-zero menuntut DB bersama.** Pod economy harus boleh mati dan lahir kembali
   ([`05-failover.md`](./05-failover.md) §6: "state ada di DB/Valkey"). File SQLite tidak bisa
   dibagi antar pod, jadi workspace ber-SQLite **wajib dipin pada satu replica** — justru
   membatalkan alasan biaya scale-to-zero ada.
3. **Node mati berarti data mati.** [`05-failover.md`](./05-failover.md) §3.5 mencatat
   konsekuensinya secara eksplisit: _"SQLite lokal — ❌ Tidak [bisa auto-failover] — file db
   hanya ada di node yang mati"_, tanpa pemulihan otomatis (intervensi Cloud Owner).
4. **Worker bersaing dengan pengguna di koneksi yang sama.** Outbox worker melakukan polling
   setiap **1 detik** dengan batch 10 (`outbox_worker.go`, default). Pada SQLite, poll itu
   mengantre di depan permintaan pengguna, bukan berjalan di kapasitas terpisah.
5. **Restore terarah per tenant tidak mungkin.** Satu file melayani seluruh workspace di node
   itu, sehingga "restore workspace X ke titik waktu Y" tidak bisa dinyatakan — padahal itu
   bagian dari jaminan backup ([`spec/backend/04-persist-backend.md`](../spec/backend/04-persist-backend.md) §3).

Poin-poin ini hanya bergantung pada jumlah koneksi dan lokasi file — bukan pada ukuran data.
Karena itu **skala kecil tidak menyelamatkannya**: workspace di tier economy bisa saja berisi
sedikit baris dan tetap tidak layak SQLite, karena yang dibutuhkan adalah replica dan mobilitas
pod, bukan kapasitas.

---

## 6. Production Single-Server dengan PostgreSQL

Memakai PostgreSQL untuk **satu server** — tanpa K8s, tanpa Operator, tanpa HA — adalah
deployment yang sah dan merupakan jalur yang memang diharapkan. Ia tidak menuntut apa pun
selain sebuah instance Postgres yang bisa dijangkau:

```bash
formspec serve --mode=production \
  --dsn "postgres://user:pass@localhost:5432/formspec" \
  --jwt-secret "…" --cors-origin "https://app.example.com"
```

Engine-nya sama seperti SQLite; yang berbeda hanya driver (`config.go` menerima `sqlite` dan
`postgres` sebagai closed set). Produksi single-server memilih Postgres karena satu alasan yang
sama dengan §5, hanya dengan skala lebih kecil: sebuah deployment yang tidak berada di dalam
satu proses tunggal tidak boleh bergantung pada file yang hanya ada di satu node.

Konsekuensi operasional yang perlu disadari: Postgres single-node **bukan** HA. Ia
menghilangkan batas satu-koneksi (§2) — sehingga `formspec serve` boleh multi-replica dan boleh
punya worker terpisah bila perlu — tetapi backup, upgrade, dan pemulihan tetap tanggung jawab
operator ([`05-failover.md`](./05-failover.md) §5).

### 6.1 Enterprise: Dedicated per Workspace

Isolasi fisik per tenant diwujudkan tanpa menyentuh manifest App: Cloud Owner meregistrasi
`kind: Datastore` terpisah (`driver: postgres`), lalu `access.filter.workspaces` membatasi
service itu ke workspace tertentu ([`spec/platform/06-datastore.md`](../spec/platform/06-datastore.md) §1, §4).
Manifest App tetap menulis `datastores: {db: pg-main}` — binding-nya yang berubah, bukan kontraknya.

Dedicated memiliki konsekuensi yang harus disadari: [`05-failover.md`](./05-failover.md) §3.5
menyatakan DB dedicated **tidak** bisa auto-failover selama ia terikat node, sehingga pemulihan
memerlukan intervensi Cloud Owner. Ini trade-off yang dibeli dengan isolasi, bukan cacat.

---

## 7. Backup & Restore per Tier

| Konteks                | Mekanisme                                                              | Batasan                                                         |
| ---------------------- | ---------------------------------------------------------------------- | --------------------------------------------------------------- |
| SQLite (dev/CI)        | Cadangkan file setelah `checkpoint` WAL (atau lewat `formspec backup`) | Seluruh file sekaligus; tidak ada restore terarah per-tenant    |
| Postgres single-server | PITR + `formspec backup` (filter `--workspace`/`--filter`)             | Restore satu tenant = operasi terarah di atas satu cluster      |
| Postgres dedicated     | PITR per instance                                                      | Paling bersih untuk per-tenant; biaya operasional per workspace |

Kontraknya sama di ketiga baris — backup penuh maupun inkremental, filterable, dengan mode
restore `skip`/`overwrite`/`remap` (`spec/backend/04-persist-backend.md` §3). Yang berbeda adalah
_seberapa sempit_ cakupan yang bisa di-address sebuah restore, dan itu ditentukan topologi,
bukan backend.

---

## 8. Batas Keputusan Ini

Dokumen ini menetapkan engine dan batas teknisnya; ia **tidak** mengubah hal-hal berikut.

**Ditolak secara desain** (bukan pekerjaan yang menunggu dijadwalkan):

- **SQLite sebagai backend multi-writer.** Menaikkan `SetMaxOpenConns` pada SQLite menuntut
  perubahan jaminan `TxScope` (`BEGIN IMMEDIATE` untuk tulis) demi keuntungan yang kecil
  dibanding memindahkan deployment ke PostgreSQL.
- **Row Level Security sebagai lapis kedua.** Isolasi tenant hari ini adalah predikat
  `tenant_id` yang ditegakkan aplikasi (`crud.go`), bukan policy database. Menambah RLS akan
  menduplikasi sumber kebenaran isolasi — dua tempat yang bisa menyimpang, dengan yang terlihat
  otoritatif belum tentu yang berlaku.
- **Metering berbasis ukuran database.** Billing berbasis resource
  ([`01-architecture-overview.md`](./01-architecture-overview.md) D-ARCH-30), bukan ukuran
  database per workspace.

**Belum dijadwalkan** (pekerjaan terbuka, punya pelacak):

- **Routing koneksi per-workspace di Resource Plane.** Hari ini satu DSN berlaku per proses dan
  satu database melayani semua workspace di dalamnya (dipisah `tenant_id`); karena itu
  "dedicated per workspace" berarti **satu deployment per workspace** (D-ARCH-31). Satu proses
  melayani banyak database tenant adalah fase cloud — tercatat di tabel **Deferred (Cloud
  Phase)** `docs_internal/plan/todo.md`.
- **Worker per-store / scheduler terdistribusi.** Mengikuti item di atas; selama worker wajib
  in-process (§3), beban latar tidak bisa dipindahkan ke proses lain.

---

## 9. Referensi

| Dokumen                                                                              | Isi                                                             |
| ------------------------------------------------------------------------------------ | --------------------------------------------------------------- |
| [`01-architecture-overview.md`](./01-architecture-overview.md)                       | D-ARCH-10, D-ARCH-30, D-ARCH-31 — failover, metering, model pod |
| [`03-deployment-flow.md`](./03-deployment-flow.md) §4                                | Dev (SQLite) vs Production (Postgres)                           |
| [`05-failover.md`](./05-failover.md) §3.2, §3.5, §6                                  | Scale-to-zero, failover eligibility, state di DB, recovery pod  |
| [`spec/platform/06-datastore.md`](../spec/platform/06-datastore.md)                  | `kind: Datastore`, workspace binding, driver compatibility      |
| [`spec/backend/04-persist-backend.md`](../spec/backend/04-persist-backend.md) §2, §3 | Kontrak PersistBackend, jaminan backup & restore                |
| [`renderers/jsonb-persist/`](../renderers/jsonb-persist/README.md)                   | Strategi skema, migrasi, query, transaksi (`txscope.go`)        |
