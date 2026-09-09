# 2026-09-08-002 — Page transition via App spec (`page_transition`)

Referensi plan: `docs_internal/plan/page-transition-app-spec.md`

Menambahkan `App.spec.page_transition` (`none | fade | slide`, default
`fade`) yang menganimasikan perpindahan halaman SPA via View Transitions
API. Karena react-router declarative membuang opsi `viewTransition`
per-navigasi, transisi dipicu manual
(`startViewTransition` + `flushSync`) di modul baru
`renderers/react-shadcn/src/lib/navigation.tsx`: `useAppNavigate()`
(drop-in `useNavigate`, dipakai di 19 file renderer/shell) dan
`AppLink`/`AppNavLink` (link shell, diimpor dengan alias `Link`/`NavLink`
sehingga JSX tidak berubah). Mode di-mirror ke `<html
data-page-transition>` dan dianimasikan via `::view-transition-*` CSS di
`index.css` (termasuk fallback `prefers-reduced-motion`). Navigasi
`replace: true` (redirect auth, update query string) dan delta
(`navigate(-1)`) sengaja tidak dianimasikan.

File terdampak: `pkg/spec/resources.go`, `internal/ui/meta.go`,
`internal/api/meta.go`, `schemas/*` (regenerate), frontend seperti di
plan. Verifikasi: `go test ./...`, `tsc --noEmit`, `make generate-schema`.

## Perbaikan lanjutan (verifikasi browser, hari yang sama)

Dua bug membuat animasi tidak pernah tampil (transisi selalu di-skip):

1. **`flushSync` tidak bisa me-flush update router** — `BrowserRouter`
   membungkus update state-nya dalam `React.startTransition` (default
   `useTransitions`); update masuk TransitionLane sehingga `flushSync`
   (SyncLane) tidak me-render-nya sinkron → snapshot "baru" identik dengan
   lama. Fix: `<BrowserRouter useTransitions={false}>` di `App.tsx`
   (update router jadi sinkron, ter-flush di dalam callback VT).
2. **Double-wrap di AppLink/AppNavLink** — `useTransitionClick` memakai
   `useAppNavigate()` (navigate yang sudah dibungkus), sehingga
   `transitionedNavigate` → callback → navigate (wrapper) →
   `transitionedNavigate` lagi → VT nested di dalam callback VT luar.
   Chrome men-skip VT luar dan meng-abort dengan "Transition was aborted
   because of invalid state". Fix: `useTransitionClick` memakai
   `useNavigate()` mentah.

Verifikasi browser (cafe, Chrome): satu VT per klik, `ready` + `finished`
resolved, DOM berubah di dalam callback.

## Iterasi 2 — scope ke konten + mode tambahan

Feedback: sidenav ikut beranimasi (aneh) dan mode terlalu sedikit.

- **Scope ke konten**: `<main>` di ketiga shell (SideNav/TopNav/NoNav)
  diberi `[view-transition-name:page-content]`; CSS menganimasikan
  `::view-transition-old/new(page-content)` alih-alih `root` — sidebar,
  topnav, dan header tetap statis; root cross-fade default tidak terlihat
  karena chrome identik antar halaman.
- **Mode tambahan**: closed set jadi `none | fade | slide | slide-up |
scale` (konstanta + validasi + schema + CSS keyframes masing-masing).
- Catatan verifikasi: VT selalu abort dengan "invalid state" saat
  `document.hidden` (tab tidak terlihat) — perilaku browser normal, bukan
  bug; verifikasi visual hanya valid saat halaman terlihat.
- **Enter-only**: snapshot halaman lama disembunyikan instan
  (`::view-transition-old(page-content) { animation: none; opacity: 0 }`)
  — hanya halaman baru yang dianimasikan masuk; tidak ada lagi konten
  lama yang "tampil lalu hilang" di belakang animasi.
- **"Wireframe kosong" intermittent**: Skeleton fallback dari
  lazy-loaded renderer ikut ter-capture VT saat navigasi pertama ke suatu
  kind (chunk belum ter-download; setelah ter-cache tidak bisa
  diulang). Fix: `lib/preload.ts` kini mem-preload SEMUA kind renderer
  (15 renderer, deferred ke `requestIdleCallback` agar tidak bersaing
  dengan fetch boot).
