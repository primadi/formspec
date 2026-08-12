# Arsitektur — Aplikasi Arisan

## Pendekatan: Spec-First

Seluruh perilaku aplikasi dideklarasikan sebagai manifest YAML
(`apiVersion: formspec.dev/v1alpha1`). Engine FormSpec me-derive:

- **REST API** untuk setiap entity (`/api/v1/{module}/{plural}`)
- **UI** (Table, Form, Page) untuk setiap entity
- **State machine** dari deklarasi `state_machine`
- **Aksi custom** dari deklarasi `actions` (script Starlark atau native)
- **Event** async + deliver channel

Tidak ada kode implementasi yang ditulis manual untuk CRUD biasa — cukup
deklarasi entity, sisanya otomatis.

## Pembagian Modul (Bounded Context)

| Modul | Context | Kinds | Isi |
|-------|---------|-------|-----|
| `arisan-master` | Data master | Module, Entity | Grup, Anggota, Keanggotaan |
| `arisan-field` | Transaksi lapangan | Module, Entity | Mutasi bank, Iuran, Periode, Penarikan |
| `arisan-report` | Pelaporan | Module, Dashboard, Widget, Report | Dashboard ringkasan + rekap iuran |

**Alur dependensi**: `arisan-master` ← `arisan-field` ← `arisan-report`.
`arisan-field` dan `arisan-report` menyatakan `depends` ke modul sebelumnya.

## Karakteristik Entity

Karakteristik menentukan perilaku yang di-derive engine:

| Karakteristik | Dipakai di | Alasan |
|---------------|------------|--------|
| `master` | `arisan-group`, `member`, `group-member` | Data stabil yang dirujuk entity lain |
| `transaction` | `bank-mutation`, `contribution`, `arisan-period`, `draw` | Data append-heavy, berelasi ke master |
| `reference` | — | Data seed read-only (tidak dipakai) |
| `summary` | — | Proyeksi system-managed (tidak dipakai) |

## Keputusan Desain Penting

### 1. `lifecycle: plain_crud` + `submit` disabled

Entity di FormSpec default-nya punya `doc_status: draft` setelah create, dan
**relasi ke record draft ditolak** ("must be submitted or lifecycle-free").
Karena aplikasi arisan ini berjalan tanpa alur approval dokumen, semua entity
memakai:

```yaml
lifecycle: plain_crud
actions:
  - name: submit
    disabled: true
```

- `plain_crud` → tidak ada `doc_status` (langsung bisa dirujuk setelah create).
- `submit` disabled → flag `submitEnabled` engine menjadi false (ini satu-satunya
  cara mematikannya, selain `disabled: true` pada aksi `submit`).

### 2. Primary Key: UUID v7

Seluruh entity memakai primary key **UUID v7** (time-ordered), di-generate
engine (bukan int auto-increment). Ini konsisten untuk SQLite dan PostgreSQL.

### 3. Natural key → `unique` + bukan PK

Kode bisnis seperti `AR-001` (grup) dan `M-001` (anggota) adalah **natural key**
— dijaga unik lewat `unique: true`, bukan dijadikan primary key. Natural key
dipakai sebagai awalan `payment_ref` transfer bank (mis. `AR-001-M-001-202608`).

### 4. Batasan uniqueness (bukan composite PK)

- `group-member`: `unique (group_id, member_id)` → satu anggota sekali per grup.
- `arisan-period`: `unique (group_id, period_no)` → satu periode per nomor urut
  per grup.
- `draw`: `unique (period_id)` → satu penarikan per periode.

### 5. Relasi lintas modul memakai dot notation

Relasi ditulis dengan nama resource berkualifikasi modul:

```yaml
- name: group_id
  type: relation
  relation: { type: belongs_to, resource: arisan-master.arisan-group }
```

## Menu & UI

- **`spec/apps/arisan.yaml`** — mendefinisikan App dengan `root_url: /app/arisan`
  dan menu yang mengadopsi 3 modul.
- Setiap modul mendefinisikan menu berkategori:
  - `arisan-master`: **Master** (Grup Arisan, Anggota, Keanggotaan)
  - `arisan-field`: **Transaksi** (Mutasi Bank, Iuran, Periode, Penarikan)
  - `arisan-report`: **Laporan** (Dashboard Arisan, Rekap Iuran)
- UI admin generik: `/default/_admin` (semua entity workspace).
- UI aplikasi: `/default/app/arisan` (menu custom per App).

## Keamanan

- Setiap aksi custom mendeklarasikan `required_permission`
  (`<module>.<entity>.<action>`), mis. `arisan-field.contribution.validate`.
- Report mendeklarasikan `required_permission` untuk view.
- Aksi kritis diberi `audit: true` (dicatat ke audit log).
- Event async di-deliver ke channel `audit_log`.

## Batasan / Catatan Engine

- **Query builder (`<Entity>.query()`) dan primitif `ctx.db`/`ctx.cache`/
  `ctx.lock`/`ctx.queue` adalah stub** — belum diimplementasikan di build ini.
  Script memakai `resource.*` (`fetch`, `create`, `set`, `save`, `call`).
- **Widget metric** tidak menghitung nilai (tampil `--`) karena evaluasi query
  widget belum diimplementasikan.
- **Bug SQLite deadlock** pada `resource.fetch()` entity berelasi dalam aksi
  custom — sudah dipatch lokal. Lihat [`engine-sqlite-deadlock.md`](./engine-sqlite-deadlock.md).
