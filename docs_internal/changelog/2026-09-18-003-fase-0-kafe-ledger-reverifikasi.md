# Fase 0 Kafe — Re-verifikasi penuh ledger (#1–#53, S1–S16) & klasifikasi ulang

**Tanggal:** 2026-09-18 · **Plan:** `examples/kafe/gaps_found/TODO.md` Fase 0
(`0.1`, `0.2`, `0.5`) · **Bukti:** `examples/kafe/gaps_found/14-temuan-fase-0.md`
§7–§8

## Apa yang diubah

Menutup seluruh Fase 0 dari ledger gap aplikasi kafe — bukan dengan membaca
kode, tetapi dengan perintah yang bisa gagal terhadap binary & schema terkini.

- **0.1 — #1–#53 diverifikasi ulang.** Hasil: **24 CLOSED · 3 PARTIAL (#1, #3,
  #21) · 22 OPEN · 1 RETIRED (#24)**. Re-verifikasi ini menemukan **nol koreksi
  status** — berbeda dari Fase 0 pertama (2026-09-14) yang membatalkan #7, #24,
  separuh #2, dan separuh #18. Semua yang ditandai ✅ memang tertutup, semua
  yang ditandai 🔴 memang terbuka.
- **0.2 — S1–S16 diverifikasi ulang** terhadap `schemas/formspec.schema.json`
  (ter-regenerasi) + `pkg/spec/*.go`. Hasil: **10 CLOSED · 2 PARTIAL (S4, S11)
  · 4 OPEN (S6, S13, S15, S16)**. Koreksi: S9 & S10 ternyata sudah tertutup
  (TODO 1.7 & 1.4), S4 bergeser ke PARTIAL (widget `qrcode` sudah ada di kedua
  kosakata tertutup).
- **0.5 — Klasifikasi ulang ledger.** SPEC 5 · ENGINE 12 · DOC 6 · PERILAKU 0 ·
  PARTIAL 5. Entri gugur dibuang dari hitungan.

## Kenapa

Aturan Fase 0 yang disepakati: **"Bukti, bukan inferensi"** — setiap item
ditutup dengan perintah yang bisa gagal + outputnya. Ledger dibuat dari
workspace lain dan memuat ≥4 klaim yang sudah terbukti salah, sehingga tidak ada
item yang boleh dikerjakan sebelum statusnya diverifikasi ulang.

## File yang terkena dampak

- `examples/kafe/gaps_found/14-temuan-fase-0.md` — §7 (re-verifikasi #1–#53) +
  §8 (re-verifikasi S1–S16) ditambahkan.
- `examples/kafe/gaps_found/README.md` — status ledger diperbarui (#8, #9, #11,
  #12, #18, #28, #35, #36, #47, #48 → ✅) + §"Klasifikasi final (Fase 0.5)".
- `examples/kafe/gaps_found/13-kelengkapan-spec-untuk-kafe.md` — marker status
  S4/S9/S10/S13/S16 + tabel prioritas §F.
- `examples/kafe/gaps_found/TODO.md` — `0.1`, `0.2`, `0.5` ditandai ✅.

## Baseline suite (2026-09-18)

| Perintah | Hasil |
| --- | --- |
| `formspec validate --schema schemas` (kafe) | **0 problem** (69 manifest) |
| `go test ./...` | **hijau** |
| `cd renderers/react-shadcn && npx vitest run` | **265 lulus** (15 file) |
| `make lint` | **0 issues** |

## Sisa

Fase 0 selesai. Pekerjaan berikutnya: **Fase 4** (stok, HPP & pembelian) —
`4.2`, `4.3`, `4.4`, `4.1`, `4.5`, `4.6`, `4.7`, `4.8`.
