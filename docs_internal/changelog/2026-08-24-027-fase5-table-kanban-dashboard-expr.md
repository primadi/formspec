# 2026-08-24-027 — Fase 5 Completion: Table/Kanban/Dashboard/FormSpecExpr (5.4.2–5.4.4, 5.5.3, 5.5.5, 5.7.3, 5.7.4, 5.11.1–5.11.3)

## Apa yang diubah

Menyelesaikan sisa item Fase 5 frontend shadcn-shell (plan: `docs/plan/fase5-completion.md`).

**Table (5.4.2–5.4.4, 5.14.1):** inline editing (`inline_edit: true`, cell editable utk field
non-readonly/computed/immutable, CAS per baris, 409 → stale badge), batch editing
(`batch_edit: [field,...]`, loop PATCH per baris, partial failure dilaporkan), dan column
derivation fix (N priority columns: natural key → label_field → status → transaction_date →
sisanya; overflow diakses via row expand — tidak pernah dibuang diam-diam). Item ini sudah
terimplementasi di code; todo di-sync dengan kenyataan.

**Kanban (5.5.3, 5.5.5):** `drag_guard` FormSpecExpr (pre-check UX sebelum drop — evaluasi
`fields`=record + `target`=status kolom tujuan; drop diblokir bila guard false; server tetap
otoritas) dan zero-config columns (`deriveKanbanColumns`: state machine states → `enum_values`
status field; empty-state hint bila tak ada).

**Dashboard (5.7.3, 5.7.4):** `customizable: true` — user add/remove/reorder widgets dari
katalog via `@dnd-kit/sortable`; layout disimpan sebagai runtime preference
(`usePrefsStore.dashboardLayouts`, localStorage — bukan YAML). Widget catalog visibility
di-filter permission `list`/`view` pada entity underlying.

**FormSpecExpr (5.11.1–5.11.3):** audit grammar (lengkap sesuai spec §2, 94 test pass);
deploy-time static validation di `formspec check` (`checkKanban` drag_guard + `checkWizard`
step fields + `validateExprGrammar` tolak `ctx.`/def/import/return/delimiter tak seimbang);
runtime error state (`strictEvalFormSpecExpr` + `FormRenderer` banner error per field —
tidak silent fail-safe).

## File terdampak

- `renderers/react-shadcn/src/engine/derive.ts` (deriveKanbanColumns, priority columns)
- `renderers/react-shadcn/src/kinds/kanban/KanbanRenderer.tsx` (drag_guard, zero-config)
- `renderers/react-shadcn/src/kinds/dashboard/DashboardRenderer.tsx` (customizable + catalog)
- `renderers/react-shadcn/src/stores/prefs.ts` (dashboardLayouts)
- `renderers/react-shadcn/src/lib/formspec-expr/` (strictEvalFormSpecExpr + tests)
- `renderers/react-shadcn/src/kinds/form/FormRenderer.tsx` (error state)
- `cmd/formspec/check.go` + `check_test.go` (checkKanban/checkWizard/grammar)
- `renderers/react-shadcn/src/types/manifest.ts` (FieldType + text/richtext/file)

## Referensi

- Plan: `docs/plan/fase5-completion.md`
- Todo: `docs/plan/todo.md` §5.4.2–5.4.4, §5.5.3, §5.5.5, §5.7.3, §5.7.4, §5.11.1–5.11.3, §5.14.1
