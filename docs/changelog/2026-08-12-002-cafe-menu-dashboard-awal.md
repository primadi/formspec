# 2026-08-12-002-cafe-menu-dashboard-awal

Menu awal (`spec.menu[0]`) App `examples/cafe/spec/apps/cafe.yaml` kini
adalah leaf **Dashboard** dari module `cafe-report` — menunjuk view
`cafe-summary-dashboard` (`module: cafe-report` + `view: cafe-summary-dashboard`,
icon `layout-dashboard`). Item terakhir yang tadinya adopt node
`module: cafe-report` diganti group `Laporan` eksplisit berisi hanya
`Rekap Penjualan` (tanpa child `Dashboard Kafe`) supaya dashboard tidak
muncul dua kali. Module adopt (`cafe-order`, `cafe-master`) tetap.

## Alasan

Pengguna aplikasi kafe diarahkan langsung ke ringkasan (pesanan terbuka,
lunas, pembayaran pending, meja tersedia) begitu membuka `/app/kafe`, tanpa
harus lewat menu "Laporan → Dashboard Kafe". Item terakhir tidak memakai
adopt node karena itu menyisipkan seluruh menu module `cafe-report` —
termasuk `Dashboard Kafe` — sehingga dashboard tampil ganda.

## Dampak

| File                                | Perubahan                                                                                                                     |
| ----------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| `examples/cafe/spec/apps/cafe.yaml` | Tambah leaf node Dashboard di `spec.menu[0]`; ganti adopt node `cafe-report` dengan group `Laporan` (hanya `Rekap Penjualan`) |

Validasi `formspec validate --spec examples/cafe/spec`: 16 manifest, 0 problem.
