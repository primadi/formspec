# Plan: Auto-kill Previous `forma dev`

## Problem

Setiap restart `forma dev` perlu `pkill` manual sebelum `go run ...`.
Ini mengganggu workflow dev, terutama saat sering rebuild frontend.

## Solusi

PID file di `.forma/dev.pid`:

1. **`autoKillPrevious()`** — dipanggil paling awal di `runDev()`, sebelum flag parsing. Baca PID dari `.forma/dev.pid`, kirim SIGTERM+SIGKILL, hapus file.
2. **`writePIDFile()`** — setelah state directory siap, tulis `os.Getpid()` ke file.
3. **`cleanupPIDFile()`** — defer hapus file saat shutdown.

## File

- `cmd/forma/dev.go` — semua fungsi baru + panggilan di `runDev()`

## Level of Effort

Small — ~60 baris, fungsi helper murni.

## Risk

- Jika hard kill (SIGKILL) diperlukan, proses bisa mati mendadak. Tidak masalah untuk dev.
- PID file tidak akan terhapus jika kill -9 di luar. autoKill akan hapus file dan lanjut.
