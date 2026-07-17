# Entity Extension

**Version:** 0.1.0 · **Status:** Draft

> Draft: isi di bawah kontrak yang berlaku.

## 1. Model Extension
Module lain menambah field/perilaku ke Document yang dimiliki module lain
(mis. `billing/invoice` dari marketplace) — tanpa fork module itu, tanpa
merusak jalur upgrade-nya, tanpa mengorbankan performa query:

```yaml
apiVersion: forma.dev/v1alpha1
kind: Document
metadata:
  name: invoice-ext
  module: my-customization
spec:
  extend_storage:
    target: billing/invoice
    namespace: kastem1
  fields:
    - name: project_code
      type: string
      index: true
    - name: cost_center
      type: string
```

Field extension diakses dari script/handler lewat pemanggilan bernamespace,
bukan lewat penamaan kolom fisik: `invoice.ext("kastem1").project_code` — ini
kontrak akses, terlepas bagaimana PersistBackend menyimpannya secara fisik
(lihat [`../../renderers/jsonb-persist/02-schema-strategies.md`](../../renderers/jsonb-persist/02-schema-strategies.md)
untuk mekanisme kolom-per-extension jsonb-persist).

## 2. Kontrak Uninstall Bersih
Extension **wajib** bisa di-uninstall tanpa sisa — ini kontrak; *cara*
mencapainya (mis. strategi kolom terpisah per namespace vs nested path di
dalam field JSON) adalah urusan masing-masing PersistBackend. Kontraknya
menolak pendekatan yang membuat uninstall destruktif atau mahal (mis. harus
menulis ulang seluruh baris tabel) — implementasi yang menyediakan operasi
uninstall metadata-only (tanpa rewrite data existing) lebih disukai, tapi
bentuk konkretnya tidak dikunci di sini.

## 3. Konflik & Presedensi
**Namespace sekali dipakai tidak boleh dipakai ulang** untuk target yang
sama — aktif maupun sudah di-drop, kecuali di-purge eksplisit. `forma apply`
menolak namespace yang sudah tercatat untuk resource yang sama, mencegah dua
module independen memilih namespace sama secara tidak sengaja.

**Nested extend (extension-atas-extension) tidak direkomendasikan.**
Extension boleh secara teknis menargetkan extension lain, tapi punya tiga
masalah: (1) coupling permanen ke identitas extension dasar — sulit di-rename
atau dilepas dengan aman kalau extension dasarnya diganti/dihapus; (2)
membuat dependency urutan migrasi yang sebelumnya tidak ada; (3) membocorkan
abstraksi — module penyusun jadi tahu bahwa target-nya adalah sebuah
extension, bukan Document biasa.

**Alternatif yang direkomendasikan:** seluruh extension tetap **flat
siblings** terhadap Document dasar, berapa pun jumlahnya. Kalau satu module
butuh field dari extension module lain, deklarasikan dependency-nya lewat
`spec.depends` di manifest Module ([`01-core-basic.md`](01-core-basic.md) §5
qualifier `module/resource`), akses field lintas-extension lewat kode
(`invoice.ext("kastem1").project_code`) — bukan lewat asumsi penamaan kolom.

## 4. Extension dan Permission
Field extension ber-`index: true` berarti mengubah DDL tabel milik module
dasar — ini titik coupling, tapi terkendali: terjadi saat migration time
(bisa di-review lewat `forma apply --dry-run`), field tanpa `index: true`
(default) sama sekali tidak menyentuh DDL. Module extension **wajib**
mendeklarasikan akses tulis ke kategori persist target lewat `uses: { db:
{ write: [<category>] } }` ([`01-core-basic.md`](01-core-basic.md) §5) —
tampil di consent footprint-nya saat instalasi, konsisten dengan seluruh
akses lintas-module lain.
