# 2026-10-05-001 — Approval inline di transisi Entity (`kind: Workflow` dihapus)

Approval tidak lagi manifest terpisah. Gate dideklarasikan langsung pada transisi
state machine Entity yang di-gate:

```yaml
- from: [paid, in_kitchen, ready, served]
  to: cancelled
  via: void-order
  approval:
    steps:
      - { name: supervisor-check, permission: supervisor-check, approvers: 1 }
    on_reject: { to: paid }
```

Alasan: (1) gate tidak bisa menyimpang dari hal yang di-gate, karena keduanya satu
deklarasi; (2) transisi multi-origin tercover **by construction** — lubang lama
(`from: paid, to: cancelled` yang hanya mengawal 1 dari 4 state asal sehingga void
dari `in_kitchen`/`ready`/`served` lolos approval tanpa error) tidak bisa
diekspresikan lagi; (3) satu manifest lebih sedikit untuk dibaca. Semantik runtime
tidak berubah: transisi ditahan, 202 + baris approval pending dibuat, quorum
dievaluasi per step, dan transisi dieksekusi lengkap (state + `emit` + audit)
saat semua step lolos. Requester tetap tidak bisa menyetujui permintaannya
sendiri.

**Identitas runtime.** Gate di-key `{module}/{entity}.{transition}`; duty step =
`workflow.{module}.{entity}.{transition}.{step}`; grant seed tetap
`{ page: "workflow:{entity}.{transition}", actions: [{name: {step}}] }`. Transisi
tanpa `via` tidak bisa diberi approval (tidak ada nama untuk merujuknya) dan
ditolak validator.

**File terdampak (kode).** `pkg/spec/entity.go` (`TransitionDecl.Approval`),
`pkg/spec/resources.go` (`ApprovalSpec{Steps, OnReject}`, `ValidateApprovalSpec`/
`ValidateApprovalSteps`; `WorkflowSpec`/`WorkflowTrigger`/`WorkflowTransitionRef`/
`WorkflowEscalation` dihapus), `pkg/spec/spec.go` (`KindWorkflow` dihapus),
`internal/manifest/loader.go` (branch Workflow + `RawSpecToWorkflowSpec` +
`KnownKinds["Workflow"]` dihapus), `internal/workflow/registry.go`
(`AddEntity` menurunkan gate dari transisi), `internal/workflow/engine.go`,
`internal/workflow/escalation.go`, `internal/api/handler.go`,
`internal/api/workflow_inbox.go`, `internal/auth/materialize.go`,
`resource/formspec.go` (`buildWorkflowRegistry` menurunkan dari Entity),
`cmd/formspec/validate_workflow.go`, `internal/genjsonschema/*`,
`internal/genkinddocs/markdown.go`, `renderers/react-shadcn/src/types/manifest.ts`
(`KIND_WORKFLOW` dihapus).

**Contoh.** 4 manifest Workflow → `approval:` inline + grant seed diperbarui:
kafe `order.void-order`, crc `checklist-document.{foreman_approve,
customer_approve, cap_approve}`, service-demo `product.discontinue`.

**Dokumentasi.** `docs/spec/backend/02-core-extended.md` §2 ditulis ulang (Workflow
→ Approval); `docs/spec/platform/03-kind-system.md` (33 kind, Data 10);
`docs/kind/data/Workflow.md` dihapus; `docs/kind/ui/ApprovalInbox.md`,
`docs/comparison/formspec-vs-frappe.md`, `docs/reference/glossary.md`,
`docs/guides/getting-started.md`, `ai_skills/*`, `.github/skills/formspec-backend`
diselaraskan. Skema digenerate ulang (`make generate-schema`).

**Bukti.** `go test ./...` hijau · `formspec validate --schema schemas`: kafe
88/0, crc 32/2, service-demo 13/1 (dua problem crc & satu service-demo
pre-existing — file report/integration/event yang tidak disentuh perubahan ini).
Plan: `docs_internal/plan/approval-inline-di-transisi.md`.
