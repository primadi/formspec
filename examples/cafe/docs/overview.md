# Overview — Aplikasi Kafe

## Apa Itu Aplikasi Ini

Aplikasi **kafe** adalah sistem untuk mengelola operasional harian sebuah
kafe: master data menu dan meja, pencatatan pesanan (order) beserta item-nya,
pembayaran, dan laporan penjualan.

## Tujuan Bisnis

1. **Master data** — kelola item menu (makanan, minuman, dessert), meja,
   anggota (member), dan karyawan.
2. **Pencatatan pesanan** — buat pesanan per meja/anggota dengan daftar item,
   quantity, dan harga.
3. **Pembayaran** — catat pembayaran per pesanan dengan metode bayar
   (tunai, QRIS, kartu, e-wallet) dan statusnya.
4. **Status pesanan** — kelola alur pesanan dari terbuka → lunas / dibatalkan.
5. **Pelaporan** — dashboard ringkasan harian (jumlah pesanan, pendapatan,
   pesanan terbuka) dan rekap penjualan.

## Tech Stack

| Layer | Teknologi |
|-------|-----------|
| Framework | FormSpec (spec-first, declarative YAML) |
| Backend | Go, module `github.com/primadi/formspec` |
| Frontend | React + TypeScript + Vite + shadcn/ui (di-generate engine) |
| Database | PostgreSQL (produksi) / SQLite (dev) |
| Scripting | Starlark (sandboxed, script aksi custom) |
| Manifest | YAML (`apiVersion: formspec.dev/v1alpha1`) |

## Prinsip Kunci

- **Manifest-first** — seluruh API, UI, permission, dan state machine
  dideklarasikan sebagai YAML; implementasi di-derive oleh engine.
- **Satu modul = satu bounded context** — `cafe-master` (data master),
  `cafe-order` (transaksi), `cafe-report` (laporan).
- **Derived by default** — setiap `Entity` otomatis menghasilkan CRUD API,
  Table, Form, dan Page di UI.
- **Permission berbasis resource+action** — `required_permission` dipakai
  di setiap aksi, tidak pernah hardcode nama role.

## Layout Proyek

```
cafe/
  formspec-app.yaml               # Config CLI (bukan kind: Config)
  spec/
    apps/
      cafe.yaml                # kind: App
    modules/
      cafe-master/             # master data
        module.yaml
        master/
          menu-item/entity.yaml
          table/entity.yaml
          member/entity.yaml
          employee/entity.yaml
      cafe-order/              # transaksi
        module.yaml
        transaction/
          order/entity.yaml
          payment/entity.yaml
      cafe-report/             # laporan
        module.yaml
        dashboards/
        reports/
  .agents/skills/              # AI skills (scaffold formspec init)
  schemas/                     # JSON Schema untuk YAML editor
```
