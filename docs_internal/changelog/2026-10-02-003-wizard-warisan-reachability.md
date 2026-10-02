# 2026-10-02-003 — Wizard mewarisi reachability dari entity-nya

Todo: **5.25.7** · Melanjutkan `2026-10-02-002` (peluncur + commit) dan
`2026-10-02-001` (`registered_views`).

## Apa

`BuildBundle` (`internal/ui/meta.go`) menambahkan satu aturan ke surface
allowlist: **wizard yang mengikat dirinya ke sebuah transisi masuk bundle bila
entity-nya routable** — tanpa perlu didaftarkan di `registered_views`.

```go
wizardRoutable := func(e *Entry[spec.WizardSpec]) bool {
    if viewRoutable(e.Module, "/wizard/"+e.Name) { return true }   // eksplisit
    // warisan: spec.entity + spec.action (== transisi `via`) dari entity routable
    ...
}
```

Pendaftaran eksplisit tetap berlaku (aditif). Zero `AppContext` (`_admin`) dan
`?grants=true` tidak terpengaruh.

## Kenapa

Peluncur (`DetailPage`) membaca `bundle.wizards`. Wizard hanya masuk bundle bila
route-nya reachable, yaitu bila terdaftar di `registered_views` — sehingga
**melepas baris `{view: cafe-order/close-shift-wizard}` diam-diam menghidupkan
lagi jalur bypass**: tombol transisi kembali ke `PATCH status` mentah, dan
`counted_cash`/`note` tak pernah terkumpul. Tidak ada gejala sampai ada yang
mengklik, dan tidak ada yang menghubungkan "transisi X punya wizard" dengan
"wizard X wajib terdaftar" — kopling itu murni kebetulan implementasi.

Wizard yang mengikat transisi **adalah UI untuk transisi itu**, bukan tujuan
mandiri: kalau entity-nya bisa dibuka, wizard-nya juga.

## File

- `internal/ui/meta.go` — `wizardRoutable` di `BuildBundle`; loop `Wizards`
  memakainya.
- `internal/ui/wizard_reachability_test.go` (baru) — 4 test.
- `examples/kafe/spec/apps/kafe-pos.yaml` — baris
  `{view: cafe-order/close-shift-wizard}` **dihapus**; komentar menjelaskan
  warisannya.

## Bukti

**Terukur (bundle nyata, setelah baris dihapus):**

| App (user)       | `wizards`                | `shift` routable |
| ---------------- | ------------------------ | ---------------- |
| kafe-pos (kasir) | `['close-shift-wizard']` | True             |
| kafe-kds (dapur) | `[]`                     | entity tidak ada |

Kafe-kds **tidak** mendapat wizard — tepat: permukaannya hanya papan Kanban,
`shift` tidak routable di sana.

**Otomatis:** `internal/ui/wizard_reachability_test.go` — (1) inherit saat entity
routable, (2) **tidak** inherit saat entity non-routable, (3) pendaftaran
eksplisit tetap berlaku, (4) tidak aktif tanpa App scope. `go test ./...` 0 FAIL;
`gofmt -l` bersih; `formspec check -f examples/kafe/spec` 0 error / 0 warning.

## Sisa

- Verifikasi klik-di-browser untuk jalur warisan belum diulang (server sudah
  dihentikan setelah cek bundle). Mekanismenya identik dengan
  `2026-10-02-002` yang sudah terverifikasi; bundle-nya sudah dipastikan
  memuat wizard.
- **5.25.2 / 5.25.3 / 5.25.6 ⏸️** tak tersentuh.
