# 2026-08-12-004-domain-infrastruktur-formspec-dev

Setup fondasi infrastruktur domain `formspec.dev` (Fase 12 todo, plan di
`docs/architecture/09-domain-map.md`): landing page, docs site, dan schema
hosting — semua dengan build hijau.

## Perubahan

- **Landing page** `site/` (Vite + React + TS + Tailwind v4): hero, 6
  `ctx.*` primitives, arsitektur 2-plane, 5 tipe impl, derived-by-default,
  marketplace, quickstart CLI. `public/` berisi `_redirects` (www→apex,
  `formspec.dev/schemas/*`→schemas subdomain), `_headers`, `robots.txt`,
  `.well-known/security.txt`. Build → `site/dist/` (Cloudflare Pages).
- **Docs site** `docs-site/` (VitePress 1.6.4): `srcDir` memakai symlink
  `docs-site/docs → ../docs` (single source of truth) + `preserveSymlinks`
  agar resolusi modul Vue benar; `rewrites` memetakan tiap `README.md` →
  `index.md` (home + index folder); `ignoreDeadLinks` men-toleransi
  referensi ke folder internal yang di-exclude + sisa rename
  forma→formspec; `srcExclude` mengecualikan `plan/`, `changelog/`,
  `presentations/`, `technical-notes/`. Build → `docs-site/dist/` (123
  halaman, changelog terverifikasi tidak ikut).
- **Schema hosting** `scripts/publish-schemas.sh` + `make publish-schemas`:
  stage layout versi `schemas/dist/{v1,latest}/` (root + `kinds/` +
  `index.json`), opsi upload ke Cloudflare R2 via `npx wrangler`.
- **Dokumentasi**: `docs/architecture/09-domain-map.md` (peta subdomain,
  DNS, hosting, email, verifikasi) + update document map README + Fase 12
  di `docs/plan/todo.md` + `.gitignore` (docs-site node_modules/dist/symlink,
  schemas/dist).

## Alasan

Domain `formspec.dev` disiapkan untuk landing, docs, schema JSON, MCP, dan
registry. Repo sudah mereferensikan `formspec.dev/schemas`,
`registry.formspec.dev`, `control.{region}.formspec.dev` — fondasi statis
dibangun lebih dulu (cepat online tanpa backend), service backend
(registry/MCP/control plane) tetap di-defer ke cloud phase.

## Dampak

| File                                        | Perubahan                                                |
| ------------------------------------------- | -------------------------------------------------------- |
| `site/` (baru)                              | Landing page Vite+React (13 file sumber + public assets) |
| `docs-site/` (baru)                         | Docs site VitePress (config, theme, public assets)       |
| `scripts/publish-schemas.sh` (baru)         | Stage + upload schema versi                              |
| `schemas/README.md` (baru)                  | Dokumentasi publikasi schema                             |
| `docs/architecture/09-domain-map.md` (baru) | Peta subdomain & DNS                                     |
| `docs/architecture/README.md`               | + 09-domain-map di document map                          |
| `docs/plan/todo.md`                         | + Fase 12 Domain Infrastruktur                           |
| `Makefile`                                  | + target `publish-schemas`                               |
| `.gitignore`                                | + docs-site & schemas/dist artifacts                     |

Verifikasi: `site` build ✅ · `docs-site` build ✅ (123 halaman, changelog
excluded) · `make publish-schemas` stage ✅ · routing docs `/`, `/spec/`,
`/kind/data/Entity` → 200.

Yang belum (manual, butuh akun): DNS Cloudflare, deploy Pages, R2 upload,
Resend email, reserve subdomain backend.
