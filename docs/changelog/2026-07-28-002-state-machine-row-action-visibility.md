# Row action visibility berdasarkan state machine

**Apa yang diubah:**  
Filter `row_actions` di TableRenderer (frontend) untuk menyembunyikan action yang tidak bisa dilakukan dari state saat ini.

**Kenapa diubah:**  
Sebelumnya row actions hanya difilter berdasarkan permission — action seperti "start-consultation" tetap muncul untuk visit berstatus "completed" meskipun state machine tidak mengizinkan transisi tersebut. Sekarang action juga difilter berdasarkan state machine: jika status row tidak termasuk `from` pada transition yang via action tersebut, tombol disembunyikan.

**File yang terkena dampak:**  
- `renderers/web/src/kinds/table/TableRenderer.tsx` — menambahkan helper `isActionAllowedForRow` + filter kedua di rendering row_actions

**Catatan:**  
Guard expressions (Starlark) tetap dievaluasi server-side. Filter frontend hanya mengecek `from` states — guard diabaikan karena memerlukan runtime Starlark.
