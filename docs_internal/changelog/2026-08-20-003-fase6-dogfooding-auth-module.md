# Fase 6 Dogfooding — Bundled Auth Module (refactor `formspec.core`)

**Tanggal**: 2026-08-20 · **Sequence**: 003
**Plan**: `docs/plan/fase6-dogfooding-auth-module.md` (Fase A)

## Apa yang diubah

Memulai Fase 6 sebagai **dogfooding**: auth framework dibangun ulang sebagai
**1 modul FormSpec** yang bisa di-merge ke project lain. `formspec.core`
dipindah dari registrasi programatik Go ke **bundled module YAML** yang
di-embed dan dimuat lewat manifest loader — jalur yang sama dengan modul user.

### Fase A (fondasi) — selesai

- **A1** `internal/auth/module/` (baru, `//go:embed module`): `module.yaml` +
  entity namespace `formspec.core`: `user`, `session`, `role`, `role-assignment`
  (dipindah dari hardcode Go di `core.go`).
- **A2** `manifest.Loader.LoadEmbedded(fsys fs.FS)` — walk `fs.WalkDir`, parse
  via `ParseBytes`, aturan skip sama dengan `Discover`.
- **A3** `entity.Registry.RegisterEmbeddedCoreModule(fsys)` — load embedded
  manifests, skip bila `HasEntity` (user override via `external/` menang),
  register dengan `Internal: true`. `internal/auth/core.go` di-refactor: hapus
  `coreUserSpec()`/`coreSessionSpec()`/`coreRoleSpec()`/`coreRoleAssignmentSpec()`.
- **A4** `ui.Registry.LoadEmbedded(fsys)` — kapabilitas muat UI manifests dari
  embedded FS (UI derived-by-default menutupi demo).
- **A5** Flag `SpecInfo.UIExposed` — entity Internal yang opt-in admin/UI via
  annotation `formspec.dev/ui-exposed: "true"` muncul di meta bundle + UI routes
  - standard permissions, tapi tidak pernah di external API. `user`/`role`/
    `role-assignment` di-expose; `session` tetap tersembunyi.
- **A6** `formspec generate auth` menyalin dari bundled module (selalu sinkron),
  bukan hardcode user+session.

## Kenapa

Dogfooding: membuktikan framework bisa membangun fitur framework-nya sendiri
sebagai modul FormSpec, dan modul itu bisa di-merge ke project lain via
`external/`/`spec/modules/` (jalur menuju Fase 13 vendoring).

## File yang terkena dampak

- `internal/auth/module/**` (baru) — bundled module manifests
- `internal/auth/core.go` — refactor, hapus hardcode EntitySpec, `ModuleFS()`
- `internal/manifest/loader.go` — `LoadEmbedded`
- `internal/entity/registry.go` — `RegisterEmbeddedCoreModule`, `UIExposed`
- `internal/ui/registry.go` — `LoadEmbedded`
- `cmd/formspec/generate_auth.go` + `generate_auth_test.go` — scaffold dari embed
- `docs/plan/fase6-dogfooding-auth-module.md` (baru), `docs/plan/todo.md`

## Verifikasi

- `go build ./...` hijau.
- `go test ./...` hijau (baseline 571+ pass, 0 fail).
- Boot `verticals/billing/spec`: login `admin/admin` → token `*`; meta bundle
  berisi `formspec.core/user`, `role`, `role-assignment` (UIExposed), `session`
  tersembunyi.
- Catatan: `verticals/reference-app` (komposisi) gagal boot karena issue
  pre-existing (report `trial-balance` gl malformed) — tidak terkait perubahan ini.
