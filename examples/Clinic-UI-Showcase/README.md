# Clinic UI Showcase — `klinik-sehat`

Contoh yang meng-exercise **semua fitur frontend renderer** (lihat
[`docs/implementation/frontend-renderer.md`](../../docs/implementation/frontend-renderer.md)
dan spec [`docs/spec/05-frontend.md`](../../docs/spec/05-frontend.md)).
Domain klinik dipilih karena contoh-contoh kanonik di spec 05 (wizard pendaftaran
pasien, kanban antrian farmasi, timeline rekam medis, struk thermal) memetakan
1:1 ke sini.

Dua module: **clinic** (pasien, dokter, kunjungan, kasir, pengaturan) dan
**pharmacy** (obat, resep) — sengaja dua supaya sidebar module-grouping,
cross-module relation, dan cross-module widget ikut teruji.

---

## 1. Matriks Coverage — Fitur Renderer → File

### Derived by Default (D17) — fitur terpenting

Entity berikut **sengaja tidak punya manifest UI sama sekali**. Renderer harus
men-derive table, form, detail page, dan menu entry-nya (milestone 4.F3.6):

| Entity | Yang diuji |
|---|---|
| `clinic/polyclinic` | Master ≤5 field → derived form **modal** (heuristik §1.6) |
| `clinic/doctor` | 6–12 field → derived form **drawer**; relation `belongs_to` → relation-picker |
| `clinic/patient` | Coverage **semua tipe field** (string/enum/date/boolean/json/email/pattern) → field-widget library; hanya detail page-nya yang di-override |
| `pharmacy/medicine` | Derived di module kedua → sidebar grouping per module |
| `clinic/daily-visit-summary` | `characteristic: summary` → hanya list+find, **tidak dapat menu**, sumber widget |

### 12 UI Kinds

| Kind | File | Fitur yang di-exercise |
|---|---|---|
| **Page** | `clinic/pages/visits-page.yaml` | Block table tunggal, permissions |
| **Page** | `clinic/pages/patient-detail.yaml` | Route `:param`, multi-block (form view + table ber-`param` + html), title interpolation `{patient.name}` |
| **Page** | `clinic/pages/system-settings.yaml` | **Varian tabs** + Configuration Page pattern (reference doc, tanpa New/Delete) |
| **Form** | `clinic/forms/visit-edit.yaml` | `render: separate_page`, multi-section, `visible_when`/`required_when` (FormaExpr), `read_only`, child grid, submit config |
| **Form** | `clinic/forms/patient-quick-create.yaml` | `render: modal` — dialog via query string |
| **Form** | `clinic/forms/payment-quick.yaml` | Modal + `required_when` bergantung field lain |
| **Form** | `clinic/forms/patient-card.yaml` | Dipakai `mode: view` sebagai block |
| **Form** | `clinic/forms/settings-*.yaml` | `mode: edit` atas row id tetap (config pattern) |
| **Form** | `clinic/forms/visit-quick.yaml` | Step wizard, dropdown berantai |
| **Table** | `clinic/tables/visit-table.yaml` | Kolom dot-path relation, sortable/width/align, `default_sort`, 4 filter (select/date_range/text), row_actions + `confirm_msg`, bulk_actions |
| **Dashboard** | `clinic/dashboards/clinic-dashboard.yaml` | Grid layout x/y/w/h, **widget lintas module**, config per placement |
| **Widget** | `clinic/widgets/*.yaml`, `pharmacy/widgets/*.yaml` | metric (sum/count/currency), chart (line, group_by), refresh_secs |
| **Report** | `clinic/reports/revenue-by-polyclinic.yaml` | Params (date range + select), groups, totals, export csv/xlsx |
| **Wizard** | `clinic/wizards/patient-registration.yaml` | 3 step (form → form ber-`depends_on` → commit `action`), `save: partial`, route `/wizard/:name`, `?step=N` |
| **Kanban** | `pharmacy/kanbans/pharmacy-queue.yaml` | 4 kolom penuh, card_template, drag = update status optimistic |
| **Kanban** | `clinic/kanbans/consultation-board.yaml` | Subset kolom (cancelled disembunyikan), transisi dijaga state machine |
| **Timeline** | `clinic/timelines/patient-medical-history.yaml` | Entity append-only (update/delete disabled), date grouping |
| **Menu** | `clinic/menus/clinic-main.yaml` | Nested 2 level, icon, order, route ke page/wizard/kanban/report; entity tanpa menu → derived entry |
| **Menu** | `pharmacy/menus/pharmacy-main.yaml` | Grup module kedua, urutan `order` antar module |
| **Print** | `clinic/prints/queue-ticket.yaml` | Format thermal |
| **Print** | `clinic/prints/visit-invoice.yaml` | Format pdf + **html** (implementasi pertama renderer) |
| **Print** | `pharmacy/prints/prescription-label.yaml` | Thermal, label per item |
| **Theme** | `clinic/themes/showcase-theme.yaml` | Token → CSS custom properties (struct belum ada — fixture 4.B1) |

### Pola Lifecycle (§1.7 spec)

| Pola | Entity | Mekanisme |
|---|---|---|
| Plain CRUD (tanpa draft/Submit) | `patient`, `doctor`, `polyclinic`, `medicine` | `actions: submit.disabled: true` |
| 2-step + auto-save (default) | `visit` | submit aktif (tidak ditulis) |
| 1-step create-submit (quick entry) | `payment` | reserved action `create-submit` |
| Reference (Update-only, tanpa New/Delete) | `setting` | `characteristic: reference` |
| Append-only (tanpa edit/hapus) | `medical-record` | `update`+`delete` disabled |

### Fitur lintas-kind

| Fitur | Di mana diuji |
|---|---|
| Permission-driven UI (§1.4) | Semua action punya `required_permission`; row_actions/menu harus hilang bila permission tidak dimiliki (uji dengan JWT ber-perms subset) |
| State machine → tombol transisi | `visit` (3 transisi + guard diagnosis), `prescription` (4 transisi) |
| FormaExpr | `visible_when`/`required_when` di 4 form, title interpolation di `patient-detail`, guard `len(resource.items) > 0` |
| Natural key + config prefix | `visit.queue_number` (reset daily, prefix dari `Config clinic.queue_prefix`) |
| Child table (jsonb) | `visit.treatments`, `prescription.items` |
| Relation lintas module | `prescription.visit_id → clinic.visit` |
| CAS/optimistic concurrency | Drag kanban & auto-save form harus kirim `version`, tangani 409 |
| Realtime (Fase 3.5) | `visit` event `completed` deliver `websocket` — sampai transport siap, renderer refetch |
| Validasi client dari rules | pattern NIK 16 digit, email, positive, precision, min/max, past/future |

---

## 2. Cara Menjalankan

Backend (API saja — Meta API & renderer belum dibangun, ini fixture untuknya):

```bash
go run ./examples/reference-app \
  --spec examples/Clinic-UI-Showcase/spec \
  --dsn "sqlite://.forma/clinic.db" \
  --addr :8080
```

Frontend (setelah Fase 4.F berjalan): renderer di `web/` membaca
`/{ws}/api/v1/_meta/ui` — app ini tidak butuh kode frontend sendiri.
Dev: `cd web && npm run dev` dengan proxy Vite ke `:8080`.

> Catatan: `impl` script memakai primitif `ctx.*` yang sebagian masih stub;
> fokus contoh ini adalah **manifest UI**, bukan business logic.

---

## 3. Status Parsing & Meta API

Seluruh gap struct yang semula ditemukan example ini **sudah diimplementasikan
di Fase 4.B** (lihat `docs/implementation/frontend-renderer.md` §7):
`PageSpec.tabs`, `FormField.widget/compute/readonly_when`, `TableSpec.search/realtime`,
`DashboardSpec.customizable/defaults/refresh`, field-field Wizard/Kanban/Timeline
lengkap, `PrintSpec.output/header/body/footer`, `ThemeSpec`, `MenuSpec.when`,
dan hint `Action.ui` — semua YAML di example ini ter-parse typed oleh
`internal/ui` dan tervalidasi silang (entity/field/action/route refs).

App ini juga menjadi fixture end-to-end Meta API:

```bash
curl http://localhost:8080/demo/api/v1/_meta/ui        # bundle lengkap, ETag/304
curl http://localhost:8080/demo/api/v1/_meta/me        # identity + permissions
curl http://localhost:8080/demo/api/v1/_meta/entities/clinic/visit
# sort/filter list API (field ber-index):
curl "http://localhost:8080/demo/api/v1/clinic/visits?sort=-transaction_date&status=waiting"
```

---

## 4. Struktur

```
spec/
├── forma.yaml                      # kind: App — klinik-sehat
└── modules/
    ├── clinic/
    │   ├── module.yaml
    │   ├── config/clinic.yaml      # kind: Config (queue_prefix, tax, footer)
    │   ├── entities/               # patient, doctor, polyclinic, visit, payment,
    │   │                           # medical-record, setting, daily-visit-summary
    │   ├── pages/  forms/  tables/  menus/
    │   ├── dashboards/  widgets/  reports/
    │   ├── wizards/  kanbans/  timelines/  prints/  themes/
    │   └── scripts/                # *.star — transisi status + register_patient
    └── pharmacy/
        ├── module.yaml
        ├── entities/               # medicine, prescription
        ├── kanbans/  widgets/  menus/  prints/
        └── scripts/
```
