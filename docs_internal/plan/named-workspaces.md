# Plan: Named Workspaces — `kind: Workspace` + CLI

Tanggal: 2026-09-07
Status: In progress
Ref: `docs/spec/platform/02-workspace-app-module.md`, `docs/runtimes/05-engine-api-layer.md`

## Tujuan

Slug workspace di URL (`/{ws}/...`) wajib terdaftar — via manifest deklaratif
`kind: Workspace` atau CLI `formspec workspace create`. Slug tak terdaftar → 404.
Slug **tetap** menjadi workspace ID (tanpa mapping UUID) agar data existing
(SQLite/Postgres keyed by slug) tidak invalid.

## Keputusan Desain

1. **Slug = workspace ID.** Registry hanya memvalidasi keberadaan; UUID
   resolution ditunda (todo ⏸️) karena semua store existing keyed by slug.
2. **Registry = entity `formspec.core/workspace`** yang sudah ada
   (`internal/auth/module/master/workspace/entity.yaml`, fields: `name`, `slug`
   unique+indexed, `owner_user_id`, `settings`). Tabel dibuat otomatis via
   `SyncSchema`. Tidak ada tabel/registry baru.
3. **Dua sumber, satu registry**: manifest `kind: Workspace` = seed deklaratif
   (dikumpulkan saat boot `App.New`/`ReloadSpec`); CLI = penambahan runtime.
   Keduanya menulis/membaca sumber sama.
4. **Seed `default` otomatis** saat boot jika registry kosong agar dev mode
   tidak langsung 404.
5. **Unify default workspace** `"demo"` (API fallback) vs `"default"` (CLI,
   frontend) → `"default"`.
6. **Resolver nil = passthrough** (backward compat: worker/test/proses tanpa
   resolver tetap jalan).

## File yang Diubah/Dibuat

| File                                                     | Aksi                                                                                         |
| -------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| `pkg/spec/spec.go`                                       | Tambah `KindWorkspace Kind = "Workspace"`                                                    |
| `pkg/spec/workspace.go`                                  | **Baru** — `WorkspaceSpec`, `ValidateWorkspaceSpec`                                          |
| `internal/manifest/loader.go`                            | `KnownKinds` + branch validasi `Workspace`                                                   |
| `internal/genjsonschema/kinds.go`                        | Entry `WorkspaceSpec`; `make generate-schema`                                                |
| `internal/api/middleware.go`                             | `WorkspaceMiddleware` resolve via resolver; unknown → 404                                    |
| `internal/api/handler.go`                                | `workspaceFromContext` default → `"default"`                                                 |
| `internal/resource/formspec.go`                          | Kumpulkan manifest Workspace, wire `SetWorkspaceResolver` (New + ReloadSpec), seed `default` |
| `cmd/formspec/workspace.go`                              | **Baru** — `workspace create/list/delete`                                                    |
| `cmd/formspec/main.go`                                   | Wire verb `workspace`                                                                        |
| `cmd/formspec/dev.go`, `logs.go`, `resource/formspec.go` | Default `"demo"` → `"default"`                                                               |
| `examples/cafe/spec/workspaces/cafe-workspaces.yaml`     | **Baru** — seed `default` + `cafe`                                                           |

## Dependensi Antar Task

1. `WorkspaceSpec` (pkg/spec) → 2. loader branch → 3. schema gen (paralel 1–3)
2. resolver API (butuh 1 untuk tipe) → 5. middleware (butuh 4) → 6. wiring resource
3. CLI (independen dari 1–5, hanya perlu DSN + EntityStore)
4. Cafe example (butuh 1–3 untuk validate)
5. Docs/changelog/todo (terakhir)

## Verifikasi

- `go test ./...` + test baru: unknown slug → 404, resolver nil → passthrough,
  `ReloadSpec` mempertahankan registry, seed `default`.
- `formspec dev` di `examples/cafe`: `/default/app/kafe` 200, `/unknown-ws/` 404,
  `/cafe/app/kafe` 200.
- `formspec workspace create/list/delete` terhadap DSN cafe.
- `formspec validate` di `examples/cafe`.

## Level of Effort

Medium. Estimasi ~6 file inti + 3 file baru + docs.
