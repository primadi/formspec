# 2026-10-03-004 — `default` pada field CHILD kini diterapkan (kafe 10.66); basis pajak diselaraskan

**Keputusan pemilik yang dieksekusi di sini.** Basis pajak dikonfirmasi:
**`tax_percent × (subtotal + service charge)`** — sama dengan yang sudah
terpasang di `order/entity.yaml` sejak `2026-10-03-003`. Konsekuensinya angka
contoh lama diselaraskan (lihat bagian kedua).

## 1. `default` field child (kafe 10.66)

**Apa yang diubah.** Helper baru `EntityStore.applyChildDefaults`
(`renderers/jsonb-persist/crud.go`) — kembar `applyDefaults` tetapi mengunjungi
`Child.Fields` **setiap baris** — dipanggil di tiga jalur tulis: **Insert**,
**Update**, dan **UpsertProjection**.

**Kenapa.** `applyDefaults` mengiterasi `s.fields`, yaitu field **induk** saja,
dan tidak ada bentuk anaknya. Jadi sebuah `default` pada field child **tidak
pernah diterapkan di jalur mana pun**: `order.lines[].line_status`
(`default: queued`) tersimpan sebagai key yang **absen**, bukan `null`. Di Update
ia penting secara khusus karena baris child diganti **utuh** — setiap baris yang
dikirim adalah baris lengkap, jadi ia layak mendapat default-nya. Default
tingkat **induk** sengaja **tidak** ikut diterapkan di Update: PATCH itu parsial,
dan menyemai ulang field induk yang tidak disebut pemanggil akan membatalkan
nilai yang mereka set sebelumnya.

**⚠️ Koreksi diagnosis saya sendiri (dari `2026-10-03-002`).** Entri 10.66 semula
menulis "`computed` diterapkan, `default` tidak — asimetri di mesin". Itu **tidak
akurat**. `evaluateComputed` **juga** berjalan di jalur baca untuk keduanya; yang
membuat `line_total` tampak "tertulis" adalah **efek samping PATCH**:
`HandleUpdate` membaca record lebih dulu (`GetByID` → `evaluateComputed` memutasi
peta di tempat), lalu menyimpan kembali hasil merge — sehingga nilai turunan ikut
tertulis tanpa diminta. Terukur pada payload tersimpan: order yang **belum
pernah** di-PATCH tidak punya `line_total`; setelah satu PATCH, ada. Asimetrinya
nyata, mekanismenya beda dari yang saya tulis.

Efek samping itu kini punya nomor sendiri — **10.70 ⏸️** — dan ia sekaligus
**mengoreksi 10.68**: klaim "kolom turunan `_total_amount` tetap NULL" **salah**;
kolomnya terisi begitu record pernah di-PATCH (dan setiap order yang dibayar
melewati PATCH). Terukur: `_total_amount` = `25000` / `28875` / `1155` pada baris
yang sudah di-PATCH, NULL pada yang belum. Kedua entri ledger sudah diperbaiki
dan saling merujuk, supaya tidak ada dua dokumen yang bertentangan.

**Bukti.** `renderers/jsonb-persist/child_defaults_test.go` — dua subtest
(**jsonb** dan **table**, jalur ekstraksinya berbeda) yang membaca **baris MENTAH
tersimpan**, bukan hasil baca, sehingga perbaikan yang hanya menyentuh salinan
hasil tidak bisa lulus. Test **gagal lebih dulu**
(`row 1 line_status = <nil>, want "queued"`), dan menegaskan default **tidak**
menimpa nilai yang dikirim pemanggil. **Verifikasi dev server:** order baru tanpa
PATCH → `line_status = queued` di payload tersimpan, `line_total` tetap **absen**
(sekaligus membuktikan computed memang bukan nilai tulis). `go test ./...` hijau.

## 2. Angka basis pajak diselaraskan

`TestKafe_OnPaidCreatesBalancedJournal` (`resource/o2c_e2e_test.go`) memakai
`tax 12500 / total 143750` — pajak atas **subtotal** saja. Itu **keliru** terhadap
aturan yang berlaku, dan karena payload-nya **disuplai tangan** (bukan hasil
`computed`), angka lama tidak pernah ketahuan: yang salah adalah **dokumen
contohnya**, bukan jurnalnya. Diselaraskan ke basis yang dikonfirmasi:

| | subtotal | service (5%) | pajak (10%) | total |
| --- | --- | --- | --- | --- |
| lama (salah) | 125000 | 6250 | **12500** | **143750** |
| baru (benar) | 125000 | 6250 | **13125** | **144375** |

Repo kini memuat **satu** basis pajak, dan komentar di test menyebut basisnya
secara eksplisit supaya angka itu tidak lagi jadi teka-teki.

**Catatan:** changelog historis (`2026-09-21-003/004`) yang memuat angka 143750
**sengaja tidak disentuh** — ia catatan apa yang terjadi saat itu, dan mengedit
sejarah akan membuat ledger berbohong.

## Dampak

`renderers/jsonb-persist/crud.go` · `renderers/jsonb-persist/child_defaults_test.go`
(baru) · `resource/o2c_e2e_test.go` · `examples/kafe/gaps_found/TODO.md`
(10.66 ✅ dengan koreksi; 10.68 dikoreksi; **10.70 ⏸️** baru; blok 10.65–10.70
diurutkan naik). `go test ./...` hijau · `gofmt` bersih · `formspec validate`
89 manifest 0 problem.
