# 2026-09-15-011 — Public grant row scope: `public_entities[].scope` (#45)

Item `examples/kafe/gaps_found/TODO.md` **2.2** (gap **#45**). Plan:
`docs_internal/plan/public-grant-row-scope.md`.

**Masalah.** Allowlist per-entity (1.2) membatasi entity/aksi mana yang bisa
disentuh anonim, tetapi tidak bisa membatasi **baris**. Akibatnya `list` pada
`order` tidak bisa diberikan sama sekali — "anonim boleh list" berarti membaca
setiap pesanan kafe — sehingga pelanggan tidak punya jalan membaca pesanannya
sendiri. Halaman status pelanggan kafe bahkan mendokumentasikan ini sebagai
GAP-06: "bentuk ideal yang belum bisa dinyatakan: allowlist per-entity + scoping
per-record yang ditegakkan server".

**Konstruk.** `public_entities[].scope` — filter baris per-permukaan: grant
mengatakan entity MANA, scope mengatakan BARIS mana. Nilainya dibaca server dari
parameter request (token tamu), jadi tidak bisa dilebarkan klien; parameter yang
tidak ada **menolak** permintaan. Hanya `from: route` diterima (permukaan publik
tidak punya identitas sesi), dan `scope` tidak bisa digabung `find` (find
me-resolve lewat id — scope tidak bisa menjaganya, jadi kombinasi itu akan tampak
terfilter padahal mengembalikan record apa pun). Kedua aturan itu ditolak saat
validasi, bukan dibiarkan menjadi 403 tanpa gejala.

**Dua bug yang ikut tertutup karena konstruk ini menyentuh jalur yang sama:**

1. **Grant publik adalah bypass permission.** Route publik menyetel
   `RequiredPermission = "public"`, sehingga pemanggil yang sudah login melewati
   permission entity pada route itu. Karena `/_ui/entity` dipakai bersama surface
   POS/KDS, memberi anonim `list` pada `order` akan **mencabut** gerbang
   `cafe-order.orders.list` dari table POS dan kanban KDS. Kini grant hanya
   mengizinkan anonim (`RequirePermissionOrAnonymous`); pemanggil terautentikasi
   tetap wajib memegang permission, dan scope anonim tidak diterapkan padanya.
2. **Parameter scope grant diparse sebagai filter field** (`?token=` → 422
   `unknown field`), perbaikan yang sama seperti yang dilakukan 1.1 untuk
   `row_scope`.

**Adopsi kafe.** `order` dapat field `guest_token` (disalin dari sesi meja oleh
`order-form-qr`); App `kafe-qr` memberi anonim `create` + `list` dengan scope
token; halaman status pelanggan memakai token sebagai kunci dan komentar GAP-06
di sana dihapus.

**Bukti runtime** (dev server + SQLite segar, dua pesanan dengan token berbeda):
tanpa token → 403; `?guest_token=TOKENA` → `total: 1`, hanya TOKENA;
`?guest_token=TOKENB` → hanya TOKENB; `?guest_token=TOKENA&guest_token[eq]=TOKENB`
→ **tetap hanya TOKENA** (klien tidak bisa melebarkan); token salah → `total: 0`;
`?status=paid` tanpa token → 403; `create` anonim → 422 validation (mencapai
handler, jalur pesan QR utuh); `list menu-item` (grant tanpa scope) → 200. Test:
`pkg/spec` 3+4 case, `internal/api` 5 case (`TestRequirePermissionOrAnonymous`
termasuk). `go test ./...` hijau, kafe `validate` 0 problem.
