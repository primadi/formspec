# 2026-08-24-026 — Asset Hardening: needs, CSP, CSS Scoped (5.9.6–5.9.8)

## Apa yang diubah

Menuntaskan Track C — hardening/keamanan asset contract (07-component-kinds.md §4):

**`5.9.6` needs declaration:**

- `types/manifest.ts` — `BlockRef.needs` (`AssetNeeds`: `actions[]` + `subscribe[]`).
- `lib/formspec-client.ts` — `withNeeds()` membungkus `formspec.api` dengan ky `beforeRequest`
  hook: parse module/entity dari URL, cek terhadap `needs.actions`/`needs.subscribe`; panggilan
  di luar `needs` **gagal client-side** (throw error).
- `AssetRenderer` menerima `needs` dan meneruskannya ke `createFormspecClient`; `PageRenderer`
  meneruskan `block.component.needs`.

**`5.9.8` CSS scoped:**

- `AssetRenderer` kini mount component ke **Shadow DOM host** (`el.attachShadow` + host div dengan
  `all: initial`) — CSS component tidak bocor ke chrome sekitarnya.

**`5.9.7` CSP sandbox:**

- `internal/api/asset.go` — response asset module diberi header `Content-Security-Policy`
  (`default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self';
img-src 'self' data:`) — component hanya boleh connect ke origin App sendiri.

## Kenapa

Menutup tiga item keamanan terakhir asset contract: deklarasi footprint (`needs`), pembatasan
koneksi (CSP), dan isolasi CSS (Shadow DOM) — konsisten dengan prinsip "jalur data satu-satunya
keluar dari component adalah formspec.\*".

## File terdampak

- `renderers/react-shadcn/src/types/manifest.ts` — `AssetNeeds` + `BlockRef.needs`
- `renderers/react-shadcn/src/lib/formspec-client.ts` — `withNeeds` enforcement
- `renderers/react-shadcn/src/shell/AssetRenderer.tsx` — Shadow DOM mount + pass needs
- `renderers/react-shadcn/src/kinds/page/PageRenderer.tsx` — pass `needs`
- `internal/api/asset.go` — CSP header
- `docs/plan/todo.md` — tandai 5.9.6, 5.9.7, 5.9.8 ✅

## Verifikasi

- `go build ./...` — lulus
- `go test ./internal/api/...` — lulus
- `npx vitest run` — 144 test lulus
- `npx tsc --noEmit` — bersih
