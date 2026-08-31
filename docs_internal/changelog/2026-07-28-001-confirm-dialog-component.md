# ConfirmDialog Component

Ganti native `window.confirm()` dengan komponen React `ConfirmDialog` yang proper.

## Apa yang diubah

1. **File baru**: `renderers/web/src/components/ui/confirm-dialog.tsx`
   - Komponen reusable menggunakan shadcn/ui Dialog (base-ui) + lucide-react icons
   - 3 variant: `default` (AlertTriangle amber), `destructive` (XCircle merah), `warning` (TriangleAlert orange)
   - Animasi smooth: fade-in + zoom-in untuk icon dengan delay, fade-out + zoom-out untuk close
   - Props: `open`, `onOpenChange`, `title`, `message`, `variant`, `confirmLabel`, `cancelLabel`, `onConfirm`, `onCancel`

2. **Diedit**: `renderers/web/src/kinds/table/TableRenderer.tsx`
   - Ganti `window.confirm(action.confirm_msg)` → intercept click, tampilkan ConfirmDialog
   - Tambah state `pendingAction` + handler `handleConfirm`
   - Row action `delete`/`cancel` → variant `destructive` (ikon XCircle merah)
   - Row action lain → variant `default` (ikon AlertTriangle amber)

3. **Diedit**: `renderers/web/src/kinds/page/DetailPage.tsx`
   - State machine transition buttons: jika `action?.ui?.confirm` terdefinisi, tampilkan ConfirmDialog dulu sebelum execute API call
   - Tambah state `pendingTransition` + handler

## Mengapa
- `window.confirm()` tidak bisa di-styling, tidak konsisten antar browser
- Tidak accessible (focus trap, keyboard navigation)
- Melanggar UX pattern SPA modern

## Files affected
- `renderers/web/src/components/ui/confirm-dialog.tsx` (new)
- `renderers/web/src/kinds/table/TableRenderer.tsx` (edit)
- `renderers/web/src/kinds/page/DetailPage.tsx` (edit)

## Referensi
- Plan: `docs/plan/todo.md` (session memory: `/memories/session/plan.md`)
