# 2026-10-07-004 — Verifikasi: nilai uang pada `order` ditentukan pemanggil (kafe 10.81)

**Plan:** —
**Konteks:** pertanyaan pemilik proyek "`discount_amount` dientry atau ambil dari
server?" atas `examples/kafe/spec/modules/cafe-order/transaction/order/entity.yaml`.
Jawaban yang diverifikasi: **di-entry**, padahal dokumen kafe sendiri
menyatakannya turunan. Verifikasi ini murni pengukuran — **belum ada perubahan
perilaku**; temuannya dicatat sebagai **kafe 10.81 ⏸️**.

## Yang diukur (probe di build ini, lalu dihapus; test terskip sebagai gantinya)

| Probe                                                              | Hasil                                                                                       |
| ------------------------------------------------------------------ | ------------------------------------------------------------------------------------------- |
| Tamu anonim POST pesanan 45000 + `discount_amount: 40000`          | **201**; tersimpan 40000; `total_amount` 51975 → **11975**                                  |
| Tamu anonim POST `points_redeemed: 999999` + `points_value: 44000` | **201**; `total_amount` = **7975**                                                          |
| `manual_discount_amount: 40500` (90%) **tanpa** alasan             | **201**; `manual_discount_reason` = `nil`; `total_amount` = 11475                           |
| `lines[].discount_amount: 40000` pada 2 × 45000                    | **201**, tersimpan; `line_total` = **90000** (bukan 50000) → diskon baris tidak berpengaruh |

## Yang paling penting: jurnal tidak bisa menjadi penjaganya

Hipotesis "jurnal akan menolak karena tidak seimbang" **salah**, dan itu
terukur — pembangunan jurnal menambahkan diskon sebagai DEBIT akun diskon,
sementara `total_amount` (yang menjadi debit Kas) sudah dikurangi diskon, jadi
kedua sisi identik dengan atau tanpa diskon palsu:

```
Kas              debit  11975
Diskon penjualan debit  40000
Omzet penjualan  credit 45000
Pajak penjualan  credit  4725
Service charge   credit  2250
                        ────────
debit 51975 = credit 51975        status: posted
```

Buku seimbang, pesanan berharga katalog 45000 dibayar **11975**, dan akun diskon
menyerap 40000 yang tidak disetujui aturan bisnis mana pun. Kesimpulannya bukan
"perbaiki jurnalnya", melainkan: **penegakannya harus di jalur tulis** — siapa
yang boleh menentukan nilai itu — karena jurnal memang dirancang untuk menerima
diskon sebagai komponen sah.

## Kenapa ini bukan sekadar nama field

`docs/domain-model.md` menyebut `discount_amount` sebagai "diskon promo +
manual" (turunan), dan `docs/architecture.md` menjanjikan _"script evaluasi
promo: pilih satu kandidat terbaik"_. Script itu **tidak ada**, dan
`promo/entity.yaml` menyebut dirinya "ATURAN promo — disimpan dan dievaluasi saat
pemesanan" padahal tidak ada yang mengevaluasi. Jadi ada tiga dokumen yang
mengandaikan sebuah perilaku yang tidak pernah dibangun, dan celahnya tidak
terlihat justru karena fieldnya mengisi dirinya sendiri dari klien.

`total_amount` sendiri aman: ia `computed`, jadi nilai kiriman klien dibuang
(`stripComputedValues`). Yang tidak dijaga adalah **komponen** yang membentuknya.

## Bukti yang ditinggalkan

- `resource/discount_authority_e2e_test.go` — lima test yang menuliskan perilaku
  SEHARUSNYA, semuanya `t.Skip` dengan alasan yang menunjuk item. Menutup 10.81
  berarti menghapus `t.Skip`-nya, dan test itu langsung menjadi test regresi.
- Item **10.81 ⏸️** di `examples/kafe/gaps_found/TODO.md` memuat angka-angka di
  atas sebagai bukti yang bisa diperiksa, plus arah penutupnya.
- Suite tetap hijau: `go test ./...` · `golangci-lint` 0 issues. Probe tidak
  dibiarkan gagal, karena test yang gagal permanen bukan bukti — ia hanya
  mencegah orang lain bekerja.

## Arah penutup (butuh keputusan pemilik, jangan dikarang)

1. **`discount_amount`** — evaluasi promo di server (`kind: Service` + Starlark):
   `promo.type` (`percent`/`fixed`/`buy_get`), `priority`, `min_purchase`,
   `applies_to` (item/kategori), jendela `start_date`/`end_date`/`days_of_week`/
   `time_from`/`time_to`, dan sikap untuk `branch_id` kosong (= lintas cabang).
   Aturan yang perlu ditegaskan: apakah hanya SATU promo + satu penukaran poin
   (seperti disebut `docs/architecture.md`), dan kandidat mana yang menang.
2. **`points_value`** — nilai tukar 1 poin ada di mana: Config, atau
   `member`? Sekali ditetapkan, ia turunan `points_redeemed × tarif`.
3. **`manual_discount_amount`** — tegakkan `manual_discount_limit_percent` /
   `..._limit_amount` di server, dan wajibkan alasannya (bukan hanya
   `required_when` di form).
4. **`lines[].discount_amount`** — masukkan ke `line_total`, atau hapus kalau
   memang tidak dipakai. Membiarkannya menerima nilai tanpa efek adalah
   janji palsu.
