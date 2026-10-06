# 2026-10-02-008 — Dua pembaca action terakhir membaca union (`actions:` ∪ `via`)

**Apa yang diubah.** Dua situs yang membaca `EntitySpec.Actions` langsung dan
seharusnya membaca union (`ActionSources()`) dimigrasikan:

1. `ValidateEntitySpec` (`pkg/spec/entity.go`) — `ValidateHooks(d.Hooks,
d.Actions)` → `ValidateHooks(d.Hooks, d.ActionSources())`. Doc comment
   `ValidateHooks` kini menyatakan bahwa argumennya **wajib** union.
2. `entityFootprint` (`internal/auth/materialize.go`) — `ActionSources()`
   dihitung **sekali** (`sources`) dan dipakai untuk peta `disabled` maupun loop
   action kustom (yang sejak L5 sudah union); pembaca `es.Actions` dihapus dari
   fungsi itu.

**Kenapa.** Menutup kafe 10.60 — lanjutan L8. Sejak L3 `via` adalah action
penuh, dan sejak L4 menulis ulang nama itu di `actions:` **ditolak**, jadi
via-only adalah bentuk yang diminta. Karena itu:

- Hook `on: before|after|on_error` yang menyebut `via` transisi **ditolak**
  `hook action "…" does not match any declared action` — penolakan palsu atas
  bentuk yang justru diwajibkan.
- Footprint grant membaca `es.Actions` sehingga action via-only bisa hilang dari
  editor grant (kelas yang sama dengan 10.47, sudah diperbaiki untuk loop kustom
  di L5; mapper `disabled` adalah sisa terakhir di fungsi itu).

Migrasi `disabled` bersifat **netral-perilaku** (action sintetis tidak pernah
`disabled`), jadi tidak ada test yang bisa membuktikannya gagal; yang dijaga
adalah pembacaan union yang load-bearing di fungsi yang sama.

**Yang sengaja TIDAK disentuh.** `resolveAction` (`internal/api/handler.go`)
tetap membaca `es.Actions` — dipakai jalur `create`/`update` dan menurut plan
`via-sebagai-action-penuh.md` §Further Considerations #1 justru **melindungi**
dari tabrakan `via: update/delete` yang belum diputuskan.

**Celah kelas yang sama ditemukan saat audit ini** → kafe **10.60a**:
`checkRateLimit` (`internal/api/resource_ratelimit.go`) memakai
`resolveAction(es, actionName)`, sehingga `rate_limit` yang dideklarasikan pada
transisi via-only **tidak ditegakkan** pada route `HandleCustomAction`
(`handler.go:2091`). Terikat pada keputusan §Further Considerations #1, jadi
tidak diputuskan sendiri. **Ditutup 2026-10-02** — ternyata tidak butuh
keputusan itu (handler sudah memegang spec union); changelog `2026-10-02-009`.

**Dampak.** `pkg/spec/entity.go` · `internal/auth/materialize.go` · test baru
`pkg/spec/entity_hooks_union_test.go` · `internal/auth/materialize_union_test.go`.

**Bukti.** Dua test pengunci, terkalibrasi: dengan regresi disuntikkan kembali
(`d.ActionSources()` → `d.Actions`, `sources := es.ActionSources()` →
`es.Actions`) keduanya **gagal** dengan pesan yang persis menggambarkan
regresinya (`hook action "mark-ready" does not match any declared action`;
footprint kehilangan `occupy`); setelah dipulihkan keduanya hijau. Kalibrasi
kedua (nama tak dikenal tetap ditolak / tetap tidak termaterialisasi) lulus di
kedua arah. `go build ./...` + `make build` hijau · `go test ./...` 39 paket
`ok`, nol `FAIL` · `formspec validate --spec examples/kafe/spec --schema schemas`
→ 89 manifest, **0 problem** · `formspec check -f examples/kafe/spec` →
**0 error, 0 warning**.

Rujukan: kafe 10.60, plan `docs_internal/plan/via-sebagai-action-penuh.md` L9.
