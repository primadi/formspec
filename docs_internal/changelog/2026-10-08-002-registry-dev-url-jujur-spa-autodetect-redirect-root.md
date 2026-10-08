# 2026-10-08-002 — Registry dev: URL jujur, SPA auto-detect, redirect root dev

## Apa yang diubah

`make registry-dev` menjalankan `go run` (tanpa `-tags formspec_spa`), sehingga
`cmd/formspec-registry/web/embed.go` — SPA asli — **tidak ikut dikompilasi** dan
`embed_stub.go` yang menjawab dengan halaman "SPA tidak ter-embed", padahal
`renderers/react-shadcn/dist` ada di checkout. Banner-nya juga mencetak
`http://localhost:8080` (tanpa prefix workspace → selalu 404) dan menunjuk
`/default/_admin`, permukaan yang panel entity-nya sudah dipensiunkan (D4).

Tiga perubahan:

1. **Resolusi SPA**: `--web-dir` → `renderers/react-shadcn/dist` (sumber renderer,
   paling segar) → `cmd/formspec-registry/web/dist` (salinan tersinkron milik binary
   itu sendiri, sisa `make build-registry` yang tetap ada di disk) → embedded
   `web/dist`. Auto-detect **dilewati** bila embed sudah berisi dist asli, supaya UI
   binary tidak ditentukan oleh checkout yang kebetulan ada di atas CWD-nya. Sebuah
   direktori hanya dihitung sebagai bundle bila memuat `index.html` (kedua produsen
   tidak atomik: `rm -rf` lalu `cp -r`). `findWebDist()` dipindah dari
   `cmd/formspec/dev.go` ke `internal/devserver` dan digeneralisasi menjadi
   `FindDistUpwards(segments...)` (satu implementasi, dua binary).
2. **Banner jujur**: URL diambil dari App yang ter-resolve
   (`App.UIAppURLs()` → `RouterBuilder.AppMountPaths()` + `spec.SurfaceURL`),
   bukan literal slug/mount. `/default/_admin` tidak lagi disebut sebagai panel —
   dicetak sebagai rute framework (`setup`, `change-password`, `oauth/*`).
   Bila hasilnya tetap placeholder, banner menyatakannya **dan** memberikan
   jalan keluar.
3. **Redirect root dev**: `GET /` → 302 ke `/{ws}/` — **hanya** di `!ProdMode`
   dan hanya bila tepat satu App ter-mount. `RouterBuilder` tidak punya
   `ProdMode`, jadi gate-nya dihitung di `resource/formspec.go`; router hanya
   menyimpan target (`SetRootRedirect`, kosong = mati) dan registrasinya
   di-gate `assets != nil` (tanpa SPA, redirect hanya mengantar ke 404 JSON
   satu hop kemudian).

## Kenapa

Semua permukaan FormSpec di bawah prefix workspace (D50), sedangkan SPA sendiri
mengasumsikan `/` adalah URL shell (`App.tsx` menavigasi `/` → `/{workspace}`).
Keduanya bertentangan kecuali server menjembatani; dan karena `/{workspace}`
adalah segmen WAJIB, `workspace` **tidak bisa** di-rename menjadi `/` (slug wajib
kebab-case + bukan reserved; `/{ws}` = batas tenant). "Root tanpa prefix" yang
tersedia adalah `App.spec.root_url: /` **di dalam** workspace — dan itulah kenapa
`/{ws}/` adalah root App.

## Sisa yang TIDAK ditutup

- Sweep klaim `/_admin` basi di `docs/` + `examples/` + `.claude/settings.json`
  → **item todo bernomor** (lihat bagian todo).
- `renderers/react-shadcn/src/App.tsx:76` masih `<Navigate to="/default" replace />`
  (hardcode slug; hanya terpakai bila SPA disajikan tanpa redirect server).
- `cmd/formspec/workspace_active.go` (peringatan workspace aktif) belum menyebut URL baru.

## File yang terpengaruh

- `internal/devserver/devserver.go` — `FindWebDist()` + `FindDistUpwards()` (dipindah ke sini dari `cmd/formspec/dev.go`)
- `internal/devserver/finddist_test.go` — guard walk-up + tolak dist tanpa `index.html`
- `cmd/formspec/dev.go` — delegasi `findWebDist()`; banner dari `App.UIAppURLs()`
- `cmd/formspec-registry/main.go` — urutan resolusi SPA + banner jujur
- `internal/api/router.go` — `AppMountPaths()`, `SetRootRedirect()`, redirect root-level
- `pkg/spec/workspace.go` — `SurfaceURL(slug, rootURL)`
- `resource/formspec.go` — `wireAppSurfaces()` (boot + reload), `UIAppURLs()`
- Test: `internal/api/router_root_redirect_test.go`, `pkg/spec/workspace_url_test.go`,
  `resource/dev_root_redirect_test.go`
- Docs: `docs/registry/05-self-hosting.md`, `docs/cli-tools/01-formspec-dev.md`

## Bukti

Boot registry (repo, `go run` tanpa tag):
`web: /workspaces/formspec/renderers/react-shadcn/dist (auto-detected; go run has no embedded SPA)`
· `GET /` → **302** `location=/default/` · mengikutinya → body memuat
`src="/assets/index-hxPnax6q.js"` (**bukan** `SPA tidak ter-embed`) · `GET /default/`
→ **200** `text/html` · `GET /default/_admin` → **200** (shell; di browser
"Page not found", karena itu banner tidak lagi menunjuk ke situ).

**Hot-reload:** sentuh `modules/portal/module.yaml` → watcher reload → `GET /`
**tetap 302** dengan target sama (membuktikan re-wire di jalur `ReloadSpec`, bukan
hanya boot). Guard dibuktikan menggigit: `SetRootRedirect(devRootRedirect(...))`
diganti `SetRootRedirect("")` → `TestDevRootRedirect_ResolvesFromAppMount` +
`TestDevRootRedirect_SurvivesReload` **FAIL**, lalu dipulihkan dari backup.

**Fallback salinan tersinkron** (dijalankan dari `/tmp/probe2` yang hanya punya
`cmd/formspec-registry/web/dist`): `web: /tmp/probe2/cmd/formspec-registry/web/dist
(auto-detected (synced copy); go run has no embedded SPA)` → `GET /default/`
mengembalikan `src="/assets/index-hxPnax6q.js"`. **Prioritas** (dijalankan dari root
di mana dua-duanya ada): `web: /workspaces/formspec/renderers/react-shadcn/dist
(auto-detected)` — sumber renderer menang.

**Fallback salinan tersinkron** (dijalankan dari `/tmp/probe2` yang hanya punya
`cmd/formspec-registry/web/dist`):
`web: /tmp/probe2/cmd/formspec-registry/web/dist (auto-detected (synced copy); go run has no embedded SPA)`
→ `GET /default/` mengembalikan `src="/assets/index-hxPnax6q.js"`. **Prioritas**
(dijalankan dari root repo, di mana dua-duanya ada):
`web: /workspaces/formspec/renderers/react-shadcn/dist (auto-detected)` — sumber
renderer menang.

**Di luar repo** (binary untagged dijalankan dari `/tmp`): banner mencetak
`⚠ placeholder … Full UI: make build-registry, or --web-dir …` dan
`⚠ The UI is NOT served — … Fix: …`. `--web-dir` eksplisit menang:
`web: cmd/formspec-registry/web/dist (from --web-dir)` → SPA asli tersaji.

Unit: `go build ./...` · `gofmt` bersih · `go test ./resource/ ./pkg/spec/
./internal/devserver/ ./cmd/formspec/` hijau · `go test ./internal/api/` hijau
kecuali `TestFileDownloadCaching_SignedIsNoStore` yang **sudah gagal sebelum
perubahan ini** (file untracked dari sesi lain: `linkStore` nil → 501 sebelum cek
token; bukan regresi di sini).

## Referensi

- Plan: `docs_internal/plan/registry-dev-ui-parity.md`
- `docs/architecture/02-admin-surfaces.md` §4 · plan `docs_internal/plan/app-scoped-login.md` D4
- `docs_internal/plan/flexible-root-url.md` · D50 (workspace prefix)
