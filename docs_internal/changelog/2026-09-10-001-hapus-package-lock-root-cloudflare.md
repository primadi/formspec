# 2026-09-10-001 — Hapus package-lock.json root yang memicu npm ci di Cloudflare

## Apa yang diubah

Menghapus `package-lock.json` di root repo (stray file, isi `packages: {}`
kosong, tanpa `package.json` pendamping) dari git, plus `node_modules/` root
yang tertinggal.

## Kenapa

Deploy Cloudflare Pages untuk `formspec-schemas` (schemas.formspec.dev, static
assets dari `schemas/dist/`) gagal: Cloudflare mendeteksi lockfile di build
root → menjalankan `npm clean-install` → `npm ci` gagal `EUSAGE` karena tidak
ada pasangan `package.json` + lockfile yang valid. Project ini pure static
assets dan tidak butuh npm install sama sekali.

## File terkena dampak

- `package-lock.json` (dihapus dari git)
- `node_modules/` root (dihapus, sudah gitignored)

## Referensi

- `schemas/wrangler.toml` + `schemas/README.md` — jalur deploy git-based,
  output `schemas/dist/`.
- Setelah push, deploy Cloudflare akan skip step install dan langsung serve
  `schemas/dist/`.
