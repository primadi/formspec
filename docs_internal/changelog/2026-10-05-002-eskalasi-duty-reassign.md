# 2026-10-05-002 — Eskalasi approval pakai duty (`reassign`), `notify_roles` dihapus

Eskalasi kini satu kosa kata dengan sisa approval: **permission**.

```yaml
steps:
  - name: supervisor-check
    permission: supervisor-check
    escalation: { after: 4h, reassign: manager-check } # duty, bukan nama role
```

## Kenapa

- **`reassign` adalah duty, bukan nama role.** Bentuk lama
  `reassign_roles: [manajer]` melanggar AGENTS.md aturan 6 (permission = resource
  - action, bukan nama role di YAML). Nama pendek di-qualify jadi
    `workflow.{module}.{entity}.{transition}.{reassign}` — aturan yang sama seperti
    `steps[].permission` — dan di-grant lewat
    `{ page: "workflow:{entity}.{transition}", actions: [{name: {reassign}}] }`.
    Mencabut grant mencabut hak takeover tanpa menyentuh manifest.
- **`notify_roles` dihapus, bukan diganti namanya.** Field itu **tidak punya
  pembaca** sejak awal — tidak ada kanal notifikasi sama sekali. Menggantinya
  menjadi permission tetap tidak memberi tahu siapa pun; yang salah bukan kosa
  katanya tapi ketiadaan pembacanya. Ia kembali sebagai `notify` (duty) saat
  kanalnya benar-benar ada.
- **Tiga bentuk ditolak `formspec validate`** karena tidak bisa berbuat apa-apa:
  `after` tanpa `reassign`; `reassign` tanpa `after`; `reassign` yang menunjuk
  duty step itu sendiri (eskalasi ke orang yang **sudah** boleh menyetujui = tidak
  mengubah apa pun).

## Perubahan kode

- `pkg/spec/resources.go` — `StepEscalation{After, Reassign}` (**bukan** lagi
  `NotifyRoles`/`ReassignRoles`); `DutyRef`; `EscalationPermission`;
  `ApprovalDuties` (duty step + duty eskalasi); `StepPermission` berbagi
  `qualifyDuty`; `validateStepEscalation`.
- `internal/workflow/engine.go` — `CanApprove` menerima duty eskalasi lewat
  `holdsEscalation`, yang juga menerima nama role lama (dual-read baris in-flight).
- `internal/workflow/escalation.go` — worker menyimpan duty ter-qualify di
  `escalated_steps`, bukan nama role.
- `internal/auth/materialize.go` — lookup `SetWorkflowDuties` mengembalikan
  `[]spec.DutyRef`, sehingga **duty takeover bisa di-grant** (ia bukan step, jadi
  tanpa ini grant tak bisa menyebutnya).
- `cmd/formspec/validate_workflow.go` — cek role `escalation` dibuang (target-nya
  bukan role lagi).

## Contoh & seed

5 eskalasi dimigrasikan + grant takeover ditambahkan:

| Gate                                            | Step              | reassign (duty baru)          | Pemegangnya (role)          |
| ----------------------------------------------- | ----------------- | ----------------------------- | --------------------------- |
| `cafe-order.order.void-order`                   | supervisor-check  | `manager-check`               | manajer                     |
| `crc-field.checklist-document.foreman_approve`  | foreman-sign-off  | `foreman-supervisor-sign-off` | crc.foreman-supervisor      |
| `crc-field.checklist-document.customer_approve` | customer-sign-off | `foreman-sign-off`            | crc.foreman                 |
| `crc-field.checklist-document.cap_approve`      | cap-sign-off      | `cap-supervisor-sign-off`     | crc.cap-approver-supervisor |
| `demo.product.discontinue`                      | manager-review    | `head-review`                 | demo.head                   |

## Bukti

`go test ./...` hijau · `formspec validate --schema schemas`: kafe **88/0**, crc
**32/2**, service-demo **13/1** (dua terakhir pre-existing). Skema digenerate
ulang. Test baru: aturan `validateStepEscalation` (4 kasus), kualifikasi duty
eskalasi, `ApprovalDuties`, dan duty takeover yang bisa di-grant.
