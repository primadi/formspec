# Implementasi Fitur Kanban Lengkap

**Tanggal:** 2026-07-29  
**Referensi Spec:** `docs/spec/frontend/06-page-kinds.md` §4 Kanban  
**Plan:** `docs/plan/kanban-full-implementation.md`  
**File:** `renderers/web/src/kinds/kanban/KanbanRenderer.tsx`

## Perubahan

Rewrite total `KanbanRenderer.tsx` dengan fitur-fitur berikut:

### 1. Drag & Drop (`@dnd-kit/core`)
- `DndContext` + `useDroppable` (kolom) + `useDraggable` (kartu)
- PointerSensor dengan activationConstraint 8px (mencegah accidental drag)
- `DragOverlay` — kartu mengambang (rotasi 3°, shadow-xl) saat di-drag
- Collision detection: `closestCorners`

### 2. Klik Kartu → Detail Page
- `KanbanCardContent.onClick` → `navigate(surfacePath(entityModule, entityPlural, id))`
- Navigasi ke halaman detail entity (terverifikasi: menampilkan semua field visit)

### 3. PATCH Status + Optimistic Rollback
- `handleDragEnd`: simpan snapshot → update state optimistik → PATCH ke server
- Gagal (error/409) → rollback ke snapshot + toast error
- Kirim `version` sebagai `If-Match` header untuk CAS

### 4. WIP Limit Enforcement
- Kolom penuh (`max_cards_per_column`) → border dashed merah + label "FULL"
- Drag ke kolom penuh ditolak dengan toast error

### 5. Row Actions
- `row_actions` dari manifest: view, edit, delete, custom actions
- Three-dot menu (`MoreHorizontal`) muncul saat hover kartu
- Confirm dialog untuk aksi yang punya `confirm_msg`
- Permission check via `checkPermission(perm, me.permissions)`
- Ikon dinamis: Eye (view), Edit2 (edit), Trash2 (delete)

### 6. Filter Columns
- `filters` dari manifest → Select dropdown per filter field
- Opsi unik dikumpulkan dari data record
- Filter client-side, komposisi dengan search

### 7. Search Improvement
- Search tetap client-side, dikombinasikan dengan filter

**Referensi:**  
- `@dnd-kit/core` v6.3.1, `@dnd-kit/sortable` v10.0.0 (dependency sudah ada)
- `apiPatch` / `apiDelete` dari `lib/api/client.ts`
- `useSurface` untuk surface-aware navigation
- `ConfirmDialog` dari `components/ui/confirm-dialog`
