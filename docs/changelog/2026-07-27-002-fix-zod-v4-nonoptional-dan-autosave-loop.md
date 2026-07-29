# Fix: Zod v4 `z.any()` Nonoptional Error + Auto-Save Infinite Loop

## Perubahan

### 1. `renderers/web/src/kinds/form/FormRenderer.tsx` — `buildZodField()`
* **Default case**: `z.any()` → `z.any().optional()`
* **Penyebab**: Proyek menggunakan Zod v4, bukan v3. Di Zod v4, `z.any()` di dalam `z.object()` menolak `undefined` dengan error `"Invalid input: expected nonoptional, received undefined"`.
* **Dampak**: Field `treatments` (child type) menghasilkan error zod saat tombol Save ditekan, karena nilainya `undefined` (tidak ada data treatment).

### 2. `renderers/web/src/kinds/form/FormRenderer.tsx` — Auto-save infinite loop
* **Tambahan**: `autoSaveBlockedRef` — ref boolean yang mencegah auto-save berulang setelah gagal.
* **Penyebab**: Setelah auto-save gagal (misal backdate policy violation), `isDirty` masih `true`, dan re-render berikutnya selalu menjadwalkan ulang auto-save setiap 2 detik → loop tak terbatas.
* **Fix**:
  * `autoSave()` set `autoSaveBlockedRef.current = true` setelah failure
  * Effect skip scheduling jika `autoSaveBlockedRef.current === true`
  * Unblock saat `isDirty` menjadi `false` (reset/undo) atau saat manual Save ditekan

## File Terkena Dampak
- `renderers/web/src/kinds/form/FormRenderer.tsx`
