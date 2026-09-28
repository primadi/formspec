# Plan — Alur sesi meja kafe (QR statis → bayar → occupied → clear meja)

**Sumber:** desain pemilik 2026-09-27. **Revisi 2** (setelah keputusan
"status meja pakai `state_machine`" + "occupied saat pelanggan membayar").
**Ledger kafe:** 10.34, 10.34c, 10.35, 10.36, 10.37, 10.38, 10.39, 10.40, 10.40b.
**Terkait:** 10.20 (landing).

> ## STATUS 2026-09-27 — §1–5 MENDARAT (6 gap tertutup), §6–8 masih terbuka
>
> | #   | Item                                                 | Status                                                     |
> | --- | ---------------------------------------------------- | ---------------------------------------------------------- |
> | 1   | **10.40** status meja `state_machine`                | ✅ sudah mendarat lebih dulu                               |
> | 2   | **10.40b** subscription `on_paid` → `occupied`       | ✅ + `on_served` → `served`, `on_cancel` → `available`     |
> | 3   | **10.36** rate limit intake anonim                   | ✅ `table-session` 20/menit, `order` 30/menit, `scope: ip` |
> | 4   | **10.34c** unique satu sesi terbuka per meja         | ✅ partial index; sesi kedua → 500                         |
> | 5   | **10.37** `submit.redirect` + token                  | ✅ token `{uuid}` (bukan `{id}` — lihat koreksi)           |
> | —   | **10.34b** `qr_token` `natural_key`                  | ✅ (prasyarat §5, bukan efek samping)                      |
> | —   | **10.35 / 2.15** halaman masuk token → sesi          | ✅ Page `table-open` + Form `table-open-form`              |
> | 6   | **10.38** PIN tamu kedua + hash                      | ⏸️ terbuka                                                 |
> | 7   | **10.39** Service publik `table-status`/`verify-pin` | ⏸️ terbuka                                                 |
> | 8   | **10.20** landing `/kafe`                            | ⏸️ terbuka                                                 |
>
> **Verifikasi:** `formspec validate` kafe **89 manifest / 0 problem**;
> 4 test Go baru (16 total kafe); 1 skenario Playwright (`make e2e-kafe`)
> menjalankan seluruh alur dari browser. Changelog
> `docs_internal/changelog/2026-09-27-017`, `-018`, `-019`, `-020`.
>
> **Tindak lanjut (changelog `-020`):** 10.42 (baris lama → transisi state 500)
> ✅ diperbaiki **di engine**; **10.35a** ✅ sesi ditutup saat meja dikosongkan;
> 10.53 ✅ `MaterializePartial`; 10.43 ✅ **superseded** (`PATCH` memang menghormati
> gate transisi); 10.54 ⛔ **ditarik — klaim saya salah** (`LIMIT 1` sudah ada).
>
> **Koreksi terhadap rencana ini (penting, karena §5 salah di dua tempat dan
> kurang satu langkah):**
>
> 1. **`{id}` tidak bisa dipakai di `submit.redirect`.** Rencana menulis
>    "redirect setelah buat sesi dengan token `{id}`". Id record ditetapkan
>    server di dalam `store.Insert`, jadi klien **tidak pernah** tahu nilainya
>    sebelum POST. Yang dipakai adalah **`{uuid}`** — satu nilai yang ditulis
>    sebagai `guest_token` (natural key), sehingga alamat dan payload-nya adalah
>    nilai yang sama dan halaman tujuan me-resolve-nya lewat jalur natural key.
> 2. **`natural_key` pada `qr_token` bukan opsional.** Rencana menyebutnya
>    "bukan blocker" — benar untuk `find`, tetapi langkah §5 menjadi mungkin
>    **karena** `natural_key` me-resolve **tanpa index** yang harus dideklarasikan
>    terpisah; `unique` saja tidak cukup untuk lookup by value.
> 3. **Rencana tidak menyebut penutupan sesi sama sekali.** Ia berhenti di
>    "kasir clear meja" tanpa menanyakan apa yang terjadi pada `table-session`.
>    Itu ternyata **bukan detail**: 10.34c (direncanakan di §4) mengizinkan satu
>    sesi TERBUKA per meja, dan §5 membuat sesi baru tiap kali kartu dipindai —
>    jadi gabungan keduanya membuat meja hanya bisa dipakai **sekali**. Tamu
>    kedua mendapat 500. Ditutup di `-020`: `release` memancarkan `on_cleared`,
>    dan `cafe-order` (pemilik `table-session`) menutup sesinya.
>
> **Dua bug renderer ditemukan** oleh verifikasi browser dan harus ada sebelum
> alur ini bisa diklik manusia sama sekali: tombol Create hilang di permukaan
> publik, dan `default_from` mengirim placeholder mentah saat `spec.context`
> async. Detail + bukti: changelog `-018`.
>
> **Keterbatasan yang diakui dan dinomori** (bukan disembunyikan sebagai prosa):
> **10.52** pembatalan mengosongkan meja yang masih berisi pesanan lain;
> **10.55** tidak ada kompensasi bila subscription occupancy gagal;
> **10.57** sesi yang dibuat lalu ditinggalkan mengunci mejanya — celah yang
> justru **terbuka** karena memperbaiki 10.35a. Yang pertama dan kedua berasal
> dari agregat lintas-record yang sama dengan **10.41** (yang kini
> tercatat 🟡 sebagian: aturan longgar landing, `semua pesanan disajikan` belum).

## Alur yang diminta (pemilik)

1. Tamu **scan QR statis meja**.
2. Tamu pilih item, **bayar**.
3. → **status meja `occupied`**; pesanan diproses dapur.
4. Dapur selesai → status pesanan terkirim.
5. Tamu selesai → **kasir/admin clear meja** → status kembali **`available`**.

Selama `occupied`, tamu yang sama boleh tambah pesanan + bayar lagi (dikenali
lewat token sesi). Tamu **berbeda** yang scan saat `occupied` harus memasukkan
**PIN 4 digit** (PIN pemilik sesi, atau tanya kasir).

## Keputusan: status meja = `state_machine` (bukan proyeksi sesi)

Revisi 1 saya mengusulkan "occupied = ada sesi terbuka" karena v1 tidak punya
timer. **Keputusan pemilik lebih baik**, dan alasannya benar: pemicunya bukan
timer melainkan **pembayaran**, jadi `state_machine` justru tepat.

```yaml
# cafe-master/dining-table
fields:
  - name: table_status
    type: enum
    enum_values: [available, occupied, served]
    default: available
    index: true
state_machine:
  field: table_status
  initial: available
  states:
    - { name: available, label: "Kosong" }
    - { name: occupied, label: "Terisi" }
    - { name: served, label: "Selesai Disajikan" }
  transitions:
    - { from: available, to: occupied, via: occupy }
    - { from: occupied, to: served, via: mark-table-served }
    - { from: served, to: occupied, via: occupy }
    - { from: occupied, to: available, via: release }
    - { from: served, to: available, via: release }
actions:
  - { name: occupy, required_permission: dining-tables.occupy }
  - { name: mark-table-served, required_permission: dining-tables.mark-served }
  - { name: release, required_permission: dining-tables.release }
```

**`served` = "semua pesanan sesi ini sudah disajikan".** `served` adalah state
sementara sebelum `release`: kasir masih harus menutup bill. **Tamu menambah
pesanan dari `served` → kembali `occupied`** — itu transisi `served → occupied`,
sehingga tidak perlu `impl`/route apa pun, hanya `impl`-less transition seperti
yang lain (dipicu `set table_status` lalu PATCH).

**Terverifikasi (2026-09-27):**

- `master` **boleh** punya `state_machine` — tidak ada aturan yang melarangnya.
  Diuji dengan menambal salinan spec: `85 manifest(s) validated, 0 problem(s)`.
- **Transisi tanpa `impl` tidak butuh route baru.** `PATCH` yang mengubah field
  state == memicu transisi: `internal/api/handler.go:1031`
  (`FindTransitionByStates`) dan `renderers/jsonb-persist/crud.go`
  `validateStateTransition` (cocok via `from` + `to`, guard dievaluasi).
  Jadi **clear meja = PATCH `table_status: available`** — tidak perlu `impl`,
  tidak perlu route. Persis pola `shift` (`{from: open, to: closed, via:
close-shift}`) dan `order`.

### Blocker baru: "semua pesanan sudah disajikan" butuh agregat lintas-record

`served` hanya berarti bila dapat ditentukan bahwa **semua** `order` dalam sesi
itu sudah `served`. Itu **agregat lintas-record**, dan di sini API script
mentok:

| Fakta terverifikasi                                                                      | Konsekuensi                                                                                                                                                             |
| ---------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `ResourceAPI.AttrNames` = `id, field, set, save, call, fetch, find, upsert, create, new` | **tidak ada** list/query/count di Starlark; `resource.find` mengembalikan **satu** record                                                                               |
| `summary` + `SummarySource{entity, filter}` + `Rebuild.strategy`                         | bisa **dideklarasikan**, tetapi `Rebuild` belum punya `trigger` → tidak ada pemicu deklaratif per-event                                                                 |
| `ctx.db().query(...)`                                                                    | **tersedia dan GAP-30 sudah diperbaiki** (2026-09-18: `Query` memakai `TxReadDB`, tidak lagi deadlock di SQLite) — tetapi ini SQL mentah, dan konvensi repo melarangnya |
| `transition.guard.expression`                                                            | FormSpecExpr **tidak punya** aggregate/`len`; `EvaluateGuard` hanya menyuntik `sum_line`/`len(resource.<child>)` untuk **satu** record                                  |

**Jalan keluar yang direkomendasikan (tanpa `pkg/spec`):** karena status pesanan
**sudah** `_status`-column terindeks, dan tiap perubahan status pesanan sudah
melewati transisi, hitung "semua disajikan" di **handler transisi `mark-served`
pada `order`** memakai `ctx.db().query` (deadlock sudah diperbaiki), dengan
`uses: primitives: [db]` dinyatakan eksplisit — pola yang sama dengan
`stock_level_apply.star` yang membaca `ingredient.min_stock` lintas entity.

**Jalan yang lebih bersih (butuh `pkg/spec`, → item 10.41):** `Rebuild.Source`
string + **`Rebuild.Trigger []string`** (nama event) sehingga summary bisa
dipelihara per-event secara deklaratif — menggantikan SQL mentah.

**Yang TIDAK mempan:** `EvaluateGuard` (tidak ada agregat), `resource.find`
(satu record), `resource.upsert` (summary-only), dan `Transition.Guard`
(FormSpecExpr tak punya agregat).

### Efek samping penting: serangan tadi jadi jauh lebih lemah

Rancangan ini memindahkan sumbu "terisi" dari **keberadaan sesi** ke
**`table_status` yang digerbangi permission**:

|                                   | Sumbu lama (sesi terbuka)                                           | Sumbu baru (`state_machine`)                                            |
| --------------------------------- | ------------------------------------------------------------------- | ----------------------------------------------------------------------- |
| Siapa yang bisa menandai "terisi" | **siapa pun**, lewat `POST table-session` anonim (terbukti **201**) | hanya jalur ber-permission: subscription (system) saat bayar, atau staf |
| Anonim bisa mengunci meja?        | **ya**                                                              | **tidak** — `public_entities` hanya memberi `find` pada `dining-table`  |

Ini menutup kelas serangan "orang iseng memesan di semua meja" **pada akarnya**
untuk status meja. (Spam pembuatan `table-session` tetap perlu ditutup → 10.34c

- 10.36, tapi tidak lagi mengunci meja.)

## Bagaimana `occupied` terpicu saat bayar

Jalurnya **sudah ada** dan terbukti dipakai untuk menulis entity lain:

```
order: { from: awaiting_payment, to: paid, via: confirm-payment, emit: on_paid }
   ↓ (durable event, outbox + retry + dead-letter)
kind: Subscription  events: [cafe-order.order.on_paid]
   ↓ handler: script_ref
script: resource.find("cafe-master.dining-table", ...) → set("table_status","occupied") → save()
```

Presedennya `gl/subscriptions/sales-to-journal.yaml` + `journalize.star`, yang
menulis `gl.journal-entry` dari event yang sama. Yang harus **dibuat**:
subscription baru (mis. `cafe-order/subscriptions/table-occupancy.yaml`) +
script handler.

**Batasan terverifikasi yang mengubah implementasi:**

- `resource.upsert` **summary-only** — `UpsertProjection` menolak entity
  non-summary (`"UpsertProjection is only valid for summary entities"`).
  `dining-table` adalah `master` → wajib **`resource.find` → `set` → `save`**.
- Subscription berjalan sebagai **system** (di luar HTTP, tanpa identity) →
  permission `dining-tables.occupy` tidak menghalanginya. Yang menggerbangi
  adalah siapa yang boleh **memicu jalur itu** (pembayaran).
- **Risiko yang harus disadari:** bila subscription gagal setelah pembayaran,
  meja tetap `available` padahal pesanan sudah lunas. Outbox retry menutup
  sebagian. Bukti pengamatan untuk sisa ini: matikan handler → bayar →
  `table_status` tetap `available`. Catat sebagai item tersendiri bila tidak
  ditutup.

## Tetap dipakai: token sesi + PIN (untuk tamu berbeda)

`table-session` tetap relevan untuk: identitas tamu, `guest_token` (kunci baris
`order`), jumlah orang, dan **PIN** untuk tamu kedua.

Yang **sudah bekerja** (terukur hari ini):

- `public_entities.order` + `scope: guest_token` → `list order` anonim **tanpa**
  token = **403**; dengan token hanya barisnya sendiri.
- Peran tamu saat occupied = `pin` (PEMILIK) vs `pin_baru` (anggota baru) →
  keduanya mendapat akses ke `table_session_id`, sehingga pesanan satu meja
  masuk ke **satu bill**.
- **find by `natural_key` sudah jalan**: anonim `GET .../menu-item/KPI-001` →
  **200**. Jadi membuka sesi dari QR cukup menandai `dining-table.qr_token`
  sebagai `natural_key` — **bukan** blocker (koreksi atas laporan awal saya).

## Blocker yang tersisa (dan yang gugur)

| Item                                     | Status setelah revisi                                                                                                                          |
| ---------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| **10.35a** clear meja tidak punya route  | **GUGUR untuk meja** — `release` transisi tanpa `impl` → cukup PATCH. Grant `dining-tables.release` tetap perlu.                               |
| **10.37** `submit.redirect` mati         | **Masih perlu** — mendaratkan tamu di `/menu/{session_id}` setelah buat sesi (`manifest.ts:900` ada, 0 pemakai di renderer)                    |
| **10.38** tidak ada random/PIN + hashing | **Masih perlu** — PIN + `guest_token` acak & PIN ter-hash. Preseden: native hook `hashUserPassword` (`resource/auth_native.go`)                |
| **10.39** Service tidak bisa publik      | **Masih perlu** — "status meja" + "verify PIN" harus di luar entity CRUD (PIN tak bisa disimpan di entity: `find` mengembalikan seluruh field) |
| **10.34c** tanpa unique sesi-per-meja    | **Masih relevan** (menutup spam sesi), tidak lagi mengunci meja                                                                                |
| **10.36** tanpa rate limit               | **Masih perlu** — PIN 4 digit (10.000 kemungkinan) tak bermakna tanpa ini                                                                      |

## Urutan kerja

| #   | Item       | Isi                                                                                                               | Effort |
| --- | ---------- | ----------------------------------------------------------------------------------------------------------------- | ------ |
| 1   | **10.40**  | `dining-table`: `table_status` + `state_machine` + actions `occupy`/`release` + grant ke kasir/supervisor/manajer | small  |
| 2   | **10.40b** | Subscription `order.on_paid` → `occupied` (+ `on_cancel` → `available`), `find`→`set`→`save`                      | medium |
| 3   | **10.36**  | `rate_limit` pada `table-session` + `order` (`scope: ip`)                                                         | small  |
| 4   | **10.34c** | unique parsial satu sesi terbuka per meja                                                                         | small  |
| 5   | **10.37**  | `submit.redirect` + token `{id}`                                                                                  | small  |
| 6   | **10.38**  | module native Go: nonce + PIN + hash + verify                                                                     | medium |
| 7   | **10.39**  | Service publik `table-status` + `verify-pin`                                                                      | medium |
| 8   | **10.20**  | landing `/kafe` → petunjuk scan QR                                                                                | small  |

1–4 bisa dikerjakan **sekarang tanpa menyentuh `pkg/spec`**. 6–7 menyentuh
`pkg/spec` → wajib `make generate-schema` + `make generate-kind-docs`.

## Yang TIDAK dilakukan

- **Tidak** menambah timer/`Schedule` — tidak ada di v1, dan tidak diperlukan.
- **Tidak** memakai "occupied = ada sesi terbuka" (sumbu yang bisa dikunci anonim).
- **Tidak** memakai pemilih meja kosong.
- **Tidak** menyimpan PIN plaintext / menaruhnya di `table-session`.

## Bukti yang harus ada saat selesai

- Bayar pesanan QR → `dining-table.table_status` = `occupied`.
- Kasir `release` → `available`; pemegang tanpa grant → **403**.
- Anonim **tidak bisa** membuat meja `occupied` (hanya `find` yang granted).
- Tamu kedua scan saat occupied → diminta PIN; PIN salah 5× → ditolak.
- `POST table-session` berulang > `max` → **429**.
- `formspec validate` kafe 0 problem · `go test ./...` hijau · `vitest` hijau.
