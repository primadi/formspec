---
name: entity-extension-authoring
description: >
  Gunakan skill ini saat menambah field/validasi ke entity yang BUKAN milik
  module sendiri — terutama entity vendor di vendors/ yang read-only.
  Trigger: percakapan menyebut "tambah field di entity vendor", "kustomisasi
  module vendor", "entity extension", "extend entity", atau kebutuhan mengubah
  perilaku entity milik orang lain.
applies_to_kind: [Entity, Module]
min_core_spec_version: "0.2.0"
metadata:
  version: "1.0"
  source: docs/spec/backend/03-entity-extension.md
---

# Entity Extension Authoring

## Kapan Extension, Bukan Edit Langsung

`vendors/` **read-only by design** (integritas checksum/signature, jalur
update aman). Tool tulis (`propose_spec_file`, `apply_draft`) MENOLAK path di
bawah `vendors/`. Untuk mengubah perilaku entity vendor:

| Kebutuhan | Mekanisme |
|---|---|
| Tambah field / validasi | **Entity Extension** (skill ini) |
| Kustomisasi presentasi (layout Form, caption, urutan) | **Shadow copy** di `overrides/` |
| Perilaku bisnis baru | Action baru di module sendiri yang memanggil entity vendor via `uses.resources` |

## Anatomi Extension

Extension dideklarasikan di module MILIK SENDIRI (bukan di vendors/), dengan
`extends` yang menunjuk entity target:

```yaml
apiVersion: formspec.dev/v1
kind: Entity
metadata:
  name: product-extension   # nama bebas, module milik sendiri
  module: my-shop
spec:
  extends: vendor-module/product
  fields:
    - name: internal_sku
      type: string
      title: "SKU Internal"
```

Field hasil extension digabung ke entity target saat boot — schema, API, UI,
dan validasi mengenalinya seperti field native.

## Aturan

1. **Tambah, jangan ubah** — extension menambah field/validasi; tidak boleh
   mengubah/menghapus field asli vendor.
2. **Tidak boleh bentrok nama** — nama field extension tidak boleh sama dengan
   field asli (validasi menolak).
3. **`depends` wajib** — module extension harus mendeklarasikan dependensi ke
   module pemilik entity target.
4. **Upgrade vendor aman** — karena vendors/ tidak disentuh, update versi
   vendor tidak merusak extension; validasi structural mengecek field asli
   yang direferensikan masih ada.
5. **Shadow copy untuk tampilan** — perubahan layout Form vendor ditaruh di
   `overrides/<module>/...` (08-project-layout.md §6.4), bukan extension.

## Validasi

Draft extension tetap lewat `propose_spec_file` — validasi structural
mengecek referensi `extends` valid, target entity ada, dan tidak ada bentrok
nama field.
