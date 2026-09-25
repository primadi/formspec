# 2026-09-22-010 — Pensiun `docs_old/` (eksekusi 9.5.1)

**Plan/Todo**: item **9.5.1**. Melanjutkan audit `2026-09-22-009`; keputusan
pemilik: **"perbaiki lalu hapus sekarang"**.

## Yang dilakukan

**1. Perbaiki rujukan dulu (16 di luar arsip), supaya tidak ada komentar yang
menunjuk path mati:**

| Lokasi | Dari | Ke |
| --- | --- | --- |
| `pkg/spec/spec.go` | `docs_old/spec/05-frontend.md` §3–13, `docs_old/spec/04-control-plane.md` ("pending migration") | `docs/spec/frontend/{01-08}`, `docs/spec/platform/{01-08}` |
| `cmd/formspec-ctl/main.go` | `docs_old/spec/11-reference.md` D43 + `cli-tools/02` | `docs/cli-tools/04-formspec-ctl.md` + `docs/runtimes/01-formspec-ctl.md` |
| `internal/ui/registry.go` | `docs_old/implementation/frontend-renderer.md` §4.1–4.2 | `docs/renderers/shadcn-shell/01-architecture.md` §4 + `02-derivation-engine.md` |
| `internal/manifest/examples_roundtrip_test.go` | "`docs_old/spec` vs `pkg/spec`" | "`docs/spec` vs `pkg/spec`" |
| `sdk/browser/src/{error,types,client}.ts` | `docs_old/spec/02-core-basic.md` §16 | `docs/spec/backend/01-core-basic.md` §8.5 |
| `sdk/browser/README.md`, `sdk/README.md` | idem + `05-frontend.md` §7 | `01-core-basic.md` §8 / §8.5 |
| `AGENTS.md` | aturan arsip "docs_old + reff_docs … akan dihapus" | hanya `reff_docs/`; `docs_old/` dicatat dipensiunkan + rujukan audit |
| `ai_skills/formspec-spec-structure/SKILL.md` + 3 salinan `examples/*/.agents/skills/` | "kode mengikuti `docs_old/spec/` sampai penerus ≥ Draft" | "kode yang berjalan adalah otoritasnya"; gotcha arsip hanya menyebut `reff_docs/` |
| `docs-site/.vitepress/config.mts` | 4 filter arsip `docs_old/**` | dihapus (folder-nya tidak ada lagi); `srcExclude` jadi `[]` + komentar |
| `docs_internal/plan/{backdate-override-resolve-stale,rename-formspec}.md` | rujukan `docs_old/…` | dibuang / dicatat dipensiunkan |
| `examples/kafe/gaps_found/{TODO.md,02-media-dan-qr.md}` | "Jangan edit docs_old", sumber `docs_old/spec/03-core-extended.md` | `reff_docs/` saja; sumber draft lama tanpa path |

**2. Hapus**: `git rm -r docs_old` — 49 file. Semua **tracked**, jadi pemulihan
tersedia lewat git history; tidak ada konten yang hanya hidup di sana.

## Koreksi yang menyertai

Draf pertama laporan audit mengklaim ada blocker kedua ("sweep entry L4–L6 ledger
belum ditutup"). Itu **salah**: `L4` di `11-reference.md:165` adalah **level
persona** ("L4 — Cloud Owner + admin"), bukan level validasi, dan `MIGRATION.md`
§5 baris 136 sudah mencatat verifikasi baris-per-baris D1–D50 **tuntas tanpa
sisa**. Klaim itu dibuang dari laporan (dengan catatan koreksi terbuka di
dokumen), dan langkah "tutup L4–L6" dihapus dari urutan rekomendasi. Jadi
penghapusan **tidak** ditahan atas dasar yang tidak ada.

## Bukti

- `grep -rn docs_old` (tanpa `reff_docs/`, `node_modules`, `dist`) → hanya
  menyisakan catatan "sudah dipensiunkan" + entri changelog **historis**
  (2026-08/09) yang memang tidak boleh diubah.
- `go build ./...` ok · `go test ./...` hijau · `tsc --noEmit` bersih (renderer
  **dan** `sdk/browser`) · `vitest run` **295** lulus.
- `npx vitepress build` di `docs-site/` → **build complete in 18.59s** (config
  yang disunting tetap valid; dead-link check tetap aktif).

**File terkena dampak**: 16 file (daftar di atas) + penghapusan 49 file
`docs_old/`, `docs_internal/audit/docs-old-pensiun-2026-09-22.md`,
`docs_internal/plan/todo.md`.
