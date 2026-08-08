# Plan: Dashboard Klinik Selalu 0 — Widget Baca Summary yang Tidak Pernah Di-Populate

**Status**: Implemented 2026-08-04 (follow-up dari `docs/plan/todo_fix_clinic.md`).
**Referensi spec**: `docs/spec/05-frontend.md` (§5 Dashboard/Widget), `docs/spec/02-core-basic.md` (entity characteristic).

---

## 1. Gejala

User: "hari ini ada 1 kunjungan, di dashboard tetap 0".

Verifikasi langsung (DB sqlite `examples/Clinic-UI-Showcase/.forma/clinic.db` + API `forma dev --dev-ui`):

- `clinic_visits` → **6 rows**, termasuk 1 kunjungan hari ini
  (`Q20260804-001`, `transaction_date: 2026-08-04`, `status: in_consultation`).
- `clinic_daily_visit_summaries` → **0 rows** (kosong). API
  `GET /default/_ui/entity/clinic/daily-visit-summary` → `total: 0`.
- Widget dashboard `visits-today` / `revenue-today` / `visits-by-polyclinic`
  semuanya `entity: daily-visit-summary` → selalu `0` / `No data`.

## 2. Root cause

Entity `daily-visit-summary` ber-`characteristic: summary` — per spec,
"system-managed projection — no CUD via API". `renderers/jsonbpersist/crud.go`
menolak semua create/update/delete untuk `CharSummary`. **Tapi tidak ada
projection engine / recompute / seed yang mengisi entity ini di mana pun**
dalam codebase (`forma seed` belum diimplementasikan, tidak ada mapping
summary→source entity di spec). Akibatnya tabel summary **permanen kosong** dan
dashboard tidak akan pernah mencerminkan data kunjungan nyata — bukan sekadar
"data kosong", melainkan jalur data yang putus.

## 3. Keputusan fix

**Repoint widget dashboard ke entity live `clinic/visit`**, konsisten dengan
precedent yang sudah berjalan di repo yang sama:

- `pharmacy-queue-count` (metric) → `entity: prescription` + `query` +
  `aggregate: count` — sudah jalan.
- Widget arisan (`internal/entity/testdata/arisan`) → baca entity master/field
  langsung + `query`.

Dua alternatif lain dievaluasi & ditolak untuk sesi ini:

- **Implement projection engine** (recompute summary saat visit berubah) —
  fitur platform besar, butuh mapping summary→source di spec + extension kind;
  out of scope untuk bug fix. Dicatat sebagai deferred gap.
- **Seed summary sekali** — tidak sinkron dengan kunjungan baru, dashboard
  kembali basi; bukan fix.

Entity `daily-visit-summary` **dipertahankan** sebagai showcase
`characteristic: summary` (read-only via API, tanpa menu derived) — hanya
bukan lagi sumber data widget.

## 4. Perubahan

| File | Perubahan |
|---|---|
| `spec/modules/clinic/transaction/visit/widgets/today.yaml` | `entity: daily-visit-summary` → `visit`; `query: date = today()` → `transaction_date = today()`; `config: { aggregate: count }` |
| `spec/modules/clinic/summary/daily-visit-summary/widgets/revenue-today.yaml` → **pindah** ke `spec/modules/clinic/transaction/visit/widgets/revenue-today.yaml` | `entity: visit`; `query: transaction_date = today()`; `aggregate: sum`, `field: total` |
| `spec/modules/clinic/transaction/visit/widgets/by-polyclinic.yaml` | `entity: visit`; `config: { x: transaction_date, group_by: polyclinic_id }` + **count mode** (tanpa `y`) |
| `spec/modules/clinic/summary/daily-visit-summary/entity.yaml` | Update komentar: entity showcase characteristic, bukan sumber widget |
| `spec/modules/clinic/dashboards/clinic-dashboard.yaml` | Update komentar (widget baca live entity list) |
| `renderers/web/src/kinds/dashboard/DashboardRenderer.tsx` | (a) `applySimpleQuery`: `today()` pakai **local date** bukan `toISOString()` (UTC) — tanggal `transaction_date`/`date` adalah tanggal lokal; (b) `ChartWidget`: dukung **count mode** (tanpa/`y: count`) — bucket per (groupBy, x), hitung row, agregasi per-x |
| `examples/Clinic-UI-Showcase/README.md` | Update baris coverage `daily-visit-summary` ("sumber widget" → showcase characteristic saja) |

### Catatan rendering

- `revenue-today` `sum(field: total)` di `visit`: kunjungan hari ini belum
  punya `total` (status `in_consultation`) → Rp 0 **jujur**.
- Chart `by-polyclinic` `group_by: polyclinic_id` → backend expand
  `polyclinic.name` (sudah terverifikasi di response API) → legend nama poli.

## 5. Verifikasi

- Spec hot-reload (dev server jalan): `GET /default/_ui/_meta/ui` → `entity` widget = `clinic/visit`.
- Simulasi logika query + aggregate (node).
- `cd renderers/web && npx tsc -b --noEmit` bersih.
- (Browser tool tidak bisa akses server di port 8080 — bukan port forwarded
  18080; verifikasi visual dilakukan lewat API + review.)

## 6. Deferred gap

- [ ] Projection engine untuk `characteristic: summary` (recompute otomatis dari
  entity transaksi) — dibutuhkan agar widget bisa kembali membaca summary entity
  sesuai intent spec §5. Lihat juga event `visit.completed` ("realtime board +
  summary") di `visit/entity.yaml`.
