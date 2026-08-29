# REST API Registry

Registry tidak punya handler hand-written — endpoint dihasilkan **generic entity
REST engine** dari `spec.expose` di entity manifests. Semua path workspace-scoped
(default workspace `default`), sesuai D50: `/api/v1` tanpa prefix deny-by-default.

## Endpoint yang dipakai client (`internal/vendor/registry.go`)

| #   | Endpoint                                                 | Tujuan                                         |
| --- | -------------------------------------------------------- | ---------------------------------------------- |
| 1   | `GET /{ws}/api/v1/registry/vendors?per_page=100`         | find vendor (filter client-side)               |
| 2   | `POST /{ws}/api/v1/registry/vendors`                     | create vendor (public_key ed25519)             |
| 3   | `POST /{ws}/api/v1/registry/vendors/{id}/submit`         | transisi state (past-draft)                    |
| 4   | `GET /{ws}/api/v1/registry/modules?per_page=100`         | find module                                    |
| 5   | `POST /{ws}/api/v1/registry/modules`                     | create module (vendor_id)                      |
| 6   | `POST /{ws}/api/v1/registry/modules/{id}/submit`         | submit module                                  |
| 7   | `POST /{ws}/api/v1/registry/module-versions`             | create version (semver, checksum, signature)   |
| 8   | `POST /{ws}/api/v1/registry/module-version/{id}/tarball` | upload tarball (multipart, `application/gzip`) |
| 9   | `GET /{ws}/api/v1/registry/module-versions?per_page=100` | lookup version                                 |
| 10  | `GET /{ws}/api/v1/registry/module-version/{id}/tarball`  | download tarball                               |

## Envelope

Sukses: `{ "data": ... }`. Error: `{ "error": { "code", "message" } }`.
Publish mengirim header `Authorization: Bearer <api-key>`.

## Sisi Server

| Kode                                                             | Peran                                               |
| ---------------------------------------------------------------- | --------------------------------------------------- |
| `registry/spec/modules/registry/entities/*.yaml` → `spec.expose` | Sumber endpoint (actions yang terekspos)            |
| `internal/api/generator.go` → `GenerateRoutes`                   | Entity registry + expose → RouteDescriptor          |
| `internal/api/router.go`                                         | Mount `/{ws}/api/v1/...`                            |
| `internal/api/handler.go` → `HandlerFactory`                     | CRUD generic + dispatch action (termasuk `submit`)  |
| `internal/api/file.go` → `HandleFileUpload`                      | Upload/download file field (7.17.1) → `ctx.storage` |

Mengubah perilaku publish (validasi, permission) = ubah **manifest entity**
(`spec.expose`, permissions, state machine guards) — bukan menulis handler baru.
Server-side signature verify (13.3.3) menyusul sebagai native service di
`cmd/formspec-registry`.
