# PersistBackend

**Version:** 0.1.0 · **Status:** Outline

> Dokumen berstatus Outline: heading di bawah menetapkan cakupan final; isi
> ditulis bertahap.

## 1. Kedudukan
PersistBackend adalah seam penyimpanan **setara Shell di sisi visual**: satu
implementasi resmi (persist-postgres, hybrid JSONB) dipakai lama, tapi seluruh
framework wajib bicara ke interface ini — tidak ada shortcut ke Postgres di kode
inti. Kind `PersistBackend` dideklarasikan formal, satu per deployment scope.

## 2. Interface Wajib
Daftar kemampuan minimal setiap backend:
- **Structural diff apply** — menerima diff skema dari framework, menerjemahkan
  ke storage-nya.
- **Query resolution** — memenuhi seluruh filter operator kontrak (identik antar
  backend).
- **`ctx.next_key`** — sequence gap-free per natural-key rule.
- **Index generation** — memenuhi `persist.indexes`.
- **Uninstall extension bersih** — tanpa sisa.

## 3. Jaminan yang Dipertahankan
Gap-free sequence, transaksionalitas, idempotensi — dirumuskan generik tanpa
kehilangan garansi yang ada.

## 4. `ctx.db` — Escape Hatch yang Mengorbankan Portabilitas
Akses SQL mentah sengaja backend-coupled: resource yang memakainya terkunci ke
PersistBackend berdialek itu. Bukan bug — konsekuensi yang harus disadari saat
memilihnya.

## 5. Batas dengan Spec Resolution API
Bentuk data yang diserahkan ke Shell tidak boleh membocorkan detail backend
(nama kolom fisik, path JSONB) — lihat spec frontend §04.

## 6. Menambah PersistBackend Baru
Syarat konformansi dan proses (lihat juga guides/authoring-a-persist-backend).
