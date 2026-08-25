# Workflow Audit Trail (7.4.6) — Approval = Signed Statement

**Tanggal:** 2026-08-25 · **Todo:** §7.4.6 · **Plan:** `docs/plan/fase7-workflow-audit-trail.md`

## Apa yang ditambahkan

Workflow engine kini mencatat setiap keputusan approval sebagai **pernyataan
bertanda tangan di audit trail bisnis** (02-core-extended.md §2, 01-core-basic.md
§11):

- **`db.WriteAuditLog`** (renderers/jsonb-persist/audit.go) — exported wrapper
  untuk `writeAuditLog` (sebelumnya hanya dipakai CRUD layer).
- **`recordWorkflowAudit`** (internal/api/handler.go) — helper best-effort yang
  mencatat keputusan workflow ke `formspec_audit_log`:
  - `workflow.approve` — actor = approver, changes = `{workflow, step, from,
    to, decision: "approve"}`.
  - `workflow.reject` — actor = approver, changes = `{workflow, step, from,
    to, decision: "reject"}`.
  - `workflow.transition` — actor = system, changes = `{to}` (saat transisi
    dieksekusi setelah semua step disetujui).
- **`SetAuditWriter`** (internal/api/router.go + HandlerFactory) — wiring di
  `resource/formspec.go` (boot + reload) via `db.WriteAuditLog`.

## Verifikasi end-to-end (via `formspec dev` + curl)

- Product discontinue → approve oleh manager → audit trail berisi
  `workflow.approve` (actor = manager-1) + `workflow.transition` (actor =
  system). ✅
- `go test ./...` hijau (816 pass).

## File terdampak

- `renderers/jsonb-persist/audit.go` (`WriteAuditLog` exported)
- `internal/api/handler.go` (`recordWorkflowAudit` + wiring approve/reject/transition)
- `internal/api/router.go` (`SetAuditWriter`)
- `resource/formspec.go` (wire audit writer boot + reload)