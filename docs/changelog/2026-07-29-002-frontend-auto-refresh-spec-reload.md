# Frontend Auto-Refresh Setelah Spec Hot-Reload

**Tanggal**: 2026-07-29

## Masalah

Setelah spec hot-reload (backend rebuild registri), browser tetap menampilkan bundle lama karena tidak ada notifikasi ke frontend.

## Solusi

Vite HMR — infrastruktur WebSocket yang sudah ada antara Vite dev server dan browser, zero polling.

### Backend

1. **`cmd/forma/dev.go`** — `watchSpecForChanges()` menerima parameter `viteHMRURL`. Setelah reload sukses, melakukan HTTP GET ke `http://localhost:<vitePort>/_dev/hmr-reload` (hanya saat `--dev-ui` aktif)
2. **`resource/forma.go`** — `App.specVersion atomic.Int64` untuk logging
3. **Relokasi**: watcher dipindah ke setelah Vite section (step 13c), agar `viteProxyURL` sudah diketahui

### Vite Plugin (`vite.config.ts`)

- Plugin `formaHMRPlugin()`:
  - Menambahkan middleware `/_dev/hmr-reload` di Vite dev server
  - Saat dipanggil, mengirim custom event `forma:spec-reloaded` ke semua browser via `server.ws.send()`
  - Plugin hanya aktif saat `vite dev` (dev mode) — tidak masuk production build

### Frontend

1. **`renderers/web/src/App.tsx`** — `useEffect` dengan `import.meta.hot.on("forma:spec-reloaded", ...)`:
   - Mendengarkan event HMR dari Vite
   - Memanggil `useMetaStore.getState().refresh()` → re-fetch full bundle
   - Cleanup `import.meta.hot.off()` saat unmount
   - Kode ini DITREE-SHAKE oleh Vite di production build (`import.meta.hot` → `undefined`)
2. **`renderers/web/src/stores/meta.ts`** — Action `refresh()` untuk re-fetch bundle

### Cara Kerja

```
Edit YAML → fsnotify → ReloadSpec() → HTTP GET → Vite /_dev/hmr-reload
                                                  ↓
                                            server.ws.send('forma:spec-reloaded')
                                                  ↓
                                            Browser (via HMR WebSocket)
                                                  ↓
                                            import.meta.hot.on() → refresh()
                                                  ↓
                                            Re-fetch /_meta/ui → Zustand → re-render
```

### File Terkena Dampak

| File | Perubahan |
|---|---|
| `cmd/forma/dev.go` | +viteHMRURL param, watcher dipindah ke after Vite, HTTP GET ke Vite setelah reload |
| `resource/forma.go` | +specVersion atomic.Int64 (logging) |
| `renderers/web/vite.config.ts` | +formaHMRPlugin() — middleware `/_dev/hmr-reload` |
| `renderers/web/src/App.tsx` | +import.meta.hot listener untuk `forma:spec-reloaded` |
| `renderers/web/src/stores/meta.ts` | +refresh() action |
