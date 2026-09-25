# 2026-09-23-002 — Seed mengunggah aset lewat storage service (`$asset`)

## Apa yang diubah

Tiga hal yang saling terkait, semuanya berangkat dari satu permintaan: _"seharusnya
ketika seed, selain mengisi data ke db, juga meng-copy seed picture ke storage,
copy-nya harus melalui service"_.

1. **Marker `$asset` pada record seed.** `photo: { $asset: "menu/sate-ayam.jpg" }`
   menyatakan **path aset** relatif ke `<module-dir>/assets/`; seed meng-INSERT
   record dulu (id tercipta untuk key), lalu **mengunggah file lewat storage
   service** dan menulis **key kanonik**
   `{ws}/{module}/{entity}/{id}/{field}/{uuid}-{nama}` ke field — bentuk key yang
   sama dengan unggah HTTP. `allowed_types` dan `max_size_mb` ditegakkan dengan
   matcher yang sama dengan handler unggah (`internal/api`), sehingga seed tidak
   bisa menulis objek yang API-nya tolak.
2. **Reconcile menggantikan skip-murni.** Record yang natural key-nya sudah ada
   tetapi field-nya berbeda kini **di-update** dan dilaporkan (`0 inserted, 22
updated, 52 skipped`), lalu idempoten (`0 updated` pada run berikutnya).
   Pengecualian: field **write-only** (`masked: true`) dan record yang
   `doc_status`-nya bukan `draft`.
3. **Aset dipindah ke module-relative** `spec/modules/cafe-master/assets/menu/`
   (dari `examples/kafe/assets/menu/`), plus 3 gambar: `es-jeruk.jpg` diganti
   (sebelumnya File:Kopi O.jpg — gambar kopi untuk menu "Es Jeruk"), dan
   `roti-bakar.jpg` + `kopi-susu.jpg` ditambahkan. Roti Bakar & Kopi Susu juga
   masuk seed sebagai record + harga.

## Kenapa

- **`cp` tidak punya padanan di prod.** Target `make seed-kafe-assets` menyalin
  file langsung ke `examples/kafe/.formspec/storage/seed/menu/` dengan key
  karangan (`seed/menu/<file>`). Itu hanya bisa menulis filesystem lokal —
  sementara prod memakai garage/minio/s3 — jadi "seed bergambar" hanya hidup di
  dev, dan tidak ada yang bisa memverifikasi bahwa field `file` menunjuk objek
  yang sah.
- **Skip murni membuat seed yang diperbaiki tidak pernah sampai.** Memperbaiki
  nilai di file seed tidak mengubah database yang sudah ada: seed terlihat benar
  sementara aplikasi tetap salah. Kasus nyata ada di DB dev kafe — `K01`/`R01`
  dibuat manual lewat UI dengan `photo` kosong.
- **Perbandingan byte diperlukan.** Mengganti `es-jeruk.jpg` dengan gambar yang
  benar tidak terkirim bila pemeriksaannya hanya "objek ada"; key kanonik
  memuat UUID baru setiap unggah, jadi kesamaan tidak bisa dinilai dari nilai
  field. Dibandingkan byte (seed = file kecil).

## Bug yang ikut tertangkap

Reconcile pertama kali menulis **`password: 'kafe123'` plaintext** ke record user:
field `password` dideklarasikan `masked: true` dan di-hash oleh hook before
create/update, sedangkan `UpdateFields` tidak menjalankan hook. Diperbaiki dengan
melewati seluruh field `masked`, dan `user` selalu `skip`. Terverifikasi: 0 user
dengan key `password`.

## File terdampak

| File                                                       | Perubahan                                                                                                                           |
| ---------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| `internal/api/file.go`                                     | `ObjectKey`, `SanitizeFilename`, `AllowedFileType`, `MinUploadLimitMB`, `ObjectExists` diekspor; handler upload memakai `ObjectKey` |
| `resource/storage_resolve.go` (baru)                       | `ResolveStorage` + `NewDatastoreRegistryFromManifests` — satu tempat memilih garage/minio/s3 vs filesystem                          |
| `resource/formspec.go`                                     | blok storage inline diganti `ResolveStorage`                                                                                        |
| `cmd/formspec/seed_asset.go` (baru)                        | resolusi module dir + aset, `$asset`, unggah + attach                                                                               |
| `cmd/formspec/seed.go`                                     | reconcile, guard `masked`/`doc_status`, `resolveDSN` + state dir, ringkasan `updated`                                               |
| `cmd/formspec/seed_asset_test.go` (baru)                   | 6 test (`$asset`, reconcile, idempotensi, pemulihan, `masked`, validasi)                                                            |
| `Makefile`                                                 | `seed-kafe-assets` dihapus; `seed-kafe` memakai path project-root-relative                                                          |
| `pkg/spec/seed.go`                                         | godoc `$asset`/`$ref`/reconcile; `schemas/` diregenerasi                                                                            |
| `examples/kafe/spec/modules/cafe-master/assets/**`         | 9 foto + `ATTRIBUTION.md` (module-relative)                                                                                         |
| `examples/kafe/spec/modules/cafe-master/seeds/master.yaml` | `$asset` + record K01/R01 + harganya                                                                                                |
| `docs/cli-tools/02-formspec-cli.md` §6                     | dokumentasi `$asset` + reconcile                                                                                                    |
| `examples/kafe/gaps_found/TODO.md`                         | 10.4 diperbarui; 10.14 ✅; 10.15/10.16/10.17 ⏸️                                                                                     |

## Bukti

`make seed-kafe` pada DB dev → `0 inserted, 25 updated, 49 skipped`, rerun →
`0 inserted, 0 updated, 74 skipped`; `rm -rf` storage lalu seed → **9 objek
dipulihkan**; DB baru → `74 inserted` dengan 9 key kanonik; **9/9 foto** via
`GET .../{id}/photo` → **200 image/jpeg, sha256 identik** dengan file di
`assets/menu/`; `go test ./...` hijau; `formspec validate --spec
examples/kafe/spec --schema schemas` → **85 manifest, 0 problem**.

Plan: `docs_internal/plan/seed-assets-and-reconcile.md`.
