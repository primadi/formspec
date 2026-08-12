# Dokumentasi Proyek — Aplikasi Arisan

Dokumentasi lengkap untuk aplikasi **Arisan** — platform iuran bulanan &
penarikan bergilir (rotating savings) yang dibangun di atas **FormSpec**
(spec-first, declarative ecosystem).

## Daftar Isi

| Dokumen | Isi |
|---------|-----|
| [`overview.md`](./overview.md) | Pengenalan proyek, tujuan bisnis, tech stack, dan status terkini |
| [`architecture.md`](./architecture.md) | Arsitektur: pembagian modul, karakteristik entity, keputusan desain |
| [`domain-model.md`](./domain-model.md) | Model data: 7 entity, relasi, state machine, indeks (dengan diagram ER) |
| [`automation.md`](./automation.md) | Script Starlark: `validate` & `run-lottery` + cara pakai REST API |
| [`permissions.md`](./permissions.md) | Model keamanan & permission, audit, event, kind: Policy |
| [`development.md`](./development.md) | Panduan pengembangan: command, validasi, testing, reset database |
| [`changelog.md`](./changelog.md) | Riwayat perubahan & status verifikasi |
| [`engine-sqlite-deadlock.md`](./engine-sqlite-deadlock.md) | Bug engine + patch lokal (SQLite deadlock pada `resource.fetch`) |

## Referensi Cepat

```bash
# Validasi semua manifest
formspec validate --spec spec

# Jalankan dev server (SQLite, hot-reload)
formspec dev --addr :18080

# UI aplikasi
# http://localhost:18080/default/app/arisan
```

- **Struktur spek**: `spec/apps/`, `spec/modules/<module>/`
- **17 manifest**, seluruhnya tervalidasi (`17 OK, 0 problems`)
- **3 modul**: `arisan-master`, `arisan-field`, `arisan-report`
- **7 entity**: `arisan-group`, `member`, `group-member`, `bank-mutation`,
  `contribution`, `arisan-period`, `draw`
- **2 script**: `validate.star`, `run-lottery.star`
