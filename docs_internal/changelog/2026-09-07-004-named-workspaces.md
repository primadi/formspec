# 2026-09-07-004 — Named Workspaces: kind `Workspace` + CLI

Plan: `docs_internal/plan/named-workspaces.md`

## Apa yang berubah

Slug workspace di URL (`/{ws}/...`) kini **wajib terdaftar** di workspace
registry (entity bawaan `formspec.core/workspace`). Slug tak terdaftar →
**404 `WORKSPACE_NOT_FOUND`** (anti-enumeration). Slug tetap menjadi
workspace ID — tidak ada mapping UUID, data existing aman.

Dua sumber konvergen ke registry yang sama:

1. **Kind baru `Workspace`** (deklaratif) — `pkg/spec/workspace.go`
   (`WorkspaceSpec`, `ValidateWorkspaceSpec`: slug kebab-case + reserved
   segments), didaftarkan di `internal/manifest/loader.go` (`KnownKinds` +
   branch validasi) dan `internal/genjsonschema/kinds.go` (schema
   `schemas/kinds/Workspace.schema.json`). Seed di-upsert saat boot dan
   hot-reload via `syncWorkspaceRegistry` (`resource/formspec.go`).
2. **CLI `formspec workspace create|list|delete`** (imperatif) —
   `cmd/formspec/workspace.go`, menulis ke registry via EntityStore.

## Perubahan perilaku

- `WorkspaceMiddleware` (`internal/api/middleware.go`) memvalidasi slug
  terhadap resolver (`api.SetWorkspaceResolver`, di-wire di `App.New` +
  `ReloadSpec`); resolver nil → passthrough (backward compat).
- Workspace `default` selalu di-seed otomatis saat boot.
- Default workspace diunifikasi: `"demo"` → `"default"`
  (`workspaceFromContext`, fallback middleware, `Config.WorkspaceID`,
  `formspec logs`).
- `viteSPAProxy` (dev-ui) kini meneruskan `/{ws}/_ui|api/` untuk **semua**
  workspace, bukan hanya workspace terkonfigurasi.
- Test resource & e2e (resource/\*\_test.go, examples/Clinic-UI-Showcase)
  dipindah dari workspace `"demo"` ke `"default"`.

## File terdampak

`pkg/spec/{spec.go,workspace.go}`, `internal/manifest/loader.go`,
`internal/genjsonschema/kinds.go`, `internal/api/{middleware.go,handler.go}`,
`internal/auth/workspace.go`, `resource/formspec.go`,
`cmd/formspec/{main.go,dev.go,logs.go,workspace.go}`,
`examples/cafe/spec/workspaces/cafe-workspaces.yaml`, docs (platform/02,
cli-tools/02, runtimes/05).

**Tambahan (follow-up)**: kind docs — `internal/genkinddocs/markdown.go`
(group `curation`) + `make generate-kind-docs` → `docs/kind/curation/
Workspace.md` (narasi manual: kapan memakai, contoh, gotchas); README
`docs/kind/` 33 → 34 kind. Regenerate juga menutup drift atribut lama di
`App.md`/`Form.md`/`Page.md` (field `auth` dll. yang belum ter-sync).

## Catatan

- `formspec validate` tanpa `--schema` masih 404 untuk kind Workspace —
  schema remote (`schemas.formspec.dev`) belum ter-publish; pakai
  `--schema schemas/` (lokal) sementara.
- UUID resolution (slug→internal ID) ditunda — lihat todo ⏸️.
