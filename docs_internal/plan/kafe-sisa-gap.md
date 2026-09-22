# Plan — Menutup Sisa Gap Aplikasi Kafe (Fase 8 & 9 + 2.6 + 3.8)

**Sumber:** `examples/kafe/gaps_found/TODO.md` — item yang masih unchecked setelah
Fase 0–7 selesai (per 2026-09-20).
**Status:** disetujui pemilik (perintah "selesaikan semua yg masih gaps").

## Status akhir (2026-09-20) — SELESAI

Seluruh item cakupan selesai. Dua bug migrasi ditemukan & diperbaiki saat
verifikasi akhir (temuan 9.4, bukan item plan):

| Item | Status | Bukti |
| --- | --- | --- |
| 8.2, 8.4, 8.6, 2.6, 3.8, 9.1–9.3 | ✅ | changelog `2026-09-20-001…014` |
| **3.11** (kolom turunan hasil ALTER tidak terisi → aturan bisnis #10 lolos) | ✅ diperbaiki | `addDerivedColumnSQL` VIRTUAL di SQLite + rebuild kolom stale (`storage_drift`); master todo 15.7 ✅; changelog `2026-09-20-015` |
| **3.10** (snapshot lama memblokir `migrate apply`) | ✅ diperbaiki | `manifest.EntitySpecFromRaw` (normalisasi di jalur CLI) + `MigrationRunner.IgnoreModules("formspec.core")`; changelog `2026-09-20-015` |
| 9.4 (walkthrough 9 skenario) | ✅ API-level; sisa UI browser tercatat di ledger | tabel skenario di TODO.md |
| 9.5 (marker `# GAP-nn` gap tertutup) | ✅ | 23 file dibersihkan, `grep TERTUTUP spec/` → 0 |
| 9.6 (workflow discipline) | ✅ | todo.md 15.7, changelog 015, plan ini |
| 2.15 (kartu meja QR), 6.3, 6.4 | ⏸️ deferred | alasan di bawah tetap berlaku |

Gerbang akhir: `go test ./...` hijau · `make lint` 0 issues · `vitest` 288 ·
kafe `validate` 0 problem · `migrate plan` (DB kafe lama) → `No pending
migrations` · `formspec diff` → `No differences`.

## Cakupan & urutan

Urutan mengikuti dependensi: item murah & menyentuh dokumen dulu (8.2/8.4/8.6),
lalu jalur cetak (2.6, prasyarat 7.1 ✅), lalu fitur (3.8), lalu verifikasi E2E
(9.1–9.6) yang menutup semuanya.

| #   | Item          | Isi ringkas                                                                                       | Ukuran | Dependensi |
| --- | ------------- | ------------------------------------------------------------------------------------------------- | ------ | ---------- |
| 1   | **8.2**       | #19 + #20: kebersihan drift dokumen (`03-kind-renderers.md`, `realtime.md`, `spec.version`)       | small  | —          |
| 2   | **8.4**       | #24: pesan error port `formspec dev` lebih menuntun                                               | small  | —          |
| 3   | **8.6**       | Regenerasi artefak: `make generate-schema` + `make generate` + `make generate-kind-docs`          | small  | 8.2, 8.4   |
| 4   | **2.6**       | Jalur cetak QR: `kind: Print` bisa memuat widget (bukan mencetak nilai jadi teks) + adopsi kafe   | medium | 7.1 ✅     |
| 5   | **3.8**       | Konteks sesi `(principal, role, cabang)` — 5 tahap plan `session-context-role-branch.md`          | large  | 1.1/1.8/3.5 ✅ |
| 6   | **9.1–9.6**   | Verifikasi E2E (validate/test/lint/vitest + walkthrough 9 skenario), hapus marker `# GAP-nn`, update ledger | medium | 1–5 |

## Sisa partial yang ikut ditutup

| Partial                    | Sisa                                   | Ditutup oleh |
| -------------------------- | -------------------------------------- | ------------ |
| **#1** (widget money/time) | `Print` belum memformat money via widget | item 4 (2.6) |
| **#3 / S4** (QR)           | jalur cetak + adopsi kafe              | item 4 (2.6) |
| **#21** (referensi)        | sudah tertutup di 8.3 (validator `validateDanglingRefs`); catatan README akan diselaraskan | item 6 (9.5) |
| **S11** (pajak)            | model pajak penuh bukan bagian 1.8 → dicatat sebagai keputusan produk tersendiri, bukan gap blocker | item 6 |

## Di luar cakupan (deferred — alasan tetap berlaku)

| Item   | Alasan                                                                                            |
| ------ | ------------------------------------------------------------------------------------------------- |
| **6.3** (#15 cross-app grant + `SyncAgent`) | Mekanisme Control Plane; Control Plane/Operator/Marketplace deferred ke cloud phase (`AGENTS.md`). Spec kafe sudah benar & valid. |
| **6.4** (#42 kepemilikan `publishes`)       | Keputusan desain bahasa spec oleh pemilik proyek (dua usulan tercatat di `11-integrasi-lintas-app.md` #42). |

Keduanya tetap ⏸️ di ledger; tidak ditebak.

## File yang diperkirakan tersentuh

- **8.2** — `docs/renderers/shadcn-shell/03-kind-renderers.md`, `docs/renderers/realtime.md`,
  dokumen `apiVersion`/`spec.version`, `examples/kafe/gaps_found/README.md` (status #19/#20).
- **8.4** — `internal/devserver/` (port resolution), `cmd/formspec/dev.go`.
- **8.6** — `schemas/**`, `renderers/react-shadcn/src/generated/**`, `docs/kind/**`.
- **2.6** — `internal/api/print.go` (resolusi sel → widget, bukan teks), `pkg/spec`
  (kind Print: kolom/`widget`), renderer `PrintRenderer`, spec kafe
  (`dining-table` `qr_url` + `widget: qrcode`, `kind: Print` kartu meja/struk).
- **3.8** — `internal/auth/{user,resolver,materialize,service,session,token,jwt}.go`,
  `internal/api/{auth_handler,scope}.go`, SPA (pemilih konteks + pengalih + `localStorage`),
  spec kafe (`employee` assignments per (role, cabang)).
- **9.x** — `examples/kafe/gaps_found/{TODO.md,README.md,validate-baseline.md}`,
  `docs_internal/plan/todo.md`, `docs_internal/changelog/`.

## Referensi spec normatif

- `docs/spec/backend/01-core-basic.md` §1.7, §3, §7, §8.6 (scope, index, transition, permission)
- `docs/spec/backend/02-core-extended.md` §2, §5, §6.1 (workflow, integrator, summary)
- `docs/spec/backend/05-field-types.md` (money/unit/percent)
- `docs/spec/frontend/06-page-kinds.md` (Print, Report)
- `docs/spec/platform/02-workspace-app-module.md` §9.3
- `docs_internal/plan/session-context-role-branch.md` (desain 3.8)

## Bukti yang harus ada saat selesai

- `formspec validate` kafe **0 problem** (schema lokal) dan gerbang registry tercatat.
- `go test ./...` hijau, `make lint` 0 issues, `vitest` hijau, `tsc` bersih.
- Walkthrough 9 skenario (9.4) dijalankan dengan bukti per skenario.
- Nol marker `# GAP-nn` tersisa untuk gap yang tertutup (9.5).
- Changelog `docs_internal/changelog/2026-09-20-008+` untuk tiap perubahan.

## Catatan penomoran

Changelog hari ini sudah memakai `2026-09-20-001` … `014` → perubahan berikutnya
mulai dari **`2026-09-20-015`** (terakhir: `2026-09-20-015-kolom-turunan-alter-dan-snapshot-migrate.md`).
