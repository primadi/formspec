# 2026-09-01-005 — Fix hot-reload pada native binary (ListenAndServe pakai handler dinamis)

## Apa

`App.ListenAndServe()` memakai `a.handler` langsung saat membuat `http.Server`:

```go
a.httpServer = &http.Server{Addr: a.cfg.Addr, Handler: a.handler}
```

Setelah `ReloadSpec()` menukar `a.handler`, `http.Server` tetap menyajikan
handler lama (dengan UI registry lama) → hot-reload "complete" tapi konten
yang disajikan (mis. subtitle page di `/_ui/_meta/ui`) tetap versi lama.

`formspec dev` tidak terdampak karena memakai `app.Handler()` — wrapper
dinamis yang membaca `a.handler` di bawah lock setiap request. Native binary
(`formspec-registry`) memakai `ListenAndServe()`, jadi kena bug ini.

Fix: `ListenAndServe()` kini memakai `a.Handler()` (wrapper dinamis) —
perilaku sama dengan `formspec dev`.

## Verifikasi

- Baseline: meta API menyajikan `Registry2 (v5)`.
- Edit `profile.yaml` → log `reload complete` → meta API menyajikan
  `Registry2 (v6-FIXED)`. ✅
- `go test ./resource/...`: 50 passed.

## File terdampak

- `resource/formspec.go` (`ListenAndServe`)

## Referensi

- Changelog terkait: `2026-09-01-004` (devserver DX untuk native binary)
- Todo: 5.2.9 (devserver DX parity)
