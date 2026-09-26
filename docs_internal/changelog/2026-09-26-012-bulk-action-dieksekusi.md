# Bulk action dieksekusi, bukan sekadar ber-tombol (todo 5.12.8)

## Apa yang diubah

`BulkActionsBar` merender satu `<Button>` per `tableSpec.bulk_actions` **tanpa
`onClick`**, dan `TableRenderer` tidak pernah mengoper handler kolektif. Jadi
bar-nya muncul, tampak bisa diklik, dan tidak melakukan apa pun — sementara
**Batch edit (5.4.3) di bar yang sama berfungsi nyata**. Dua fitur bersebelahan
berperilaku berbeda tanpa cara membedakannya.

Kini `bulk_actions` dijalankan per baris, dengan kontrak yang **sama** seperti
batch edit:

| Keputusan                                                                                    | Alasan                                                                                                                       |
| -------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| `view`/`edit` **ditolak**, bukan dijalankan per baris                                        | keduanya navigasi satu-baris; "edit 12 baris" tidak punya arti tanpa nilai. Pesannya mengarahkan ke aksi baris.              |
| Tombol yang tidak bisa dijalankan **di-disable**, bukan disembunyikan                        | aksi yang dideklarasikan tapi tidak berlaku harus terlihat, supaya penulis tahu deklarasinya dibaca.                         |
| Permission diperiksa **sebelum** baris pertama disentuh                                      | tanpa itu, penolakan bisa meninggalkan separuh seleksi termutasi, dan gagalnya lalu tampak seperti masalah data, bukan izin. |
| Gate permission berjalan **sebelum** dialog konfirmasi                                       | aksi yang memang tidak boleh tidak boleh memunculkan prompt yang terlihat meyakinkan.                                        |
| Destruktif (`delete`/`cancel`) **minta konfirmasi**, dan tombolnya menyebut **jumlah baris** | pada aksi massal operator tidak bisa melihat dari seleksi saja apa yang akan kena.                                           |
| Kegagalan **dilaporkan per baris** (`BulkResultReport`, meniru `BatchEditReport`)            | sebagian gagal bukan sukses dan bukan gagal total; baris 409 ditandai stale seperti jalur aksi baris.                        |

## Kenapa

Item 5.12.8 ditemukan saat audit permission-gating tombol (changelog
`2026-09-22-007`), dan mencatat akarnya persis ("yang kurang handler, bukan
permission"). Yang membuatnya layak dikerjakan: **`batch_edit` di sebelahnya
sudah melakukan hal yang sama dengan benar**, jadi kontraknya sudah ada —
bukan keputusan desain baru, hanya satu jalur yang belum mengikutinya.

## File terdampak

- `renderers/react-shadcn/src/kinds/table/TableRenderer.tsx` — `canRunBulk`,
  `runBulkAction`, `requestBulkAction`, state `bulkResults`/`pendingBulkAction`,
  `BulkActionsBar` (prop `onRun`/`isDisabled`), `BulkResultReport` (baru),
  dialog konfirmasi massal
- `renderers/react-shadcn/src/kinds/table/bulk-actions.test.ts` — **baru** (8 test)

## Bukti

- `npx tsc -b` bersih; `npx vitest run` **511 lulus** / 37 file (baseline sesi ini
  503; +8 dari file ini).
- **Dibuktikan gagal:** `onClick` dan `disabled` dihapus dari tombol (persis bug
  lamanya) → 2 test gagal (`wires onClick through to the collective runner`,
  `disables actions that cannot run collectively`), hijau sesudah dikembalikan.
- Test juga mem-pin urutan yang mudah terbalik tanpa terlihat: gate permission
  harus dievaluasi **sebelum** `confirmMsg` di `requestBulkAction`.

## Catatan (sisa yang tidak ditutup)

- **Belum diverifikasi di browser nyata.** Semua test di sini memeriksa kontrak
  lewat sumber; alur klik-seleksi-jalankan-belum pernah dijalankan dengan server
  hidup. Ini kelas yang sama dengan todo 17.7 (klaim DOM vs browser nyata) dan
  **tidak** diklaim selesai di sini.
- **Payload aksi massal belum punya permukaan deklaratif.** Aksi yang butuh
  parameter (mis. `void_reason`) tidak bisa mengumpulkannya lewat bar ini —
  ia mengirim POST tanpa body, dan hanya aksi tanpa parameter yang berguna hari
  ini. Tidak ada item pelacak sebelumnya; dicatat di bawah.

## Rujukan

Todo **5.12.8** (tertutup) · audit permission-gating changelog `2026-09-22-007` ·
Batch edit 5.4.3 (`applyBatchEdit`, kontrak yang ditiru).
