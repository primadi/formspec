# 2026-09-08-001 — WorkspaceMiddleware: passthrough segmen reserved (fix 404 root assets)

## Apa

`WorkspaceMiddleware` kini melewati (skip) registry check untuk first path
segment yang termasuk `spec.ReservedWorkspaceSlugs`, langsung set workspace
default di context dan serahkan ke chi router. `favicon.svg`, `icons.svg`,
dan `manifest.json` ditambahkan ke `ReservedWorkspaceSlugs`.

## Kenapa

Chi menjalankan middleware global sebelum routing, sehingga `GET /assets/...`
(favicon/asset Vite berpath absolut yang keluar dari prefix workspace) ditelan
sebagai `404 WORKSPACE_NOT_FOUND` — slug `assets` dianggap workspace. Akibatnya
App dengan `root_url: /` di workspace mana pun gagal memuat JS/CV build
(`/assets/index-*.js` 404), meskipun route root-level `/assets/*` sudah
terdaftar di `BuildHTTP` (tidak pernah terjangkau).

## File terkena dampak

- `internal/api/middleware.go` — `WorkspaceMiddleware` skip segmen reserved
- `pkg/spec/workspace.go` — reserved set + pesan error validasi slug
- `internal/api/workspace_middleware_test.go` — test passthrough baru

## Referensi

- plan `docs_internal/plan/named-workspaces.md`; debug session cafe
  (`root_url: /` → asset 404). Test: `go test ./internal/api/ ./pkg/spec/` ✅
