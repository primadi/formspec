# Panduan Pengembangan — Aplikasi Arisan

## Prasyarat

- **Go** toolchain (untuk CLI `forma`).
- Binary **`forma`** tersedia di `PATH` (di mesin ini:
  `C:\Users\prima\go\bin\forma.exe`).
- Tidak perlu database terpasang untuk dev — pakai **SQLite** via `dsn`.

## Command Utama

```bash
# 1) Validasi seluruh manifest YAML terhadap schema + engine loader
forma validate --spec spec
# → 17 manifest(s) validated, 0 problem(s) found

# 2) Jalankan dev server (SQLite, hot-reload terhadap perubahan spec)
forma dev --addr :18080

# 3) (Opsional) Register manifest ke Control Plane — HANYA jika ada CP
#    di :8443. Untuk kerja lokal TIDAK dipakai; `forma dev` membaca spec
#    langsung dari folder.
forma apply --spec spec
```

### Konfigurasi `forma-app.yaml`

```yaml
spec: spec                    # folder manifest
dsn: sqlite:.forma/arisan.db  # SQLite untuk dev
# runtime: node               # aktifkan bila ada sidecar app/ (belum dipakai)
```

Ubah `dsn` ke PostgreSQL untuk produksi, mis.:

```yaml
dsn: postgres://user:pass@localhost:5432/arisan?sslmode=disable
```

## Alur Kerja

1. Tulis/ubah manifest YAML di `spec/` (mulai dari `kind: Entity`).
2. Jalankan `forma validate --spec spec` sampai bersih.
3. `forma dev` — server hot-reload setiap ada perubahan spec (log:
   `spec change detected ... reloading`).
4. Tes lewat REST API atau UI:
   - UI aplikasi: `http://localhost:18080/default/app/arisan`
   - UI admin: `http://localhost:18080/default/_admin`

## Testing

Smoke test end-to-end (tanpa framework test):

```bash
# Buat record master
curl -X POST http://localhost:18080/default/api/v1/arisan-master/arisan-groups \
  -H "Content-Type: application/json" \
  -d '{"code":"AR-001","name":"Arisan Kantor","monthly_amount":200000,"term_months":8}'

# Cek list + find (relasi ter-resolve)
curl http://localhost:18080/default/api/v1/arisan-master/arisan-groups

# Aksi custom: validasi iuran terhadap mutasi
curl -X POST http://localhost:18080/default/api/v1/arisan-field/contributions/<id>/validate \
  -H "Content-Type: application/json" -d '{"mutation_id":"<mutation-id>"}'

# Aksi custom: undian & tutup periode
curl -X POST http://localhost:18080/default/api/v1/arisan-field/arisan-periods/<id>/run-lottery \
  -H "Content-Type: application/json" \
  -d '{"member_id":"<member-id>","contribution_id":"<contribution-id>"}'
```

> Respons list berisi `data`, `meta`, `links`. Field kueri seperti `page_size`
> **belum didukung** (VALIDATION_ERROR) di build ini.

## Reset Database Dev

Database SQLite dev berada di `.forma/arisan.db`. Untuk reset bersih:

```bash
# Hentikan server, lalu hapus file DB
Remove-Item .forma\arisan.db -Force
# Restart `forma dev` — engine akan membuat skema dari nol
```

## Catatan SQLite vs PostgreSQL

| Aspek | SQLite (dev) | PostgreSQL (produksi) |
|-------|--------------|------------------------|
| Aksi custom `resource.fetch()` entity berelasi | ⚠️ butuh patch engine (lihat bawah) | ✅ normal |
| Koneksi | tunggal (bisa deadlock) | pool multi-koneksi |
| Mode | `forma dev` | `dsn: postgres://...` |

## Bug Engine: SQLite Deadlock pada `resource.fetch`

Aksi custom yang memanggil `resource.fetch()` pada entity **berelasi** dapat
**deadlock di SQLite** karena `resolveRelations` memakai koneksi base alih-alih
koneksi transaksi aksi. Sudah dipatch lokal (module cache) dan `forma.exe`
di-rebuild.

- Detail lengkap: [`engine-sqlite-deadlock.md`](./engine-sqlite-deadlock.md)
- ⚠️ Patch ada di Go module cache (tidak ter-commit). Hilang bila
  `go clean -modcache` atau engine di-upgrade — lihat dokumen untuk cara
  mengulang.

## Struktur File Referensi

| Path | Isi |
|------|-----|
| `forma-app.yaml` | Konfigurasi CLI (bukan manifest) |
| `spec/apps/arisan.yaml` | Manifest `kind: App` |
| `spec/modules/<mod>/module.yaml` | Manifest `kind: Module` |
| `spec/modules/<mod>/<char>/<entity>/entity.yaml` | Manifest `kind: Entity` |
| `spec/modules/<mod>/<char>/<entity>/scripts/*.star` | Script aksi custom |
| `spec/modules/arisan-report/dashboards/` | Dashboard + Widget |
| `spec/modules/arisan-report/reports/` | Report |
| `schemas/` | Schema JSON engine (referensi) |
