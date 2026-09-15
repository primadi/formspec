# 2026-09-14-006 — S2: row scope server-side (`from: session|route`)

Item `TODO.md` 1.1 (S2, gap #6/#9). Tujuannya membuat "kasir cabang A tidak
melihat pesanan cabang B" dan "pelanggan melihat pesanannya sendiri lewat token"
menjadi **jaminan server**, bukan filter yang di-merge browser.

**Kenapa di Entity, bukan di kind.** `fixed_filters` pada Table/Kanban di-merge
di browser (`renderers/react-shadcn/src/lib/filters.ts`) lalu dikirim sebagai
query param. Klien mana pun bisa menghilangkannya — jadi ia tidak pernah bisa
menjadi kontrol otorisasi. Karena itu scope otoritatif ditempatkan di **Entity**
(`EntitySpec.Scope`), tempat server selalu melihatnya.

Yang diubah:

- `pkg/spec/frontend.go` — `FilterSpec` dapat `from: session|route`, `attr`, `param`.
- `pkg/spec/entity.go` — `EntitySpec.Scope []FilterSpec` baru + validasi:
  field harus ada di entity, dan `from` wajib `session`/`route`
  (`scope[0] (branch_id): from must be "session" or "route", got "cookie"`).
- `internal/api/scope.go` (baru) — `applyRowScope` + `sessionAttr`:
  `from: session` mengambil atribut identitas dan **meng-override** nilai klien
  pada field yang sama; `from: route` mengambil parameter query yang
  dideklarasikan (mis. token tamu yang memang jadi kredensial). Nilai yang tidak
  bisa diselesaikan → **403 fail closed**, tidak pernah degrade menjadi "tanpa
  filter".
- `internal/api/handler.go` — `applyRowScope` dipanggil di `HandleList`;
  `parseListQuery` melewati parameter scope yang dideklarasikan (kalau tidak,
  `?branch=A` diparse sebagai filter field `branch` → 422).
- `internal/auth/auth.go`, `jwt.go` — `Identity.Attributes` + klaim JWT `attrs`,
  sumber nilai untuk `from: session`.
- Schema diregenerasi (`make generate-schema` → `schemas/formspec.schema.json`,
  `schemas/kinds/Entity.schema.json`).

Verifikasi runtime (spec uji dua baris A/B, binary di-rebuild lebih dulu, raw JSON):

| Permintaan                  | Hasil                                                            |
| --------------------------- | ---------------------------------------------------------------- |
| tanpa param                 | `403 row scope on branch_id: missing "branch" request parameter` |
| `?branch=A`                 | `total: 1`, hanya A                                              |
| `?branch=A&branch_id[eq]=B` | `total: 1`, **hanya A** — klien tidak bisa melebarkan            |
| `?branch=B`                 | `total: 1`, hanya B                                              |
| `?branch_id[notnull]=1`     | `403` — tidak bisa dilewati lewat operator lain                  |

Test: 7 case `internal/api/scope_test.go` + `TestValidateEntitySpec_Scope`.
`go test ./...` → 35 paket `ok`; spec kafe tetap **0 problem** (schema registry
maupun schema lokal).

**Dua hal yang sengaja belum dilakukan (dan alasannya):**

1. **Adopsi di spec kafe.** `from: session, attr: branch_id` butuh atribut sesi
   yang terisi, dan mekanisme penugasan (employee → cabang) baru ada di 1.8/3.5
   (S5 `assignments`). Dipasang sekarang, seluruh daftar kasir akan 403 — benar
   secara fail-closed, tapi aplikasi tak terpakai. Sebaliknya `from: route` pada
   `table-session` akan menuntut `?token=` di **semua** surface, termasuk POS
   kasir yang tidak punya token; scope per-permukaan adalah S3 (1.2). Jadi adopsi
   menunggu 1.8 dan 1.2.
2. **Publikasi schema.** `formspec validate` tanpa `--schema` memakai cache
   registry, yang menolak properti baru
   (`additional properties 'scope' not allowed`). Schema lokal sudah benar; yang
   dipublikasikan di registry perlu di-refresh sebelum aplikasi mana pun memakai
   `scope:` — kalau tidak, spec yang benar akan tampak salah.

Referensi: `examples/kafe/gaps_found/TODO.md` (1.1), `README.md` (#6, #8).
