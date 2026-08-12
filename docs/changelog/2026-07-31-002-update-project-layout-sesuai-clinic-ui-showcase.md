# 2026-07-31-002 — Update Project Layout sesuai Clinic-UI-Showcase

**Apa:** Menulis ulang `docs/spec/platform/08-project-layout.md` agar layout
project yang dikontrakkan match dengan layout nyata yang dipakai contoh
`examples/Clinic-UI-Showcase/spec/` (kanonik). §3, §4, §6 dipertahankan utuh
sebagai target desain karena banyak dokumen lain mereferensikan numbering-nya.

## Perubahan docs

- **§1 Direktori Standar** ditulis ulang penuh (dengan subsection baru):
  - `formspec-app.yaml` kini dideskripsikan sebagai **config dev/serve** yang
    diparse CLI (bukan `kind: Config` — koreksi error label lama), dengan field
    `spec`/`dsn`/`runtime`/`app-dir`/`app-entrypoint`/`listen`/`themes`; nama
    legacy `formspec-sidecar.yaml`.
  - Container `spec/` (lokasi bisa diubah via `--spec`/`spec:`), `spec/apps/`
    (multi-App, satu module di-mount beberapa App), dan struktur module
    entity-centric ber-grouping characteristic (`master/`/`transaction/`/
    `reference/`/`summary/`) plus kind level module (`config/`, `dashboards/`,
    `pages/`, `reports/`, `themes/`).
  - Kontrak loader zero-folder-assumption: scan rekursif `.yaml`, skip hidden/
    `node_modules`/`impl/`, referensi resolve relatif ke direktori entity.
- **§2** diperluas: tabel tiga jenis file, **konvensi folder entity** (manifest
  `entity.yaml` + subfolder UI per kind + nama file role-based dengan
  `metadata.name` = `<entity>-<role>` yang dipakai `resolveForm()`), script
  colocated (`ref` cukup nama file), dan lokasi kode handler dua pola
  (`app/` root via `app-dir` = model sekarang; `impl/` per module = native Go +
  target multi-runtime §3).
- **§5 Status Implementasi** diperbarui: mencatat bahwa contoh berjalan dengan
  model satu-runtime-global via `formspec-app.yaml runtime: node` + `app/` root.
- **§7 Referensi** ditambah referensi contoh kanonik + catatan dua konvensi
  folder di repo (entity-centric kanonik vs kind-based lama).

## File terdampak

- `docs/spec/platform/08-project-layout.md` (docs only — tidak ada perubahan
  kode/schema).

## Referensi

- Contoh kanonik: `examples/Clinic-UI-Showcase/spec/`, `formspec-app.yaml`,
  `app/src/handlers/otc_sell.ts`
- Konvensi menu: `docs/spec/platform/02-workspace-app-module.md` §4
- Kontrak loader: `internal/manifest/loader.go` (`Discover`)
- Config dev: `cmd/formspec/dev_config.go` (`configFile`, `findConfigFile`)
