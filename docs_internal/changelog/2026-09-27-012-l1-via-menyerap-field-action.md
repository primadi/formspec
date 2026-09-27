# 2026-09-27-012 — L1 tuntas: `via` menyerap `uses`/`params`/`expose`/`rate_limit`

**Apa:** Menutup L1 (`docs_internal/plan/via-sebagai-action-penuh.md`).
`TransitionDecl` kini menyerap `uses`, `params`, `expose`, dan `rate_limit`,
dan `ActionSources()` menyalinnya ke action sintetis.

**Kenapa:** Pengukuran kesiapan L4 menunjukkan migrasi 83 deklarasi ganda akan
**lossy**, bukan lossless:

| field pada deklarasi ganda | jumlah | tanpa field baru                                   |
| -------------------------- | ------ | -------------------------------------------------- |
| `required_permission`      | 76     | pindah ke `require_permission` (gate)              |
| `uses`                     | 11     | **hilang** — footprint consent menyempit diam-diam |
| `params`                   | 2      | **hilang** — kontrak input hilang                  |
| bersih                     | 9      | aman dihapus apa adanya                            |

`uses` yang hilang adalah yang paling berbahaya: consent footprint adalah
justru hal yang membuat script boleh menyentuh resource — menghapusnya saat
"merapikan manifest" akan **melebarkan** akses tanpa satu pun sinyal.

**Jebakan dua-tempat sudah ditutup di akarnya.** Plan L1 memperingatkan field
harus ditulis di struct DAN di struct lokal `UnmarshalYAML`, kalau tidak field
hilang diam-diam saat load. Sejak sesi sebelumnya `UnmarshalYAML` memakai
`type plain TransitionDecl` + `,inline`, jadi sekarang cukup SATU tempat — dan
test pengunci `TestTransitionDecl_UnmarshalCarriesEveryField` sudah menutup
kelasnya.

**Bukti:** `TestActionSources_TransitionCarriesUsesAndParams`
(`pkg/spec/action_sources_test.go`) mengurai YAML nyata dan menuntut
`uses`/`params`/`expose`/`rate_limit`/`impl` sampai ke action sintetis.
Terkalibrasi: menghapus blok penyalinan field → test gagal dengan
`` `uses` dropped: a transition's consent footprint must survive the L4 migration ``.

**File:** `pkg/spec/entity.go`, `pkg/spec/action_sources_test.go`,
`schemas/formspec.schema.json` (regenerasi via `make generate-schema`, 35 kind /
167 tipe bersama).

**Verifikasi:** `go test ./internal/... ./pkg/... ./cmd/...` hijau;
`formspec validate --spec examples/kafe/spec --schema schemas` →
`85 manifest(s) validated, 0 problem(s)`.

→ L4 siap dimulai (validator anti-duplikat, lalu migrasi per-module).
