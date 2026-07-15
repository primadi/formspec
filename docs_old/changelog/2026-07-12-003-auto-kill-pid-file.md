# Auto-kill Previous `forma dev` via PID File

## Perubahan

Sebelumnya setiap restart `forma dev` perlu `pkill` manual atau `Ctrl+C`
sebelum menjalankan lagi. Sekarang `runDev()` otomatis:

1. Baca PID file `.forma/dev.pid` dari instance sebelumnya
2. Kirim SIGTERM + SIGKILL ke proses lama
3. Tulis PID baru ke file
4. Hapus PID file saat shutdown (`ctx.Done` atau server error)

## Detail Teknis

| Fungsi | Lokasi | Tugas |
|---|---|---|
| `autoKillPrevious()` | `cmd/forma/dev.go` | Baca PID file, kill proses lama, hapus file |
| `writePIDFile()` | `cmd/forma/dev.go` | Tulis PID saat ini ke `.forma/dev.pid` |
| `cleanupPIDFile()` | `cmd/forma/dev.go` | Hapus file saat shutdown (defer) |

PID file diletakkan di `.forma/dev.pid` (dalam `defaultStateDir`).
Tidak butuh flag baru — berjalan otomatis di setiap `forma dev`.

## File Terkena Dampak

- `cmd/forma/dev.go` (+3 fungsi, ~60 baris)

## Referensi

- Plan: `docs/plan/auto-kill-pid-file.md`
