# 2026-09-29-002 — Chrome region (`tier: app` slots) + landing & auth-exit fix

**Plan:** `docs_internal/plan/chrome-regions.md` (baru)

## Apa yang diubah

Dua bug yang dilaporkan di `/kafe` diperbaiki, sekaligus model chrome ditata
ulang menjadi **region**.

**Fase A — bug (perilaku):**

1. **Landing buta permission.** `DefaultRedirect` (`src/App.tsx`) kini memilih
   entity pertama yang **rute list-nya benar-benar terdaftar** untuk pemanggil
   (`authorized_actions`), bukan sekadar entity pertama non-summary. Dipindah ke
   `src/shell/landing.ts` (`canLandOnList`/`pickLandingEntity`) agar teruji
   langsung. Terukur: `/kafe` (anonim) kini mendarat di
   `/kafe/cafe-master/menu-categories` (grant `list`), bukan
   `dining-tables` (grant `find` saja → "Page not found").
2. **State jujur bila tidak ada yang bisa dibuka.** `src/shell/NoAccessState.tsx`
   menggantikan "No entities found. Load a manifest to get started." — pesan itu
   menuduh manifest rusak padahal masalahnya hak akses (kafe 10.20/10.22).
   Selalu menyediakan jalan keluar (Sign out bila sesi, Sign in bila anonim).
3. **Jalan keluar sesi.** `AuthArea` merender user menu (→ Sign out) **sebelum**
   cek `mode`, jadi `chrome.auth: none` (default `no-nav`) hanya mematikan titik
   masuk anonim — bukan jalan keluar. Menutup kafe **10.18** (bagian "tidak ada
   jalan keluar").

**Fase B — model region (`tier: app` slots):**

4. `App.spec.chrome.regions` — peta `topbar/sidebar/rightbar/bottombar/footer` →
   `none | auto | <component-ref>` (`pkg/spec/resources.go`: `AppChrome.Regions`,
   `ChromeRegionNames`/`ChromeRegionSet`, validasi key+value di `ValidateAppSpec`).
   Region standar intrinsik ke `kind: App`; komponen `tier: component` memasang
   dirinya lewat `implements_slot: <region>` (`internal/manifest/renderer.go`).
5. `internal/ui/meta.go` — `ChromeConfig.Regions` + `chromeRegionPreset()`:
   **archetype = preset** (`sidebar-nav` = `no-nav` + `sidebar: auto`; `topnav` =
   `no-nav` + `topbar: auto`). Flag boolean lama tetap berlaku sebagai **gula**
   untuk **isi** region `auto`; `regions.footer` mencerminkan `footer` sehingga
   keduanya tak bisa berbeda.
6. Renderer: **satu** shell — `src/shell/RegionShell.tsx` — merakit region
   (dengan `src/shell/regions.ts` untuk preset/gula, `navLinks.ts`,
   `AuthArea`, `AssetRenderer`, `OverlayHost`). `SideNavShell`/`TopNavShell`/
   `NoNavShell` **dihapus** (logika topbar/sidebar dipindah ke RegionShell).
   `types/manifest.ts` memakai union literal untuk nama/isi region.
   `OverlayHost` kini dipasang di semua komposisi (dulu absen di `no-nav`).
7. `examples/kafe/spec/apps/kafe-qr.yaml` — `chrome.regions` eksplisit
   (topbar/footer `auto`) sehingga App publik punya chrome tempat menaruh
   kontrol sesi; komentar §4.2 diperbarui.

**Dokumentasi & schema:**

8. `docs/spec/frontend/05-app-kinds.md` §4–§5 ditulis ulang (region, preset,
   aturan auth sesi); `02-visual-spec-kind.md` §4 (region standar);
   `docs/renderers/shadcn-shell/03-kind-renderers.md` §2;
   `docs/reference/glossary.md` (entri "Chrome region").
9. `make generate-schema` + `make generate-kind-docs`.

## Kenapa

Model boolean lama mencampur dua sumbu — "apakah region tampil?" vs "region ini
diisi apa?" — sehingga `nav` jadi no-op di `sidebar-nav`/`topnav`, `brand: hide`
ikut menghapus kontrol auth, dan `no-nav` tampak sebagai "tanpa chrome" padahal
seharusnya "tanpa bar default yang bisa diisi developer".

## File terkena dampak

`pkg/spec/resources.go`, `internal/ui/meta.go`, `internal/manifest/renderer.go`,
`renderers/react-shadcn/src/{App.tsx,types/manifest.ts,shell/*}`,
`examples/kafe/spec/apps/kafe-qr.yaml`, `schemas/`, `docs/{spec,renderers,reference}`.

## Test

- Go: `pkg/spec` (`TestValidateAppSpec_ChromeRegions`), `internal/ui`
  (`TestResolveChrome_*` — preset + override + gula) — hijau.
- Frontend: `src/shell/{landing,NoAccessState,AuthArea,RegionShell}.test.*`
  (598 test) — hijau.
- Browser (Vite `:5174`, `-spec examples/kafe/spec`): `/kafe` → list yang boleh
  dibuka; sesi kasir aktif di `/kafe` → user menu + Sign out hadir di chrome.

## Sisa

- Aturan auth diperkuat di klien; invarian validator ("App privat wajib punya
  jalan masuk") **belum** ditambahkan → kafe `10.18` (keputusan a) tetap ⏸️.
- Gerbang App backend (`access_permission` → 403) belum dikerjakan →
  `docs_internal/plan/app-entry-gate.md`, kafe `10.21`/`10.22` ⏸️.
- `useAutoLogout` masih mati di permukaan publik (sesi publik tak kedaluwarsa).
