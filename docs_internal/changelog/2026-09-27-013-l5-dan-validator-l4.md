# 2026-09-27-013 — L5 tuntas + validator L4: migrasi L4 TIDAK boleh buta

**Apa:** L5 selesai (`docs_internal/plan/via-sebagai-action-penuh.md`), validator
L4 mendarat, dan entri `actions:` pertama dimigrasi (`examples/kafe`).

**Kenapa L4 tidak sesederhana "hapus duplikat":** pengukuran atas repo nyata
menunjukkan penolakan buta akan **merugikan otorisasi**.

| field pada deklarasi ganda | jumlah |
| -------------------------- | ------ |
| `required_permission`      | 76     |
| `uses`                     | 11     |
| `params`                   | 2      |
| bersih                     | 9      |

Dan bila entri dihapus, permission route **berubah di 36 dari 77 kasus** (41
tidak berubah). Tiga pola nyata: (B1) entri mempersempit permission action
lifecycle dari plural ke singular — `cafe-order.order.cancel` vs
`cafe-order.orders.cancel`, dan tanpa entri itu **setiap pemegang `update` bisa
membatalkan pesanan ber-uang**; (B2) dua action memakai satu permission
(`start-compounding` + `mark-ready` → `prescriptions.compound`), jadi keduanya
**tidak bisa dibedakan** untuk otorisasi; (B3) `plural: moduleversion` salah.

**Jebakan metodologis yang saya buat dan koreksi (dicatat supaya tidak
terulang):** pengukuran pertama melaporkan **76 dari 77 berubah**, dan itu
**salah** — skrip saya memakai `strings.Contains(perm, ".")` sebagai tes "sudah
qualified", padahal `permission.AutoPrefixPermission` **juga** mem-prefix nilai
2-segmen. Setelah memanggil fungsi produksinya, hasilnya 41/36.
**Alat ukur harus memanggil fungsi produksi, bukan meniru logikanya.**

**Validator** (`ValidateActionTransitionDuplication`) menolak duplikat yang
tidak menambahkan apa pun, dengan **dua pengecualian wajib**:
`ReservedActionNames` (6 entitas memakai ini untuk mempersempit route
`/{id}/cancel`) dan entri yang membawa `required_permission`/`impl` eksplisit.

**L5 lebih luas dari rencana — dua pembaca tambahan ditemukan lewat bukti bundle:**
`buildEntitySchema` masih membaca `es.Actions`, sehingga `schema.actions`
**kosong** begitu entri `actions:` dihapus (ini prasyarat keras migrasi L4);
dan `entityFootprint` juga — terukur `authorized_actions` kasir kehilangan
`release`/`reserve` tanpa perbaikan.

🔴 **Bug dedup ditemukan bukti bundle, bukan test:** `ActionSources()`
mensintesis satu action **per transisi**, jadi `dining-table` melaporkan
`['occupy','occupy',…,'occupy']` (tiga transisi memakai `via: occupy`) —
duplikat nama di `schema.actions` → React key ganda di daftar tombol. Diperbaiki:
satu action per nama, transisi pertama menang (konsisten dengan `GetActionSpec`).

**Bukti end-to-end (server hidup + browser):** blok `actions:` kafe
`dining-table` **dihapus seluruhnya**; `/kafe/_ui/_meta/ui?app=kafe-pos`
mengembalikan 7 action tanpa duplikat + `authorized_actions: [list, find,
update, release, reserve]`; tombol **"Tandai meja dipesan"** tampil di halaman
detail; PATCH `table_status: reserved` → **200**, status menjadi `reserved`.
`formspec validate` → 85 manifest, 0 problem.

**Kalibrasi:** `pkg/spec/duplication_test.go` (4 test) — gagal saat penolakan
dinetralkan, dan gagal dengan `` `occupy` appears 3 times `` saat dedup dihapus.

**File:** `pkg/spec/entity.go`, `pkg/spec/duplication_test.go` (baru),
`internal/ui/meta.go`, `internal/auth/materialize.go`,
`examples/kafe/spec/modules/cafe-master/master/dining-table/entity.yaml`,
`docs_internal/plan/l4-validator-anti-duplikat.md` (baru).

**Sisa → kafe 10.51:** migrasi 36 Kategori B **butuh keputusan pemilik** dulu
(bentuk permission mana yang benar). 35 Kategori A bisa menyusul setelah itu.
