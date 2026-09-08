# 2026-09-06-003 — Dokumentasi Clinic UI Showcase

## Apa

Menambahkan folder `docs/` untuk example `examples/Clinic-UI-Showcase`
mengikuti pola dokumentasi example lain (`examples/arisan/docs`,
`examples/cafe/docs`):

- `docs/README.md` — index dokumentasi + referensi cepat
- `docs/overview.md` — tujuan showcase, tech stack, prinsip, layout proyek
- `docs/architecture.md` — 2 App / 2 module, karakteristik entity (semua 4),
  pola lifecycle §1.7, keputusan desain (natural key, child jsonb, hooks,
  backdate policy, sidecar Node, menu suggestion)
- `docs/domain-model.md` — 11 entity dengan diagram ER + state machine
  (visit, prescription, otc-sale), tabel natural key, daftar script Starlark
- `docs/development.md` — menjalankan, validasi, testing e2e, reset DB,
  tips mengedit spec

## Kenapa

Example Clinic-UI-Showcase adalah fixture renderer terlengkap tetapi belum
punya folder `docs/` — dokumentasinya tersebar di README root (matriks
coverage) dan `how-to-run.md`. Set dokumentasi ini melengkapi keduanya tanpa
menggantikannya, dengan README root tetap menjadi sumber matriks fitur →
file.

## File Terdampak

- `examples/Clinic-UI-Showcase/docs/` (baru, 5 file)

## Referensi

- Pola: `examples/arisan/docs/`, `examples/cafe/docs/`
- Kontrak: `docs/spec/05-frontend.md`
