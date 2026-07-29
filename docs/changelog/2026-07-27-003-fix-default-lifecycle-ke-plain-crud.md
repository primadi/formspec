# Fix: Default Lifecycle `plain_crud` + Submit Button `two_step_autosave`

## Perubahan

### 1. `renderers/web/src/engine/lifecycle.ts` — Default lifecycle
* **Sebelum**: `default` case sama dengan `two_step_autosave` → auto-save aktif untuk semua entity tanpa explicit lifecycle
* **Sesudah**: `default` → `plain_crud` → auto-save hanya jika entity explicit set `lifecycle: two_step_autosave`
* **Alasan**: Auto-save sebagai default tidak intuitif dan bisa menyebabkan error (contoh: backdate policy violation pada auto-save)

### 2. `renderers/web/src/kinds/form/FormRenderer.tsx` — Submit button
* **Sebelum**: Submit button hanya tampil untuk `two_step_manual`
* **Sesudah**: Submit button tampil untuk `two_step_manual` DAN `two_step_autosave`
* **Alasan**: `two_step_autosave` di lifecycle.ts punya `hasSubmit: true` tapi tombol Submit tidak pernah dirender — inkonsisten dengan pattern spec

## File Terkena Dampak
- `renderers/web/src/engine/lifecycle.ts`
- `renderers/web/src/kinds/form/FormRenderer.tsx`
