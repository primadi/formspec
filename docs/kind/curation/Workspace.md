# Workspace

<!-- generated:meta -->
| | |
|---|---|
| Grup | `curation` |
| Plane | `resource` |
| Spec struct | `WorkspaceSpec` |

<!-- /generated:meta -->

## Kapan Memakai

Gunakan `kind: Workspace` saat proyek perlu **lebih dari satu workspace
bernama** dalam satu deployment — mis. satu server melayani beberapa tenant
(`cafe`, `kopi`) atau pemisahan environment per slug (`demo`, `prod-internal`).
Manifest ini adalah **seed deklaratif**: slug di-upsert ke workspace registry
(entity bawaan `formspec.core/workspace`) saat boot dan hot-reload, sehingga
`/{ws}/...` langsung routable tanpa langkah manual.

Jangan pakai kind ini untuk:

- **Konfigurasi runtime** (DSN, sidecar, themes) — itu `formspec-app.yaml`
  (config CLI dev, bukan manifest).
- **Settings platform terstruktur** (locale, currency, auth providers) — itu
  `kind: Config` dengan `settings:`.
- **Unit deployment** — workspace adalah unit _tenancy_; satu workspace bisa
  berisi banyak App (lihat `docs/spec/platform/02-workspace-app-module.md` §1).

Pola desain: deklarasikan workspace inti proyek sebagai manifest (ikut
version-control dan ter-deploy bersama App); workspace yang dibuat runtime
(tenant baru) lewat CLI `formspec workspace create` — keduanya konvergen ke
registry yang sama.

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1
kind: Workspace
metadata:
  name: cafe
  description: "Workspace utama contoh aplikasi kafe"
spec:
  display_name: "Kafe Demo"
```

Dengan slug eksplisit (berbeda dari `metadata.name`):

```yaml
apiVersion: formspec.dev/v1
kind: Workspace
metadata:
  name: kopi-kita-main
spec:
  slug: kopi
  display_name: "Kopi Kita"
```

Runtime: `formspec workspace create kopi --name "Kopi Kita" --dsn sqlite:.formspec/cafe.db`.

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `slug` | `string` | — | cafe | URL-facing workspace slug — defaults to metadata.name; kebab-case, unique across the deployment |
| `display_name` | `string` | — | Kafe Demo | Human-readable workspace name — shown in admin surfaces |
| `description` | `string` | — | Workspace utama aplikasi kafe demo | Description explains the workspace's purpose (AI readability). |
| `owner_user_id` | `string` | — |  | Optional user ID seeded as workspace owner — empty = no seeded owner (first admin via setup wizard) |
| `settings` | map | — |  | Free-form workspace metadata persisted to the registry row (json) — structured platform settings belong to kind: Config |

<!-- /generated:attributes -->

## Gotchas

- **Slug = workspace ID.** Tidak ada mapping slug→UUID — semua store, session,
  dan permission di-scope langsung oleh slug. Mengubah slug berarti tenant
  baru (data lama tetap di scope slug lama).
- **Slug tak terdaftar → 404.** `WorkspaceMiddleware` memvalidasi setiap slug
  URL terhadap registry; slug yang tidak terdaftar dianggap tidak ada
  (anti-enumeration). Workspace `default` selalu di-seed otomatis saat boot.
- **Reserved segments** — slug tidak boleh `_ui`, `api`, `_admin`, `assets`,
  `health`, `login`, `register`, `_ws`, `print` (bentrok dengan surface
  router). Validasi: `spec.ValidateWorkspaceSpec`.
- **Bukan Entity.** Workspace tidak punya CRUD API bisnis; registry-nya
  framework-owned (`formspec.core/workspace`). Jangan menambah field bisnis
  di `settings:` — itu metadata bebas, bukan kontrak.
- **Seed bersifat upsert.** Manifest menang atas registry saat reload untuk
  `display_name`/`owner_user_id`; workspace yang dibuat via CLI tidak dihapus
  oleh reload — hapus eksplisit via `formspec workspace delete --confirm`.
- Cross-reference: `docs/spec/platform/02-workspace-app-module.md` §1.1,
  `docs/cli-tools/02-formspec-cli.md` §12, `ai_skills/formspec-kinds`.
