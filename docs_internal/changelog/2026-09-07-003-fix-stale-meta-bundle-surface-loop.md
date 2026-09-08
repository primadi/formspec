# 2026-09-07-003 — Fix Loop Setup↔Login: Stale Meta Bundle Lintas Surface

**Plan**: session plan (unify dev/prod auth — follow-up bug loop, lanjutan
changelog 2026-09-07-002)

## Apa

Bug: masuk lewat **app surface** (mis. `/default` → resolve ke app
klinik-internal) saat DB kosong → redirect ke setup wizard → setup sukses →
redirect ke login → **loop redirect tak terbatas**. URL `forward` tumbuh
rekursif: `setup?forward=/default/_admin/login?returnTo=/default/_admin/login?...`

Akar: `useMetaStore` menyimpan **satu bundle global** tanpa catatan surface.
Saat pindah surface (app → admin), bundle app yang masih
`setup_required=true` dianggap valid oleh guard admin surface → redirect
balik ke setup; SetupScreen mount-check melihat `setup_required=false` (server)
→ navigate balik ke login → ulang terus. Load meta yang gagal 403/401 juga
meninggalkan bundle lama di store.

Perbaikan:

- `stores/meta.ts`: state baru `loadedSurface` ("admin" | "app" | null) —
  di-set saat load/refresh sukses; load yang gagal (403/401/error) mengosongkan
  `bundle` + `loadedSurface` sehingga guard tidak pernah membaca bundle salah
  surface.
- `App.tsx` (SurfaceShell): bundle dari surface lain dianggap `null` — boot
  effect me-reload untuk surface aktif, guard tidak pernah memakai bundle
  stale.
- `SetupScreen.tsx`: reset meta store sebelum navigate ke login setelah setup
  sukses / saat mount-check mendeteksi setup sudah selesai.

## Kenapa

User melaporkan loop redirect terus-menerus saat create admin pertama lewat
`/default` (app surface). Kasus beda dengan laporan sebelumnya yang masuk
langsung ke `/_admin/setup` (tidak me-load bundle app dulu).

## File terdampak

- `renderers/react-shadcn/src/stores/meta.ts` — `loadedSurface` + clear on
  failed load
- `renderers/react-shadcn/src/App.tsx` — SurfaceShell ignore bundle lintas
  surface
- `renderers/react-shadcn/src/shell/SetupScreen.tsx` — reset store sebelum
  navigate

## Verifikasi

E2E browser (DB fresh): `/default` → setup wizard → create admin → login →
mendarat di `/default/dashboard/clinic-dashboard` (app klinik-internal),
tanpa loop. `tsc --noEmit -p tsconfig.app.json` bersih; dist di-rebuild.
