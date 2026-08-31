# 2026-08-14-005 — Schema registry landing page

## Apa yang diubah

`https://schemas.formspec.dev` (base URL) sebelumnya 404 karena `schemas/dist/`
hanya berisi folder versi (`v1/`, `latest/`), tanpa `index.html` di root.

1. `scripts/publish-schemas.sh` sekarang meng-generate `schemas/dist/index.html`
   (landing page statis) saat stage — menampilkan link ke root schema (`v1` &
   `latest`), `index.json`, dan semua schema per kind (di-loop dari daftar kinds).
2. `site/src/components/Nav.tsx` & `Footer.tsx`: link "Schema"/"JSON Schema"
   tetap menunjuk ke `https://schemas.formspec.dev` (root) — karena kini root
   menampilkan landing page, bukan lagi 404. Tidak perlu arahkan langsung ke
   `v1/formspec.schema.json` (raw JSON di browser, UX buruk).
3. `schemas/dist/` di-stage ulang via `make publish-schemas`.

## Kenapa

Base URL registry harus bisa dibuka manusia (browsable) dan menampilkan
versi + daftar schema, bukan 404. URL JSON langsung tetap kanonik untuk
tooling (CLI, `yaml.schemas`) via `index.json`.

## File terkena dampak

- `scripts/publish-schemas.sh`
- `site/src/components/Nav.tsx`
- `site/src/components/Footer.tsx`
- `schemas/dist/index.html` (generated, di-ignore git)

## Deploy

- Re-deploy `schemas/dist/` ke Cloudflare (R2/Pages) agar landing page live.
- `site/` tidak perlu di-rebuild (link source kembali ke nilai semula).
