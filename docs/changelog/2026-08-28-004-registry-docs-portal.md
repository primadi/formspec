# 2026-08-28-004 — Registry docs terpusat + portal publik + endpoint register

## Apa yang diubah

Tiga deliverable (Plan A + B.0–B.3 dari plan registry):

- **Docs terpusat `docs/registry/`** (7 dokumen): README (index + jalur baca),
  `01-concepts` (model data, signing, immutability, install≠aktivasi, alias,
  shadow copy), `02-quickstart` (E2E keygen→sign→publish→install),
  `03-cli-reference` (flag lengkap sign/module/override/verify),
  `04-rest-api` (endpoint table + sisi server), `05-self-hosting` (dev +
  production + native binary Plan C), `06-trust-tier`. Docs usang dikoreksi:
  `docs/cli-tools/02-formspec-cli.md` §9 (hapus "belum diimplementasikan",
  todo 13.4.2) dan `docs/spec/platform/08-project-layout.md` §6 header + §6.5
  (todo 13.4.1). Link dari `docs/README.md`.
- **Folder `registry/`** — dipindah dari `verticals/registry/` (git mv).
  App `registry` kini `no-nav` + `access: public`, root `/`, modules
  `[registry, portal]`.
- **Module `portal`** (baru): landing page `pages/home.yaml` (section blocks:
  hero, feature_grid, cara pemakaian, cta) + `listings/module-catalog.yaml`
  (katalog module publik, search + filter trust_tier).
- **Endpoint register** — `POST /{ws}/_ui/auth/register` di UI surface
  (public, tanpa auth): `internal/api/auth_handler.go` `HandleRegister`
  (validasi username/password/display_name, bcrypt, rate limiter khusus
  register 3/30s per IP), `internal/auth/service.go` `Register`, mount di
  `internal/api/router.go` (UI surface; external surface opt-in menyusul).

## Verifikasi

- `go build ./...` + `go test ./internal/auth/... ./internal/api/...` hijau.
- `formspec validate --spec registry/spec` — 8 manifest, 0 problem.
- E2E smoke (binary rebuild): boot registry app → `GET /health` 200 →
  `POST /default/_ui/auth/register` → `{"created":true}` →
  `GET /default/_ui/_meta/ui` 200 (app no-nav/public ter-resolve).
- Fix: menu leaf App wajib `module:` (boot gagal sebelum ditambahkan).

## Catatan

- Binary `bin/formspec` stale menyebabkan 404 palsu saat smoke pertama —
  rebuild diperlukan setelah ubah handler.
- Deferred: server-side signature verify (13.3.3), workflow trust tier (13.3.5).

## Lanjutan (13.5.5, same day)

- Entity `Vendor` + field `owner_username` (link akun registry → vendor).
- Module `portal`: Form `vendor-signup` (create, public key + owner) + Page
  `/portal/vendor-signup` (hero + form + card 3 langkah) — ditautkan dari
  landing CTA "Sign up Vendor".
- Verifikasi: validate 10 manifest 0 problem; E2E register → `POST
/_ui/entity/registry/vendor` (anon create, app public) → vendor tercatat
  dengan owner + public key. Catatan: path UI surface memakai nama entity
  singular (`/vendor`), bukan plural.
- Docs: `docs/registry/02-quickstart.md` + section "0. Daftar sebagai vendor
  (via portal web)".

## Referensi

- Plan: session plan "FormSpec Registry — 3 Plan Terpisah" (Plan A + B)
- Docs baru: `docs/registry/README.md`
