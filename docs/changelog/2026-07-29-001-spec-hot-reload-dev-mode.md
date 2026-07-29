# Spec Hot-Reload di `forma dev`

**Tanggal**: 2026-07-29
**Plan**: docs/plan/spec-hot-reload.md (bagian dari diskusi dengan AI)

## Perubahan

### 1. `resource/forma.go` — `App.ReloadSpec()` method

- Menambahkan `sync.RWMutex` (`mu`) ke `App` struct untuk proteksi atomic swap
- Menambahkan `database` (db.DB) dan `driver` (db.DriverType) ke `App` — menyimpan referensi koneksi database untuk digunakan ulang saat reload
- Menambahkan `nativeHandlers` map ke `App` — menyimpan native Go handlers yang di-register via `RegisterNative()` agar bisa di-re-register setelah reload
- `RegisterNative()` sekarang menyimpan handler yang sudah di-wrap ke `nativeHandlers`
- `Handler()` sekarang mengembalikan wrapper yang membaca pointer handler terkini (protected by RWMutex) — reload tidak memutus request in-flight
- `Routes()`, `RouteCount()`, `Registry()` menggunakan read lock
- `ReloadSpec()` method baru: membaca ulang semua YAML, rebuild entity registry, UI registry, App resolution, router, dispatcher, lalu atomic swap pointer
- Mempertahankan WSHub yang sudah ada (WebSocket connections tidak terputus)

### 2. `internal/api/router.go` — `SetHub()` method

- Menambahkan `SetHub(h *WSHub)` ke `RouterBuilder` — memungkinkan transfer WSHub yang sudah ada saat reload, sehingga WebSocket connections tidak terputus

### 3. `cmd/forma/dev.go` — File watcher

- `watchSpecForChanges()` function: menggunakan `fsnotify` untuk memantau folder `spec/` dan semua subdirektori
- Auto-watch subdirektori baru yang dibuat setelah watcher start
- Debounce 300ms untuk mengkoalesensi event save editor (write/create/remove/rename)
- Filter: hanya file `.yaml`, `.yml`, dan `.star`
- Panggil `app.ReloadSpec()` setelah debounce timer fire
- Berhenti saat context di-cancel (SIGINT/SIGTERM)

### File yang terkena dampak

| File | Perubahan |
|---|---|
| `resource/forma.go` | +App.ReloadSpec(), +mu/database/driver/nativeHandlers fields, refactor Handler/Routes/RouteCount/Registry |
| `internal/api/router.go` | +SetHub() method |
| `cmd/forma/dev.go` | +watchSpecForChanges(), import fsnotify |

### Catatan

- Dev mode only — `watchSpecForChanges` hanya jalan saat `forma dev`, tidak di `forma serve`/produksi
- Full reload approach (Opsi A): semua spec dibaca ulang, registri dibuild dari nol, atomic swap pointer
- Schema migration idempoten: `ApplyMigrations` cek `forma_schema_migrations` — hanya applied jika checksum berbeda
- Native Go handlers preserved via `nativeHandlers` map
