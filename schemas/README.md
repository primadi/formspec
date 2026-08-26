# FormSpec JSON Schema — schemas.formspec.dev

JSON Schema (Draft-07) untuk semua resource kind FormSpec, di-generate dari
`pkg/spec` (Go types) via `make generate-schema`.

## Sumber

- `schemas/formspec.schema.json` — root discriminator schema
- `schemas/kinds/{Kind}.schema.json` — per-kind spec schema (34 kinds)

Generator: `cmd/formspec-gen-schema/` (dipanggil `make generate-schema`).

## Publikasi ke schemas.formspec.dev

Jalur deploy yang dipakai: **git-based**. `schemas/dist/` ter-commit di repo dan
di-serve statis oleh Cloudflare (auto-build on push via GitHub integration di
dashboard Cloudflare). Alur publish:

```bash
make publish-schemas          # 1. regenerate + stage ke schemas/dist/
git add schemas/dist          # 2. commit dist (sudah tracked, add normal cukup)
git commit -m "..."           # 3. commit
git push                      # 4. push → Cloudflare auto-build → live update
```

> Jalur R2 bucket (`--upload`) tidak dipakai — butuh `CLOUDFLARE_API_TOKEN` +
> wrangler login, dan tidak memberi traceability git. Script tetap mendukungnya
> sebagai opsi cadangan.

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

**Opsi A — Cloudflare Pages/Worker via git (dipakai, auto-build on push):**

1. `schemas/dist/` ter-commit di repo (sudah tracked).
2. Cloudflare Pages/Worker terhubung ke repo (GitHub integration) dengan
   folder root `schemas/dist`, no build command, custom domain
   `schemas.formspec.dev`.
3. Setiap `git push` → Cloudflare auto-build → live update. Tanpa kredensial
   tambahan, riwayat deploy = riwayat git.

**Opsi B — Cloudflare R2 public bucket (cadangan, tidak dipakai):**

1. Buat bucket `formspec-schemas`, set **public access** + custom domain
   `schemas.formspec.dev` (Cloudflare dashboard).
2. `make publish-schemas ARGS="--upload --bucket formspec-schemas"` (butuh
   `CLOUDFLARE_API_TOKEN` + wrangler login).

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
