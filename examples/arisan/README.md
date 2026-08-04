# Arisan — Aplikasi Iuran & Penarikan Bergilir

> Rotating savings club (arisan): sekelompok anggota menyetor iuran bulanan
> tetap ke rekening bersama, lalu setiap bulan satu anggota memenangkan undian
> dan menerima total pot.

Dibangun di atas **Forma** — ekosistem spec-first & declarative: API, UI,
permission, state machine, dan event semuanya dideklarasikan sebagai manifest
YAML, sisanya di-derive oleh engine.

## Fitur

- **Master data**: grup arisan, anggota, keanggotaan
- **Transaksi**: mutasi bank, iuran, periode, penarikan
- **Validasi otomatis**: cocokkan iuran dengan mutasi bank (nominal + grup)
- **Undian**: `run-lottery` membuat penarikan & menutup periode
- **Laporan**: dashboard ringkasan & rekap iuran (export PDF/CSV/XLSX)

## Quick Start

```bash
# 1) Validasi seluruh manifest
forma validate --spec spec          # → 17 manifest(s) validated, 0 problems

# 2) Jalankan dev server (SQLite, hot-reload)
forma dev --addr :18080

# 3) Buka UI
#    Aplikasi : http://localhost:18080/default/app/arisan
#    Admin    : http://localhost:18080/default/_admin
```

> Prasyarat: binary `forma` di `PATH` (Go toolchain). Untuk produksi, ubah
> `dsn` di `forma-app.yaml` ke PostgreSQL.

## Struktur

```
arisan/
  forma-app.yaml    # config CLI (bukan manifest)
  spec/             # seluruh manifest YAML (App, Module, Entity, dll.)
  docs/             # dokumentasi proyek (lihat di bawah)
  schemas/          # schema JSON engine
  .agents/skills/   # AI skills untuk Copilot
```

## Dokumentasi

Baca [`docs/README.md`](docs/README.md) sebagai pintu masuk, atau langsung ke:

| Dokumen | Isi |
|---------|-----|
| [`docs/overview.md`](docs/overview.md) | Pengenalan, tujuan, tech stack, status |
| [`docs/architecture.md`](docs/architecture.md) | Arsitektur & keputusan desain |
| [`docs/domain-model.md`](docs/domain-model.md) | 7 entity, relasi, state machine, diagram ER |
| [`docs/automation.md`](docs/automation.md) | Script Starlark + REST API |
| [`docs/permissions.md`](docs/permissions.md) | Model keamanan & permission |
| [`docs/development.md`](docs/development.md) | Panduan dev, testing, reset DB |
| [`docs/changelog.md`](docs/changelog.md) | Riwayat perubahan |
| [`docs/engine-sqlite-deadlock.md`](docs/engine-sqlite-deadlock.md) | Bug engine + patch lokal |

## Lisensi

Proyek internal. Seluruh konten bersumber dari spesifikasi Forma
(`github.com/primadi/forma`).
