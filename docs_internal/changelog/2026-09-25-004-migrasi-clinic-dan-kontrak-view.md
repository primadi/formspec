# Migrasi contoh `clinic` + perbaikan kontrak Form/Table sebagai target `view`

**Tanggal**: 2026-09-25 · **Plan**: `docs_internal/plan/routing-docs-and-menu-visibility.md`
**Todo**: 5.22.4 · **Changelog terkait**: `2026-09-25-002`, `2026-09-25-003`

## Apa yang diubah

- `examples/Clinic-UI-Showcase/spec/modules/clinic/module.yaml` — item
  "Pengaturan": `when: "user.has('clinic.settings.update')"` →
  `permissions: [clinic.settings.update]`, dengan komentar yang menyebut alasan
  (bentuk lama tidak bisa dievaluasi **dan** melanggar §3, sehingga item tampil
  untuk semua orang sambil terlihat dijaga).
- `docs/spec/platform/02-workspace-app-module.md` §4 — **hapus** klaim "`Form`
  dan `Table` **bukan** target `view` yang valid"; tambah kind navigasi yang
  sebelumnya tidak disebut (Calendar, Listing, ApprovalInbox,
  NotificationCenter); tambah sub-bagian "Visibilitas item menu — dua sumbu"
  (`permissions` vs `when`, plus catatan bahwa `_admin` tidak memakai keduanya).
- `docs/kind/curation/App.md` — kontradiksi narasi diperbaiki: baris "…bukan
  Form/Table" dan baris "…SEMUA visual kinds termasuk Form/Table" hidup
  berdampingan di satu daftar gotcha, saling bertentangan.
- `schemas/` + `docs/kind/` — hasil `make generate-schema` /
  `make generate-kind-docs` setelah `MenuItem.Permissions` ditambahkan.
- `.github/skills/formspec-frontend/SKILL.md` — baris routing + dua sumbu menu +
  himpunan callable tertutup.

## Kenapa

Kontradiksi Form/Table bukan sekadar salah tulis: `docs/kind/curation/App.md`
menyatakan keduanya sah (lewat derived Page wrapper), dan **kode mengikuti yang
itu** — `internal/auth/materialize.go` bahkan bergantung padanya lewat footprint
`{entity}-page`. Pembaca yang membuka spec platform lebih dulu akan menyimpulkan
kebalikannya, lalu menghapus fitur yang dipakai.

## Bukti

- Regenerasi mempertahankan narasi yang diperbaiki ("Narrative sections are
  preserved; only generated regions were refreshed") — `docs/kind/curation/App.md`
  diff 4+/2−, hanya narasi.
- `MenuItem` di `schemas/formspec.schema.json` kini: `children, icon, label,
module, permissions, route, type, view, when` — 9 properti, dengan deskripsi
  RBAC pada `permissions`.
- Sebelum regenerasi, spec contoh **gagal** schema di
  `/spec/menu/0/children/5: validation failed` (field `permissions` belum ada di
  schema) — jadi langkah ini memang wajib, bukan kosmetik.
- Validasi ulang seluruh `examples/`: kafe 85 manifest **0 problem**, storefront
  8/0, arisan 17/0; sisa problem di empat contoh lain adalah drift lama yang tidak
  menyebut `menu`/`when`/`permissions`/`listing` (ConfigKey, `report.parameters`,
  `deliver.channel`, dan honesty `uses`).
