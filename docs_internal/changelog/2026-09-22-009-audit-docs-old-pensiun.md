# 2026-09-22-009 — Audit kelayakan pensiun `docs_old/` (todo 9.5.1)

**Plan/Todo**: item **9.5.1** (`docs_internal/plan/todo.md`).
**Keputusan pemilik**: "audit dulu, laporkan temuannya, baru putuskan" —
karenanya **tidak ada file yang dihapus** dalam perubahan ini.

**Laporan lengkap**: `docs_internal/audit/docs-old-pensiun-2026-09-22.md`.

**Temuan utama**

1. **Migrasi kontennya TUNTAS.** 49 file diperiksa terhadap peta di
   `docs_old/MIGRATION.md` §2; setiap file yang punya penerus memang punya
   penerus **yang benar-benar ada di disk** (32 path diverifikasi; satu-satunya
   "MISS" adalah rename `forma-*` → `formspec-*`, penerusnya ada). Konsistensi
   ukuran ikut dicek (`04-control-plane.md` 237 → 323 baris; `05-frontend.md`
   603 baris → pecah ke `frontend/01–08`).
2. **Invariant `docs/` bersih**: `grep -rn docs_old docs/` → **0 hasil**.
3. **Tetapi belum boleh dihapus**: **8 rujukan di kode nyata masih menunjuk ke
   dalam `docs_old/`** — `cmd/formspec-ctl/main.go:11`,
   `sdk/browser/src/{error,types,client}.ts`, `internal/ui/registry.go:6`,
   `internal/manifest/examples_roundtrip_test.go:13`, `pkg/spec/spec.go:7–8`.
   Menghapus arsipnya lebih dulu membuat komentar itu menunjuk path mati —
   persis yang dilarang `MIGRATION.md` ("Kode boleh sementara menunjuk
   `docs_old/spec/...` sampai S5/S9").
4. **Satu invariant lain belum ditutup**: sweep eksplisit entry **L4–L6** ledger
   `11-reference.md`, yang `MIGRATION.md` §1 sendiri catat sebagai terbuka.

**Rekomendasi berurutan**: (a) perbaiki 8 rujukan kode → penerusnya sudah ada
semua (`docs/spec/backend/01-core-basic.md` §8.5, `docs/renderers/shadcn-shell/01-architecture.md`,
`docs/runtimes/01-formspec-ctl.md`, `docs/spec/frontend/01–08`,
`docs/spec/platform/04-control-plane.md`); (b) tutup catatan L4–L6; (c) baru
hapus `docs_old/`.

**File terkena dampak**: `docs_internal/audit/docs-old-pensiun-2026-09-22.md`
(baru), `docs_internal/plan/todo.md`. Tidak ada kode atau arsip yang diubah.

**Cakupan**: `reff_docs/` (1,1 MB) **di luar lingkup** dan tidak ikut dihapus —
ia tetap sumber eksternal/historis yang sah dirujuk kode (`docs/runtimes/05-…`).

**Bukti**: `grep -rn docs_old docs/` → 0; `grep -rn docs_old --include=*.go
--include=*.ts --include=*.tsx --include=*.mts .` (tanpa `node_modules`/`dist/`)
→ 8 rujukan non-`docs-site` yang terdaftar di atas.
