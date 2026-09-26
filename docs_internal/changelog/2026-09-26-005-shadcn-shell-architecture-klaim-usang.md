# `shadcn-shell/01-architecture.md` §5: enam klaim usang diperbaiki (todo 5.22.8)

## Apa yang diubah

§5 "Status Implementasi Hari Ini" memuat **enam** klaim yang sudah tidak benar —
semuanya menandai sesuatu sebagai belum ada atau kode mati padahal sudah
berfungsi. Setiap klaim diverifikasi dulu ke kode, baru ditulis:

| Klaim lama                                                       | Kenyataan (terverifikasi)                                                                                                                   |
| ---------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------- |
| `OverlayHost` "ada tapi tidak terhubung ke jalur hidup mana pun" | Dipasang di **tiga** shell (`SideNavShell:153`, `TopNavShell:248`, `AuthPage`) dan dibuka lewat `?action=&form=&mode=` dari `TableRenderer` |
| `engine/registry.tsx` kode mati                                  | **File-nya sudah dihapus** (`ls` → tidak ada)                                                                                               |
| `deriveMenuItems()` kode mati                                    | Dipakai `hooks/useResolvedMenu.ts:109` untuk cabang `_admin` (todo 5.14.2)                                                                  |
| `TableRenderer` hardcode `/_admin`                               | `grep -n "_admin" TableRenderer.tsx` → **0 hit**; navigasi memakai `useSurface().surfacePath`                                               |
| Realtime "belum ada implementasi apapun"                         | `hooks/useRealtime.ts` ada dan dipakai 7 renderer (Table, Kanban, Calendar, Dashboard, Timeline, ApprovalInbox, NotificationCenter)         |
| Component contract `asset` "belum sama sekali"                   | `shell/AssetRenderer.tsx` memuat ES module, `mount(el, props, formspec)` / `unmount(el)`                                                    |

**Yang penting: §5 bertentangan dengan dokumen di repo yang sama.**
`03-kind-renderers.md:60` sudah lama menulis dengan benar ("Navigasi memakai path
permukaan aktif (`useSurface().surfacePath`), bukan prefiks `/_admin`", "`Form.render`
**dihormati**"). Jadi pembaca yang membandingkan dua halaman itu mendapat jawaban
berlawanan — persis kelas misinformasi yang `AGENTS.md` §3 larang.

Section juga diberi satu paragraf **cara pakai**: ia memuat divergensi yang masih
berlaku, dan barisnya dihapus begitu tertutup (bukan ditumpuk narasi "dulu X
sekarang Y" — itu ke `docs_internal/changelog/`). Rujukan ke `05-routing.md`
ditambahkan sebagai dokumen otoritatif untuk routing/visibilitas menu.

## Kenapa

Menutup item 5.22.8, sisa dari Fase 5.22 (routing). Item itu menyebut tiga klaim
(a/b/c); verifikasi menunjukkan **enam**, karena tiga lainnya ikut usang pada
periode yang sama. Ketiga yang disebut item semuanya benar-benar sudah berubah —
item ini tidak salah, hanya belum lengkap.

## File terdampak

- `docs/renderers/shadcn-shell/01-architecture.md` — §5 ditulis ulang

## Bukti

- `grep -rn "OverlayHost" renderers/react-shadcn/src/` → 3 shell + definisi + export
- `grep -rn "deriveMenuItems" renderers/react-shadcn/src/` → `useResolvedMenu.ts`
- `grep -n "_admin" renderers/react-shadcn/src/kinds/table/TableRenderer.tsx` → 0 hit
- `ls renderers/react-shadcn/src/engine/registry.tsx` → _No such file or directory_
- `grep -rln "useRealtime" renderers/react-shadcn/src/` → 11 file (7 renderer)
- `head -50 renderers/react-shadcn/src/shell/AssetRenderer.tsx` → `mount`/`unmount` + `formspec` client
- Tidak ada build/test yang berubah (hanya dokumen), tetapi suite tetap dijalankan
  setelah batch ini.

## Rujukan

Todo **5.22.8** (tertutup) · Fase 5.22 (`docs/renderers/shadcn-shell/05-routing.md`,
changelog `2026-09-25-001..005`).
