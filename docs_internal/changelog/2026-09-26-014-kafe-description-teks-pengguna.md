# Kafe: `description` field ditulis sebagai teks pengguna, bukan catatan desain (todo 5.23.2)

## Apa yang diubah

`Field.description` (Entity) diwarisi sebagai `help` field di setiap permukaan
(todo 5.23.1, `docs_internal/changelog/2026-09-25-006`), yang membuatnya **teks
pengguna**: ia dirender di bawah input. Di `examples/kafe` banyak yang ditulis
sebagai catatan rekayasa, sehingga help di bawah input berbunyi:

```
"compute: kas awal + tunai masuk - kas keluar"
"Array angka 1=Senin..7=Minggu, mis. [1,2,3,4,5]"
"Posting — script membuat stock-movement (adjust) sebesar selisih"
"Dipakai memilih satu promo terbaik bila beberapa cocok (aturan bisnis #7)"
"ID pesanan / PO / opname — untuk telusur balik"
```

**18 deskripsi di 11 berkas** ditulis ulang. Prinsipnya bukan menghapus detail,
melainkan **memindahkannya ke tempat yang benar**:

| Informasi                                                   | Tempat baru                                                   |
| ----------------------------------------------------------- | ------------------------------------------------------------- |
| rumus/computed (`kas awal + tunai masuk − kas keluar`)      | komentar YAML `#` di atas field — tetap ditemukan engineer    |
| penanda tipe (`Array angka 1=Senin..7=Minggu`)              | komentar YAML di atas `options`/`multiple`                    |
| rujukan ledger (`aturan bisnis #7`, `(S12)`, `(D5)`)        | komentar YAML                                                 |
| yang dinamai implementasi (`script membuat stock-movement`) | komentar YAML; deskripsi menyebut **hasilnya** untuk pengguna |

Contoh hasilnya: `expected_cash` — dulu `"compute: kas awal + tunai masuk - kas
keluar"`, kini `"Kas yang seharusnya ada di laci menurut sistem."` (rumusnya
pindah ke komentar). `stock-movement.source_ref` — dulu `"ID pesanan / PO /
opname — untuk telusur balik"`, kini `"Dokumen yang menyebabkan pergerakan ini."`

Yang **sengaja dibiarkan**: contoh nilai (`"Mis. KFE-JKT-01"`, `"Mis. NET 30,
CBD"`). Itu justru help yang berguna — ia memberi tahu pengguna apa yang harus
diketik. Percobaan pertama saya menandai frase `"Mis. "` sebagai pelanggaran dan
itu **salah**; marker-nya dipersempit ke `"mis. ["` (literal array = dokumentasi
bentuk tipe).

## Kenapa

Item 5.23.2 mencatatnya sebagai `small–medium` dan "mekanis, tanpa keputusan
kontrak" — kontrak `description` sudah diperbarui di 5.23.1, hanya isi contohnya
yang tertinggal. Yang membuatnya layak dikerjakan: tanpa ini, warisan
`description → help` **memperburuk** UI (catatan desain kini tampil sebagai
kalimat kepada kasir), jadi 5.23.1 setengah selesai tanpa 5.23.2.

## File terdampak

- `examples/kafe/spec/modules/cafe-order/transaction/{shift,order,payment}/entity.yaml`
- `examples/kafe/spec/modules/cafe-master/master/{member,promo}/entity.yaml`
- `examples/kafe/spec/modules/cafe-stock/{master/{recipe,ingredient},transaction/{stock-opname,stock-movement,purchase-order}}/entity.yaml`
- `examples/kafe/spec/modules/gl/entities/gl-balance.yaml`
- `cmd/formspec/kafe_field_description_test.go` — **baru**, guard

## Bukti

- **Guard baru**: `TestKafeFieldDescriptionsAreUserFacing` men-scan
  `examples/kafe/spec` untuk 12 penanda catatan-rekayasa. **Dibuktikan gagal**
  saat satu deskripsi dikembalikan ke bentuk lamanya
  (`compute: kas awal + tunai masuk - kas keluar`), hijau sesudahnya. Sebelum
  perbaikan ia melaporkan **22 pelanggaran**; sesudahnya **0**.
- `go test ./cmd/formspec/` hijau; `formspec validate --spec examples/kafe/spec
--schema schemas` → **85 manifest, 0 problem**; `formspec check` → **0 error /
  0 warning** (komentar YAML tidak mengubah makna manifest, dan itu terverifikasi).
- `go build ./...` bersih; `gofmt -l` bersih.

## Catatan (sisa yang tidak ditutup)

- **`verticals/`, `examples/Clinic-UI-Showcase/`, `cmd/formspec-registry/` belum
  diperiksa.** Changelog 5.23.1 menyebutnya sebagai tempat deklarasi serupa; item
  5.23.2 hanya menyebut `examples/kafe`. Guard-nya sengaja hanya menegakkan kafe —
  memperluasnya ke seluruh repo saat ini akan gagal pada berkas yang belum diaudit
  dan memaksa perbaikan yang belum ditinjau.
- **Contoh kafe lain yang belum diaudit:** deskripsi yang tidak cocok salah satu
  dari 12 marker (mis. kalimat yang terlalu teknis tanpa kata kunci) tidak
  tertangkap. Guard ini menutup kelas yang sudah terukur, bukan seluruh kelas.

## Rujukan

Todo **5.23.2** (tertutup), melengkapi **5.23.1** (warisan `description → help`) ·
kontrak teks pengguna: `docs/spec/backend/05-field-types.md` ·
`ai_skills/` skill entity-authoring.
