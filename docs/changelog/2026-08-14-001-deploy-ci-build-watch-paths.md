# 2026-08-14-001 — Deploy CI: Build watch paths (optimasi build monorepo)

Optimasi deploy Cloudflare Pages: perubahan data di `docs/` sebelumnya
memicu rebuild **dua** project (`formspec-docs` **dan** `formspec-site`),
padahal seharusnya hanya `formspec-docs`. Dioptimasi via fitur native
Cloudflare Pages **Build watch paths**.

## Perubahan

- Dokumentasi `docs/architecture/09-domain-map.md`: tambah section
  "Optimasi build monorepo: Build watch paths" — tabel include paths per
  project (`formspec-site` → `site/*`; `formspec-docs` →
  `docs/*, docs-site/*`), semantik wildcard, urutan evaluasi include/exclude,
  dan catatan alternatif GitHub Actions + Deployment Hook (config-as-code).
- `docs/plan/todo.md`: update catatan item 12.8 (Deploy CI) + Last Updated.

## Alasan

Git integration Cloudflare Pages tidak punya path filter — tiap push ke
`main` men-trigger build **semua** project Pages yang terhubung ke repo
(`formspec-site`, `formspec-docs`). Perubahan konten docs di `docs/` ikut
memicu rebuild landing `formspec-site` yang tidak perlu (buang build quota,
menunda publish). Build watch paths memfilter per project tanpa perlu GitHub
Actions, secret, atau mematikan PR preview.

## Manual (dashboard Cloudflare — sekali saja)

1. Tiap project Pages → **Settings** → **Build** → **Build watch paths**:
   - `formspec-site`: include `site/*`
   - `formspec-docs`: include `docs/*, docs-site/*`
2. Save. Tidak perlu mengubah repo / workflow — Git integration tetap aktif,
   PR preview tetap jalan.

## File terdampak

- `docs/architecture/09-domain-map.md`
- `docs/plan/todo.md`

## Referensi

- Cloudflare Docs: _Build watch paths_ (monorepo)
- `docs/plan/todo.md` item 12.8
