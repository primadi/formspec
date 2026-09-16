# 2026-09-16-009 — `row_scope` dinyalakan: isolasi cabang ditegakkan engine (3.5)

Item `examples/kafe/gaps_found/TODO.md` **3.5** (gap #8/S5). Plan:
`docs_internal/plan/row-scope-enforced.md`.

**Masalah.** Konstruknya sudah ada sejak 1.1/1.8 (`row_scope`, `assignments`,
`read_all`) tetapi **belum dinyalakan**: kasir masih melihat seluruh cabang.

**Yang dikerjakan.** `row_scope: [{field: branch_id, op: eq, from: session}]`
dipasang di 10 entity yang dibaca hanya lewat permukaan terautentikasi (`order`,
`payment`, `shift`, `cash-movement`, `stock-level`, `stock-movement`,
`purchase-order`, `stock-opname`, `waste-entry`, `menu-cost`). Nilainya
diselesaikan server dari `cafe-master.employee` (username → branch_id), tidak
bisa dilebarkan lewat query string, dan pemegang `read_all` dilewati.

**Aturan yang harus ditambahkan agar ini tidak mematikan permukaan publik.**
`row_scope` menyeluruh mem-403 anonim: pemanggil anonim tidak punya atribut sesi,
dan `from: session` memang fail closed. `applyRowScope` kini melewati permintaan
anonim yang datang lewat grant publik yang **punya scope sendiri** — di situ yang
membatasi baris adalah `applyPublicScope`. Entity tanpa grant publik tetap fail
closed untuk anonim.

**Celah terakhir yang ikut ditutup.** Grant publik `menu-item-price` (katalog QR)
kini ber-scope `{field: branch_id, from: route}`: sebelumnya daftar harga anonim
terbuka tanpa syarat, sehingga satu permintaan tanpa parameter mengembalikan
harga **seluruh cabang**.

**Bukti runtime** (dev server + DB segar + dua pesanan B1/B2, token dev
ditandatangani dengan secret dev sehingga klaim sesi nyata): kasir B1 → `total: 1`
hanya `B1-1`; kasir B1 + `?branch_id[eq]=B2` → **tetap hanya `B1-1`**; kasir B2 →
hanya `B2-1`; pemilik (`.list` + `.read_all`, tanpa atribut) → `total: 2`;
identitas `.list` tanpa atribut → **403** fail closed; anonim `menu-item-price`
tanpa `?branch_id` → **403**, dengan → `200`. Nuansa: `read_all` **tanpa** `.list`
menghasilkan **404** (gerbang permission, bukan scope — `read_all` mengecualikan
penyaringan, bukan memberi hak baca).

Test: `TestKafeRowScopeSpec_ScopeAndSource`,
`TestKafeAssignmentSources_EmployeeMapsUsernameToBranch`,
`TestKafePublicGrants_ScopedWhereRowsMatter`. `go test ./...` hijau · kafe
`validate` 0 problem.

**Sisa.** Supervisor pemegang dua cabang belum bisa dinyatakan (`assignments`
satu nilai per dimensi); `dining-table`/`table-session` belum di-scope sesi karena
masih dibaca permukaan tamu.
