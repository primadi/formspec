# 2026-09-27-016 — 10 warning kafe terakhir: semua gate dipindahkan ke transisi

**Apa:** Sepuluh action kafe terakhir yang mendeklarasikan `required_permission`
tanpa ditegakkan kini digerbangi di transisinya; deklarasi di `actions:`
dihapus. `formspec check` kafe: **21 → 0 warning**.

**Cara memutuskan (berbasis pengukuran, bukan selera).** Enam dari sepuluh
permission itu **tidak diberikan ke role mana pun**, dan permission deklarasinya
sudah **identik** dengan bentuk konvensi `{module}.{plural}.{action}` — jadi
menghapusnya adalah **no-op terverifikasi**, bukan perubahan otorisasi:

| group                  | actions                                                                                              | grant     | keputusan                                                                                           |
| ---------------------- | ---------------------------------------------------------------------------------------------------- | --------- | --------------------------------------------------------------------------------------------------- |
| no-op, tanpa grant     | `submit-order`, `abandon`, `close-session`, `record-count`, `reopen`, `post-opname`, `cancel-opname` | tidak ada | gate ke transisi (makna sebenarnya: pembatas `update`)                                              |
| no-op, grant sudah ada | `submit-po`, `cancel-po`                                                                             | manajer   | gate ke transisi — membuat grant manajer **benar-benar membatasi**, tanpa mengubah siapa yang boleh |

Tidak ada permission yang dicabut, tidak ada akses yang melebar atau menyempit
untuk pemegang yang sudah ada.

**Grant ditambahkan ke role yang memang sudah memegang halamannya** — sehingga
batasnya menjadi eksplisit dan bisa dicabut terpisah, bukan melebur ke `update`:

| role       | action yang kini eksplisit                                                           |
| ---------- | ------------------------------------------------------------------------------------ |
| kasir      | `submit-order`, `abandon`, `close-session`                                           |
| pelayan    | `submit-order`, `abandon`                                                            |
| supervisor | `close-session`, `abandon`                                                           |
| manajer    | `close-session`, `abandon`, `record-count`, `reopen`, `post-opname`, `cancel-opname` |

**Bukti diskriminator baru (server hidup):** barista punya `update` pada `order`
tetapi **tidak** `submit-order`:

> `PATCH status=awaiting_payment` pada `order` `draft` → **403**
> `missing permission: cafe-order.orders.submit-order (required for transition draft -> awaiting_payment)`
> — sedangkan kasir (punya) → **200** `awaiting_payment`.

Sebelumnya barista bisa memajukan pesanan apa pun karena hanya `update` yang
diperiksa. Gate ini juga mengembalikan makna `submit-order` yang selama ini
**mati** (tidak ada yang pernah menerimanya, jadi tidak pernah menjadi pembatas).

**Verifikasi:** `formspec validate` kafe **85 manifest, 0 problem**;
`formspec check` kafe **0 error, 0 warning**; `go test ./internal/... ./pkg/...
./cmd/...` hijau. Validator `ValidateActionTransitionDuplication` **tidak menolak
satu pun** manifest di repo — diperiksa di `examples/{cafe,arisan,crc-management,
Clinic-UI-Showcase}` dan `verticals/{billing,reference-app}`. Kegagalan yang
ditemukan di sana **pra-eksisting** dan berbeda kelas (`honesty`/`USES_VIOLATION`,
integrator symmetry), bukan dari perubahan ini.

**File:** `examples/kafe/spec/modules/cafe-order/transaction/{order,table-session}/entity.yaml`,
`examples/kafe/spec/modules/cafe-stock/transaction/{purchase-order,stock-opname}/entity.yaml`,
`examples/kafe/spec/modules/formspec.core/seeds/roles.yaml`.

**Sisa:** kafe 10.51 (bagian L4-migrasi, O1–O5, O7, O8) dan **O9** — 43 transisi
tanpa gate sama sekali di `examples/` + `verticals/` (di luar kafe), yang
`check` tidak bisa lihat karena check menangkap kontradiksi, bukan omisi.
