# 2026-08-31-009 — Route guard `public: false` di renderer (redirect ke /login)

## Apa

Renderer react-shadcn kini menghormati `public: false` pada manifest Page di
level ROUTE, bukan hanya level App surface:

- `renderers/react-shadcn/src/shell/router.tsx`: komponen baru
  `RequireSession` + helper `guard()`. Route Page dengan `spec.public ===
false` dibungkus guard — pengunjung anonim (tanpa token, atau `_meta/me`
  = "anonymous") di-redirect ke `{surface}/login?returnTo=<path>` sehingga
  setelah sign-in kembali ke halaman semula.
- `renderers/react-shadcn/src/types/manifest.ts`: field `public?: boolean`
  ditambahkan ke `PageSpec` dan `FormSpec` (selaras `pkg/spec/frontend.go`).

## Kenapa

Halaman `vendor-signup` di Registry sudah diset `public: false`
(changelog 008), tapi tetap bisa dibuka anonim karena App `registry` bersifat
`access: public` — surface boot anonim dan router tidak pernah memeriksa flag
`public` per-manifest. Backend memang tetap menolak submit anonim (401/403),
tapi UX-nya buruk (form tampil → submit gagal). Kini gerbangnya di router:
anonim langsung diarahkan ke login.

## Verifikasi

- `tsc --noEmit` bersih; `vitest run src/shell` 11 pass.
- `make build-registry` sukses (SPA embed di-rebuild).

## File terdampak

- `renderers/react-shadcn/src/shell/router.tsx`
- `renderers/react-shadcn/src/types/manifest.ts`

## Catatan

Guard diterapkan pada Page routes (blok #1 builder). Kind lain (Dashboard,
Report, dll.) juga punya flag `Public` di `pkg/spec/frontend.go` — bisa
menyusul dengan helper `guard()` yang sama bila dibutuhkan. Auto-fill
`owner_username` dari `_meta/me` tetap deferred (butuh dukungan FormField
default-from-identity).
