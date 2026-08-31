# 2026-08-31-002 — Enforce ceiling DATASTORE_PERMISSION_DENIED per-operasi

**Plan:** `docs_internal/plan/infra-registry-3-level.md` (fase B2 follow-up)

## Apa yang diubah

Permission ceiling dari Workspace Binding (fase B2) kini **di-enforce
per-operasi** — sebelumnya ceiling hanya tersimpan/terekspose via
`Permission(service)`, belum memblokir operasi:

- `pkg/spec/datastore.go`: `DatastorePermission.Allows(op)` — nil = tanpa
  ceiling; `read` hanya operasi read; `write` implies read; `read_write`
  semua. Rules[] (glob module.table) tidak berlaku untuk operasi
  primitive-level.
- `resource/permissionguard.go` (BARU): decorator `permissionGuard` yang
  membungkus koneksi hasil `Resolve`/`ResolveNamed` bila ceiling restrictif.
  Mengimplementasikan semua capability interface secara struktural
  (Query/Get/Set/Delete/Acquire/Release/Enqueue/Dequeue/Publish/Subscribe/
  Upload/Download/Log); operasi yang melampaui ceiling →
  `DATASTORE_PERMISSION_DENIED` (platform/06-datastore.md §6), sisanya
  didelegasikan. Klasifikasi: read = query/get/download/subscribe; write =
  set/delete/acquire/release/enqueue/dequeue/publish/upload/log.
- `resource/datastoreregistry.go`: `Resolve` + `ResolveNamed` membungkus
  koneksi via `wrapWithPermission` bila ceiling restrictif (read_write/nil
  = tanpa guard, koneksi langsung).

## Kenapa

Menutup gap terakhir model 3-level: ceiling permission dari Workspace
Binding kini benar-benar membatasi — workspace dengan binding `read` tidak
bisa menulis ke service tersebut, sesuai normatif spec §4/§6.

## File terdampak

- `pkg/spec/datastore.go` — `Allows(op)`
- `resource/permissionguard.go` — BARU (decorator)
- `resource/datastoreregistry.go` — `wrapWithPermission` di
  Resolve/ResolveNamed
- Test: `resource/datastoreregistry_test.go` (+1 PermissionCeiling; update
  SnapshotBinding — koneksi snapshot kini dibungkus guard sesuai ceiling)

Verifikasi: 981 test Go lulus tanpa regresi.
