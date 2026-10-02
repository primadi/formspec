# Plan — Chrome Regions (`tier: app` slots) + landing & auth-exit

**Status:** Fase A–C selesai (2026-09-29) · sisa di §14.c todo
**Tanggal:** 2026-09-29
**Changelog:** `docs_internal/changelog/2026-09-29-002-chrome-region-model-landing-auth-fix.md`
**Referensi spec:** `docs/spec/frontend/05-app-kinds.md` §4–§5,
`docs/spec/frontend/02-visual-spec-kind.md` §4
**Plan terkait:** `chrome-composition-spec.md`, `app-entry-gate.md`

## Latar belakang

Dua masalah dilaporkan pemilik proyek, keduanya di contoh kafe:

1. `http://localhost:5174/kafe` mendarat di `/kafe/cafe-master/dining-tables`
   ("Page not found") **dan tidak ada jalan logout**.
2. Page utama `kafe-qr` (`no-nav`) tampak **tanpa chrome sama sekali** —
   padahal chrome seharusnya tetap ada, hanya kosong, dan developer boleh
   mengisi topbar/sidebar/rightbar/bottombar sendiri.

Akar masalah (terukur, rujuk plan `app-entry-gate.md` + kafe TODO):

- **Landing buta permission.** `DefaultRedirect` (`src/App.tsx:654`) memilih
  `bundle.entities.find(e => e.characteristic !== "summary")` — urutan bundle
  alfabetis (`internal/entity/registry.go`), **tanpa** melihat
  `authorized_actions`. Untuk `kafe-qr` yang anonim, entity pertama adalah
  `cafe-master/dining-table` (grant hanya `find`), sehingga rute **list**-nya
  tidak terdaftar (`allowsRoute`, `src/shell/router.tsx:143`) → "Page not
  found". Ini kafe **10.20 ⏸️**.
- **Auth hanya hidup di chrome.** `AuthArea` dirender hanya di header ketiga
  shell, dan `no-nav` me-resolve `chrome.auth: none` → `AuthArea` `return null`.
  `auth_action` tidak punya `logout`; `useAutoLogout` mati di permukaan publik.
  Jadi sesi aktif di `/kafe` tidak punya jalan keluar. Ini kafe **10.18 ⏸️**.
- **Model chrome mencampur dua sumbu.** Flag `chrome.brand/nav/auth/footer/
breadcrumbs/theme_switcher` menjawab "apakah region predefined tampil?" —
  boolean tertutup — bukan "region ini diisi apa". Akibatnya `nav` jadi no-op di
  `sidebar-nav`/`topnav`, dan `brand: hide` ikut menghapus kontrol auth.

## Keputusan pemilik proyek (2026-09-29)

1. Satu plan besar: perbaikan bug **dan** model region chrome.
2. Pakai **slot resmi `tier: app`** (`accepts_slots`/`implements_slot`), bukan
   hanya `chrome.regions` berbasis asset.
3. **Kontrol sesi selalu dirender bila ada token** — `auth: none` hanya
   mematikan tombol masuk anonim (Sign in/Sign up), tidak pernah menghapus
   jalan keluar.
4. `sidebar-nav` = **`no-nav` + chrome predefined** (menu di sidebar).
   `no-nav` = chrome tetap ada, region tidak diisi otomatis.

## Desain

### Region

App memiliki himpunan **region** tetap:

| Region      | Posisi                      |
| ----------- | --------------------------- |
| `topbar`    | atas, full-width            |
| `sidebar`   | kiri, di bawah topbar       |
| `content`   | tengah (implicit, `Outlet`) |
| `rightbar`  | kanan, di bawah topbar      |
| `bottombar` | bawah, di atas footer       |
| `footer`    | paling bawah, full-width    |

Nilai per region: `none` (tidak ada) · `auto` (isi default archetype) ·
`<component-ref>` (asset komponen ber-`implements_slot: <region>`).

### Archetype = preset

- `no-nav` → semua region `none` (chrome ada, kosong).
- `sidebar-nav` → `no-nav` + `sidebar: auto` (menu predefined).
- `topnav` → `no-nav` + `topbar: auto` (menu predefined).

`chrome.brand/nav/auth/footer/breadcrumbs/theme_switcher` **tetap ada** sebagai
gula (sugar) yang dipetakan ke isi region — kompatibilitas `registry.yaml`,
kafe, dan docs lama. Explicit `chrome.regions` menang.

### Auth sebagai isi region

Kontrol auth (Sign in/Sign up/Sign out/user menu) adalah isi region (default
kanan topbar / bawah sidebar), bukan milik shell yang di-hardcode. Aturan baru:
**bila ada token, kontrol sesi (user menu → Sign out) SELALU dirender**, apa pun
`chrome.auth`-nya. `chrome.auth: none` hanya menyembunyikan Sign in/Sign up
anonim.

## Fase

### Fase A — perbaikan bug (small–medium)

| File                                                        | Perubahan                                                                                                                             |
| ----------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------- |
| `renderers/react-shadcn/src/App.tsx`                        | `DefaultRedirect` sadar `authorized_actions`: pilih entity pertama yang **punya `list`** (skip summary); bila tidak ada → state jujur |
| `renderers/react-shadcn/src/shell/NoAccessState.tsx` (baru) | State "tidak ada halaman untuk akun Anda" + tombol Sign out (bila sesi) / Sign in (bila anonim) — dirender **di dalam shell**         |
| `renderers/react-shadcn/src/shell/AuthArea.tsx`             | `if (token) return <UserMenu/>` **sebelum** cek `mode` — sesi selalu punya jalan keluar                                               |
| `renderers/react-shadcn/src/shell/index.ts`                 | Export `NoAccessState`                                                                                                                |

### Fase B — model region (`tier: app` slots) (large)

| File                                                      | Perubahan                                                                                                                           |
| --------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| `pkg/spec/resources.go`                                   | `AppChrome.Regions map[string]string`; konstanta `ChromeRegions`; validasi key ∈ region & value ∈ `none/auto/<ref>`; region standar |
| `internal/manifest/renderer.go`                           | `ValidateSlotTiers`: `implements_slot` boleh menargetkan region standar App                                                         |
| `internal/ui/meta.go`                                     | `ChromeConfig.Regions map[string]string`; `resolveChrome` → preset archetype + override `regions` + peta gula boolean               |
| `renderers/react-shadcn/src/types/manifest.ts`            | `ChromeConfig.regions`; union literal (bukan `string`) untuk nilai region                                                           |
| `renderers/react-shadcn/src/shell/RegionShell.tsx` (baru) | Grid region; region `none` kolaps; isi default (Sidebar, menu, breadcrumbs, theme switcher, AuthArea) atau `AssetRenderer`          |
| `renderers/react-shadcn/src/shell/index.ts`               | Export `RegionShell`                                                                                                                |
| `renderers/react-shadcn/src/App.tsx`                      | `APP_SHELLS` memetakan ketiga archetype ke `RegionShell`; `OverlayHost` di semua kombinasi                                          |

### Fase C — wiring, docs, schema (medium)

| File                                               | Perubahan                                                               |
| -------------------------------------------------- | ----------------------------------------------------------------------- |
| `examples/kafe/spec/apps/kafe-qr.yaml`             | Region topbar (brand + sesi) supaya `/kafe` punya chrome + jalan keluar |
| `registry/spec/apps/registry.yaml`                 | Verifikasi `chrome: {nav: menu, auth: links}` tetap identik             |
| `docs/spec/frontend/05-app-kinds.md`               | §5 ditulis ulang ke model region + tabel preset + aturan auth baru      |
| `docs/spec/frontend/02-visual-spec-kind.md`        | §4 region standar App                                                   |
| `docs/renderers/shadcn-shell/03-kind-renderers.md` | Perilaku shell terhadap region                                          |
| `docs/reference/glossary.md`                       | Entri "chrome region"                                                   |
| `schemas/dist/latest/`                             | `make generate-schema`                                                  |
| `docs_internal/plan/todo.md` + kafe TODO           | Tutup 10.18/10.20 atau sisakan item ⏸️                                  |

## Level of effort

Fase A **small–medium** · Fase B **large** · Fase C **medium**.

## Sisa yang belum diputuskan

1. Region standar **intrinsik** pada `kind: App` (rekomendasi, tanpa
   boilerplate) vs setiap App mendeklarasikan `accepts_slots`.
2. `useAutoLogout` di permukaan publik (`!isPublic` di `App.tsx:386`) — belum
   dipilih pemilik proyek; sesi di App publik tetap tidak kedaluwarsa.
3. Gerbang App backend (`access_permission` → 403, `app-entry-gate.md`, kafe
   10.21/10.22) — **di luar lingkup** plan ini.
4. `brand: hide` + auth aktif: ditutup struktural oleh model region; invariant
   validator ditambahkan bila masih perlu.
