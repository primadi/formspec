# 8.4 — Pesan error port `formspec dev` kini menuntun (#24)

**Tanggal:** 2026-09-20 · **TODO:** `examples/kafe/gaps_found/TODO.md` 8.4 · **Plan:** `docs_internal/plan/kafe-sisa-gap.md`

## Apa yang diubah

Gap #24 klaim aslinya (server `formspec dev` menolak start) sudah dibatalkan —
servernya normal; yang tersisa adalah **pesannya tidak menuntun**. Dua pesan
error di `internal/devserver/devserver.go` (`EnsurePort`) kini memberi jalan
keluar, bukan hanya menyatakan kegagalan:

- Port dipegang proses asing → `port 8080 is already in use by "nginx" (PID 1234)
— it is not a previous formspec instance, so it is left alone.` + baris kedua
  berisi **dua cara lanjut**: `kill 1234`, atau jalankan di port lain
  `--addr :8081`.
- Pemilik port tidak bisa diidentifikasi → pesan menyebut port + error aslinya
  dan tetap menawarkan `--addr :<port+1>` alih-alih menggantung di "cannot
  identify the owner".

Nomor port alternatif dihitung dari port yang sibuk (`port+1`), jadi petunjuknya
konkret, bukan contoh generik.

## Kenapa

`formspec dev` sudah benar menolak port milik proses lain (ia tidak boleh
membunuh proses yang bukan miliknya). Yang salah adalah pesannya: pengguna hanya
tahu "port terpakai" tanpa tahu siapa pemakainya dan apa langkah berikutnya —
persis keluhan #24.

## Bukti

| Perintah | Hasil |
| --- | --- |
| `go test ./internal/devserver/ -run TestEnsurePort -v` | **PASS** — `TestEnsurePort_ForeignOwnerMessageIsActionable` (pesan menyebut port + `--addr`; gagal bila pesan lama dikembalikan) dan `TestEnsurePort_FreePortReturnsNil` |
| `go build ./...` | exit 0 |

Test baru `internal/devserver/devserver_test.go` (file test pertama di paket itu):
menahan port di proses test lalu memastikan `EnsurePort` menolak proses asing
dengan pesan yang bisa ditindaklanjuti.
