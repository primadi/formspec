# CRC Checklist Management System — Contoh App Baru

**2026-07-30**

Membuat contoh App standalone baru `examples/Crc-Checklist/` yang mereplikasi
CRC (Checklist Management System) untuk Trakindo Utama — heavy equipment dealer.

## Yang dibuat

- **29 file spec YAML** — 1 App manifest, 1 module manifest, 1 theme, 1 config,
  5 master entities, 4 transaction entities (2 dengan state machine), 9 pages,
  17 forms, 9 tables, 1 dashboard, 4 widgets, 2 reports
- **README.md** — dokumentasi lengkap domain, fitur, struktur, cara menjalankan
- **`forma.yaml`** — dev config untuk `forma dev`

## Domain

- Master: customer, equipment, employee, part, checklist-template
- Transaction: work-order (state machine: Open→InProgress→Completed→Approved),
  checklist-result (child jsonb items), service-report, part-request
  (state machine: requested→approved/rejected→fulfilled)
- 4 role: super_admin, serviceman, foreman, warehouseman

## Fitur yang di-exercise

Entity lifecycle, state machine + guard, natural key sequence (yearly reset),
child jsonb with sequence_field, relation belongs_to, dot-path relation column,
permission-gated actions, action UI hints, events, pages/tables/forms per entity,
dashboard + metric widgets, reports, hierarchical menu, theme tokens.

## Referensi plan

`docs/plan/todo.md` (master plan) — ini adalah contoh baru, tidak terkait
langsung fase-fase yang sedang berjalan.
