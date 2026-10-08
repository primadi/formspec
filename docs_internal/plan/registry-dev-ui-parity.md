# Plan — `formspec-registry`: banner jujur, SPA dev parity, redirect root

**Status**: Implemented (2026-10-08) · **Effort**: small–medium · **Changelog**: `2026-10-08-002`

Referensi: `docs/registry/05-self-hosting.md` §Mode Dev · `docs/runtimes/05-engine-api-layer.md`
§routing · `docs/architecture/02-admin-surfaces.md` §4 (permukaan `_admin` dipensiunkan,
plan `app-scoped-login.md` D4).

## Masalah (terukur 2026-10-07)

`make registry-dev` menjalankan `scripts/run-registry.sh` → `go run ./cmd/formspec-registry`.
Dua keluhan nyata muncul bersamaan:

| Pengamatan                                                   | Akar                                                                                                                                                                                                                             |
| ------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GET /` → `404 page not found` (teks polos)                  | Semua permukaan di bawah `r.Route("/{workspace}")` (`internal/api/router.go:587`); `WorkspaceMiddleware` mengganti slug kosong → `default` hanya di **context**, bukan di URL                                                    |
| Banner mencetak `✓ Server starting on http://localhost:8080` | `cmd/formspec-registry/main.go:161` mencetak origin tanpa prefix workspace → URL yang dicetak selalu 404                                                                                                                         |
| `GET /default/` → halaman "SPA tidak ter-embed"              | `go run` tanpa `-tags formspec_spa` → `web/embed_stub.go` (`//go:build !formspec_spa`) yang dikompilasi; `web/embed.go` **tidak ikut build sama sekali**. `web/dist` juga di-gitignore dan hanya disinkron `make build-registry` |
| `formspec dev` juga mencetak `/default/_admin`               | Permukaan panel entity derived **dipensiunkan** (D4); `/{ws}/_admin` kini hanya rute framework (`setup`, `change-password`, `oauth/callback`, `oauth/link-callback`)                                                             |

Symmetri yang ingin dicapai: `formspec dev` sudah punya `findWebDist()` + `spaCacheDir()`
(`cmd/formspec/dev.go`), `cmd/formspec-registry` tidak punya keduanya.

## Keputusan pemilik (2026-10-07)

1. **Banner** diperbaiki (wajib).
2. Embed `web/dist` **tetap** jalur deploy default; yang diperbaiki DX dev-nya.
3. Auto-detect `renderers/react-shadcn/dist` **di binary** (varian A), bukan hanya di script.
4. Redirect dev `/` → `/{ws}/` **dipilih** (varian B), di-gate `!ProdMode`.
5. Sapu klaim `/_admin` basi di `docs/` = item `[⏸️]` terpisah, bukan sekarang.
6. Workspace **tidak** bisa di-rename menjadi `/` (slug wajib kebab-case, `/{ws}` = batas tenant D50).

## File yang dibuat/diubah

| File                                        | Perubahan                                                                            | Effort |
| ------------------------------------------- | ------------------------------------------------------------------------------------ | ------ |
| `internal/devserver/devserver.go`           | `FindWebDist()` + `FindDistUpwards()` (dipindah dari `cmd/formspec/dev.go`)          | small  |
| `internal/devserver/finddist_test.go`       | guard walk-up + tolak dist tanpa `index.html`                                        | small  |
| `cmd/formspec/dev.go`                       | pakai `devserver.FindWebDist()`; banner dari `App.UIAppURLs()`                       | small  |
| `cmd/formspec-registry/main.go`             | resolusi SPA (`--web-dir` → repo dist → synced copy → embed) + banner jujur          | small  |
| `internal/api/router.go`                    | `AppMountPaths()`, `SetRootRedirect()`, redirect root-level gated                    | small  |
| `resource/formspec.go`                      | `SetApps`/`SetWeb*`/redirect dalam satu wiring varian (boot + reload), `UIAppURLs()` | small  |
| `internal/api/router_root_redirect_test.go` | guard redirect + `AppMountPaths`                                                     | small  |
| `resource/dev_root_redirect_test.go`        | guard redirect end-to-end (resolve, tidak di prod, selamat reload)                   | small  |
| `pkg/spec/workspace_url_test.go`            | guard `SurfaceURL`                                                                   | small  |
| `docs/registry/05-self-hosting.md`          | urutan resolusi SPA + URL yang benar                                                 | small  |
| `docs/cli-tools/01-formspec-dev.md`         | URL SPA benar (bukan `/_admin`)                                                      | small  |

## Aturan yang dipilih

| Keadaan                                                              | Perilaku                                                                |
| -------------------------------------------------------------------- | ----------------------------------------------------------------------- |
| `--web-dir` di-set                                                   | dipakai apa adanya (CLI menang)                                         |
| `--web-dir` kosong, `renderers/react-shadcn/dist` ketemu di atas CWD | **dipakai** + dicetak `(auto-detected)` — sumber renderer, paling segar |
| `--web-dir` kosong, hanya `cmd/formspec-registry/web/dist` yang ada  | **dipakai** + dicetak `(auto-detected (synced copy))`                   |
| `--web-dir` kosong, tidak ada keduanya                               | `cfg.WebFS = web.DistFS()` (embed `web/dist` atau placeholder)          |
| embed sudah berisi dist asli (`-tags formspec_spa`)                  | auto-detect **dilewati**                                                |
| direktori dist tanpa `index.html`                                    | **bukan** bundle (produsennya tidak atomik: `rm -rf` lalu `cp -r`)      |
| masih placeholder (stub, tanpa dist)                                 | banner mencetak **peringatan** + cara memperbaikinya                    |
| `!ProdMode` + tepat satu mount App                                   | `GET /` → `302 /{ws}{root_url}` (selalu berakhiran `/`)                 |
| `ProdMode` / mount >1 / tanpa SPA                                    | tanpa redirect (perilaku lama: 404 JSON/teks)                           |

## Bukti verifikasi

- `go build ./...` · `go test ./internal/api ./internal/devserver ./resource ./cmd/formspec/...`
- Banner registry: mencetak `App: http://localhost:8080/default/` + `Setup: .../_admin/setup`;
  **bukan** lagi `http://localhost:8080` polos dan bukan `/_admin` sebagai panel.
- `curl -si http://127.0.0.1:8080/default/` → memuat `src="/assets/`, bukan `SPA tidak ter-embed`
  (Fase 2 — membuktikan auto-detect bekerja saat `go run` tanpa build tag).
- `curl -s -o /dev/null -w '%{http_code} %{redirect_url}\n' http://127.0.0.1:8080/`
  → `302 http://127.0.0.1:8080/default/`, dan **tetap** 302 setelah satu hot-reload spec
  (membuktikan re-wire di jalur `ReloadSpec`, bukan hanya boot).
- `curl -s -o /dev/null -w '%{http_code}\n' .../default/_admin` → 200 (shell) tetapi
  di browser = "Page not found" → banner tidak boleh menunjuk ke situ.

## Sisa (deferred, jadi item `[⏸️]` bernomor)

- Sapu klaim `/_admin` basi di `docs/` + `examples/` + `.claude/settings.json`
  (`docs/cli-tools/01-formspec-dev.md:36`, `docs/guides/how-to-run.md:31,64,284-288`,
  `docs/guides/getting-started.md:219`, `docs/guides/order-to-cash-tutorial.md:399`,
  `docs/architecture/01-architecture-overview.md:314,342,529`,
  `docs/renderers/shadcn-shell/01-architecture.md:14`,
  `docs/runtimes/02-formspec-resource.md:41,183`, `examples/{arisan,Clinic-UI-Showcase}/**`).
- `renderers/react-shadcn/src/App.tsx:76` masih `<Navigate to="/default" replace />` —
  hardcode slug; hanya terpakai saat SPA disajikan tanpa redirect server (mis. Vite langsung).
- `cmd/formspec/workspace_active.go` (peringatan workspace aktif) belum menyebut URL baru.
