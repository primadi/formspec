# 2026-10-04-011 — Eskalasi level workflow ditolak; eskalasi crc jadi nyata (5.13.14)

Menutup **5.13.14**.

## Deklarasi yang tidak bisa berbuat apa-apa

`WorkflowEscalation` (`spec.escalation`) hanya punya `after` + `notify_roles` —
**tanpa `reassign_roles`**. Sementara satu-satunya efek eskalasi yang
diimplementasikan adalah **reassignment** (`EscalationWorker` membaca
`step.Escalation.ReassignRoles`), dan pengiriman notifikasi **belum ada sama
sekali**. Jadi bentuk itu tidak bisa menghasilkan apa pun, di level mana pun:

> "approval ini dieskalasi setelah 48h" — dan tidak pernah terjadi apa-apa.

**Terukur:** ketiga approval `crc-management` mendeklarasikannya di level
workflow. Eskalasi 48h/72h mereka tidak pernah berjalan sejak awal.

## Yang landing

1. **`formspec validate` menolaknya**, dengan alasan dan jalan keluar: pindahkan
   timeout ke `steps[].escalation`, satu-satunya tempat yang punya
   `reassign_roles` — dan satu-satunya tempat yang bisa menyatakan **apa yang
   harus terjadi**. Struct-nya tetap ada (seperti `mode: all`) supaya penulis yang
   menulisnya mendapat penjelasan, bukan "unknown field".
2. **Ketiga workflow crc dimigrasikan** ke eskalasi step-level dengan reassign
   yang **nyata**, memakai role supervisor yang dibuat di 5.13.9:

   | Workflow            | Eskalasi sekarang                                              |
   | ------------------- | -------------------------------------------------------------- |
   | `foreman-review`    | `after: 48h` → `reassign_roles: [crc.foreman-supervisor]`      |
   | `cap-approval`      | `after: 48h` → `reassign_roles: [crc.cap-approver-supervisor]` |
   | `customer-approval` | `after: 72h` → `reassign_roles: [crc.foreman]`                 |

   `notify_roles` **sengaja tidak ditulis** di sana: deklarasi yang tidak punya
   akibat hanya menyesatkan pembaca (lihat 5.13.15).

3. **Contoh di dokumentasi diperbaiki** — `docs/spec/backend/02-core-extended.md`
   §2, `docs/kind/data/Workflow.md`, dan `ai_skills/formspec-kinds/SKILL.md`
   semuanya memakai bentuk mati itu di contohnya. Sekarang eskalasi ditulis di
   step, dan spec **menyatakan terus terang** bahwa `notify_roles` belum
   dikirim: sebuah deklarasi yang tampak berbuat sesuatu tetapi tidak melakukan
   apa pun adalah persis kelas kegagalan yang berkas itu tutup di tempat lain.

## `notify_roles`: dinyatakan, bukan disembunyikan

Saya **tidak** menjadikannya error, karena field-nya masih dipakai (kafe memakai
`notify_roles: [manajer]` di step-nya) dan menghapusnya akan membuang pernyataan
niat yang berguna. Yang saya lakukan: **mengatakannya di tempat yang akan dibaca
orang** — spec §2.1 dan kind reference kini menyatakan bahwa tidak ada kanal yang
memberi tahu siapa pun, dan itu dilacak sebagai **5.13.15 ⏸️**.

## Bukti

- **`TestValidateWorkflowSteps_WorkflowLevelEscalationIsRefused`**: bentuk level
  workflow ditolak dengan pesan yang menyebut `no effect`, `reassign_roles`, dan
  `steps[].escalation`; timeout yang sama **di step** diterima.
- Validator juga memeriksa nama role `escalation` (termasuk `reassign_roles` dan
  `notify_roles`): **dibuktikan** dengan mengganti `reassign_roles` menjadi role
  yang tidak ada → merah, menyebut nama yang dicari dan daftar role yang ada.
- **Seed crc dijalankan sungguhan** (5 role) dan grant duty-nya tersimpan di kolom
  `grants` seperti yang diharapkan.
- `go test ./...` hijau · `make lint` **0 issues** · kafe **89/0** · crc **2**
  (pra-ada) · service-demo **1** (pra-ada) · storefront **0** · skema
  di-regenerate.

## File terdampak

- `pkg/spec/resources.go` — `ValidateWorkflowSteps` menolak `wf.Escalation`;
  doc-comment `WorkflowEscalation` menyatakan kenapa.
- Ketiga workflow `crc-management` → eskalasi step-level.
- `docs/spec/backend/02-core-extended.md`, `docs/kind/data/Workflow.md`,
  `ai_skills/formspec-kinds/SKILL.md`.
- `pkg/spec/workflow_step_test.go` — test baru.

## Sisa (→ todo)

- **5.13.15 ⏸️** (baru) — `notify_roles` belum dikirim: tidak ada kanal yang
  memberi tahu role yang dideklarasikan. Implementasikan kanal notifikasi, atau
  hapus field-nya dari kontrak.
- **5.13.12 ⏸️** — apakah eligibilitas `roles`-saja di-deprecate (`roles` tetap
  wajib untuk `mode: sequential`).
- **5.13.7 ⏸️** — realtime inbox.
