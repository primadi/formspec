# Plan — Menyalakan `row_scope` per cabang (kafe TODO 3.5)

Sumber: `examples/kafe/gaps_found/TODO.md` 3.5.

## Konteks

Konstruknya sudah lengkap sejak 1.1 (`row_scope`), 1.8 (`assignments`, sumber
nilai) dan keputusan `read_all` (pengecualian eksplisit). Yang kurang: spec kafe
belum memasangnya, jadi isolasi cabang masih bergantung disiplin UI.

## Aturan yang ditetapkan di item ini

| Aturan                                                                                                 | Alasan                                                                                            |
| ------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------- |
| `row_scope: from session` hanya pada entity yang dibaca lewat permukaan terautentikasi                 | anonim tak punya atribut sesi                                                                     |
| permintaan anonim lewat grant publik **ber-scope** dilewati dari `row_scope`                           | yang membatasi barisnya adalah grant itu (`applyPublicScope`); kalau tidak, permukaan publik mati |
| entity tanpa grant publik tetap fail closed untuk anonim                                               | entity yang memang tidak dimaksudkan terbuka                                                      |
| entity yang dibaca anonim (katalog) **tidak** di-scope sesi, melainkan diberi scope di grant publiknya | sumber dayanya adalah parameter route, bukan sesi                                                 |

## Entity ter-scope (10)

`order`, `payment`, `shift`, `cash-movement`, `stock-level`, `stock-movement`,
`purchase-order`, `stock-opname`, `waste-entry`, `menu-cost`.

Sengaja **tidak**: `menu-item-price` (dibaca picker katalog → diberi scope grant),
`menu-item`/`menu-category` (katalog global), `dining-table`/`table-session`
(masih dibaca permukaan tamu lewat `find` by id), `member`/`employee` (dibaca
admin lintas cabang).

## File

- `internal/api/scope.go` — aturan "anonim + grant ber-scope → lewati".
- 10 entity kafe — `row_scope`.
- `apps/kafe-qr.yaml` — scope pada grant `menu-item-price`.
- `cmd/formspec/kafe_row_scope_test.go` — 3 test.

## Bukti

Runtime: 6 probe dengan token dev sungguhan (kasir B1/B2, pelebaran query,
pemilik `read_all`, fail closed, anonim pada grant ber-scope) — tabelnya ada di
ledger 3.5. Unit: 3 test tentang koherensi spec (entity ter-scope tepat himpunan
itu, sumber `assignments` tersedia, grant publik ber-scope selalu `from: route`
dan tak digabung `find`).

## Sisa

Daftar nilai (multi-cabang) dan scope berdasarkan record tamu.

## Estimasi: **medium**
