# Forma — Petunjuk Repo

## Dokumentasi

- **`docs/` adalah satu-satunya dokumentasi otoritatif dan eksternal-facing.**
  Terstruktur contract-vs-renderer: `docs/spec/` (kontrak normatif:
  platform/backend/frontend) dan `docs/renderers/` (implementasi resmi:
  shadcn-shell, persist-postgres). Ditulis present-tense — tanpa narasi
  historis.
- **`docs_old/` dan `reff_docs/` adalah arsip internal/historis.** Jangan kutip
  sebagai kontrak berlaku dan jangan edit (read-only). Keduanya hanya source
  material selama migrasi docs dan akan dihapus. Peta migrasi + aturan "tree
  otoritatif per topik selama transisi": `docs_old/MIGRATION.md`.
- Jangan menambahkan konten historis ("dulu X, diganti Y", changelog, decision
  ledger) ke `docs/` — catatan semacam itu ke `docs_old/MIGRATION.md` atau
  cukup di git history.
- Dokumen `docs/spec/` berstatus `Outline → Draft → Final`. Yang masih
  `Outline` belum menjelaskan perilaku kode; perilaku kode yang berjalan masih
  mengikuti `docs_old/spec/` sampai dokumen penerusnya ≥ Draft.

## Konvensi

- Bahasa docs: Indonesia; nama kind/field/istilah teknis tetap English.
- Commit lokal saja; push hanya jika diminta.
