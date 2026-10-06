# 2026-10-04-007 — Grant duty approval hidup + kafe bebas role hardcoded (Fase 4a)

Fase 4a dari plan `docs_internal/plan/approval-duty-permission.md`; lanjutan
`2026-10-04-006`.

## Masalah yang ditutup

Grant yang menjadi tujuan seluruh plan —
`{ page: "workflow:order-void-approval", actions: [{ name: supervisor-check }] }`
— **mematerialisasi ke nol**, karena `navigationFootprint`
(`internal/auth/materialize.go`) hanya mengenal enam kind
(`dashboard/report/wizard/kanban/timeline/print`). Terukur sebelum perubahan:
`/_ui/_meta/me` untuk `supervisor` berisi **44** permission, **tidak satu pun**
`workflow.*`. Jenis kegagalan yang sama dengan typo page: konfigurasi terlihat
benar dan tidak menegakkan apa pun.

## Yang landing

- **`case "workflow"`** di `navigationFootprint`: satu `FootprintAction` per step
  yang mendeklarasikan duty, di-key oleh **nama step** dan dengan permission
  **diturunkan** lewat `spec.StepPermission` — bukan ditranskripsi. Grant yang
  menamai step tanpa duty tetap dilaporkan sebagai problem, bukan diabaikan.
- **`Materializer.SetWorkflowDuties`** (lookup yang di-wire, bukan registry) —
  pola yang sama dengan `HandlerFactory.grantScopeLookup`, supaya `internal/auth`
  tidak bergantung pada engine workflow. **`nil` = grant `workflow:` dilaporkan
  sebagai problem**, bukan dibuang tanpa suara.
- **`workflow.Registry.GetByName`** — grant menulis nama workflow seperti yang
  dibaca penulis (`workflow:order-void-approval`), tanpa module; konvensi key
  `module/name` tetap di satu tempat.
- **Di-wire di `resource/formspec.go`** bersama materializer.

## Kafe: `roles:` dihapus dari step (bentuk yang dituju plan tercapai)

`order-void-approval.yaml` kini **tidak punya `roles:`** pada step-nya — hanya
`permission: supervisor-check`. Siapa boleh menandatangani ditentukan oleh grant,
bukan oleh nama role di manifest workflow. Itu inti permintaan awal.

**Konsekuensinya terbukti live, bukan diklaim:**

| Langkah                                                                | Hasil terukur                                                                                                                                         |
| ---------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| Grant duty dipasang                                                    | `/_ui/_meta/me` supervisor: **45** permission, termasuk `workflow.cafe-order.order-void-approval.supervisor-check`                                    |
| Alur penuh (duty-only)                                                 | `PATCH` → **202** · task di inbox `can_decide: true` · `POST` approve → **200** `transition_completed` · record `cancelled` + `void_reason` tersimpan |
| **Grant duty DICABUT** (DB saja, **manifest workflow tidak disentuh**) | permission turun ke **44**, `workflow.*` **hilang**                                                                                                   |
| Permintaan void setelah pencabutan                                     | `PATCH` → **202** (gate transisi masih dipegang)                                                                                                      |
| Inbox setelah pencabutan                                               | **0 task** — pemanggil tidak lagi eligible                                                                                                            |
| Approve setelah pencabutan                                             | **403** — `"user holds neither the step's duty permission (…) nor any of its roles"`; record tetap `paid`                                             |

Baris terakhir adalah buktinya: mengubah siapa yang boleh menyetujui **tidak lagi
memerlukan perubahan pada manifest workflow**. (Catatan operasional yang ikut
terukur: resolver meng-cache permission per sesi — `workspaceID/userID` — jadi
pencabutan grant baru berlaku setelah restart atau `Invalidate`.)

## Aturan baru: step yang tidak bisa disetujui siapa pun

`ValidateWorkflowStepNames` menolak step yang **tidak** mendeklarasikan `roles`
maupun `permission`. `hasAnyRole` terhadap daftar kosong bernilai false untuk
semua orang, jadi step seperti itu tidak akan pernah mencapai kuorum — dan itu
tidak dilaporkan di runtime, hanya "menunggu", yang terbaca sebagai "belum ada
yang sempat" alih-alih "tidak mungkin".

## Bukti

- **Gate yang sudah ada menangkap pekerjaan ini:** `TestKafeSeed_GrantsAllResolve`
  (setiap grant seed harus resolve) **MERAH** begitu grant duty dipasang, karena
  test itu membangun materializer seperti boot — dan boot kini memerlukan lookup
  workflow. Test itu diperbaiki dengan **mewire lookup yang sama seperti
  produksi**, bukan dengan melonggarkan asersinya.
- Test baru `internal/auth/workflow_duty_grant_test.go` (6): duty diturunkan &
  4 segmen · **tanpa registry dilaporkan, bukan dibuang** (regresi untuk
  pengukuran di atas) · workflow tanpa duty → "nothing to grant" · typo nama
  workflow · action yang menamai step tanpa duty · grant hanya menghidupkan step
  yang dinamainya (rantai multi-step).
- Test baru `pkg/spec/workflow_step_test.go` (4): step tanpa roles & permission
  ditolak · aturan nama (escalation, duty, unik, bentuk identifier) · duty
  4 segmen tidak dimulai dengan module (jadi `cafe-order.*` tidak menjangkaunya).
- `go test ./...` hijau · `make lint` **0 issues** · kafe `validate` **89/0** ·
  `vitest` 636/636 · `tsc` bersih · live end-to-end hijau.

## File terdampak

- `internal/auth/materialize.go` — `case "workflow"`, `SetWorkflowDuties`.
- `internal/workflow/registry.go` — `GetByName`.
- `resource/formspec.go` — wiring lookup.
- `pkg/spec/resources.go` — aturan step un-approvable.
- `examples/kafe/spec/modules/cafe-order/workflows/order-void-approval.yaml`
  (`roles:` dihapus), `.../formspec.core/seeds/roles.yaml` (grant duty).
- `internal/auth/kafe_seed_grants_test.go` (wire lookup), test baru di atas.

## Sisa (→ todo)

- **5.13.12 ⏸️** (baru) — `roles` **belum** dihapus sebagai hard error: 4 workflow
  di `crc-management` (3) dan `service-demo` (1) masih memakai role yang bahkan
  tidak dideklarasikan di tree-nya (**5.13.9 ⏸️**). Keduanya keputusan produk per
  example (siapa sebenarnya yang menyetujui), bukan penggantian nama mekanis —
  jadi migrasinya dan penghapusan `roles` dikerjakan bersama.
- **5.13.11 ⏸️** tetap: kuorum `mode: all` masih `len(step.Roles)`.
- **5.13.7 ⏸️**, **5.25.10 ⏸️** tidak berubah.
