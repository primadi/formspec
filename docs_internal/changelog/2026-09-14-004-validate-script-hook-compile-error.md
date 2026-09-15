# 2026-09-14-004 — `formspec validate`: script hook yang gagal kompilasi kini error

Gap #50 dari ledger kafe ditutup. Ditemukan saat Fase 0: `formspec validate`
melaporkan **0 problem** pada spec kafe padahal tiga guard script yang dirujuk
`hooks:` tidak bisa dikompilasi (`script compile error: got string literal, want
','`). Akibatnya satu-satunya gerbang otomatis proyek tidak dapat membedakan spec
yang sehat dari spec yang setiap penulisannya gagal.

Akar masalahnya bukan "tidak ada pemeriksaan", tetapi **pemeriksaannya tidak
lengkap**: honesty scan (`cmd/formspec/honesty.go`) sudah melaporkan `parseErr`
lewat `compareUses` untuk **action**, sementara loop **hook** hanya memeriksa
`usage.envBranch` dan membuang `parseErr`. Jadi error kompilasi script hook tidak
pernah muncul.

Yang diubah — `cmd/formspec/honesty.go`:

- `scriptLoadIssue(source, path, where, usage)` baru: script yang tidak dapat
  **diselesaikan** (`path == ""` → `impl.ref` menunjuk file yang tidak ada) atau
  tidak dapat **dikompilasi** (`parseErr`) menghasilkan issue `severity: error`.
- Loop hook memanggilnya untuk impl `script_ref`; loop action juga memakainya
  (dan berhenti sebelum `compareUses` bila script tidak ditemukan, agar tidak
  membanjiri laporan dengan "uses tidak dideklarasikan" yang menyesatkan).
- Inline script (`impl.script`) tidak terpengaruh — tidak ada file untuk di-resolve.

Diverifikasi dengan spec uji (hook → script rusak + hook → script hilang):

```
[FAIL] …/thing/entity.yaml#0
       honesty: hook before (action create): script failed to compile: …/broken.star:4:23: got string literal, want ','
       honesty: hook before (action update): script not found (looked for scripts/<ref>.star next to the manifest and in modules/<module>/scripts/)
```

`examples/kafe/spec` tetap **0 problem** (script-nya sudah diperbaiki di
changelog 2026-09-14-001). `go test ./...` → 35 paket `ok`; test baru
`TestHonestyScan_HookScriptCompileError`.

Dampak samping (positif): bagian `impl.ref` dari gap **#21** (validator tidak
menangkap referensi menggantung) kini tertutup; sisa #21 adalah `App.spec.modules`
dan menu `view:` (TODO 8.3).

Referensi: `examples/kafe/gaps_found/TODO.md` (8.7, 8.3),
`examples/kafe/gaps_found/validate-baseline.md`.
