# 2026-10-03-006 — Field-level `required_permission` kini juga menjaga TULIS (kafe 10.67 penelusuran)

**Cara item ini sampai ke sini.** 10.67 berbunyi "aturan bisnis #1 ditegakkan
kolom Kanban, bukan API". Sebelum menambal, saya memeriksa mekanisme yang
tersedia — dan menemukan dua hal:

1. **Batas status per-peran tidak bisa dinyatakan.** `row_scope`
   (`01-core-basic.md` §1.7) hanya menerima `from: session|route` — ia memfilter
   **siapa**, bukan **nilai status**. Lebih penting: `row_scope` bersifat
   **per-entity**, jadi menambahkan filter status di sana akan membutakan
   **kasir** terhadap draft-nya sendiri. Mekanisme "filter baris per-peran"
   memang belum ada.
2. **Field-level `required_permission` hanya separuh terpasang.** §5.3 normatif
   untuk **dua arah** — *"tidak boleh melihat **atau menyetel** field sensitif ini
   tanpa permission tambahan … penyetelannya di payload **ditolak**"* — tetapi
   hanya arah BACA yang ada (`internal/api/fieldsec.go` `sanitizeData`).

(2) adalah cacat yang lebih tajam daripada (1), dan bisa ditutup sekarang.

## Cacat yang diukur

Fixture `acme/customer` punya `salary` ber-`required_permission:
acme.customers.salary.view`. Pemanggil `limited` (hanya `list`,`view`,`create`,
`update`):

| Jalur | Sebelum | Sesudah |
| --- | --- | --- |
| `GET` (baca) | `salary` di-strip ✅ | tetap di-strip |
| `POST` membawa `salary: 999999` | **diterima & tersimpan 999999** ❌ | **403 FORBIDDEN** ✅ |
| `PATCH` membawa `salary: 1` | **tersimpan 1** ❌ | **403 FORBIDDEN** ✅ |

Arah yang bocor itu yang lebih buruk: pemanggil yang **tidak boleh melihat** nilai
gaji tetap bisa **menentukannya**, lalu tidak punya cara untuk melihat apa yang
ia tetapkan. Untuk field seperti `salary`, seluruh gunanya deklarasi ini adalah
"hanya peran berwenang yang menentukan nilainya".

## Perbaikan

- `forbiddenFieldWrites` + `denyForbiddenFieldWrites`
  (`internal/api/fieldsec.go`): mengembalikan field di payload yang tidak boleh
  diset pemanggil, **terurut** (payload itu map — urutan acak akan melaporkan
  field berbeda untuk permintaan yang sama).
- Dipanggil dari **`HandleCreate`** dan **`HandleUpdate`**. Pada update ia
  memeriksa **BODY**, bukan record hasil merge: nilai yang sudah tersimpan dan
  tidak dikirim pemanggil bukan niat mereka, dan menandainya akan membuat setiap
  PATCH atas record semacam itu gagal.
- **Ditolak, bukan di-strip diam-diam** — itu bunyi §5.3, dan menjawab 200 untuk
  permintaan yang nilainya dibuang akan membuat pemanggil tidak punya cara tahu
  input-nya diabaikan. Jawabannya `403 FORBIDDEN` dengan `details[].field`.
- **Cakupan: jalur HTTP.** Jalur `resource.save()` (script) tidak membawa
  identitas, jadi tidak bisa diperiksa di sana — kelas yang sama dengan gerbang
  transisi (10.46 ⏸️). Dicatat sebagai sisa, tidak diklaim selesai.

## Bukti

`resource/field_permission_write_e2e_test.go` (baru) — **gagal lebih dulu** pada
kedua subtest (`SET salary to 999999`, `CHANGED salary to 1`), hijau setelah
perbaikan.

⚠️ **Satu jebakan yang tertangkap di tengah jalan dan layak dicatat:** versi
pertama subtest `update` **lulus tanpa perbaikan apa pun** — karena `ProdMode`
mode ketat menuntut `If-Match`, jadi PATCH-nya dijawab **409 `If-Match header
required`** dan penulisannya tidak pernah berjalan. Assertion "nilai tidak
berubah" menjadi **vakum**. Test kini mengirim `If-Match: version=<N>` dan
**menolak** hasil 409 secara eksplisit, supaya tidak bisa lulus tanpa menulis.
Ini pola "test hijau karena hal lain" yang sama kelasnya dengan catatan lama di
repo ini.

**Dampak ke kafe: nol**, dan itu diverifikasi bukan diasumsikan: seluruh
`required_permission` di spec kafe ada pada **action/transisi**, tidak satu pun
pada **field** (`grep` → 9 kemunculan, semuanya action), jadi tidak ada jalur
kafe yang kini ditolak. Diuji ulang di dev server pada DB bersih setelah
perubahan: create QR tanpa angka uang → `11550` tersimpan, forge ditolak,
`?sort=` monoton. `go test ./...` hijau.

**Dampak.** `internal/api/fieldsec.go` · `internal/api/handler.go` ·
`resource/field_permission_write_e2e_test.go` (baru) ·
`resource/http_headers_test_helper_test.go` (baru — helper `doAuthedWithHeaders`,
lahir dari jebakan di atas) · `examples/kafe/gaps_found/TODO.md`.
