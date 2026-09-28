# 2026-09-28-005 — Transisi `via`+`impl` dapat route REST (tutup 5.24.3)

**Plan:** `docs_internal/plan/via-sebagai-action-penuh.md` (koreksi L3)
**Menutup:** todo `5.24.3` ⏸️ (dibuka `2026-09-28-004`)

## Apa yang diubah

`GenerateCustomActionRoutes` (`internal/api/generator.go`) kini membaca
**`EntitySpec.ActionSources()`** — gabungan `actions:` yang dideklarasikan dengan
transisi state machine yang menamai `via` — bukan lagi `es.Actions` langsung.
Ini menyamakannya dengan tiga situs lain yang sudah memakai union:
`UICustomActionRoutesForEntity`, `generatePrepareRoutes`, dan cabang `custom` di
router.

Akibatnya, manifest yang **hanya** mendeklarasikan transisi (`via: post` +
`impl`, tanpa entri `actions:`) sekarang:

- punya route `POST /api/v1/{module}/{plural}/{id}/{action}`, dan
- muncul di `formspec generate` dengan method + tipe params.

Sebelumnya ia hanya punya route `/_ui/entity/…`, sehingga klien TypeScript tidak
punya cara memanggilnya meski endpoint UI-nya ada.

Sepanjang jalan ditemukan hal yang tidak ada di item 5.24.3 semula: **satu aksi
bisa mendapat dua descriptor untuk `(Method, Path)` yang sama.**
`GenerateRoutes`/`UIRoutesForEntity` memutuskan "apakah aksi ini punya `impl`
sendiri" dari `es.Actions`, sementara generator custom memutuskan dari union —
jadi transisi `via: submit|cancel|amend` ber-`impl` (tanpa entri `actions:`)
tidak terlihat oleh yang pertama. Karena `mergeRoutes` menyimpan descriptor
**pertama**, handler generik menang dan `impl` yang dideklarasikan **diam-diam
tidak pernah berjalan**. Diperbaiki dengan param `customHandled` pada
`generateRESTRoutes`: nama yang punya `impl` tidak pernah mendapat route generik.

## Kenapa

Klaim L3 (2026-09-27) menyebut `GenerateCustomActionRoutes` "membaca registry
gabungan", tetapi kodenya masih `es.Actions`. Bukti pengukuran L3 hanya menyentuh
surface `/_ui/entity/` (404 → 403), jadi celah di `/api/v1/…` tidak terlihat.
Prosa plan-nya dikoreksi di commit yang sama, supaya tidak ada dua dokumen yang
saling bertentangan tentang status yang sama.

## File yang terdampak

- `internal/api/generator.go` — `GenerateCustomActionRoutes` → `ActionSources()`;
  helper `implBackedActionNames`; param `customHandled` di `generateRESTRoutes`
- `internal/api/api_test.go` — 4 call site `generateRESTRoutes` ikut param baru
- `internal/api/generator_via_routes_test.go` (baru) — 3 test
- `cmd/formspec/generate_inputs_test.go` — fixture diubah ke `via`-only (menutup
  komentar yang jadi tidak benar) + komentar diperbarui
- `docs_internal/plan/via-sebagai-action-penuh.md` — koreksi klaim L3
- `docs/runtimes/05-engine-api-layer.md` — deskripsi sumber route custom +
  invariant "satu aksi satu route"

## Bukti

- **Sebelum:** `GenerateCustomActionRoutes` untuk transisi `via: post` ber-`impl`
  → `[]` (nol route). **Sesudah:** `POST /api/v1/cafe-order/orders/{id}/post`
  dengan permission `cafe-order.orders.post`.
- **Sebelum:** test invariant `(Method, Path)` unik **gagal di dua tempat** —
  `POST /_ui/entity/cafe-order/order/{id}/submit` terdaftar sebagai `auto:submit`
  **dan** `custom`. **Sesudah:** tidak ada duplikat, di kedua surface.
- **`formspec generate` end-to-end** (spec baru, transisi `post` ber-`impl` +
  `params.inputs: [post_note]`, `expose: [list, find]`, tanpa entri `actions:`):

  ```ts
  export interface GlJournalEntryPostParams {
    "post_note": string;
  }
  ...
  post: (id: string, params: GlJournalEntryPostParams): Promise<unknown> =>
    client.action("gl", "journal-entries", id, "post", params),
  ```

  Keduanya **tidak ada** sebelum perbaikan ini.

**Test:** `go test ./internal/api/ ./cmd/formspec/ ./resource/ ./internal/manifest/`
hijau (termasuk kafe E2E, yang memuat semua route). Invariant duplikat path
dikunci `TestGeneratedRoutes_HaveNoDuplicatePath`, dan `TestGenerateCustomActionRoutes_IncludesTransitionVia`
dibuktikan gagal (menerima `[]`) sebelum `ActionSources()` dipasang.

## Catatan kalibrasi

Tiga percobaan sebelum sampai ke bentuk akhir, semuanya karena membaca kode
sebelum menebak:

1. Percobaan pertama hanya mengganti `GenerateCustomActionRoutes` → test duplikat
   gagal di surface **UI**, bukan REST. Ternyata blok `implActions` yang saya
   tebak ada di jalur REST berada di `UIRoutesForEntity` — jalur yang berbeda.
2. Percobaan kedua mengganti `implActions` di `UIRoutesForEntity` → duplikat UI
   hilang, duplikat REST muncul. Titik skip-nya harus di **helper**
   `generateRESTRoutes`, bukan di salah satu pemanggil.
3. Gagasan memfilter `exp.Actions` **ditolak**: bila `exp.Actions` menyebut satu
   nama saja dan nama itu dibuang, `useAll` menjadi true dan seluruh CRUD ikut
   ter-expose — pelebarkan otorisasi, bukan perbaikan.

## Sisa

Tidak ada sisa baru dari perubahan ini. `5.24.2` ⏸️ (Tier 2 `params.form.ref`) dan
`5.24.4` ⏸️ (kafe void lewat approval end-to-end) tetap terbuka, tidak
tersentuh oleh perbaikan ini.
