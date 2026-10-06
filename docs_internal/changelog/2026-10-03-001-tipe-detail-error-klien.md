# 2026-10-03-001 — Tipe detail error klien disamakan dengan wire (kafe 10.64)

**Apa yang diubah.** `renderers/react-shadcn` memakai SATU tipe untuk dua hal,
dan tipe itu tidak pernah cocok dengan wire:

```ts
// sebelum
ErrorResponse.error.details?: ErrorDetail[]
export interface ErrorDetail { field?: string; code: string; message: string }
```

Server mengirim `{level, field?, message}` (`internal/api.ErrorDetailItem`,
dicerminkan `pkg/spec.ErrorDetail`; normatif di
`docs/spec/backend/05-field-types.md` §4 dan `02-core-extended.md` §14) dan
**tidak pernah mengirim `code`** di dalam entri detail. Jadi `code` diwajibkan
tipe tetapi selalu absen saat runtime, dan `level` selalu ada tetapi tak terlihat
compiler.

Sesudah: dua tipe yang terpisah, masing-masing mencerminkan satu hal nyata.

- `ErrorDetail` — **envelope**: `{code, message, details?, choices?}`
  (= `internal/api.ErrorDetail`).
- `ErrorDetailItem` — **entri**: `{level?, field?, message}`
  (= `internal/api.ErrorDetailItem`, `pkg/spec.ErrorDetail`, dan
  `sdk/browser` `ErrorDetailItem` — field demi field sama).
- `ErrorResponse` kini `{error: ErrorDetail}` (satu sumber; sebelumnya bentuk
  inline yang menduplikasi definisinya).
- `FormaApiError.details` bertipe `ErrorDetailItem[]`;
  `ApiErrorEnvelope.error.details` idem.

**Kenapa.** Kafe 10.64, ditemukan saat mengerjakan 10.61. Cacatnya **laten**:
tidak ada konsumen di SPA yang membaca `details` (semua call site menampilkan
`err.message`), jadi tidak ada yang terasa sampai ada fitur yang menampilkan
kesalahan **per field** — persis kegunaan `details` yang baru saja diberi isi
oleh 10.61. Memperbaikinya sekarang murah; setelah ada konsumen, perbaikannya
jadi perubahan perilaku.

**Dua koreksi klaim (keduanya milik saya, tercatat supaya tidak terulang).**

1. Klaim di 10.61 bahwa SPA menampilkan "Internal server error" **sudah
   dikoreksi** di changelog `2026-10-02-012`: klien mem-`toast.error(err.message)`
   dan `message` berasal dari server.
2. Di draf pertama changelog ini saya menulis "`sdk/browser.ErrorDetailItem`
   mengharuskan `code`". Itu **salah** — saya mencampur `ErrorDetail` (envelope,
   yang memang punya `code`) dengan `ErrorDetailItem` (entri, yang tidak).
   `sdk/browser/src/types.ts` **sudah** `{level?, field?, message}`. Karena itu
   perbaikan ini juga membuat SPA **sepakat** dengan SDK, bukan memperbaiki SDK.

**Bukti.**

- `npx tsc -b` bersih · `npm run lint` (oxlint) **0 error** · `npx vitest run`
  **49 berkas / 616 test lulus** (naik dari 615; +5 test baru, satu berkas).
- Test baru `src/lib/api/errorDetailContract.test.ts` (5 test) memakai tiga
  lapis penjagaan:
  1. **compile-time** — literal ber-`satisfies` yang menyalin respons 422 yang
     **terukur** dari dev server kafe (10.61). Kalau `ErrorDetailItem` kembali
     menuntut `code`/menghapus `level`, `tsc -b` gagal di literal itu.
  2. **runtime** — `buildFormaApiError` atas envelope nyata: `details[0].field`
     = `"zzz"`, `level` = `"field"`, dan entri **tidak** memuat `code`.
     Ditambah kasus field-level **tanpa** `field` (kesalahan record-level).
  3. **paritas sumber** — pola `ErrorDetailItem` di `sdk/browser/src/types.ts`
     dibaca sebagai teks dan dipastikan tidak menuntut `code`.
- **Guard terkalibrasi:** tipe dikembalikan ke bentuk lama (`{field?, code,
message}`) → `tsc -b` gagal dengan **6 error** di berkas test
  (`'level' does not exist`, `Property 'code' is missing`) sementara vitest tetap
  hijau (jelas: cacatnya adalah cacat **tipe**). Dipulihkan dari backup `/tmp`
  (bukan `git checkout`).
- **Kalibrasi anti-vakum untuk penjagaan paritas:** test memastikan pola
  `interface ErrorDetailItem [\s\S]* \bcode\??: string` **cocok** dengan snippet
  yang sengaja dikembalikan ke bentuk lama — jadi assertion "tidak menuntut
  `code`" benar-benar membedakan, bukan regex yang tak pernah cocok.

**Dampak.** `renderers/react-shadcn/src/types/manifest.ts`,
`src/lib/api/errors.ts`, test baru
`src/lib/api/errorDetailContract.test.ts`. Tidak ada perubahan perilaku runtime
(perubahan tipe murni; tidak ada konsumen `details` hari ini).
