# 2026-09-28-003 — P3: form bisa memanggil Service, dan redirect bisa memakai responsnya

**Apa:** kemampuan engine ketiga yang dibutuhkan fitur "tamu kedua JOIN ke sesi
meja" (plan `docs_internal/plan/kafe-join-session-kode.md`). Dua bagian:
`FormSubmit.Call` (submit form menjadi pemanggilan Service, bukan penulisan
entity) dan `{response.*}` pada `submit.redirect` (menjangkau nilai yang
diputuskan server). Fitur kafe-nya sendiri (P4–P5) belum dikerjakan.

## Kenapa ini tidak bisa dinyatakan dengan entity CRUD

Check-in tamu harus memutuskan, **server-side dan atomik**, antara _"meja bebas
→ buat sesi"_ dan _"meja terisi → join sesi itu bila kodenya cocok"_. Sebuah
`create` biasa tidak bisa menyatakan kedua sisi: ia menuntut baca-lalu-tulis yang
tidak boleh dipercayakan ke klien, dan `list table-session` anonim tidak bisa
di-scope per baris (tamu belum punya token) sekaligus akan membocorkan kode
gabung. Jadi Service memang satu-satunya bentuk yang benar.

## Yang mendarat

| Bagian                              | Isi                                                                                                                                                                           |
| ----------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `FormSubmit.Call`                   | `"module.service.action"`. Bila diset, submit memanggil Service dan **tidak** menulis entity — service yang memiliki mutasi                                                   |
| `{response.*}` di `submit.redirect` | Satu-satunya cara menjangkau nilai yang diputuskan server. Setelah join, token yang harus dibawa tamu adalah milik sesi yang **sudah ada**, yang tidak pernah diketahui klien |
| `lib/serviceCall.ts`                | `serviceCallPath()` — satu rumah untuk aritmetika prefix `../service/...`                                                                                                     |
| `lib/submitRedirect.ts`             | `resolveSubmitRedirect()` — resolusi token + tipe hasil eksplisit                                                                                                             |
| Validasi                            | `submit.call` harus 3 segmen (`internal/ui/validate.go`)                                                                                                                      |

## Tiga keputusan yang perlu dibaca sebelum mengubahnya

**1. Tombol submit TIDAK digating permission entity saat `submit.call` diset.**
Gate yang ada memeriksa `create`/`update` **entity**, dan form seperti ini tidak
menulis entity itu — memeriksanya berarti memeriksa hal yang salah. Ia juga gagal
tepat pada alur yang memotori fitur ini: check-in tamu berjalan **anonim**, dan
begitu Service yang menulis, grant `create` anonim pada `table-session` justru
**dicabut** (grant itu adalah keluhan asli 10.34 — create anonim tanpa bukti
kehadiran). Klien tidak punya metadata service untuk menggantikannya
(`Bundle` tidak memuat daftar service), jadi posisi jujurnya: render, biarkan gate
milik service yang memutuskan. **Biayanya nyata dan diterima** — pada form privat
yang memanggil service terbatas, tombolnya tampil dan server menjawab 403. Dicatat
sebagai **10.58** (kirim metadata service di bundle supaya klien bisa pra-cek).

**2. `{response.*}` TIDAK di-spread ke render context.** Ia di-namespace di bawah
`response` supaya sebuah service tidak bisa **menutupi** slot framework (`route`,
`user`, `session`, `fields`) dan mengalihkan pemanggil ke tempat yang ia pilih.
Dikunci test khusus ("does not let the response SHADOW a framework context slot").

**3. Redirect yang tokennya tak terpecahkan TIDAK dinavigasi.** Ini kegagalan
yang dicegah, bukan diperbaiki. `interpolateTokens` sengaja membiarkan token tak
dikenal **verbatim** — benar untuk nilai field (placeholder terlihat lebih baik
daripada kosong misterius, yang membuat "kenapa branch_id kosong" jadi pencarian
20 menit), dan **salah untuk alamat**: `/menu/{response.guest_token}` tampak
seperti URL yang bekerja, mendaratkan tamu di halaman kosong, dan tidak
melaporkan apa pun. `resolveSubmitRedirect` mengembalikan hasil eksplisit dan
renderer menampilkan error alih-alih menavigasi.

## Yang diuji, dan kenapa bentuknya begitu

- **`serviceCallPath` diuji terhadap server HTTP nyata** (`serviceCall.test.ts`),
  bukan sebagai string. Pertanyaannya bukan "string apa yang dikembalikan" tapi
  "URL apa yang benar-benar diterima server", karena `../` harus keluar dari
  segmen `entity` lalu mendarat di `/{ws}/_ui/service/...`. Salah di sini gagal
  **total dan senyap**: setiap panggilan service 404, tanpa error tipe, tanpa
  lint, dan tidak ada yang terlihat sampai seseorang mengklik tombolnya. Server
  perekam membuktikan `/kafe/_ui/service/cafe-order/table-access/open`, plus
  assertion negatif untuk dua bentuk yang salah.
- **9 test `submitRedirect`** mencakup: resolve `{response.*}`, token context
  (tanpa regresi), keduanya dalam satu template, jalur tanpa token, **gagal** saat
  respons tidak membawa token, **gagal** saat tidak ada respons, **gagal** pada
  token context tak dikenal, **tidak bisa di-shadow** oleh respons, dan leading
  zero dipertahankan (`004213` ≠ `4213` — kode gabung adalah string digit).
- **5 sub-test validasi Go**: bentuk 3 segmen diterima, 2/4 segmen ditolak,
  segmen kosong ditolak, dan form tanpa `call` tetap sah.

## Verifikasi

`go test ./...` hijau · vitest **548** (42 file, +12 dari 536) ·
`formspec validate` kafe 89 manifest/0 problem · `make e2e-kafe` 1 passed ·
`tsc --noEmit` bersih · oxlint **41** (baseline) · `gofmt` bersih ·
`make generate-schema` dijalankan.

## File

`pkg/spec/frontend.go` · `internal/ui/validate.go` ·
`internal/ui/validate_submit_call_test.go` ·
`renderers/react-shadcn/src/types/manifest.ts` ·
`src/kinds/form/FormRenderer.tsx` · `src/lib/serviceCall.ts` (+test) ·
`src/lib/submitRedirect.ts` (+test) · `schemas/` ·
`examples/kafe/gaps_found/TODO.md` (10.58 baru, 10.57 diperbarui)
