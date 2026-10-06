# Plan — Sumber data `ApprovalInbox` (opsi b: endpoint `/_ui/workflow/approvals`)

Menutup **5.13.6 ⏸️**. Observasi pemicu: `GET /kafe/app/pos/approval-inbox/supervisor-inbox`
→ **"No approval source configured"**.

## Masalah (terverifikasi)

| Bukti                                                                         | Hasil                                                                   |
| ----------------------------------------------------------------------------- | ----------------------------------------------------------------------- |
| `ApprovalInboxRenderer.tsx` `APPROVAL_ENTITY_REFS`                            | mencari `formspec.core.approval` / `approval-task` / `workflow-task`    |
| `grep -rn 'name: approval' --include='*.yaml'`                                | 0 hasil — entity itu **tidak ada**                                      |
| `grep -rn 'formspec_workflow_approval' internal/ renderers/jsonb-persist/`    | hanya store + migrasi; **tidak ada route**                              |
| `docs/spec/frontend/06-page-kinds.md` §11                                     | kind ini **zero-config**: sumbernya step Workflow pending, bukan entity |
| `sqlite3 .formspec/kafe.db "select count(*) from formspec_workflow_approval"` | **0** (belum ada void yang diajukan — bukan penyebab pesan di layar)    |

Kontrak §11 bertentangan dengan renderer: renderer menuntut entity, kontrak
menyatakan sumbernya tabel approval framework. Karena `formspec_workflow_approval`
bukan Entity, tidak ada satu pun route yang mengeksposnya.

## Keputusan

Opsi **(b)**: endpoint khusus, satu-satunya yang sesuai kontrak.

- (a) entity bawaan: mustahil — `WorkflowApprovalRow` bukan baris entity (tak ada
  `title`/`display_fields`), dan tabel framework tak bisa dipetakan jadi Entity.
- (c) hapus `APPROVAL_ENTITY_REFS`: menyimpang dari kontrak zero-config.

## Bentuk kontrak baru

```
GET  /{ws}/_ui/workflow/approvals?app=<app>
POST /{ws}/_ui/workflow/approvals/{id}   {"decision":"approve"|"reject"}
```

Aturan yang dipilih (masing-masing punya alasan yang bisa diperiksa):

1. **Tenant + App scoping.** Baris difilter `tenant_id = workspace` dan
   `module ∈ App.modules`. `WorkflowApprovalStore.ListPending` **tidak** memfilter
   tenant (worker eskalasi menyapu lintas workspace) → dipakai method baru
   `ListPendingForTenant`.
2. **Eligibility = role pada step.** `workflow.Engine.CanApprove` — predikat yang
   **sama** dengan jalur approve nyata di `handleWorkflowApproval`. Requester
   tidak melihat task-nya sendiri (7.4.5).
3. **`can_decide` per baris** = caller memegang permission yang **sama** dengan
   route transisi (`RouteDescriptor.RequiredPermission` dari `b.routes`; fallback
   `{module}.{plural}.update` untuk transisi tanpa route). Inbox tidak boleh jadi
   bypass gate transisi — dan baris yang tidak bisa dieksekusi ditandai, bukan
   disembunyikan.
4. **Nilai `display_fields` hanya bila caller memegang `{module}.{plural}.view`.**
   Store read di sini melewati pemeriksaan permission HTTP, jadi pemberian nilai
   record harus dipagari eksplisit. Task-nya tetap terlihat; nilainya tidak.
5. **Approve/reject didelegasikan ke `handleWorkflowApproval`** — bukan
   diimplementasi ulang. Jalur itu yang menegakkan kuorum, 7.4.5, audit, dan
   (sejak 2026-09-21) emit event transisi.

## File

| File                                                                        | Perubahan                                                               | Effort |
| --------------------------------------------------------------------------- | ----------------------------------------------------------------------- | ------ |
| `renderers/jsonb-persist/workflow_approval.go`                              | `ListPendingForTenant`                                                  | small  |
| `internal/api/workflow_inbox.go` _(baru)_                                   | 2 handler + DTO + helper permission/plural                              | medium |
| `internal/api/router.go`                                                    | registrasi `/_ui/workflow/*`                                            | small  |
| `internal/api/workflow_inbox_test.go` _(baru)_                              | test HTTP (list, scope, eligibility, can_decide, decision)              | medium |
| `renderers/react-shadcn/src/lib/approvalInbox.ts` _(baru)_                  | path builder + test (pola `serviceCall.ts`: dibuktikan ke server nyata) | small  |
| `ApprovalInboxRenderer.tsx`                                                 | buang `APPROVAL_ENTITY_REFS`; baca endpoint                             | medium |
| `docs/runtimes/06-ui-rest-contract.md`                                      | kontrak endpoint                                                        | small  |
| `docs/spec/frontend/06-page-kinds.md` §11 + `docs/kind/ui/ApprovalInbox.md` | selaraskan (bukan lagi "entity konvensional")                           | small  |

## Sisa yang akan dibuka

- **5.13.7 ⏸️** — realtime: WS hub ber-topic `module/entity`, sedangkan approval
  bukan entity → `realtime: true` di manifest belum dihormati.
- **5.25.10 ⏸️** (sudah ada) — `navigationFootprint` belum mengenal
  `approval-inbox:`/`notification-center:`; tidak berubah oleh plan ini.
