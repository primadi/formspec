# CRC Checklist Management System — `crc-checklist`

Contoh App standalone yang mereplikasi **CRC (Checklist Management System)**
untuk **Trakindo Utama** — heavy equipment/Caterpillar dealer di Indonesia.

> **Tujuan**: Proof of concept bahwa Forma bisa "clone" aplikasi jadi.
> Domain checklist management untuk servis alat berat dengan multi-role,
> state machine, child JSON items, dan dashboard.

---

## Domain Overview

### 5 Master Entities

| Entity | Karakter | Deskripsi |
|---|---|---|
| `customer` | master | Perusahaan pemilik alat berat |
| `equipment` | master | Alat berat — serial number, merek, engine hours |
| `employee` | master | Karyawan dengan 4 role (super_admin/serviceman/foreman/warehouseman) |
| `part` | master | Sparepart — dengan stock quantity & min stock |
| `checklist-template` | master | Template checklist dengan items di child jsonb |

### 4 Transaction Entities

| Entity | Karakter | State Machine | Child |
|---|---|---|---|
| `work-order` | transaction | Open→InProgress→Completed→Approved | — |
| `checklist-result` | transaction | — (draft/submit via doc_status) | items (jsonb, pass/fail) |
| `service-report` | transaction | draft→submitted | — |
| `part-request` | transaction | requested→approved/rejected→fulfilled | items (jsonb) |

### Workflows

1. **Work Order**: Serviceman membuat WO → Foreman assign → Serviceman kerja →
   Serviceman complete → Foreman approve
2. **Checklist**: Serviceman pilih template → isi items → submit hasil
3. **Part Request**: Serviceman request → Foreman approve/reject →
   Warehouseman fulfill
4. **Service Report**: Serviceman buat laporan → submit setelah servis selesai

### Roles & Permissions

| Role | Akses Utama |
|---|---|
| **Super Admin** | Semua akses — master data, transaksi, laporan |
| **Serviceman** | Buat/isi WO, checklist, service report, request part |
| **Foreman** | Assign WO, approve WO, approve/reject part request |
| **Warehouseman** | Fulfill part request, manage part stock |

---

## Fitur Forma yang Di-exercise

| Fitur | Di mana |
|---|---|
| Entity lifecycle (doc_status) | Semua entities |
| State machine dengan guard | `work-order` (4 states, 4 transitions), `part-request` (3 states) |
| Natural key sequence (yearly reset) | `work-order.number`, `checklist-result.number`, `service-report.number`, `part-request.number` |
| Child jsonb dengan sequence_field | `checklist-template.items`, `checklist-result.items`, `part-request.items` |
| Relation belongs_to | customer→equipment, employee→work-order, dll |
| Dot-path relation column | `customer.name` di equipment table, `work_order.number` di checklist table |
| Permission-gated actions | Setiap action punya `required_permission` |
| Action UI hints (confirm, icon, style) | `start-work`, `complete`, `approve`, `cancel`, `reject`, `fulfill` |
| Events + deliver channels | `work-order.completed`, `work-order.approved` |
| 11 Page kinds — list per entity + detail WO | Pages untuk setiap entity |
| 20 Table kinds — list + dot-path columns | Tables untuk setiap entity |
| 17 Form kinds — create/edit per entity | Forms untuk setiap entity |
| Dashboard + metric widgets | 4 widgets (total WO, in progress, completed, chart by priority) |
| 2 Report kinds | WO summary, checklist completion |
| Menu hirarkis 2 level dengan view resolver | Sidebar: Data Master → 5 sub, Transaksi → 4 sub |
| Theme tokens | CRC/Trakindo brand colors (#FFC107, #212529, #009F53) |
| App standalone | `forma.yaml` + `spec/apps/crc.yaml` |

---

## Struktur File

```
examples/Crc-Checklist/
├── forma.yaml                              # Dev config
├── README.md
└── spec/
    ├── apps/
    │   └── crc.yaml                        # App manifest
    ├── config/
    │   └── app.yaml                        # App config (prefix keys)
    ├── themes/
    │   └── crc-theme.yaml                  # Trakindo brand colors
    └── modules/
        └── crc/
            ├── module.yaml                 # Module + menu
            ├── master/
            │   ├── customer/               # Entity + table + forms
            │   ├── equipment/
            │   ├── employee/
            │   ├── part/
            │   └── checklist-template/
            ├── transaction/
            │   ├── work-order/             # Entity + table + forms + pages
            │   ├── checklist-result/
            │   ├── service-report/
            │   └── part-request/
            ├── pages/                      # List pages + detail page
            ├── dashboards/
            ├── widgets/
            └── reports/
```

---

## Cara Menjalankan

### Sebagai App standalone (via reference-app)

```bash
cd examples/Crc-Checklist
go run ../reference-app \
  --spec ./spec \
  --dsn "sqlite://.forma/crc.db" \
  --addr :8080
```

### Via `forma dev`

```bash
cd examples/Crc-Checklist
go run ../../cmd/forma/ dev
```

### Verifikasi

```bash
# Health check
curl http://localhost:8080/health

# Meta UI — semua manifest UI
curl http://localhost:8080/demo/api/v1/_meta/ui | jq .

# CRUD — buat customer baru
curl -X POST http://localhost:8080/demo/api/v1/crc/customers \
  -H "Content-Type: application/json" \
  -d '{"name":"PT Trakindo Utama","phone":"0211234567","email":"info@trakindo.co.id"}'

# List customers
curl http://localhost:8080/demo/api/v1/crc/customers | jq .
```

---

## Inspirasi

App ini terinspirasi dari **demo1.mitral.biz.id** — CRC Checklist Management
System yang menggunakan React + Ant Design dengan brand Trakindo
(kuning `#FFC107`, hitam `#212529`, hijau `#009F53`).
