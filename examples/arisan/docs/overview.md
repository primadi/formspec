# Overview — Aplikasi Arisan

## Apa Itu Arisan

**Arisan** adalah aplikasi untuk mengelola arisan berbentuk **rotating savings
club**: sekelompok anggota menyetor iuran bulanan tetap ke rekening bersama,
lalu setiap bulan salah satu anggota memenangkan undian dan menerima total pot.

## Tujuan Bisnis

1. **Master data** — kelola grup arisan, anggota, dan keanggotaan (anggota
   mana yang ikut grup mana).
2. **Pencatatan transaksi** — catat mutasi bank rekening grup dan iuran yang
   dibayar anggota.
3. **Validasi iuran** — cocokkan iuran dengan mutasi bank (nominal + grup)
   untuk memastikan pembayaran benar.
4. **Undian & penarikan** — jalankan undian per periode, catat pemenang, tandai
   penarikan yang sudah dibayarkan.
5. **Pelaporan** — dashboard ringkasan & rekap pembayaran iuran per grup/periode.

## Tech Stack

| Layer | Teknologi |
|-------|-----------|
| Framework | Forma (spec-first, declarative YAML) |
| Backend | Go, module `github.com/primadi/forma` |
| Frontend | React + TypeScript + Vite + shadcn/ui (di-generate engine) |
| Database | PostgreSQL (produksi) / SQLite (dev) |
| Scripting | Starlark (sandboxed, script aksi custom) |
| Manifest | YAML (`apiVersion: forma.dev/v1alpha1`) |

## Prinsip Kunci

- **Manifest-first** — seluruh API, UI, permission, state machine, dan event
  dideklarasikan sebagai YAML; implementasi di-derive oleh engine.
- **Satu modul = satu bounded context** — `arisan-master` (data master),
  `arisan-field` (transaksi), `arisan-report` (laporan).
- **Derived by default** — setiap `Entity` otomatis menghasilkan CRUD API,
  Table, Form, dan Page di UI.
- **Permission berbasis resource+action** — `required_permission` dipakai
  di setiap aksi, tidak pernah hardcode nama role.

## Status Proyek (2026-08-03)

| Area | Status |
|------|--------|
| 17 manifest (App, 3 Module, 7 Entity, Dashboard, 4 Widget, Report) | ✅ Tervalidasi |
| CRUD API semua entity | ✅ Berfungsi |
| Aksi `validate` (script penuh, `resource.fetch`) | ✅ Berfungsi (setelah patch engine) |
| Aksi `run-lottery` (script + `resource.create`) | ✅ Berfungsi |
| Guard state machine (periode ganda, dll.) | ✅ Berfungsi |
| UI admin & aplikasi (menu, tabel, dashboard, report) | ✅ Render |
| Widget metric (nilai kalkulasi) | ⚠️ Render, nilai `--` (query evaluasi belum diimplementasikan engine) |

> **Catatan engine**: aksi custom yang memanggil `resource.fetch()` pada entity
> berelasi sempat **deadlock di SQLite** (bug engine). Sudah dipatch lokal.
> Lihat [`engine-sqlite-deadlock.md`](./engine-sqlite-deadlock.md).

## Layout Proyek

```
arisan/
  forma-app.yaml               # Config CLI (bukan kind: Config)
  spec/
    apps/
      arisan.yaml              # kind: App
    modules/
      arisan-master/           # master data
        module.yaml
        master/
          arisan-group/entity.yaml
          member/entity.yaml
          group-member/entity.yaml
      arisan-field/            # transaksi
        module.yaml
        transaction/
          bank-mutation/entity.yaml
          contribution/entity.yaml
            scripts/validate.star
          arisan-period/entity.yaml
            scripts/run-lottery.star
          draw/entity.yaml
      arisan-report/           # laporan
        module.yaml
        dashboards/
          arisan-summary-dashboard.yaml
          widget-active-groups.yaml
          widget-open-periods.yaml
          widget-paid-count.yaml
          widget-pending-count.yaml
        reports/
          payment-recap-report.yaml
  docs/                        # dokumentasi ini
  schemas/                     # schema JSON engine
  .agents/skills/              # AI skills untuk Copilot
```
