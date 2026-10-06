# 2026-10-02-011 — Walkthrough 9.4 skenario 5 diulang pada DB seed baru; 3 temuan jadi item bernomor

**Apa yang dilakukan.** Menjalankan skenario 5 walkthrough kafe (siklus shift
kasir) dari nol pada DB hasil `make seed-kafe` + dev server `:8080` — permintaan
pengguna untuk memverifikasi fitur sisi kasir sebelum lanjut. Alurnya:
`POST shift` (kas awal) → `POST shift` kedua (aturan bisnis #10) → 2×
`POST cash-movement` → `PATCH status=closed` (jalur commit wizard) → baca
`difference`. Tidak ada perubahan kode; yang dicatat adalah **bukti** dan
**tiga sisa** yang ditemukan di jalurnya.

**Kenapa dicatat.** AGENTS.md §3: pekerjaan terbuka **wajib** jadi item `[⏸️]`
bernomor, bukan prosa di dalam item `[x]`. Dua dari tiga temuan sudah hidup
hanya sebagai prosa (di 9.4 dan 5.25.5), sehingga tidak bisa di-grep sebagai
pekerjaan terbuka.

**Hasil (terukur).**

| Langkah                                                           | Hasil                                                                                 |
| ----------------------------------------------------------------- | ------------------------------------------------------------------------------------- |
| `POST shift` (kas awal 500000)                                    | **201**, `status: open`                                                               |
| `POST shift` kedua (cabang+kasir sama)                            | **409 `CONFLICT`** — "a record with this value already exists: branch_id, cashier_id" |
| `POST cash-movement` in 200000 / out 50000                        | **201** / **201**                                                                     |
| `PATCH status=closed` (+ `counted_cash`, `expected_cash`, `note`) | **200** → `difference {-10000 IDR}` (640000 − 650000)                                 |
| `POST shift` baru (kasir sama, setelah shift ditutup)             | **201** — index parsial hanya mengunci baris `open`                                   |
| `POST shift` (kasir LAIN, cabang sama)                            | **201**                                                                               |

Aturan bisnis #10 ditegakkan database **dan** pesannya sudah benar (409 yang
menyebut field) — perbaikan `2026-09-28-001` terkonfirmasi pada DB segar.
`difference` computed (`counted_cash - expected_cash`) bekerja, termasuk nilai
negatif.

**Temuan baru (semua jadi item bernomor di `examples/kafe/gaps_found/TODO.md`).**

- **10.61 ⏸️** — tiga kelas kesalahan **masukan pemanggil** dijawab
  `500 INTERNAL_ERROR`, bukan 4xx: field tak dikenal, tanggal tak bisa diurai,
  dan nilai enum di luar `enum_values` (CHECK constraint). Pelanggaran unik
  sudah 409; jalur state machine (`PATCH`) sudah 422. Terukur pada
  `POST /_ui/entity/cafe-order/shift`.
- **10.62 ⏸️** — `opened_at`/`closed_at` pada `shift` tidak pernah ditulis
  (field mati). Terukur: `closed_at: null` setelah tutup shift berhasil. Efek:
  laporan rekap shift kehilangan sumbu waktu.
- **10.63 ⏸️** — `expected_cash` tidak punya penulis, sementara
  `close-shift-wizard` memamerkannya `read_only` dan `difference` dihitung
  terhadapnya. Terukur: selisih benar hanya bila `expected_cash` disuplai
  manual.

**Dampak.** `examples/kafe/gaps_found/TODO.md` (baris 9.4 skenario 5 diperbarui
dengan bukti run ini; catatan prosa `expected_cash` kini menunjuk 10.63; tiga
item `[⏸️]` baru 10.61/10.62/10.63). Tidak ada berkas kode yang berubah.

**Referensi.** Walkthrough 9.4 (`examples/kafe/gaps_found/TODO.md`); item
terkait yang sudah ada: 10.11 ⏸️ (menu vs grant), 10.46 ⏸️ (gate tidak berlaku
di jalur script/UI).
