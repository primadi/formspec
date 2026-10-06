# 2026-10-04-005 — Validator nama role di `kind: Workflow` (kafe 5.13.8, Fase 0–1)

Fase 0–1 dari plan `docs_internal/plan/approval-duty-permission.md`.

## Masalah

`steps[].roles` adalah **lookup nama** yang tidak pernah diperiksa. Kafe
mendeklarasikan `roles: [cafe-order.supervisor]`, sedangkan role yang di-seed
bernama `supervisor`. Akibatnya berlapis:

- `formspec validate` membalas **89 manifest / 0 problem**;
- `workflow.Engine.CanApprove` mencocokkan nama **literal** → `403 "user does not
hold any of the step's required roles"`;
- `GET /_ui/workflow/approvals` membalas **daftar kosong** untuk satu-satunya
  approver yang ada;
- dan seandainya user memegang nama qualified itu, `RoleStore.GetByName`
  (`FindByField` eksak) tidak menemukan role-nya → resolver `continue // unknown
role — skip` → user itu **nol permission**. Nama yang salah jadi rusak dua kali.

Jadi alur void kafe **tidak bisa dijalankan**, dan tidak ada satu pun gate yang
mengatakannya. Kegagalan hanya terlihat dengan mencoba menyetujui.

## Yang landing (Fase 0)

`workflowRoleError` di `cmd/formspec/validate_workflow.go`, di-wire bersama
`workflowRejects` (`validate.go`):

- `buildRoleNameIndex(manifests)` membaca record role dari setiap `kind: Seed`
  (pola yang **sudah dipakai** `validate_seed_grants.go`: `isRoleEntityName` +
  `RawSpecTo[spec.SeedSpec]`), ditambah empat role owner yang **di-seed auth
  service sendiri** (`SeedOwnerRoles`) — tanpa itu, workflow yang menjaga
  transisi atas workspace-owner akan ditolak sebagai typo.
- Memeriksa **tiga** tempat yang menamai role: `roles`,
  `escalation.reassign_roles`, `escalation.notify_roles`.
- Pencocokan **eksak**, sesuai perilaku runtime — melonggarkan di sini berarti
  memberi tahu penulis sesuatu yang tidak disetujui engine.

**Keputusan yang sempat salah, lalu diperbaiki:** versi pertama saya melewati
pemeriksaan bila tree tidak mendeklarasikan role **sama sekali**. Preseden
checker sebelah (`validateScopeSources`: `canSupply` menyalakan error ketika tak
ada yang bisa memasok) menunjukkan itu keliru — "tidak ada role sama sekali"
berarti nama itu **pasti** tidak valid, dan melewatinya membuat instance paling
nyaring dari bug ini tidak dilaporkan. Guard itu dihapus.

## Yang landing (Fase 1)

`examples/kafe/spec/modules/cafe-order/workflows/order-void-approval.yaml`:
`roles: [cafe-order.supervisor]` → `[supervisor]`, `notify_roles:
[cafe-order.manajer]` → `[manajer]`.

Komentar yang **salah** dihapus: _"`cafe-order.supervisor` memetakan ke employee
berposisi supervisor/manajer"_. Tidak ada jembatan `position`→role di repo —
`position` hanya enum di `cafe-master/employee` dan tidak ada hook/script yang
menyentuh `roles`. Komentar yang menjelaskan mekanisme yang tidak ada itulah yang
membuat nama qualified tampak masuk akal.

## Temuan tambahan: tiga workflow lain di dua example

Pemeriksaan ini menemukan **4 workflow yang sama-sama tidak bisa disetujui** di
tree yang tidak punya role seed sama sekali:

| Example          | Workflow                       | Role yang tidak ada |
| ---------------- | ------------------------------ | ------------------- |
| `crc-management` | `cap-approval`                 | `crc.cap-approver`  |
| `crc-management` | `foreman-review`               | `crc.foreman`       |
| `crc-management` | `customer-approval`            | `crc.customer`      |
| `service-demo`   | `product-discontinue-approval` | `demo.manager`      |

Keempatnya **sudah merah sebelumnya** karena sebab lain (schema/integrator), jadi
tidak ada tree yang dari hijau menjadi merah — tetapi temuannya nyata dan
diserahkan ke Fase 4 (migrasi contoh) sebagai item tersendiri.

## Bukti

- **Properti pembuktinya terpenuhi:** setelah Fase 0 dan **sebelum** Fase 1, kafe
  menjadi **MERAH** dengan pesan yang menyebut nama yang dicari dan daftar role
  yang ada. Setelah Fase 1: **89 manifest / 0 problem**.
- `go test -count=1 ./cmd/formspec/` hijau. Test baru (5):
  `TestValidateWorkflows_RejectsUnknownRole` (regresi 5.13.8),
  `_AcceptsDeclaredRole`, `_ChecksEscalationRoles` (2 sub-test),
  `_AcceptsFrameworkOwnerRole`.
- Fixture lama kini mendeklarasikan role-nya lewat `withRoles(...)` — kalau tidak,
  mereka gagal karena pemeriksaan baru dan **menutupi** asersi yang sebenarnya
  diuji.
- **Dibuktikan gagal tanpa pemeriksaan:** `_RejectsUnknownRole` dan kedua sub-test
  `_ChecksEscalationRoles` merah saat pemanggilan `workflowRoleError` dihapus.
- `go test ./...` hijau · `make lint` 0 issues.

## File terdampak

- `cmd/formspec/validate_workflow.go` — `buildRoleNameIndex`, `workflowRoleError`,
  `describeRoleNames`, `frameworkRoleNames`.
- `cmd/formspec/validate_workflow_test.go` — `rolesSeedManifest`, `withRoles`,
  5 test baru, fixture lama di-seed.
- `examples/kafe/spec/modules/cafe-order/workflows/order-void-approval.yaml`.
- `docs_internal/plan/approval-duty-permission.md` (baru), `todo.md`.

## Sisa (→ todo)

- **5.13.8 ✅ DITUTUP** (Fase 0 + 1): nama role kini divalidasi, dan kafe memakai
  nama yang benar.
- **5.13.9 ⏸️** (baru) — 4 workflow di `crc-management` (3) dan `service-demo` (1)
  menamai role yang tidak pernah dideklarasikan di tree-nya. Ditinggalkan sebagai
  item karena perbaikannya adalah keputusan produk per example (role mana yang
  benar-benar menjadi approver), bukan mekanis.
- Fase 2–4 plan (identitas step, duty sebagai permission, grant
  `workflow:{name}`, drop `roles`) — belum dikerjakan, masih di plan.
