# Domain Map — formspec.dev

**Status:** Draft · **Tanggal:** 2026-08-12

Peta subdomain dan layanan publik di bawah `formspec.dev`. Dokumen ini
otoritatif untuk pemetaan domain → layanan → hosting, dan menjadi acuan
setup DNS di Cloudflare serta deployment.

## Prinsip

- **Landing & konten statis** di Cloudflare Pages (gratis, integrasi DNS).
- **Layanan backend** (registry, MCP, control plane) di VPS terpisah
  (Hetzner/DigitalOcean) — Fase 5 (deferred, cloud phase).
- **Satu source of truth** untuk konten: `docs/` (docs site baca langsung),
  `schemas/` (di-generate dari `pkg/spec`).
- URL yang sudah didokumentasikan CLI **wajib tetap hidup** via redirect.

## Peta Subdomain

| Subdomain                       | Fungsi                                                            | Hosting             | Fase | Status                                      |
| ------------------------------- | ----------------------------------------------------------------- | ------------------- | ---- | ------------------------------------------- |
| `formspec.dev` (apex)           | Landing page / marketing                                          | Cloudflare Pages    | 1    | ✅ dirancang (`site/`)                      |
| `www.formspec.dev`              | Redirect → apex                                                   | Cloudflare Redirect | 1    | ✅ dirancang (`site/public/_redirects`)     |
| `docs.formspec.dev`             | Dokumentasi (VitePress, baca `docs/`)                             | Cloudflare Pages    | 2    | ✅ dirancang (`docs-site/`)                 |
| `schemas.formspec.dev`          | JSON Schema per kind (`v1` + `latest`)                            | Cloudflare R2/Pages | 3    | ✅ dirancang (`scripts/publish-schemas.sh`) |
| `formspec.dev/schemas/*`        | Redirect → `schemas.formspec.dev` (URL yang didokumentasikan CLI) | Cloudflare Redirect | 3    | ✅ dirancang                                |
| `registry.formspec.dev`         | Module registry / marketplace API                                 | VPS                 | 5    | ⏸️ deferred                                 |
| `mcp.formspec.dev`              | `formspec-remote-mcp` (Streamable HTTP + pgvector)                | VPS                 | 5    | ⏸️ deferred                                 |
| `api.formspec.dev`              | Public API gateway / Spec Resolution API                          | VPS                 | 5    | ⏸️ deferred                                 |
| `control.{region}.formspec.dev` | Control plane per region                                          | VPS                 | 5    | ⏸️ deferred                                 |
| `ops.formspec.dev`              | Admin/ops surfaces                                                | VPS                 | 5    | ⏸️ deferred                                 |
| `try.formspec.dev`              | Playground / live demo reference-app                              | VPS/Pages           | 5    | ⏸️ deferred                                 |
| `status.formspec.dev`           | Status/uptime page                                                | Cloudflare Pages    | 5    | ⏸️ deferred                                 |
| `assets.formspec.dev` / `cdn`   | Artifact statis (renderer, theme, module signed)                  | Cloudflare R2       | 5    | ⏸️ deferred                                 |
| `send.formspec.dev`             | Subdomain pengirim email (SPF/DKIM isolasi)                       | Resend              | 0    | 🔲 belum di-setup                           |

## Referensi URL di Repo (yang sudah ada)

| Referensi                                 | Lokasi                                                                              |
| ----------------------------------------- | ----------------------------------------------------------------------------------- |
| `formspec.dev/schemas` (JSON Schema)      | `docs/cli-tools/02-formspec-cli.md` §2                                              |
| `registry.formspec.dev` (module registry) | `docs/cli-tools/02-formspec-cli.md` §9, `docs/plan/rename-formspec.md`              |
| `formspec-remote-mcp` (hosted MCP)        | `docs/ai/04-forma-remote-mcp.md`                                                    |
| `control.{region}.formspec.dev`           | `docs/architecture/01-architecture-overview.md`, `docs/runtimes/01-formspec-ctl.md` |
| `formspec/ops.{region}.formspec.dev`      | `docs/architecture/02-admin-surfaces.md`                                            |

## DNS Setup (Cloudflare)

> Domain dibeli langsung di Cloudflare → **nameserver sudah Cloudflare, tidak
> perlu pindah/ganti NS**. Cukup pastikan zone `formspec.dev` aktif di
> dashboard (DNS → Overview → Active).

**Konsep CNAME:** record CNAME punya dua bagian — _name_ (subdomain, sisi
kiri, menjadi domain publik) dan _target_ (sisi kanan, hostname internal
Pages `<project>.pages.dev`). Contoh: `docs` → `formspec-docs.pages.dev`
berarti `docs.formspec.dev` dilayani dari project Pages `formspec-docs`.
`*.pages.dev` adalah hostname internal bawaan project, **bukan** domain
publik.

> 💡 **Custom domain di Pages otomatis membuat CNAME.** Saat menambahkan
> domain di halaman project Pages → _Custom domains → Add_, Cloudflare
> membuat record CNAME yang dibutuhkan sendiri. Membuat record manual
> hanya perlu bila ingin reserve sebelum project dibuat.

**Setup per project Pages:**

| Project            | Tambahkan custom domain                   | Hasil record (auto)                                            |
| ------------------ | ----------------------------------------- | -------------------------------------------------------------- |
| `formspec-site`    | `formspec.dev` **dan** `www.formspec.dev` | `@ → formspec-site.pages.dev`, `www → formspec-site.pages.dev` |
| `formspec-docs`    | `docs.formspec.dev`                       | `docs → formspec-docs.pages.dev`                               |
| `formspec-schemas` | `schemas.formspec.dev`                    | `schemas → formspec-schemas.pages.dev` (atau bucket R2)        |

**Record tambahan (manual):**

| Type | Name                 | Target                                                |
| ---- | -------------------- | ----------------------------------------------------- |
| TXT  | `_dmarc`             | `v=DMARC1; p=quarantine; rua=mailto:...`              |
| TXT  | `default._domainkey` | DKIM dari Resend (`send.formspec.dev`)                |
| TXT  | `send` (SPF)         | `v=spf1 include:amazonses.com ~all` (sesuai provider) |
| TXT  | `@` (SPF)            | `v=spf1 -all` (atau sesuai provider)                  |

**SSL/TLS & Rules:**

1. SSL/TLS mode **Full (strict)**.
2. Redirect Rules:
   - `www.formspec.dev/*` → `https://formspec.dev/:splat` (301) — canonical ke apex
   - `http://*` → `https://*` (301)
   - `formspec.dev/schemas/*` → `https://schemas.formspec.dev/:splat` (302)
3. Reserve subdomain backend (registry/mcp/api/ops/status/try/control.\*) — CNAME placeholder atau catatan DNS agar tidak di-squat.

## Cara Membuat Project Cloudflare Pages

> UI Pages terbaru memakai **flow berbasis wrangler** — form utama hanya
> berisi _Project name_, _Build command_, _Deploy command_, dan _Build for
> non-production branch_. **Output directory diambil dari `wrangler.toml`**
> (`[assets] directory`), dan **Root directory ada di Advanced settings**
> — bukan di form utama. `site/wrangler.toml` & `docs-site/wrangler.toml`
> sudah berisi `[assets] directory = "./dist"`.

Diulang 1× per project (site, docs, schemas). Prasyarat: repo
`github.com/primadi/formspec` sudah di-push.

1. Login dashboard Cloudflare → pilih domain `formspec.dev` (zone).
2. Menu kiri: **Workers & Pages** → tab **Create** → **Pages** → **Connect to Git**.
3. Pilih provider **GitHub** → Authorize → pilih repo `primadi/formspec`.
   - _Production branch_: `main` (deploy otomatis tiap push ke main).
4. Isi form utama:

   | Project | Project name       | Build command                                          | Deploy command                  |
   | ------- | ------------------ | ------------------------------------------------------ | ------------------------------- |
   | Landing | `formspec-site`    | `npm install && npm run build`                         | `npx wrangler deploy` (default) |
   | Docs    | `formspec-docs`    | `npm install && ln -sfn ../docs docs && npm run build` | `npx wrangler deploy` (default) |
   | Schemas | `formspec-schemas` | `npx wrangler deploy` (tanpa build — statis)           | `npx wrangler deploy` (default) |
   - **Build for non-production branch**: biarkan `true` (buat preview
     deployment untuk tiap PR/branch) atau set `false` kalau hanya mau
     produksi.
   - **Deploy command**: biarkan default `npx wrangler deploy` — wrangler
     membaca `wrangler.toml` di root directory untuk tahu output folder.

   > ⚠️ `formspec-docs` **wajib** menyertakan `ln -sfn ../docs docs` di build
   > command — symlink di-ignore git, tanpa itu build gagal resolve Vue.

5. Buka **Advanced settings** → **Root directory**:

   | Project            | Root directory |
   | ------------------ | -------------- |
   | `formspec-site`    | `site`         |
   | `formspec-docs`    | `docs-site`    |
   | `formspec-schemas` | `schemas`      |

   > Root directory menentukan folder tempat build + `wrangler.toml`
   > dieksekusi. Output folder tidak perlu diisi — sudah ada di
   > `wrangler.toml` (`[assets] directory = "./dist"`).

6. Klik **Save and Deploy** → tunggu build pertama selesai (Status: Ready).
7. **Pasang custom domain**: halaman project → tab **Custom domains** →
   **Set up a custom domain** → ketik domain sesuai tabel di atas (mis.
   `docs.formspec.dev`) → **Activate domain**. Cloudflare otomatis membuat
   record CNAME-nya.
8. Setelah aktif, verifikasi HTTPS via `curl -I https://<domain>`.

> **Catatan untuk `formspec-schemas`:** `schemas/dist/` di-ignore git, jadi
> dua pilihan: (a) **R2 bucket** (direkomendasikan — `make publish-schemas
ARGS="--upload --bucket formspec-schemas"`), atau (b) Pages statis dengan
> commit `schemas/dist/` secara paksa (`git add -f schemas/dist/`). Kalau
> memilih R2, project Pages `formspec-schemas` tidak perlu dibuat.

> **Alternatif Direct Upload** (tanpa GitHub): project → _Upload assets_ →
> drag folder build output. Tidak ada auto-deploy; upload ulang manual tiap
> perubahan. Untuk repo ini lebih baik _Connect to Git_ agar auto-deploy.

## Email (Resend)

- Outbound via subdomain `send.formspec.dev` (isolasi SPF/DKIM dari apex).
- DMARC mulai `p=quarantine`; naikkan ke `p=reject` setelah deliverability stabil.
- Inbox (Google Workspace/Zoho/MX) — keputusan ditunda; mulai outbound-only.

## Verifikasi Produksi

```bash
dig formspec.dev NS            # → nameserver Cloudflare (sudah otomatis)
dig _dmarc TXT formspec.dev    # → policy DMARC
dig docs CNAME formspec.dev    # → formspec-docs.pages.dev
curl -I https://formspec.dev                         # → 200
curl -I https://www.formspec.dev                     # → 301 ke apex
curl -I https://formspec.dev/schemas/formspec.schema.json  # → redirect ke schemas
curl -I https://schemas.formspec.dev/v1/formspec.schema.json  # → 200
curl -I https://docs.formspec.dev                    # → 200
```

## Catatan Implementasi

- Landing: `site/` (Vite + React). Docs: `docs-site/` (VitePress, symlink
  `docs-site/docs → ../docs`). Schema: `scripts/publish-schemas.sh`.
- Registry/MCP/control plane **deferred** ke cloud phase — lihat
  `docs/plan/todo.md` "Deferred (Cloud Phase)".
