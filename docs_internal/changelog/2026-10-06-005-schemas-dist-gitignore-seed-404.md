# 2026-10-06-005 — `schemas/dist` ter-ignore `.gitignore`: kind baru tak pernah ter-commit → `Seed` 404

## Apa yang diubah

- **`.gitignore`**: tambah negasi `!schemas/dist/` setelah pola `dist/`, plus
  komentar alasan.
- **`schemas/dist/{v1,latest}/kinds/Seed.schema.json`**: di-stage (file-nya sudah
  ada di disk sejak 2026-09-25 tetapi tidak pernah masuk git).
- **`scripts/publish-schemas.sh`**: guard baru — tiap file `schemas/dist/` dicek
  `git check-ignore --no-index`; ada yang ter-ignore → `exit 1` beserta sebab dan
  saran negasi.
- **`schemas/README.md`**: langkah verifikasi `git status --short schemas/dist`
  pada alur publish; catatan bahwa per-kind schema bukan schema mandiri (`$ref`
  di-resolve dari `$defs` root `formspec.schema.json`).

## Kenapa

Item todo **3.6.7** mencatat registry online ketinggalan kontrak dan kind `Seed`
404: `formspec validate` (mode registry, tanpa `--schema`) gagal
`fetch https://schemas.formspec.dev/v1/kinds/Seed.schema.json: unexpected status 404`
dengan exit 2 pada **setiap** proyek ber-`kind: Seed`. Regenerasi sudah hijau dan
`schemas/kinds/Seed.schema.json` sudah ada, jadi teka-tekinya bukan generator.

Akar masalahnya di lapis git: pola telanjang `dist/` (ditambahkan 2026-09-10
untuk artefak build Go) **juga mencocoki `schemas/dist/`**
(`git check-ignore -v schemas/dist/v1/kinds/Seed.schema.json` →
`.gitignore:45:dist/`). Karena file `dist/` lama sudah tracked, ignore tidak
berlaku bagi mereka — jadi `git add schemas/dist` (persis instruksi di
`schemas/README.md`) tampak berhasil sementara **setiap file kind BARU dilewati
senyap**. Terukur: 34 file di `schemas/dist/v1/kinds/` di disk, 33 tracked;
`git ls-tree -r HEAD` hanya memuat `schemas/kinds/Seed.schema.json`; commit
terakhir yang menyentuh `dist` hanya membawa root schema + `index.json` tanpa
`kinds/`. Efeknya `index.json` (tracked) menyebut `Seed` yang file skemanya tak
ada di registry.

## Bukti

- `git check-ignore --no-index schemas/dist/v1/kinds/Seed.schema.json` → tidak
  match; delapan `dist/` lain (`./dist`, `renderers/react-shadcn/dist`,
  `site/dist`, `docs-site/dist`, `sdk/typescript/dist`, `cmd/formspec/dist`,
  `cmd/formspec-registry/web/dist`, `sdk/browser/dist`) tetap IGNORED.
- Registry-mirror lokal (`python3 -m http.server` di `schemas/dist`, tanpa
  `--schema`): `service-demo` **13 manifest / 0 problem**, `kafe` **88 / 0** —
  sebelumnya keduanya mati di `404 .../kinds/Seed.schema.json` (exit 2).
- `make publish-schemas` → `✅ Guard: semua file schemas/dist/ terlihat git`;
  dengan negasi dihapus sementara → guard merah, `exit 1`, 74 file terdeteksi.

## Sisa

- Verifikasi live (butuh `git push`; tidak bisa dibuktikan lokal): `curl -sI
https://schemas.formspec.dev/v1/kinds/Seed.schema.json` → 200 dan
  `formspec validate --spec examples/kafe/spec` tanpa `--schema` → 0 problem.
  → **3.6.7 ⏸️**.

## Referensi

- Plan: `docs_internal/plan/schema-registry-sync.md` (§ 2026-10-06)
- Todo: 3.6.7 (registry legacy + `Seed` 404), 2.11.10 (publish schema kind baru)
