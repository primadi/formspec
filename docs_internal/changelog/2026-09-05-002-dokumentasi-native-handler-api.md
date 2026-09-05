# 2026-09-05-002 — Dokumentasi Native Handler API

Menutup gap dokumentasi native handler yang teridentifikasi saat review
`cmd/formspec-registry/main.go` (API `RegisterNatives` tidak terdokumentasi,
format ref tidak dirinci, dan deskripsi auto-scan `impl/**/*.go` tidak sesuai
implementasi aktual).

- `docs/spec/backend/06-script-runtime.md` §7 ditulis ulang: mendokumentasikan
  API publik `App.RegisterNative`/`RegisterNatives`, signature `NativeHandler`
  - `NativeParams`, 3 format `ref` + urutan resolusi `NativeExecutor`, dan
    handler bawaan engine. Auto-scan `impl/**/*.go` dikoreksi menjadi "target
    desain" (belum diimplementasikan — registrasi eksplisit adalah perilaku
    aktual hari ini).
- `docs/runtimes/02-formspec-resource.md`: tambah §5.3 "Native Handler API"
  (ringkasan + pointer ke spec), perbaiki gap §7 #7 yang outdated ("tak ada
  handler yang pernah diregistrasi" → handler kini terdaftar).
- `docs/registry/05-self-hosting.md`: tambah handler `registry.vendor.approve`
  ke daftar native handler binary registry.

File terdampak: `docs/spec/backend/06-script-runtime.md`,
`docs/runtimes/02-formspec-resource.md`, `docs/registry/05-self-hosting.md`.
