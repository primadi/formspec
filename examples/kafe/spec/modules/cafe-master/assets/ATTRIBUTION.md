# Atribusi Aset — Kafe

Foto menu di folder ini diunduh dari **Wikimedia Commons** dan dipakai sebagai
data contoh aplikasi kafe. Semuanya berlisensi bebas yang **mewajibkan atribusi**
(CC BY / CC BY-SA) — atribusi ini harus ikut bila aset dipakai ulang.

Versi thumbnail (lebar 640 px) untuk menjaga ukuran di bawah batas
`menu-item.photo.max_size_mb: 2`.

| File              | Judul Commons                                           | Lisensi      | Pembuat         | Sumber                                                                                       |
| ----------------- | ------------------------------------------------------- | ------------ | --------------- | -------------------------------------------------------------------------------------------- |
| `nasi-goreng.jpg` | File:Nasi goreng.jpg                                    | CC BY-SA 3.0 | Sakurai Midori  | <https://commons.wikimedia.org/wiki/File:Nasi_goreng.jpg>                                    |
| `sate-ayam.jpg`   | File:Sate ayam.jpg                                      | CC BY-SA 4.0 | Shinta amalia   | <https://commons.wikimedia.org/wiki/File:Sate_ayam.jpg>                                      |
| `gado-gado.jpg`   | File:Gado-gado.jpg                                      | CC BY-SA 4.0 | Fitri Penyalai  | <https://commons.wikimedia.org/wiki/File:Gado-gado.jpg>                                      |
| `nasi-uduk.jpg`   | File:Nasi uduk.jpg                                      | CC BY-SA 3.0 | Sakurai Midori  | <https://commons.wikimedia.org/wiki/File:Nasi_uduk.jpg>                                      |
| `es-teh.jpg`      | File:Iced tea with ice cubes.jpg                        | CC BY-SA 2.5 | Editor at Large | <https://commons.wikimedia.org/wiki/File:Iced_tea_with_ice_cubes.jpg>                        |
| `kopi-tubruk.jpg` | File:Kopi Tubruk Jakarta.jpg                            | CC BY 2.0    | Mo Riza         | <https://commons.wikimedia.org/wiki/File:Kopi_Tubruk_Jakarta.jpg>                            |
| `es-jeruk.jpg`    | File:Orange juice half glass.jpg                        | CC BY-SA 4.0 | Corn cheese     | <https://commons.wikimedia.org/wiki/File:Orange_juice_half_glass.jpg>                        |
| `roti-bakar.jpg`  | File:Roti Bakar.jpg                                     | CC BY-SA 4.0 | Supardisahabu   | <https://commons.wikimedia.org/wiki/File:Roti_Bakar.jpg>                                     |
| `kopi-susu.jpg`   | File:Es kopi susu kekinian di Yogyakarta, Indonesia.jpg | CC BY-SA 4.0 | Gunarta         | <https://commons.wikimedia.org/wiki/File:Es_kopi_susu_kekinian_di_Yogyakarta,_Indonesia.jpg> |

## Koreksi 2026-09-23

`es-jeruk.jpg` sebelumnya adalah **File:Kopi O.jpg** — gambar kopi, bukan jeruk,
jadi "Es Jeruk" tampil sebagai segelas kopi. Diganti dengan
`File:Orange juice half glass.jpg`. Sekaligus ditambahkan dua gambar yang belum
pernah ada: `roti-bakar.jpg` dan `kopi-susu.jpg` (kedua menu itu sebelumnya hanya
ada di database dev, tanpa entri seed maupun foto).

## Kenapa aset disimpan di repo, bukan disitir sebagai URL

Seed tidak boleh bergantung pada jaringan: `formspec seed` harus bisa dijalankan
di laptop tanpa internet, dan URL Commons bisa berubah (rename/delete) sehingga
seed yang menyitir URL akan rusak tanpa perubahan apa pun di repo ini. Karena itu
gambar di-commit sebagai file, dan seed menyebutnya lewat marker `$asset`.

## Hubungan `$asset` dengan storage

Field `file` menyimpan **object key**, bukan path, dan key kanonik memuat **id
record** (`{workspace}/{module}/{entity}/{id}/{field}/{uuid}-{nama}`) — yang belum
ada sebelum insert. Karena itu seed **tidak** menulis key sendiri:

```yaml
photo: { $asset: "menu/sate-ayam.jpg" } # path relatif ke <module-dir>/assets/
```

Seed melakukan insert lebih dulu (id tercipta), lalu **mengunggah file lewat
storage service** (datastore registry: filesystem di dev, garage/minio/s3 di
prod) dan menulis key kanonik hasilnya ke field — bentuk key yang sama persis
dengan unggah HTTP, sehingga unduhan dan `visibility` bekerja tanpa cabang baru.

Konsekuensi praktis: **`make seed-kafe` saja sudah cukup** (target
`seed-kafe-assets` dihapus), dan menghapus folder storage tidak lagi permanen —
menjalankan ulang seed memulihkan objeknya.

Lokasi aset adalah `<module-dir>/assets/` (module-relative), jadi aset ikut
terbawa saat module di-vendor ke project lain.
