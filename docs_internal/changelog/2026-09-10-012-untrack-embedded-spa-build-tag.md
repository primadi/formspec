# 2026-09-10-012 — untrack embedded SPA dist + build tag formspec_spa

## Apa

`cmd/formspec/dist/` dan `cmd/formspec-registry/web/dist/` (salinan SPA build
untuk `go:embed`) tidak lagi di-commit:

- `.gitignore` menambah kedua path; 351 file dist di-`git rm --cached`
- Go embed dipecah per build tag `formspec_spa`:
  - `cmd/formspec/spa_embed.go` (SPA asli) / `spa_stub.go` (placeholder
    `spa_stub/index.html`) — `spaFS`, `spaEmbedRoot`, `spaEmbedded`
  - `cmd/formspec-registry/web/embed.go` (SPA asli) / `embed_stub.go`
    (placeholder `stub/index.html`) — `DistFS()`, `Embedded`
- `dev.go` / registry `main.go` menampilkan pesan jelas bila binary dibangun
  tanpa embedded SPA
- Makefile: `build-formspec`, `build-registry`, `release` kini `rm -rf`
  dist lama sebelum copy (asset hash lama tidak menumpuk lagi) dan build
  dengan `-tags formspec_spa`; build default (go install/go test) tanpa tag
  → stub

## Kenapa

dist sebelumnya di-commit, dan `cp -r` tanpa pembersihan membuat file hash
lama menumpuk (5 versi `DashboardRenderer-*.js` dst., 306 file di git).
Tapi tidak bisa sekadar gitignore: `go:embed` butuh file ada saat compile, dan
`go install` tidak bisa menjalankan npm/vite. Solusi: build tag — binary
release (`make build`, `make release`) embed SPA asli; build default
(`go install`, `go test`) memakai placeholder kecil sehingga tetap compile.

## File terkena dampak

- `Makefile`, `.gitignore`
- `cmd/formspec/{spa_embed,spa_stub}.go`, `cmd/formspec/spa_stub/index.html`,
  `cmd/formspec/main.go`, `cmd/formspec/dev.go`
- `cmd/formspec-registry/web/{embed,embed_stub}.go`,
  `cmd/formspec-registry/web/stub/index.html`,
  `cmd/formspec-registry/main.go`
- `docs/guides/install.md` — catatan UI untuk binary `go install`

## Verifikasi

- `go build ./...` OK; build ±tag `formspec_spa` untuk kedua binary OK
- `go vet` + `go test ./cmd/...` OK
- `git ls-files` dist = 0; `git check-ignore --no-index` match

## Referensi

- `docs/guides/releasing.md` (langkah 3 — make release)
