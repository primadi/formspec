# Fix: Dashboard Klinik selalu 0 — widget baca summary entity yang tidak pernah di-populate

## Perubahan

- **Bug**: Widget dashboard (`visits-today`, `revenue-today`, `visits-by-polyclinic`)
  membaca entity `clinic/daily-visit-summary` (`characteristic: summary`) yang
  **tidak pernah di-populate** — summary entity read-only via API dan belum ada
  projection engine/recompute (`formspec seed` juga belum diimplementasikan).
  Akibatnya dashboard selalu `0` / `No data` walau ada kunjungan nyata.
  Diverifikasi: `clinic_daily_visit_summaries` = 0 rows, `clinic_visits` = 6
  (1 hari ini `Q20260804-001`).
- **Fix**: Repoint 3 widget ke entity live `clinic/visit` (konsisten dengan
  precedent `pharmacy-queue-count` dan widget arisan yang baca entity live):
  - `visits-today` → `entity: visit`, `query: "transaction_date = today()"`,
    `aggregate: count`.
  - `revenue-today` → dipindah ke `transaction/visit/widgets/`,
    `entity: visit`, `query: "transaction_date = today()"`,
    `aggregate: sum`, `field: total` (format currency).
  - `visits-by-polyclinic` → `entity: visit`, `x: transaction_date`,
    `group_by: polyclinic_id`, **count-mode** (tanpa `config.y`).
- Renderer `DashboardRenderer.tsx`:
  - `ChartWidget`: dukung **count mode** — bucket per (groupBy, x), hitung row,
    agregasi per-x (multiple row di tanggal sama digabung jadi satu point).
  - `applySimpleQuery`: `today()` kini memakai **local date** (YYYY-MM-DD)
    bukan `toISOString()` (UTC) — tanggal `transaction_date`/`date` adalah
    tanggal lokal, jadi tidak bergeser di zona waktu non-UTC dekat tengah malam.
- Entity `daily-visit-summary` dipertahankan sebagai showcase
  `characteristic: summary` (read-only, tanpa menu) — bukan lagi sumber widget;
  komentar entity/dashboard + README coverage diupdate.

## Dampak

Dashboard Klinik sekarang menampilkan data nyata: "Kunjungan Hari Ini" = 1
(kunjungan `Q20260804-001`), "Pendapatan Hari Ini" = Rp 0 (jujur — kunjungan
belum selesai/tidak punya `total`), chart "Kunjungan per Poliklinik" render
seri Poli Kulit & Poli Jantung (sebelumnya "No data").

## Files affected
- `examples/Clinic-UI-Showcase/spec/modules/clinic/transaction/visit/widgets/today.yaml`
- `examples/Clinic-UI-Showcase/spec/modules/clinic/transaction/visit/widgets/revenue-today.yaml` (pindah dari `summary/daily-visit-summary/widgets/`)
- `examples/Clinic-UI-Showcase/spec/modules/clinic/transaction/visit/widgets/by-polyclinic.yaml`
- `examples/Clinic-UI-Showcase/spec/modules/clinic/summary/daily-visit-summary/entity.yaml` (komentar)
- `examples/Clinic-UI-Showcase/spec/modules/clinic/dashboards/clinic-dashboard.yaml` (komentar)
- `renderers/web/src/kinds/dashboard/DashboardRenderer.tsx` — count mode + local-date `today()`
- `examples/Clinic-UI-Showcase/README.md` — coverage table

## Referensi
- Plan: `docs/plan/fix-clinic-dashboard-summary.md`
- Deferred gap: projection engine untuk `characteristic: summary` (agar widget
  bisa kembali membaca summary entity sesuai intent spec §5).
