# Fix Table Auto-Refresh Setelah Overlay (Modal/Drawer) Close

**Tanggal**: 2026-08-24 · **Plan**: `docs/spec/05-frontend.md` §1.6 (render mode) · **Todo**: —

## Apa yang diubah

- **Bug**: setelah membuat (create) atau mengedit (edit) record lewat form
  modal/drawer (`?action=create&...` / `?action=edit&...`), daftar Table
  kembali tampil tapi **tidak auto-refresh** — record baru/ubah tidak muncul
  sampai halaman di-reload manual.
- **Akar masalah**: overlay create/edit bersifat URL-driven (`OverlayHost`
  membaca `action`/`form`/`entity`/`id`/`mode` dari query params). Saat
  overlay ditutup setelah save, URL params dihapus tetapi komponen
  `TableRenderer` tidak pernah unmount — dan efek fetch-nya tidak bergantung
  pada search params, sehingga list tidak di-refetch. Realtime juga tidak
  aktif untuk table derived (tidak ada `realtime` di `deriveTable`), jadi
  tidak ada mekanisme lain yang memicu refresh.
- **Perbaikan** (`renderers/react-shadcn/src/kinds/table/TableRenderer.tsx`):
  - `useSearchParams` kini menangkap `searchParams` (sebelumnya hanya
    setter-nya).
  - Tambah efek yang mendeteksi transisi overlay **open → closed**
    (`action` param ada → hilang) dan memicu **silent refetch**
    (`silentRefetch.current = true` + `setReloadKey`) — tanpa flash
    "Loading..." penuh, konsisten dengan mekanisme realtime yang sudah ada.
  - Berlaku untuk semua pemakaian `TableRenderer`: route CRUD derived
    standalone maupun table block/tab di dalam `PageRenderer`.

## File yang terkena dampak

- `renderers/react-shadcn/src/kinds/table/TableRenderer.tsx`

## Verifikasi

- Reproduksi bug: buat Table baru via modal → list tidak bertambah sampai
  reload. Setelah fix: list auto-refresh menampilkan record baru.
- Edit via modal → perubahan nama langsung tampil di list tanpa reload.
- `vitest run`: 126 test pass.
