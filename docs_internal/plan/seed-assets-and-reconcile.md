# Plan: `$asset` pada seed + reconcile + hapus `seed-kafe-assets`

**Status**: in progress (2026-09-23)
**Diminta**: "seharusnya ketika seed, selain mengisi data ke db, juga meng-copy
seed picture ke storage, copy-nya harus melalui service" + hapus target
`seed-kafe-assets` + perbaiki gambar (es jeruk salah, roti bakar & kopi susu
belum ada).

---

## 1. Masalah yang diselesaikan

### 1.1 Asset seed ditanam "di luar engine"

`make seed-kafe` mengisi DB, lalu `make seed-kafe-assets` **menyalin file dengan
`cp`** ke `examples/kafe/.formspec/storage/seed/menu/`. Dua langkah, dua sumber
kebenaran, dan langkah kedua sama sekali **bukan** jalur storage engine:

- tidak memakai `ctx.storage` / datastore registry → **di prod tidak ada
  padanannya** (prod memakai garage/minio/s3, bukan filesystem), jadi resep
  "seed bergambar" hanya bisa jalan di dev;
- key `seed/menu/<file>` adalah **key karangan** yang tidak mengikuti konvensi
  `{workspace}/{module}/{entity}/{id}/{field}/{uuid}-{name}` — sehingga tidak ada
  yang bisa memverifikasi bahwa field `file` benar-benar menunjuk objek yang sah;
- `rm -rf .formspec/` membuat semua foto hilang sampai `cp` dijalankan ulang,
  tanpa gejala apa pun selain gambar rusak di katalog.

Akar masalahnya ada di komentar `Makefile` sendiri: _"a seed cannot produce an
upload key"_ — karena key kanonik memuat **id record**, yang belum ada sebelum
insert. Itu benar, tetapi jalan keluarnya bukan `cp`; jalan keluarnya adalah
**mengunggah setelah insert lewat service storage**, yang justru tahu id itu.

### 1.2 Idempotensi = skip, sehingga drift tidak pernah sembuh

`seedAll` melewati record yang sudah ada (`skip ... (already exists)`).
Akibatnya seed yang **diperbaiki** (mis. `photo` yang tadinya kosong) tidak
mengubah apa pun pada DB yang sudah ada — seed terlihat benar, tapi diam.
Kasus nyata ada di DB dev kafe: `K01 Kopi Susu` & `R01 Roti Bakar` dibuat manual
lewat UI (di luar seed), `photo` kosong; menambahkannya ke seed dengan semantik
`skip` tidak akan pernah mengisi fotonya.

### 1.3 Gambar yang salah / hilang

`es-jeruk.jpg` adalah **File:Kopi O.jpg** (kopi, bukan jeruk) — salah gambar
sejak awal. "Roti Bakar" & "Kopi Susu" tidak punya entri seed sama sekali
(baik record maupun gambar), padahal keduanya muncul di katalog kafe karena
dibuat manual.

---

## 2. Keputusan desain

| #   | Keputusan                                                                                                            | Alasan                                                                                                                                                                                                                                                                                                                 |
| --- | -------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| D1  | Marker eksplisit `$asset: "<path di assets module>"` — sejajar dengan `$ref`                                         | Nilai `file` hari ini **adalah** key storage, dan key itu juga bisa ditulis literal (mis. `resource/storage_reload_e2e_test.go`). Deteksi implisit membuat "ini asset yang harus diunggah" vs "ini key literal" tidak bisa dibedakan. `$asset` menyatakan niat, dan resolusinya memakai walker yang sama dengan `$ref` |
| D2  | Key kanonik yang dipakai: `{ws}/{module}/{entity}/{id}/{field}/{uuid}-{name}` — **identik** dengan jalur unggah HTTP | Satu konvensi key untuk semua jalur tulis; `Download`/link/visibility yang sudah ada bekerja tanpa cabang baru                                                                                                                                                                                                         |
| D3  | Upload lewat **datastore registry** (`Resolve("storage", ...)`), bukan `memory.NewStorage` langsung                  | Itu arti "melalui service": dev → filesystem, prod → garage/minio/s3, tanpa `if prod` di CLI                                                                                                                                                                                                                           |
| D4  | Validasi unggah (`allowed_types`, `max_size_mb`) memakai helper yang **sama** dengan handler HTTP                    | Seed tidak boleh bisa menulis objek yang API-nya tolak (konvensi "seed = payload yang shape-nya sama dengan create body")                                                                                                                                                                                              |
| D5  | Reconcile: record yang sudah ada tapi field-nya berbeda **di-update** dan dilaporkan (`updated`)                     | Menutup lubang 1.2. Dilaporkan per-field, bukan senyap. Tidak ada duplikat (identitas tetap natural key)                                                                                                                                                                                                               |
| D6  | Reconcile **dilewati** untuk record dengan `doc_status` bukan `draft`                                                | `UpdateFields` melewati lifecycle; mengubah dokumen yang sudah di-submit dari seed bukan perilaku yang diinginkan                                                                                                                                                                                                      |
| D7  | Asset reconcile: field kosong → unggah; objek ada → biarkan; objek **hilang** dari storage → unggah ulang            | Kunci unik per record berarti "sama" tidak bisa dinilai dari nilai field. Yang bisa dinilai: apakah objeknya ada. Ini juga yang membuat `rm -rf .formspec/` sembuh sendiri (menggantikan `seed-kafe-assets`)                                                                                                           |
| D8  | Aset disimpan **module-relative**: `spec/modules/<module>/assets/<path>`                                             | Route asset menyajikan `modules/{module}/assets/{path}`; module harus self-contained agar bisa di-vendor/di-publish (workflow 13.1.4)                                                                                                                                                                                  |
| D9  | `resolveDSN` dipakai di `seed` (seperti `dev`/`backup`/`repl`) + `make seed-kafe` tidak lagi `cd`                    | DSN relatif harus di-anchor ke project root (plan `dsn-spec-anchored.md`); tanpa itu `make -C`/IDE menjalankan seed menulis DB di CWD yang berbeda                                                                                                                                                                     |
| D10 | State dir fallback: sqlite → turunan DSN; non-sqlite → `<project-root>/.formspec`                                    | `StateDirFromDSN("postgres://…")` mengembalikan `.formspec` relatif **CWD** — di prod itu tidak deterministik                                                                                                                                                                                                          |

---

## 3. File yang berubah

| File                                                                       | Perubahan                                                                                                                                                 | Effort |
| -------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- | ------ |
| `internal/api/file.go`                                                     | Ekstrak + ekspor helper yang dipakai dua jalur: `ObjectKey`, `SanitizeFilename`, `AllowedFileType`, `MinUploadLimitMB`/`UploadLimitMBFor`, `ObjectExists` | small  |
| `resource/storage_resolve.go` (baru)                                       | `ResolveStorage(dsReg, stateDir)` — satu tempat yang memilih garage/minio/s3 vs filesystem fallback; boot (`formspec.go`) dan CLI memakai ini             | small  |
| `resource/formspec.go`                                                     | Blok inline storage (baris ~688–733) diganti pemanggilan `ResolveStorage`                                                                                 | small  |
| `cmd/formspec/seed_asset.go` (baru)                                        | Resolusi module dir + aset, penyisipan `$asset`, unggah + set field                                                                                       | medium |
| `cmd/formspec/seed.go`                                                     | `$asset` di walker, reconcile (`updated`), guard `doc_status`, lazy storage, `resolveDSN` + state dir, pesan ringkasan                                    | medium |
| `cmd/formspec/seed_test.go`, `cmd/formspec/backup_test.go`                 | Ikut signature baru + test `$asset`/reconcile                                                                                                             | small  |
| `Makefile`                                                                 | Hapus `seed-kafe-assets`; `seed-kafe` tanpa `cd`                                                                                                          | small  |
| `examples/kafe/spec/modules/cafe-master/assets/` (baru)                    | 9 gambar (7 lama + 2 baru, 1 diganti) + `ATTRIBUTION.md` pindah ke sini                                                                                   | small  |
| `examples/kafe/spec/modules/cafe-master/seeds/master.yaml`                 | `photo: {$asset: …}`, + Roti Bakar (R01) & Kopi Susu (K01) + harganya                                                                                     | small  |
| `examples/kafe/gaps_found/TODO.md`, `docs/cli-tools/02-formspec-cli.md` §6 | Dokumentasi mekanisme baru                                                                                                                                | small  |

Dependensi: `internal/api` helper → `resource.ResolveStorage` → `cmd/formspec`
(tidak ada arah balik; `resource` sudah mengimpor `internal/api`).

---

## 4. Kontrak `$asset`

```yaml
- entity: menu-item
  records:
    - code: MKN-002
      name: "Sate Ayam Madura"
      # Bukan key storage: path relatif terhadap `<module-dir>/assets/`.
      # Seed mengunggahnya lewat storage service, lalu menulis KEY kanonik
      # ke field `photo` — sama seperti jalur unggah HTTP.
      photo: { $asset: "menu/sate-ayam.jpg" }
```

Aturan:

1. `$asset` hanya sah untuk field ber-type `file` (atau `attachment`). Pada field
   lain → error per record, bukan diam.
2. `allowed_types` + `max_size_mb` field diberlakukan (parity D4).
3. Kegagalan unggah **tidak** membatalkan baris yang sudah ter-insert (insert
   sudah commit), tetapi dilaporkan sebagai `failed` untuk record itu, dengan
   pesan yang menyebut field + path. Baris dengan field kosong itu benign dan
   sembuh saat seed dijalankan ulang.
4. Aset yang tidak ditemukan → error yang menyebut path yang dicoba + module
   dir yang dipakai (bukan "file not found" telanjang).
5. Tanpa `$asset`, seed **tidak menyentuh** storage sama sekali (resolusi lazy) —
   penting supaya test/seed yang murni data tetap jalan tanpa storage.

---

## 5. Verifikasi (bukti, bukan klaim)

| #   | Langkah                                                              | Bukti yang diharapkan                                                                             |
| --- | -------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------- |
| V1  | `make seed-kafe` pada DB dev yang sudah ada                          | `R01`/`K01` **di-update** (photo terisi) + 7 foto lain diunggah; rerun → `0 inserted / 0 updated` |
| V2  | `rm -rf examples/kafe/.formspec/storage && make seed-kafe`           | foto kembali ada **tanpa** `cp` manual                                                            |
| V3  | `GET /kafe/_ui/entity/cafe-master/menu-item/{id}/photo` untuk 9 menu | 200 + `image/jpeg` + byte > 0 untuk semuanya                                                      |
| V4  | `GET .../photo` untuk `es-jeruk`, `roti-bakar`, `kopi-susu`          | magic `FF D8 FF` dan bukan lagi gambar kopi untuk es jeruk (dinilai dengan mata)                  |
| V5  | `go test ./...`                                                      | hijau, termasuk test baru `$asset` + reconcile                                                    |
| V6  | `formspec validate --spec examples/kafe/spec`                        | 0 problem                                                                                         |

---

## 6. Sisa yang diketahui (masuk todo sebagai `[⏸️]`)

- `docs/kind/` belum punya halaman `Seed` (kind terdaftar tapi tidak ada di
  `kindGroups`, jadi `make generate-kind-docs` melewatinya → README bilang
  "34 kind"). Dokumentasi `$asset` sementara ditaruh di godoc `pkg/spec/seed.go`
  - `docs/cli-tools/02-formspec-cli.md` §6.
- Runtime (`formspec dev`) masih memakai `StateDirFromDSN` yang CWD-relatif untuk
  DSN non-sqlite (D10 hanya diperbaiki di jalur seed).
- `formspec backup` belum menyertakan objek storage (sudah tercatat 4.8.1).
