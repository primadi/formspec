# Fase 5 Completion — sisa item frontend shadcn-shell

**Status:** ✅ Complete (2026-08-24) · **Tanggal:** 2026-08-24
**Referensi:** `docs/spec/frontend/` (01–08), `docs/plan/todo.md` §5
**Todo:** `docs/plan/todo.md` §5.4.2–5.4.4, §5.5.3/5.5.5, §5.6, §5.7.3/5.7.4, §5.11.1–5.11.3/5.11.5, §5.13.1–5.13.4, §5.14.1, §5.16
**Changelog:** `docs/changelog/2026-08-24-027` s/d `2026-08-24-033`

## Konteks (state 2026-08-24)

Item Fase 5 yang **sudah terimplementasi & diverifikasi** (todo sudah di-update ✅):
5.2.3/5.2.4/5.2.5 (Page variants), 5.4.5 (realtime table), 5.7.1/5.7.2 (stat/chart widget),
5.8.1–5.8.3 (realtime hook), 5.11.4 (title interpolation), 5.12.1–5.12.3 (ETag, label_field,
entity schema shape), 5.14.2/5.14.3/5.14.4 (menu, form mode heuristic, lifecycle patterns),
5.15.2 (OverlayHost).

Sisa pekerjaan di bawah adalah yang **belum** terimplementasi.

## Workstream

### WS-A — Table: inline & batch editing (5.4.2, 5.4.3) ✅

- `inline_edit: true` — cell editable utk field non-readonly/computed/immutable; CAS per baris
  (`If-Match` + `recordVersion`); baris yang sudah di-submit menolak inline-edit.
- `batch_edit: [field, ...]` — update per baris (loop PATCH), partial failure dilaporkan
  (bukan all-or-nothing); checkbox selection sudah ada di `TableRenderer`.
- File: `kinds/table/TableRenderer.tsx`, `types/manifest.ts`, `pkg/spec/frontend.go` (bila perlu).

### WS-B — Table: column derivation fix (5.4.4, 5.14.1) ✅

- `engine/derive.ts` `deriveTable()` saat ini `slice(0, 8)` — kolom di luar 8 **diam-diam
  dibuang**. Ganti dengan N priority columns: `natural key` → `label_field` → `status` →
  `transaction_date` → sisanya (declaration order), lalu overflow diakses via row
  expand/detail — **tidak pernah dibuang diam-diam**.
- File: `engine/derive.ts`, `kinds/table/TableRenderer.tsx` (row expand).

### WS-C — Kanban: drag_guard + zero-config (5.5.3, 5.5.5) ✅

- `drag_guard` FormSpecExpr — pre-check UX sebelum drop (evaluasi guard terhadap record +
  target column; drop diblokir bila guard false).
- Zero-config — bila `columns` tidak di-author, derive dari `state_machine` states atau
  `group_by` enum field.
- File: `kinds/kanban/KanbanRenderer.tsx`, `engine/derive.ts`.

### WS-D — Calendar kind (5.6.1–5.6.6; 5.6.7 deferred) ✅

- Kind baru `Calendar` end-to-end: spec (`pkg/spec/frontend.go`), registry, bundle
  (`internal/ui/meta.go`), route, renderer `kinds/calendar/CalendarRenderer.tsx`.
- Views month/week/day/resource; event dari `date_field` + `end_field`, title dari
  `label_field`/`title_field`; click event → detail; click empty slot → Form create
  date pre-filled; drag reschedule via `update` action (server-enforced); RRULE
  (RFC 5545) expand render-time; resource view per `resource_field` + `color_field`.
- 5.6.7 (RRULE exception per-instance) tetap deferred.

### WS-E — Dashboard customizable + widget catalog (5.7.3, 5.7.4) ✅

- `customizable: true` — user add/remove/reorder widgets dari catalog; preference
  disimpan sebagai runtime preference (bukan YAML) — reuse `@dnd-kit` (sudah ada di
  Kanban) atau sortable.
- Widget catalog visibility — derived dari permission `list`/`view` user pada entity
  underlying (filter sudah ada utk widget terpasang; perluas ke catalog add).
- File: `kinds/dashboard/DashboardRenderer.tsx`, store preference.

### WS-F — FormSpecExpr (5.11.1, 5.11.2, 5.11.3, 5.11.5) ✅

- 5.11.1 Audit grammar vs `08-formspec-expr.md` §2 — pastikan lexer→parser→evaluator
  mendukung semua operator.
- 5.11.2 Deploy-time static validation — `formspec apply`/`check` menolak field reference
  tak-resolve + grammar invalid (ERROR).
- 5.11.3 Runtime error state — field reference tak ada → **visible error state** (saat ini
  `eval.ts` mengembalikan `null` diam-diam — melanggar spec; ubah ke error yang terlihat).
- 5.11.5 Cross-shell conformance test suite — interpretasi identik antar shell.
- File: `lib/formspec-expr/`, `internal/manifest/` (validation), `cmd/formspec/`.

### WS-G — Report: totals/subtotal/export + source.filter (5.13.1, 5.13.1a) ✅ (export async ⏸️)

- Fix bug totals row (nilai dihitung tapi `<tr>` kosong); grouping + subtotal.
- Export sebagai async job → file mendarat di download tray (bukan CSV Blob client-side).
- `source.filter` parameterized deklaratif (`":param"` placeholder).
- File: `kinds/report/ReportRenderer.tsx`, backend export job.

### WS-H — Print PDF server-side (5.13.2) ✅

- `format: html` via `window.print()` sudah ada; tambah PDF server-side generation.
- File: `kinds/print/PrintRenderer.tsx`, backend.

### WS-I — ApprovalInbox + NotificationCenter (5.13.3, 5.13.4) ✅

- `ApprovalInbox` — pending approvals list, `approve`/`reject` inline, badge count,
  `realtime: true`.
- `NotificationCenter` — notification list, badge unread, `mark-read`, `realtime: true`,
  deep-link on click.
- Kind baru end-to-end (spec, registry, bundle, route, renderer).

### WS-J — Renderer registry & resolution (5.16.1–5.16.3) ✅

- 5.16.1 Renderer resolution engine — pilih via `(implements, stack_family)`; hanya
  `official` auto-select; tanpa official → `formspec apply` error + saran kandidat;
  override via `renderers:` map + `renderer:` per-instance.
- 5.16.2 Slot-tier validation at apply.
- 5.16.3 `stack_family` compatibility check.
- File: `internal/manifest/`, `pkg/spec/`, `cmd/formspec/`.

## Urutan eksekusi (dependensi)

1. WS-B (derive columns) → WS-A (inline/batch edit) — keduanya di Table.
2. WS-C (Kanban).
3. WS-E (Dashboard) — kecil, cepat.
4. WS-F (FormSpecExpr) — backend + frontend.
5. WS-D (Calendar) — kind baru besar.
6. WS-I (ApprovalInbox + NotificationCenter) — kind baru.
7. WS-G (Report) + WS-H (Print).
8. WS-J (Renderer registry) — backend.

## Level of effort

| WS  | Effort |
| --- | ------ |
| A   | medium |
| B   | small  |
| C   | medium |
| D   | large  |
| E   | medium |
| F   | medium |
| G   | medium |
| H   | medium |
| I   | large  |
| J   | large  |
