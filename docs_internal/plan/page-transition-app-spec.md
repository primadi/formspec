# Plan — Page Transition via App Spec (`page_transition`)

Tanggal: 2026-09-08 · Level of effort: medium · Status: implemented

## Tujuan

Perpindahan halaman di SPA renderer dianimasikan memakai **View Transitions
API** (opsi 2), dan modenya **dikustom via App spec** — closed set
`none | fade | slide`, default `fade`.

## Desain

- **Spec field**: `App.spec.page_transition` (`pkg/spec/resources.go`) —
  divalidasi `ValidateAppSpec`, di-resolve `internal/ui/meta.go`
  (`resolvePageTransition`: empty/unknown → `fade`) dan di-expose sebagai
  `app.page_transition` di bundle `/_meta/ui`.
- **Trigger**: react-router declarative (`<BrowserRouter>`) **membuang**
  opsi `viewTransition` per-navigasi (data-router only), jadi transisi
  dipicu manual dengan pola standar:
  `document.startViewTransition(() => flushSync(() => navigate(...)))`.
- **Frontend module baru** `renderers/react-shadcn/src/lib/navigation.tsx`:
  - `useAppNavigate()` — drop-in `useNavigate()`; skip transisi saat
    `mode === "none"`, navigasi delta (`navigate(-1)` — popstate async,
    timing snapshot tidak reliable), `replace: true` (redirect auth /
    update query string), atau API tidak tersedia.
  - `AppLink` / `AppNavLink` — Link/NavLink dengan intercept klik
    (modifier keys / middle-click / `_blank` / defaultPrevented
    diteruskan apa adanya). Diimpor dengan alias
    `AppLink as Link` / `AppNavLink as NavLink` sehingga JSX shell tidak
    berubah.
  - `usePageTransitionEffect()` — mirror mode ke
    `<html data-page-transition>`; dipanggil di `App.tsx` `Root()`.
- **CSS** (`index.css`): `::view-transition-old/new(root)` per mode
  (fade 150ms cross-fade, slide 200ms horizontal), plus
  `prefers-reduced-motion` → instant cut.

## File yang diubah

Backend:

- `pkg/spec/resources.go` — field + konstanta + `PageTransitionNames` + validasi
- `internal/ui/meta.go` — `AppContext.PageTransition`, `AppSummary.PageTransition`, `resolvePageTransition`
- `internal/api/meta.go` — plumbing AppContext
- `schemas/` — regenerate via `make generate-schema`

Frontend (`renderers/react-shadcn/src/`):

- `lib/navigation.tsx` (baru), `App.tsx`, `index.css`, `types/manifest.ts`
- Swap `useNavigate` → `useAppNavigate`: 8 kind renderers + 11 shell files
- Swap Link/NavLink → AppLink/AppNavLink (alias): Sidebar, TopNavShell,
  SideNavShell, NoNavShell, AuthArea, LoginScreen, SectionBlocks

## Referensi spec

- `docs/spec/05-frontend.md` (App kind, chrome §5)
- MDN View Transitions API; react-router v7 `viewTransition` (data mode only)

## Verifikasi

- `go build ./... && go test ./...` ✅
- `npx tsc --noEmit` ✅
- `make generate-schema` ✅ (`page_transition` muncul di App.schema.json)
