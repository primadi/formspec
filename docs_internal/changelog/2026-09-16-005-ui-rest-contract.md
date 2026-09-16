# 2026-09-16-005 — Kontrak REST `/_ui/`: halaman + `formspec describe` mencetak dari generator

Item `examples/kafe/gaps_found/TODO.md` **2.7** (gap **#47**). Plan:
`docs_internal/plan/ui-rest-contract.md`.

**Masalah.** `/_ui/` adalah kontrak publik (SPA bawaan memakainya) tetapi tidak
terdokumentasi: bentuk body, envelope, dan endpoint aksi hanya ditemukan dengan
gagal berkali-kali. Untuk project yang menjanjikan manifest sebagai satu-satunya
sumber kebenaran, justru permukaan HTTP-nya yang paling tidak terdokumentasi.

**Yang dikerjakan.** (a) Halaman `docs/runtimes/06-ui-rest-contract.md`: path
(singular! plural hanya untuk permission/`api/v1`), tabel method/aksi/permission,
body **flat** (envelope `{"data": …}` ditolak 400), tiga envelope respons, query
list (`per_page` max 100, 13 operator, sort type-aware), catatan bahwa parameter
`row_scope` bukan filter, dan ringkasan surface publik. (b) `formspec describe
entity <name>` mencetak kontrak HTTP entity itu, dengan route **digenerate** dari
generator yang sama dengan server.

**Refactor yang membuat (b) mungkin tanpa database.**
`GenerateUIRoutes`/`GenerateUICustomActionRoutes` dipecah menjadi per-entity
(`UIRoutesForEntity`, `UICustomActionRoutesForEntity`); kedua fungsi lama kini
memanggilnya, sehingga CLI dan server tidak bisa berbeda. Ini penting karena
kontraknya penuh pengecualian yang mudah salah ditulis tangan: aksi
`disabled: true` tidak punya route, lifecycle-free tanpa `submit`/`cancel`/
`amend`, `summary` hanya `list`+`find`, dan **transisi state machine tanpa `impl`
tidak punya endpoint sendiri** (diterapkan lewat `update`) — klaim terakhir kini
turut tercetak supaya tidak dibaca sebagai route yang hilang.

**Bukti.** `formspec describe entity order` (kafe) mencetak tepat 4 route
list/find/create/update — cocok dengan spec kafe yang mematikan `delete` +
`submit`; `TestUIRoutesForEntity_LifecycleAndDisabled` (lifecycle penuh vs kafe
`order` vs `summary`); `TestUICustomActionRoutesForEntity_OnlyActionsWithImpl`;
`go test ./...` hijau; kafe `validate` 0 problem.

**Sisa.** Halaman belum digenerate dari `pkg/spec` (usulan #47 poin 2);
Report/Print/dashboard dan kontrak `api/v1` tetap di luar cakupannya.
