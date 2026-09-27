# `--dev-ui` Vite proxy sadar multi-workspace (`localhost:5174/kafe`)

**Plan:** `docs_internal/plan/dev-ui-vite-proxy-multi-workspace.md`
**Terkait:** todo 2.11.7 (koreksi klaim), 2.11.7a (baru)

## Masalah

`http://localhost:5174/kafe` (port forward dari Vite `:5173`) berhenti di
"Loading..." lalu hanya menampilkan `Unexpected token '<', "<!doctype "... is not
valid JSON`. Terukur: `GET /kafe/_ui/_meta/apps` lewat Vite menjawab
`200 text/html` (index.html), sedangkan `/default/_ui/_meta/apps` menjawab
`application/json`.

Akar masalah: **dua** proxy memegang kontrak "prefix API per-workspace", tetapi
hanya satu yang diperbaiki item 2.11.7. `viteSPAProxy` Go
(`cmd/formspec/dev.go:635` `isWorkspaceAPIPath`) sudah meneruskan `/{ws}/_ui|api/`
untuk **semua** slug; `server.proxy` Vite (`renderers/react-shadcn/vite.config.ts`)
masih memakai key literal `"/default/api/v1"` + `"/default/_ui/"`. Karena
`formspec dev --dev-ui` (dan `npm run dev`) menyajikan SPA **langsung dari Vite**,
bukan lewat `:8080`, slug apa pun selain `default` cocok dengan nol aturan proxy →
Vite menjawab SPA → SPA gagal parse.

## Perubahan

- `renderers/react-shadcn/vite.config.ts` — kedua key diganti key RegExp
  `^/[a-z0-9-]+/api/v1` dan `^/[a-z0-9-]+/_ui/`. `[a-z0-9-]+` = charset slug
  workspace (`pkg/spec/workspace.go` `workspaceSlugPattern`); ancor `^` membuat
  `_ui`/`api` hanya cocok sebagai segmen pertama setelah slug (paritas dengan
  `isWorkspaceAPIPath`); `ws: true` dipertahankan di keduanya untuk realtime.
- `renderers/react-shadcn/src/lib/api/devProxyConfig.test.ts` (baru) — guard yang
  menjalankan **Vite dev server nyata** di atas `vite.config.ts` asli, dengan
  `server.proxy[*].target` diarahkan ke stub backend (`node:http`) pada port
  ephemeral, lalu mem-probe path dan meng-assert `PROXIED` vs SPA. 10 test.
- `docs/guides/how-to-run.md` §2 — snippet proxy diperbarui + catatan multi-workspace.

## Bukti

- **Guard gagal tanpa fix** (kalibrasi): disuntikkan kembali key literal
  `/default/...` ke salinan config → **7 dari 10 test gagal** (semua kasus
  workspace non-`default`, termasuk assert WebSocket upgrade). Dipulihkan →
  10 lulus.
- **Dev server nyata**: `GET /kafe/_ui/_meta/apps` lewat `:5173` →
  `application/json` (sebelumnya `text/html`).
- Browser di `http://localhost:5174/kafe`: halaman boot — judul `kafe-qr`,
  bundle termuat (sebelumnya berhenti di pesan parse HTML).
- `npx tsc -b` bersih · `npx vitest run` **529 lulus** / 39 file (baseline 519, +10).

## Sisa

- **`/kafe` mendarat di "Page not found"** — **bukan regresi fix ini**, melainkan
  gap kafe **10.20 ⏸️** yang sudah terlacak: `kafe-qr` ber-`root_url: /` tidak
  punya halaman route `/`, `no-nav` mengosongkan `bundle.menu`, sehingga
  `DefaultRedirect` jatuh ke entity pertama → `/{ws}/cafe-master/dining-tables`,
  yang tidak punya route turunan bagi anonim. Fix proxy inilah yang membuat SPA
  akhirnya sampai ke titik itu (sebelumnya mati sebelum render). Kunjungan
  pelanggan yang dimaksud App ini tetap lewat `/kafe/menu/{session_id}`.
- `/health` tidak ikut di-proxy Vite (Vite menjawab SPA) — gap kecil yang sengaja
  dibiarkan; SPA tidak pernah memanggilnya. Rujuk 2.11.7a.
