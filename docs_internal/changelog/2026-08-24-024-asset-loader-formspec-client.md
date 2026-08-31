# 2026-08-24-024 — Asset Loader + formspec Client (5.9.1 + 5.9.2)

## Apa yang diubah

Melengkapi Track C — asset contract inti (07-component-kinds.md §4):

**`5.9.1` Dynamic ES module loader:**

- `shell/AssetRenderer.tsx` (baru) — memuat ES module asset via dynamic `import()`
  (`/* @vite-ignore */`), memanggil `mount(el, props, formspec)` saat mount dan
  `unmount(el)` saat unmount; state loading/error.
- Backend `internal/api/asset.go` (baru) — `HandleAsset()`: `GET /_ui/assets/{module}/{path*}`
  menyajikan `{root}/modules/{module}/assets/{path}` dari manifest roots; tolak path traversal
  (`..`); 404 jika tidak ada. `SetAssetRoots` di `HandlerFactory` + `RouterBuilder`.
- `resource/formspec.go` — asset roots di-wire (`cfg.SpecPath` + `cfg.ExternalDir`).
- `internal/api/asset_test.go` (baru) — serve 200, missing 404, traversal 400.

**`5.9.2` formspec client injection:**

- `lib/formspec-client.ts` (baru) — `createFormspecClient()` membangun objek `formspec`:
  `api` (ky client), `subscribe(entity, cb)` (imperatif via `subscribeRealtime`),
  `navigate(page, params)`, `theme` (token CSS), `ui` (dari `lib/ui.ts`), `components`
  (registry widget dasar dari `widgets/`).
- `hooks/useRealtime.ts` — tambah `subscribeRealtime()` (subscribe imperatif untuk asset).
- `kinds/page/PageRenderer.tsx` — block `component.asset` kini dirender via `AssetRenderer`
  (sebelumnya placeholder).

## Kenapa

Membuat kontrak asset berfungsi end-to-end: developer bisa menulis component custom
(`assets/*.js` dengan `mount`/`unmount`) dan memasangnya di Page via `component: { asset: ... }`,
dengan client `formspec` ter-inject (api/subscribe/navigate/theme/ui/components).

## File terdampak

- `renderers/react-shadcn/src/shell/AssetRenderer.tsx`, `lib/formspec-client.ts` — baru
- `renderers/react-shadcn/src/hooks/useRealtime.ts` — `subscribeRealtime`
- `renderers/react-shadcn/src/kinds/page/PageRenderer.tsx` — wire AssetRenderer
- `internal/api/asset.go`, `asset_test.go` — baru
- `internal/api/handler.go`, `router.go` — `SetAssetRoots` + route
- `resource/formspec.go` — wire asset roots
- `docs/plan/todo.md` — tandai 5.9.1, 5.9.2 ✅

## Verifikasi

- `go build ./...` — lulus
- `go test ./internal/api/...` — lulus (termasuk `TestHandleAsset`)
- `npx vitest run` — 144 test lulus
- `npx tsc --noEmit` — bersih

## Catatan

- Sisa Track C: `5.9.4` formspec.files (upload/download tray), `5.9.5` formspec.form (headless
  form engine), `5.9.6` needs declaration, `5.9.7` CSP sandbox, `5.9.8` CSS scoped.
- Konvensi path asset: `{root}/modules/{module}/assets/{path}` (sesuai layout contoh).
