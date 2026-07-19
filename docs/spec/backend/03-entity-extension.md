# Entity Extension

**Version:** 0.1.0 · **Status:** Draft

> Draft: isi di bawah kontrak yang berlaku.

## 1. Model Extension
Module lain menambah field/perilaku ke Entity yang dimiliki module lain
(mis. `billing/invoice` dari marketplace) — tanpa fork module itu, tanpa
merusak jalur upgrade-nya, tanpa mengorbankan performa query:

```yaml
apiVersion: forma.dev/v1alpha1
kind: Entity
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
extension, bukan Entity biasa.

**Alternatif yang direkomendasikan:** seluruh extension tetap **flat
siblings** terhadap Entity dasar, berapa pun jumlahnya. Kalau satu module
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

## 5. Validasi Tambahan (`validate:`)
Selain field, Entity Extension boleh menambah pemeriksaan `business_rules`
miliknya sendiri lewat `spec.validate` — **aditif, bukan pengganti**:

```yaml
apiVersion: forma.dev/v1alpha1
kind: Entity
metadata:
  name: invoice-ext
  module: my-customization
spec:
  extend_storage:
    target: billing/invoice
    namespace: shipping_info
  fields:
    - name: shipping_method
      type: enum
      enum_values: [regular, express]
      rules: [required]

  validate:
    - script: |
        def validate(resource, params, ctx):
          if resource.ext("shipping_info").shipping_method == "express" \
             and resource.total < 100000:
            return fail("Express shipping butuh minimum order")
          return ok()
      on: [create, update]
```

- **Urutan eksekusi** — validasi bawaan Entity dasar (L1–L6,
  [`02-core-extended.md`](02-core-extended.md) §14) selalu jalan lebih dulu;
  kontrak module asal tetap utuh. Validasi Extension jalan sesudahnya, pola
  yang sama dengan priority handler yang sudah ada di hook/event
  ([`02-core-extended.md`](02-core-extended.md) §15 Hook Spec) — bukan
  mekanisme baru.
- **Tidak boleh override** — script `validate:` Extension hanya boleh
  *menambah* pemeriksaan baru. Ia tidak bisa melemahkan, mengganti, atau
  melewati validasi bawaan Entity dasar.
- **Akses field** — boleh membaca field Entity dasar (`resource.<field>`,
  read-only) untuk keperluan pemeriksaan silang, tapi hanya berhak
  menuntut/mewajibkan field miliknya sendiri di namespace-nya
  (`resource.ext("<namespace>").<field>`).

## 6. Visibilitas Default di Layer 0
Field Extension otomatis ikut Layer 0 (spec-only, auto-generated CRUD) —
begitu Extension aktif, field-nya otomatis muncul di form/list yang
di-generate, dikelompokkan default di section bernama sesuai
`metadata.name` Extension. Dua kasus turunan:

- **Mau tampil tapi beda posisi/caption dari default** — shadow copy Form
  ([`../platform/08-project-layout.md`](../platform/08-project-layout.md)
  §6.4), opsional.
- **Memang tidak boleh pernah terlihat** (internal/computed/API-only) —
  bukan urusan Form sama sekali, cukup `exclude: [ui]` di level field
  Extension itu sendiri ([`05-field-types.md`](05-field-types.md) §5.3).
