# Plan: Implementasi Fitur Kanban yang Belum Ada

**Tanggal:** 2026-07-29  
**Referensi Spec:** `docs/spec/frontend/06-page-kinds.md` §4 Kanban  
**Status Renderer:** `docs/renderers/shadcn-shell/03-kind-renderers.md` §3  
**Todo:** `docs/plan/todo.md` §5.5  

## Ringkasan Gap

| Fitur | Prioritas | Level of Effort |
|---|---|---|
| Drag & drop antar kolom | High | Large |
| Klik kartu → detail page/form | High | Small |
| PATCH status_field + optimistic rollback | High | Medium |
| WIP limit enforcement | Medium | Small |
| Row actions pada kartu | Medium | Medium |
| Filter columns (filters dari manifest) | Medium | Small |
| Search di kartu | ✅ sudah ada | — |
| Realtime subscription | Low | Large (tergantung §5.8) |
| drag_guard FormSpecExpr | Low | Medium |
| Zero-config derivasi kolom | Low | Medium |

## File yang Terkena

- `renderers/web/src/kinds/kanban/KanbanRenderer.tsx` — utama
- `renderers/web/src/types/manifest.ts` — jika perlu update type
- `renderers/web/src/lib/api/client.ts` — apiPatch sudah ada
- `renderers/web/src/engine/entityRef.ts` — resolveEntityRef sudah ada

## Dependensi

- `@dnd-kit/core` — sudah di `package.json`
- `@dnd-kit/sortable` — sudah di `package.json`
- `@dnd-kit/utilities` — perlu cek

## Urutan Implementasi

1. **Drag & drop** — wire DndContext + SortableContext per kolom
2. **Klik kartu → detail** — onClick navigate ke halaman entity detail
3. **PATCH status + optimistic rollback** — onDragEnd kirim PATCH, rollback on 409
4. **WIP limit** — cegah drop jika kolom penuh
5. **Row actions** — render tombol aksi di kartu
6. **Filter columns** — proses filters dari manifest

## Referensi

- Spec Kanban: `docs/spec/frontend/06-page-kinds.md` line 254+
- Type KanbanSpec: `pkg/spec/frontend.go` line 260+
- Type KanbanCard: `pkg/spec/frontend.go` line 280+
- Manifest contoh: `examples/Clinic-UI-Showcase/spec/modules/clinic/transaction/visit/kanbans/board.yaml`
