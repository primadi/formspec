# 2026-09-22-007 — Permission-gating tombol action di renderer (lanjutan 5.12.4)

**Plan/Todo**: sisa dari **5.12.4** (`docs_internal/plan/todo.md`) — keputusan
manusia "tutup sekarang (server kirim can_\* / renderer sembunyikan)".

Temuan dari verifikasi 5.12.4: `ActionSummary.permission` selalu dikirim server,
tetapi **renderer tidak pernah membacanya** (`grep '\.permission'` → 0 hit), jadi
tombol action selalu tampil dan kegagalan baru terlihat sebagai toast 403 setelah
klik.

**Akar yang lebih buruk dari sekadar "tidak dibaca"**: empat situs render menulis
sendiri bentuk turunannya, `${entity.module}.${entity.plural}.${action}` — yang
**mengabaikan `required_permission` eksplisit** pada action. Itu tepat kasus yang
`ActionSummary.permission` ada untuk menyampaikannya, jadi untuk action dengan
permission kustom string yang dipakai untuk gating **salah**.

**Perbaikan** — dua helper bersama di `engine/permissions.ts`:

- `entityActionPermission(entity, action)` — permission yang benar-benar diminta
  sebuah action (pakai `ActionSummary.permission` bila ada, jika tidak bentuk
  turunan).
- `canDoEntityAction(me, entity, action)` — fail-closed bila identitas null.

Dipakai di 5 situs: Table (row action, batch edit, inline edit), Kanban (menu
baris), DetailPage (tombol transisi), Form (Save/Create-Submit/Submit).

**Dua bug nyata yang ketemu saat pemasangan:**

1. **`can()` TS tidak punya cabang wildcard tingkat-module.** Komentarnya
   mengklaim "parity with Go", tetapi hanya menangani `{module}.{entity}.*`;
   `billing.*` **gagal** mencocokkan `billing.orders.delete`. Itu bukan skenario
   teoretis — `billing.*` persis grant `RoleModuleOwner`
   (`internal/auth/owner_test.go:11`). Akibatnya pemegang module-owner akan
   **kehilangan seluruh tombol action** walau server mengizinkan. Cabangnya
   ditambahkan (paritas dengan `Identity.HasPermission`) + 7 test baru
   (`src/engine/permissions.test.ts`).
2. **Inkonsistensi antar-renderer**: untuk entity yang di-allow lewat `view`
   (tanpa `list`), Table sudah menyembunyikan tombol sementara DetailPage /
   Kanban / Form tidak. Kini semuanya memakai aturan yang sama.

**File terkena dampak**: `renderers/react-shadcn/src/engine/permissions.ts`,
`.../src/engine/permissions.test.ts` (baru), `.../src/kinds/table/TableRenderer.tsx`,
`.../src/kinds/kanban/KanbanRenderer.tsx`, `.../src/kinds/page/DetailPage.tsx`,
`.../src/kinds/form/FormRenderer.tsx`, `docs_internal/plan/todo.md`.

**Bukti**: `src/engine/permissions.test.ts` **gagal lebih dulu** pada kasus
`billing.*` (`true` diharapkan, `false` didapat) lalu hijau setelah perbaikan;
`vitest` **295 lulus** (20 file, +7); `npx tsc --noEmit` bersih; `go test ./...`
hijau.

**Catatan jujur**: `row_actions` yang diturunkan engine (view/edit/delete) kini
ikut tersaring bila caller tidak punya permission-nya. Perubahan perilaku ini
**disengaja** (tombol yang tak bisa dipakai tidak ditawarkan) tetapi **belum
diverifikasi di browser nyata**.

**Sisa → item 5.12.8 ⏸️** (baru): `BulkActionsBar` merender tombol per
`bulk_actions` **tanpa `onClick`** — fitur itu dekoratif, sementara Batch edit di
bar yang sama berfungsi nyata. Bukan bagian dari item ini (yang kurang handler,
bukan permission); dicatat dengan bukti pengamatan di todo.
