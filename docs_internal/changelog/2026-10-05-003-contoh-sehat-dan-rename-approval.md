# 2026-10-05-003 — Tiga problem contoh ditutup + rename runtime approval (7.4.9)

## Bagian 1 — tiga problem validasi contoh

Dua kelas berbeda, dan hanya satu yang benar-benar "manifest salah".

| #   | Gejala                                                                                | Sebab sebenarnya                                                                                                                                                                                                                                                                                                                            | Perbaikan                                                                                                                                                                                           |
| --- | ------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | `report/checklist-summary-report.yaml` → `/spec/parameters/2/type: validation failed` | Parameter report memakai `type: date_range`, padahal itu kosa kata **`FilterSpec`** (terimplementasi di `TableRenderer`), bukan **`ReportParamType`** (`text · date · datetime · select · relation`). Report memakai pola dua tanggal.                                                                                                      | `transaction_date`+`date_range` → `date_from`/`date_to` bertipe `date`, sesuai contoh normatif `06-page-kinds.md` §8                                                                                |
| 2   | `product/entity.yaml` → `/spec/events/0/deliver/1/channel: validation failed`         | **Drift kode-vs-implementasi.** `EventChannel` tidak memuat `pubsub`, padahal `renderers/jsonb-persist/event_handler.go` **mengimplementasikannya** (`case "pubsub"`: non-durable at-most-once, channel dari `target.scope`). Enum-nya sendiri mengklaim "every name is implemented …" → schema menolak manifest yang bisa dilayani engine. | `pubsub` ditambahkan ke `EventChannel` (+ union TS `manifest.ts` + test); deskripsi enum dibuat jujur soal `queue` (belum dikirim, todo 7.7.6)                                                      |
| 3   | `crc-report/integration/sharepoint-archiver.yaml` → tanpa penangan pembatalan (7.7.2) | Integrator membuat efek samping saat `completed` (unggah arsip) sementara `crc-field.checklist-document` **tidak punya event pembatalan** — dokumen `completed` masih bisa menuju `cancelled` (`via: cancel`, `from: "*"`), jadi arsipnya bisa basi tanpa jalan menariknya.                                                                 | Event `on_cancel` pada entity + `emit: on_cancel` pada transisi `cancel`; action pembalik `remove` (idempoten, audit) pada Service `sharepoint-upload`; integrator baru `sharepoint-archive-cancel` |

Catatan #2: `queue` juga belum diimplementasikan tetapi tetap di enum (Core Basic

- dipakai contoh lain) — sisa itu sudah tercatat di **7.7.6 ⏸️**, bukan ditutup
  di sini.

## Bagian 2 — rename runtime (7.4.9)

Approval bukan kind sejak `2026-10-05-001`, tetapi nama runtime masih "workflow".
Nama Go diselaraskan dengan kontrak:

| Lama                                                                                                                                         | Baru                                                                      |
| -------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------- |
| paket `internal/workflow`                                                                                                                    | `internal/approval`                                                       |
| `spec.WorkflowStep` / `WorkflowStepMode` / `IsWorkflowStepMode` / `workflowStepModes` / `validateWorkflowStepMode` / `validWorkflowStepName` | `ApprovalStep` / … → `Approval*`                                          |
| `spec.WorkflowReject`                                                                                                                        | `ApprovalReject`                                                          |
| `db.WorkflowApprovalStore` / `WorkflowApprovalRow` / `NewWorkflowApprovalStore`                                                              | `ApprovalRequestStore` / `ApprovalRequestRow` / `NewApprovalRequestStore` |
| `Registry.WorkflowsFor` · `WorkflowInfo`                                                                                                     | `ApprovalsFor` · `ApprovalInfo`                                           |
| `SetWorkflowRegistry` / `SetWorkflowApprovalStore` / `SetWorkflowDuties`                                                                     | `SetApprovalRegistry` / `SetApprovalRequestStore` / `SetApprovalDuties`   |
| `handleWorkflowApproval` / `executeWorkflowTransition` / `recordWorkflowAudit`                                                               | `handleApproval` / `executeApprovalTransition` / `recordApprovalAudit`    |
| `Approval.WorkflowName` / `WorkflowModule` (dan row-nya)                                                                                     | `GateName` / `GateModule`                                                 |
| file `workflow_inbox.go`, `workflow_approval.go`, `validate_workflow.go`                                                                     | `approval_inbox.go`, `approval_request.go`, `validate_approval.go`        |
| test `TestWorkflowApproval_*`, `TestValidateWorkflows_*`, `TestMaterialize_Workflow*`                                                        | `TestApproval_*`, `TestValidateApprovals_*`, `TestMaterialize_Approval*`  |

**Sisa yang ikut ditemukan dan dibuang:** entri kind `"Workflow"` masih ada di
`internal/genkinddocs/markdown.go` (grup `data`) dan di union `ResourceKind`
(`renderers/react-shadcn/src/types/manifest.ts`). Keduanya lolos dari penghapusan
kind karena bukan anggota `KnownKinds` — persis kelas "daftar kedua" yang AGENTS
peringatkan.

**Sengaja TIDAK di-rename** — kontrak persisten, dinyatakan di kode dan
`docs/runtimes/06-ui-rest-contract.md` §5:

- tabel `formspec_workflow_approval`, kolom `workflow_module`/`workflow_name`
  (butuh migrasi DB);
- route `/_ui/workflow/approvals`;
- field wire `workflow`, `workflow_module` (kompatibilitas klien). Go field-nya
  kini `Gate`/`GateModule` dengan JSON tag lama, jadi klien tidak berubah.

**Bukti:** `go build ./...` + `go vet ./...` bersih · `go test ./...` hijau ·
`npx tsc -b` bersih · `vitest` area approval-inbox **10/10** · `formspec validate`
kafe **88/0**, crc **33/0**, service-demo **13/0**. Satu error TS pra-ada di
`formatApprovalField.test.ts` (objek uji kekurangan `value` yang wajib) ikut
diperbaiki karena membuat `tsc -b` gagal.
