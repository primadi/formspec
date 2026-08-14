# FormSpec JSON Schema — schemas.formspec.dev

JSON Schema (Draft-07) untuk semua resource kind FormSpec, di-generate dari
`pkg/spec` (Go types) via `make generate-schema`.

## Sumber

- `schemas/formspec.schema.json` — root discriminator schema
- `schemas/kinds/{Kind}.schema.json` — per-kind spec schema (34 kinds)

Generator: `cmd/formspec-gen-schema/` (dipanggil `make generate-schema`).

## Publikasi ke schemas.formspec.dev

```bash
make publish-schemas                          # stage v1 lokal (schemas/dist/v1)
make publish-schemas ARGS="--upload --bucket formspec-schemas"   # + upload R2
make publish-schemas ARGS="--version v2 --upload --bucket formspec-schemas"
```

### Konsumsi oleh CLI (online)

`formspec validate`, `formspec init`, dan `formspec schema` mengambil schema
langsung dari registry (bukan embed di binary):

- Versi spec dibaca dari `apiVersion` manifest (`formspec.dev/v1` → versi `v1`).
- Schema di-cache lokal di `os.UserCacheDir()/formspec/schemas/<version>`.
- `index.json` tiap versi memuat daftar `kinds` — registry bersifat
  self-describing, sehingga `formspec init`/`formspec schema fetch` tahu set
  schema lengkap tanpa daftar hardcoded di CLI.
- Registry base URL: env `FORMSPEC_SCHEMA_REGISTRY` > `schema-registry:` di
  `formspec-app.yaml` > default `https://schemas.formspec.dev`.

### Layout versi

```
schemas/dist/
  v1/
    formspec.schema.json
    kinds/{Kind}.schema.json
    index.json            # metadata versi + URL
  latest/                 # alias → v1 (selalu menunjuk versi terbaru)
```

URL publik:

```
https://schemas.formspec.dev/v1/formspec.schema.json
https://schemas.formspec.dev/v1/kinds/Entity.schema.json
https://schemas.formspec.dev/latest/formspec.schema.json
https://schemas.formspec.dev/latest/kinds/Entity.schema.json
```

### Opsi deploy

**Opsi A — Cloudflare R2 public bucket (direkomendasikan, no build):**

1. Buat bucket `formspec-schemas`, set **public access** + custom domain
   `schemas.formspec.dev` (Cloudflare dashboard).
2. `make publish-schemas ARGS="--upload --bucket formspec-schemas"` (butuh
   `CLOUDFLARE_API_TOKEN` + wrangler login).

**Opsi B — Cloudflare Pages (static):**

1. Deploy folder `schemas/dist/` sebagai project Pages statis (folder root
   `schemas/dist`, no build command), custom domain `schemas.formspec.dev`.
2. Jalankan `make publish-schemas` (stage) lalu deploy folder.

### Redirect dari formspec.dev/schemas

CLI (`formspec validate`, `docs/cli-tools/02-formspec-cli.md`) mereferensikan
`formspec.dev/schemas`. Redirect ke `schemas.formspec.dev` diatur lewat
`site/public/_redirects` (landing page):

```
/schemas/*  https://schemas.formspec.dev/:splat  302
```

### Versioning

Bump versi (`v1` → `v2`) ketika kontrak spec berubah secara tidak kompatibel
(spec Draft → Final, atau breaking change). `latest/` selalu menunjuk versi
terbaru; `validate` yang menginginkan pinning memakai URL versi eksplisit.

## Referensi

- CLI validator: `formspec validate` (engine + schema), `docs/cli-tools/02-formspec-cli.md`
- Generator: `cmd/formspec-gen-schema/`, `internal/genjsonschema/`
- Publish: `scripts/publish-schemas.sh`
