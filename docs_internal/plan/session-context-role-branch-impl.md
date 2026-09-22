# Plan implementasi — Konteks sesi: (principal, role, cabang)

**Untuk:** TODO 3.8 (`examples/kafe/gaps_found/TODO.md`).
**Desain (disetujui 2026-09-16):** `docs_internal/plan/session-context-role-branch.md`.
**Dikerjakan:** 2026-09-20.

## Keputusan bentuk yang diambil (detail yang tidak dipatok di desain)

| Pertanyaan | Keputusan | Alasan |
| --- | --- | --- |
| Bentuk pilihan `assignment` di body login/switch | **string `<role>@<value>`** (mis. `sales@cafe-master.branch/B1`). Split pada `@` **pertama** — nama role tidak boleh mengandung `@` (`^[a-z][a-z0-9_]*$`), jadi nilai boleh mengandung `@`. | Cocok dengan kosakata desain (`sales@A`), self-describing (tahan urutan daftar), dan tidak butuh escaping. `id` di `choices` memakai bentuk yang sama. |
| `choices` di respons 409 | `choices: [{id, role, dimension, value}]` di dalam envelope error (`error.choices`) | Sesuai desain; klien menampilkan pemilih lalu mengirim ulang `id`. |
| Nilai `attrs` klaim | `{<dimension>: <value>}` — mis. `{"branch": "…"}` | Desain: "attrs (dimensi→nilai)". Supaya `row_scope: {field: branch_id, from: session}` tetap jalan **tanpa mengubah spec kafe**, resolver scope mencoba nama atribut berurutan: `attr` eksplisit → `scope.field` → `scope.dimension`. |
| Permission saat konteks dipilih | grant **role terpilih** saja + permission langsung akun (`permissions:`) | `permissions` adalah grant tingkat-akun, bukan role; cache resolver diberi kunci per-role agar tidak bocor antar konteks. |
| Kredensial endpoint `switch` | `POST /_ui/auth/switch` body `{assignment, refresh_token}`; publik + rate-limited (seperti `refresh`) | `refresh_token` adalah kredensial yang sudah dipegang klien dan sekaligus **identitas sesi lama** yang harus di-revoke — `access_token` tidak punya `jti`. Tanpa ini "tidak ada dua konteks hidup" tidak bisa dijamin, dan switch akan gagal begitu access token (15 menit) kedaluwarsa. |
| Akun tanpa assignment | perilaku lama (union role, tanpa `role`/`attrs`) | Syarat backward compatible. |
| Assignment tidak lengkap (`role`/`dimension`/`value` kosong) | bukan pilihan yang sah (diabaikan) | Entitas mendeklarasikan ketiganya `required`. |
| Assignment dicabut / role dihapus | `409 CONTEXT_REQUIRED` + `choices` (fail closed) — **bukan** lanjut tanpa konteks | Desain tahap 4. |

## Tahapan → file

| # | Isi | File |
| --- | --- | --- |
| 1 | Model assignment | `internal/auth/module/master/user/entity.yaml` (field `assignments` json), `internal/auth/user.go` (`Assignment`, `User.Assignments`, `userFromRecord`, semua `Data:` map update) |
| 2 | Login memilih konteks | `internal/auth/context.go` (baru), `internal/auth/service.go` (`LoginWithContext`), `internal/api/auth_handler.go`, `internal/auth/resolver.go` (`ResolveForRole`) |
| 3 | Klaim sesi + record sesi | `internal/auth/token.go`, `internal/auth/jwt.go`, `internal/auth/auth.go`, `internal/auth/session.go`, `internal/auth/module/transaction/session/entity.yaml`, `internal/api/scope.go` |
| 3b | OAuth pakai pilihan terakhir + switch | `internal/api/oauth_handler.go`, `internal/api/router.go`, `internal/api/handler.go` (`ErrorDetail.Choices`) |
| 5 | Adopsi + dokumentasi | `examples/kafe/spec/modules/cafe-master/master/employee/entity.yaml` (catatan), `docs/spec/backend/01-core-basic.md` (§ sesi) |

## Verifikasi

- `go test ./...`, `make build`, `make lint` (0 issues).
- `cd examples/kafe && ../../bin/formspec validate --schema ../../schemas` → 0 problem.
- Bila menyentuh frontend: `npx tsc -p tsconfig.app.json --noEmit` + `npx vitest run`.
