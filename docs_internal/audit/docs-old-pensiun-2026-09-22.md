# Audit `docs_old/` — kelayakan pensiun (todo 9.5.1)

**Tanggal**: 2026-09-22
**Sifat**: **laporan saja** — tidak ada file yang dihapus. Keputusan menghapus
ada di tangan pemilik (dijawab "audit dulu, laporkan temuannya, baru putuskan").
**Sumber aturan**: `docs_old/MIGRATION.md` §1 (aturan tree otoritatif), §2 (peta
migrasi per file), `AGENTS.md` (docs_old = arsip read-only yang akan dihapus).

---

## 1. Kesimpulan singkat

**Migrasi kontennya sendiri TUNTAS** — setiap file di `docs_old/` yang punya
penerus memang sudah punya penerus **yang benar-benar ada di disk** (diverifikasi
file-per-file). Satuan ukur yang dipakai `MIGRATION.md` §1 ("selesai") karenanya
terpenuhi.

**Tapi `docs_old/` belum boleh dihapus hari ini**, karena **8 rujukan di kode
nyata masih menunjuk ke dalam `docs_old/`**. Menghapus arsipnya sekarang akan
membuat komentar-komentar itu menunjuk ke hutan yang sudah ditebang. Ini persis
kondisi yang dilarang `MIGRATION.md` ("Kode boleh sementara menunjuk
`docs_old/spec/...` **sampai S5/S9**").

> **Koreksi (koreksi diri, 2026-09-22).** Draft pertama laporan ini menyebut ada
> blocker kedua: "sweep eksplisit entry L4–L6 ledger `11-reference.md` belum
> ditutup". Itu **salah** dan sudah dibuang. Dua kekeliruan: (a) yang saya baca
> sebagai "level validasi L4–L6" ternyata **level persona** —
> `11-reference.md:165` berbunyi "L4 — Cloud Owner + admin" (matriks peran
> L1–L4), bukan `business_rules`/`cross_validate`/`consistency`; (b) catatan di
> `MIGRATION.md` §1 yang saya jadikan dasar ("bagian D-ledger `11-reference.md`
> yang belum di-sweep eksplisit") adalah **stale** — §5 baris 136 mencatat
> verifikasi baris-per-baris D1–D50 tuntas: *"39 absorbed, 3 obsolete, 8 missing
> → semuanya sudah dilebur pada pass yang sama … Ledger D1–D50 kini tuntas
> ter-absorb/obsolete **tanpa sisa**"*. Jadi **hanya ada satu blocker**, bukan
> dua.

---

## 2. Verifikasi peta migrasi (file per file)

Semua 49 file diperiksa. Ringkasan per kelompok:

| Kelompok | Jumlah | Fate deklarasi | Verifikasi 2026-09-22 |
| --- | --- | --- | --- |
| `docs_old/spec/*` | 14 | rewrite / split / moved | ✅ **semua penerus ada** |
| `docs_old/implementation/*` | 4 | historis + rewrite | ✅ penerus ada (`docs/renderers/…`, `docs/runtimes/05-…`) |
| `docs_old/changelog/*` | 23 | historis (tidak dibawa) | ✅ memang tidak punya penerus — tidak diperlukan |
| `docs_old/audit/*` | 3 | historis | ✅ snapshot gap lama |
| `docs_old/plan/*` | 5 | historis | ✅ catatan kerja |
| `docs_old/README.md`, `MIGRATION.md` | 2 | rewrite / dokumen kerja | ✅ `docs/README.md` ada |

Penerus yang diverifikasi ada di disk (32 path diperiksa, termasuk yang
penamaannya berubah `forma*` → `formspec*`): `docs/README.md`,
`docs/spec/platform/01–08`, `docs/spec/backend/01–06` + `error-glossary.yaml` +
`03-entity-extension.md`, `docs/spec/frontend/01–08`,
`docs/renderers/jsonb-persist/01–04`, `docs/renderers/shadcn-shell/01–04`,
`docs/runtimes/04-formspec-sidecar.md`, `docs/runtimes/05-engine-api-layer.md`,
`docs/reference/glossary.md`, `docs/guides/order-to-cash-{tutorial,companion}.md`,
`docs/architecture/08-repo-structure.md`, `docs/comparison/formspec-vs-laravel.md`.

Satu-satunya "MISS" adalah `docs/comparison/forma-vs-laravel.md`, yang semata
karena rename `forma` → `formspec` (penerus `formspec-vs-laravel.md` ada). Tidak
ada penerus yang hilang.

**Konsistensi ukuran** (indikasi penerus bukan placeholder): `docs_old/spec/
04-control-plane.md` 237 baris → `docs/spec/platform/04-control-plane.md` 323
baris; `docs_old/spec/05-frontend.md` 603 baris → pecah ke
`docs/spec/frontend/01–08`.

---

## 3. Blocker A — 8 rujukan kode ke `docs_old/`

Diperiksa dengan `grep -rn docs_old --include=*.go --include=*.ts --include=*.tsx --include=*.mts` (mengecualikan `node_modules` dan `**/dist/`).

| # | Lokasi | Menunjuk ke | Penerus yang benar (sudah ada) | Bentuk perbaikan |
| --- | --- | --- | --- | --- |
| 1 | `cmd/formspec-ctl/main.go:11` | `docs_old/spec/11-reference.md` D43 | `docs/cli-tools/04-formspec-ctl.md` + `docs/runtimes/01-formspec-ctl.md` (keduanya sudah menyebut D43) | buang rujukan ledger lama |
| 2 | `sdk/browser/src/error.ts:5` | `docs_old/spec/02-core-basic.md` §16 | `docs/spec/backend/01-core-basic.md` §8.5 (Shared Contract) | ganti path + nomor § |
| 3 | `sdk/browser/src/types.ts:3` | idem | idem | idem |
| 4 | `sdk/browser/src/client.ts:21` | idem | idem | idem |
| 5 | `internal/ui/registry.go:6` | `docs_old/implementation/frontend-renderer.md` §4.1–§4.2 | `docs/renderers/shadcn-shell/01-architecture.md` (+ `02-derivation-engine.md`) | ganti path |
| 6 | `internal/manifest/examples_roundtrip_test.go:13` | "`docs_old/spec` vs `pkg/spec`" | `docs/spec/` | perbarui narasi |
| 7 | `pkg/spec/spec.go:7` | `docs_old/spec/05-frontend.md` §3–13 | `docs/spec/frontend/01–08` | hapus catatan "pending migration" |
| 8 | `pkg/spec/spec.go:8` | `docs_old/spec/04-control-plane.md` | `docs/spec/platform/04-control-plane.md` | hapus catatan "pending migration" |

Tambahan yang **bukan blocker** (memang menyaring arsip, dan tetap benar setelah
penghapusan): `docs-site/.vitepress/config.mts` (4 kemunculan — exclude arsip
dari build & link-check). `sdk/browser/dist/*.d.ts` adalah artefak build dari
`sdk/browser/src/*` yang sama — perbaikannya ikut saat build ulang.

Berbeda dari `reff_docs/`, yang **memang** sumber eksternal/historis yang sah
dirujuk (`docs/runtimes/05-…` menyebutnya sebagai sumber audit) dan **tidak**
ikut dihapus.

---

## 4. Blocker B — tidak ada (koreksi diri)

Draf pertama laporan ini mengklaim ada invariant terbuka: sweep entry **L4–L6**
ledger `11-reference.md`. Setelah diperiksa ulang, **klaim itu salah**:

- `11-reference.md` hanya punya `L4` sebagai **level persona** ("L4 — Cloud Owner
  + admin", matriks peran L1–L4) — bukan level validasi tinggi.
- `MIGRATION.md` §5 baris 136 mencatat verifikasi baris-per-baris D1–D50
  **tuntas**: 39 absorbed, 3 obsolete, 8 missing (D5/D23/D29/D34/D37/D44/D46/D50)
  semuanya dilebur pada pass yang sama, ditutup dengan *"Ledger D1–D50 kini
  tuntas ter-absorb/obsolete tanpa sisa"*.
- Catatan §1 yang menyebut "belum di-sweep eksplisit" adalah **teks stale** yang
  tidak diperbarui setelah §5 melengkapinya. Ia pantas ikut terhapus bersama
  arsipnya, bukan dijadikan penundaan.

---

## 5. Invariant yang bersih (bukti)

| Invariant (`MIGRATION.md`) | Hasil |
| --- | --- |
| `docs/` tidak boleh mereferensikan `docs_old/` | ✅ `grep -rn docs_old docs/` → **0 hasil** |
| Ledger D1–D50 ter-sweep tanpa sisa | ✅ `MIGRATION.md` §5 baris 136 (verifikasi baris-per-baris 2026-07-16) |
| `docs/` jadi dokumentasi otoritatif per topik | ✅ semua penerus ada (bagian 2) |

---

## 6. Rekomendasi

**Urutan yang aman:**

1. Perbaiki 8 rujukan kode (bagian 3) → hapus ketergantungan pada arsipnya.
2. Baru hapus `docs_old/` (dan `MIGRATION.md` bersamanya, seperti dinyatakan
   dokumen itu sendiri).

**Ukuran**: langkah 1 small (8 komentar, satu sesi); langkah 2 trivial secara
mekanis (butuh commit yang jelas).

**Risiko bila dihapus lebih dulu**: komentar kode menunjuk path mati —
silent, tidak ketahuan test, dan justru menyesatkan pembaca berikutnya.

**Koreksi urutan**: rekomendasi awal menyisipkan langkah "(b) tutup catatan
L4–L6" di antara keduanya. Langkah itu dibuang karena catatannya memang sudah
tuntas (§4); menyisipkannya berarti menahan penghapusan atas dasar yang tidak
ada.

**Catatan**: `reff_docs/` (1,1 MB, 9 file draft v0.x) **di luar lingkup** audit
ini dan tidak ikut dihapus.
