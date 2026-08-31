# Fix: Kanban Card Template — UUID Tidak Informatif

**Tanggal:** 2026-07-29  
**File:** `renderers/web/src/kinds/kanban/KanbanRenderer.tsx`

## Perubahan

`KanbanCard` component sebelumnya mengakses `card.title` / `card.subtitle` / `card.assignee` langsung dari entity record — field yang tidak ada di data API → fallback ke `card.id` (UUID).

### Fix
1. **`resolveField(record, path)`** — helper untuk resolve dot-path dari record (mis. `patient.name` → `record.patient.name`)
2. **`KanbanCard`** sekarang menerima `record` + `template` (dari `entry.spec.card_template`) dan merender:
   - `title` dari `template.title` → resolved dari record (mis. `queue_number`)
   - `subtitle` dari `template.subtitle` → resolved (mis. `patient.name`)
   - `badge` dari `template.badge` → resolved
   - `assignee` dari `template.assignee` → resolved
   - `fields[]` dari `template.fields` → resolved per item (mis. `polyclinic.name`, `doctor.name`, `complaint`)
3. Tanpa `card_template` di manifest, fallback ke `record.name` / `record.id` (backward compatible)

**Referensi:**  
- Spec: `docs/spec/frontend/06-page-kinds.md` §4 Kanban — `card_template` dengan field `title`, `subtitle`, `badge`, `assignee`, `fields`  
- Manifest: `examples/Clinic-UI-Showcase/spec/modules/clinic/transaction/visit/kanbans/board.yaml`
