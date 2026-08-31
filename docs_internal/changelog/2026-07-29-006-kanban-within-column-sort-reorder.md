# Kanban — Within-Column Sort + Drag-to-Reorder

**Tanggal:** 2026-07-29  
**Referensi Spec:** `docs/spec/frontend/06-page-kinds.md` §4 Kanban, `docs/spec/backend/01-core-basic.md` §6  
**Plan:** `docs/plan/kanban-full-implementation.md` (via `/memories/session/plan.md`)  
**File:** `renderers/jsonbpersist/crud.go`, `pkg/spec/frontend.go`, `renderers/web/src/kinds/kanban/KanbanRenderer.tsx`, `examples/.../visit/entity.yaml`, `examples/.../kanbans/board.yaml`, `docs/spec/`

## Perubahan

### Step 0: Type-aware cast di `columnRefExpr` (`renderers/jsonbpersist/crud.go`)
- `columnRefExpr()` sekarang lookup `Field.Type` dari entity spec
- Integer → `::integer`, decimal/number → `::numeric`/`::REAL`, boolean → `::boolean`, date → `::date`, datetime → `::timestamp`
- Fungsi bantu baru: `castTypeForField(ft spec.FieldType, driver DriverType) string`
- Developer tidak perlu `index: true` untuk sort/filter numeric yang benar
- Backward compat: field tidak dikenal tetap fallback ke text tanpa cast

### Phase A: `KanbanSpec` — field `Sortable` + `PositionField`
- `Sortable bool` — enable within-column drag-to-reorder (default false)
- `PositionField string` — nama field entity penyimpan posisi (misal `"queue_position"`)
- `sort_by` / `sort_order` dihapus — renderer auto-default `?sort=position_field`

### Phase B: Entity `visit` — field `queue_position`
- `{ name: queue_position, type: integer }` — nullable, tanpa index
- Posisi diisi via drag-to-reorder; NULL sort di akhir

### Phase C: Kanban manifest — `sortable` + `position_field`
- `consultation-board.yaml`: `sortable: true`, `position_field: queue_position`

### Phase D: Frontend — Sort + Drag-to-Reorder dalam Kolom
- `fetchRecords`: auto-append `?sort=position_field` saat `sortable: true`
- `KanbanColumn`: wrapping cards dalam `SortableContext` + `verticalListSortingStrategy`
- `DraggableCard` → `SortableCard`: ganti `useDraggable` dengan `useSortable`
- `handleDragEnd`: bedakan cross-column drop vs within-column reorder
- Posisi kalkulasi midpoint (gap 1000) antara tetangga saat reorder
- Cross-column juga reset `queue_position` ke max+1 kolom tujuan

### Phase E: Spec docs
- `docs/spec/frontend/06-page-kinds.md` §4: dokumentasi `sortable`, `position_field`
- `docs/spec/backend/01-core-basic.md` §6: dokumentasi type-aware sort & filter
