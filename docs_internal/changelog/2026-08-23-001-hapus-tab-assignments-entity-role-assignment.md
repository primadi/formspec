# 2026-08-23-001 — Hapus Tab Assignments & Entity role-assignment

## Apa yang diubah

Tab **Assignments** di Access Management dan entity `formspec.core.role-assignment`
dihapus karena redundan: penetapan role sudah dilakukan langsung di form User
(field `roles`), dan `role-assignment` **belum ter-wire ke enforcement** —
`PermissionResolver` hanya membaca `user.roles` (bukan record role-assignment).

### 1. Hapus tab Assignments

- `internal/auth/module/pages/access-management.yaml` — hapus tab
  `Assignments` (ref `role-assignment-table`); Access Management kini hanya
  Users + Roles.

### 2. Hapus entity role-assignment

- Hapus direktori `internal/auth/module/transaction/role-assignment/`
  (entity.yaml, forms/form.yaml, tables/list.yaml).
- `cmd/formspec/generate_auth_test.go` — hapus `transaction/role-assignment/
entity.yaml` dari daftar file yang diverifikasi; jumlah manifest 16 → 13
  (module + 6 entities + 3 forms + 2 tables + 1 page).

### 3. Bersihkan referensi

- Komentar Go: `internal/auth/core.go`, `resolver.go`, `service.go`,
  `resource/formspec.go` — hapus sebutan role-assignment.
- Deskripsi manifest: `internal/auth/module/module.yaml`,
  `master/role/forms/form.yaml`, `transaction/app-membership/entity.yaml`,
  `examples/cafe/spec/modules/formspec.core/module.yaml`.
- Spec: `docs/spec/platform/02-workspace-app-module.md` §9 (hapus baris
  `role-assignment`), `docs/spec/platform/08-project-layout.md`.
- Todo: `docs/plan/todo.md` (6.3.2, 6.3.5, 5.12.5).

## Kenapa diubah

`role-assignment` adalah mekanisme penetapan role per-App yang dirancang di
spec (`platform/02` §8), tapi implementasi saat ini memakai `user.roles`
(workspace-level) sebagai sumber enforcement — record role-assignment tidak
berpengaruh apa pun. Menampilkan tab yang tidak berfungsi membingungkan admin.
Entity dihapus total; jika penetapan role per-App diperlukan di masa depan,
bisa diimplementasikan ulang sesuai spec (todo terpisah).

## File yang terkena dampak

- `internal/auth/module/pages/access-management.yaml` — hapus tab Assignments
- `internal/auth/module/transaction/role-assignment/` — dihapus (3 file)
- `cmd/formspec/generate_auth_test.go` — update daftar file + jumlah manifest
- `internal/auth/core.go`, `resolver.go`, `service.go`, `resource/formspec.go`
  — komentar
- `internal/auth/module/module.yaml`, `master/role/forms/form.yaml`,
  `transaction/app-membership/entity.yaml`,
  `examples/cafe/spec/modules/formspec.core/module.yaml` — deskripsi
- `docs/spec/platform/02-workspace-app-module.md`,
  `docs/spec/platform/08-project-layout.md`, `docs/plan/todo.md` — referensi

## Referensi

- Spec: `docs/spec/platform/02-workspace-app-module.md` §8 (Identitas User &
  Membership), §9 (Resource Bawaan)
- Todo: 6.3.1, 6.3.2, 6.3.5, 5.12.5

## Verifikasi

- `go build ./...` hijau
- `go test ./...` hijau (733 pass)
- Browser: Access Management hanya menampilkan tab Users + Roles; entity
  role-assignment tidak muncul di bundle/grants editor.

## Catatan — bug hot-reload (pre-existing)

Edit pada `examples/cafe/spec/modules/formspec.core/module.yaml` memicu
hot-reload yang **menjatuhkan entity embedded module** (`formspec.core.user`,
`role`, dll) — reload hanya me-reload spec filesystem, bukan module embedded
(`internal/auth/module/`). Gejala: tab Users/Roles merender fallback
"Table: user-table" karena entity tidak ter-resolve. Solusi: restart dev server
(me-register ulang embedded module saat startup). Bug ini pre-existing, bukan
disebabkan penghapusan role-assignment.
