# Plan — Approval inline di transisi Entity (hapus `kind: Workflow`)

Status: **selesai** (2026-10-05) — changelog `docs_internal/changelog/2026-10-05-001-approval-inline-di-transisi.md`

Tindak lanjut (2026-10-05): eskalasi ikut disatukan — `escalation: { after,
reassign }` dengan `reassign` berupa DUTY (permission), `notify_roles` dihapus.
Changelog `2026-10-05-002`.

Hasil: `go test ./...` hijau · kafe 88/0 · crc 32/2 & service-demo 13/1 (keduanya
pre-existing, tidak terkait perubahan ini).

**Sisa yang sengaja tidak dikerjakan** (bukan bagian permintaan, dan berisiko):
nama runtime masih memakai kata "workflow" — paket `internal/workflow`, tipe
`spec.WorkflowStep`/`WorkflowReject`, tabel `formspec_workflow_approval`,
`WorkflowApprovalStore`. Mengganti nama ini menyentuh skema DB dan API persisten,
jadi dipisahkan sebagai pekerjaan tersendiri. Fungsional tidak ada masalah.

## Keputusan

Approval dideklarasikan **langsung pada transisi** state machine Entity, bukan
sebagai manifest `kind: Workflow` terpisah.

```yaml
state_machine:
  field: status
  transitions:
    - from: [paid, in_kitchen, ready, served]
      to: cancelled
      via: void-order
      require_permission: orders.void-order
      approval:
        steps:
          - name: supervisor-check
            permission: supervisor-check
            approvers: 1
            mode: any
            escalation: { after: 4h, reassign: manager-check }
        on_reject: { to: paid }
```

Alasan: gate menyatu dengan hal yang di-gate, transisi multi-origin tercover
by construction, dan tidak ada dua manifest yang bisa menyimpang.

## Identitas runtime

- Workflow key = `{module}/{entity}.{transition}` (mis. `cafe-order/order.void-order`).
- Duty permission = `workflow.{module}.{entity}.{transition}.{step}`.
- Grant seed tetap `{ page: "workflow:{entity}.{transition}", actions: [{name: {step}}] }`.
- Row approval tetap menyimpan `WorkflowModule` + `WorkflowName`; `WorkflowName`
  kini `{entity}.{transition}`.

## Perubahan file

| File                                                                  | Perubahan                                                                                                                                                            |
| --------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `pkg/spec/entity.go`                                                  | `TransitionDecl.Approval *ApprovalSpec`                                                                                                                              |
| `pkg/spec/resources.go`                                               | `ApprovalSpec{Steps, OnReject}`; hapus `WorkflowSpec`/`WorkflowTrigger`/`WorkflowTransitionRef`/`WorkflowEscalation`; `ValidateApprovalSpec`/`ValidateApprovalSteps` |
| `pkg/spec/spec.go`                                                    | hapus `KindWorkflow` (const, IsValidKind, AllKinds)                                                                                                                  |
| `internal/manifest/loader.go`                                         | hapus branch validasi Workflow + `RawSpecToWorkflowSpec` + `KnownKinds["Workflow"]`                                                                                  |
| `internal/workflow/registry.go`                                       | `AddEntity(module, entity, es)` menurunkan approval dari transisi                                                                                                    |
| `internal/workflow/engine.go`                                         | tipe `*spec.ApprovalSpec`; `RequiresApproval(entity, transition)`                                                                                                    |
| `internal/workflow/escalation.go`                                     | `activeStep(*spec.ApprovalSpec, ...)`                                                                                                                                |
| `internal/api/handler.go`                                             | titik intersepsi + `handleWorkflowApproval`                                                                                                                          |
| `internal/api/workflow_inbox.go`                                      | `transitionNameOf`, `canRunTransition`                                                                                                                               |
| `internal/auth/materialize.go`                                        | doc + lookup `{entity}.{transition}`                                                                                                                                 |
| `resource/formspec.go`                                                | `buildWorkflowRegistry` → turunkan dari entity spec                                                                                                                  |
| `cmd/formspec/validate_workflow.go`                                   | validasi lintas-manifest dibaca dari transisi                                                                                                                        |
| `internal/genjsonschema/kinds.go`, `internal/genkinddocs/markdown.go` | hapus Workflow                                                                                                                                                       |
| examples (kafe, crc-management, service-demo)                         | 4 workflow → `approval:` di entity; update grant seed                                                                                                                |
| `docs/spec/backend/02-core-extended.md`                               | §2 ditulis ulang                                                                                                                                                     |
| `docs/kind/data/Workflow.md`                                          | dihapus; Entity.md menyebut `approval`                                                                                                                               |
| `docs/**`, `ai_skills/**`, `.github/skills/**`                        | rujukan `kind: Workflow` diselaraskan                                                                                                                                |

## Risiko

- Step identity, kuorum, eskalasi, requester exclusion dipin oleh test — tetap
  dipertahankan apa adanya; yang berubah hanya sumber deklarasi.
- `schemas/` digenerate ulang (`make generate-schema`).

## Verifikasi

`go build ./...` → `go test ./pkg/spec/... ./internal/workflow/... ./internal/api/...`
→ `go test ./...` → `bin/formspec validate examples/kafe` (dan crc/service-demo).
