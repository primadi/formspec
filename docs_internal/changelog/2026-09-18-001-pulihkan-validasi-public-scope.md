# 2026-09-18-001 — Pulihkan validasi `public_entities[].scope` yang hilang

**Apa yang diubah.** Blok validasi `public_entities[].scope` di
`ValidateAppSpec` (`pkg/spec/resources.go`) dipulihkan — 22 baris, isinya sama
persis dengan yang ada di commit `2d816b4`. Tiga aturan yang kembali ditegakkan
saat `formspec validate`:

| Aturan                                      | Kalau tidak divalidasi                                                                              |
| ------------------------------------------- | --------------------------------------------------------------------------------------------------- |
| `scope[].field` wajib                       | filter tanpa field: perilakunya bergantung backend, bukan ditolak                                    |
| `scope[].from` harus `route`                | permukaan publik tidak punya identitas sesi → `from: session` **menolak setiap bacaan** anonim       |
| `find` tidak boleh digabung `scope`         | `find` me-resolve lewat id dan scope tidak bisa menjaganya → grant terlihat terfilter padahal tidak  |

**Kenapa.** Commit `ceaaf2a` ("many updates") menghapus blok itu di hunk
`ValidateAppSpec` (`@@ -1204,28 +1102,6 @@`) sementara test-nya
(`pkg/spec/public_entities_test.go`, dari `2d816b4`) tetap mengharapkannya —
sehingga `go test ./pkg/spec/` **merah di HEAD** untuk tiga kasus:
"session scope on a public surface", "scope without a field", dan
"find granted together with a scope".

Yang **selamat** dan diverifikasi: penegakan runtime-nya utuh —
`applyPublicScope` (`internal/api/handler.go`), `PublicScope` di
`internal/api/descriptor.go`, dan `b.publicScope(...)` di `internal/api/router.go`.
Jadi yang hilang hanya **gerbang validasinya**. Konsekuensinya persis pola yang
diulang di ledger gap repo ini: manifest bisa mendeklarasikan `scope` yang tidak
masuk akal, lolos `formspec validate`, lalu gagal (atau lebih buruk: terlihat
aman) saat runtime — bukan saat spec ditulis.

**File terdampak.** `pkg/spec/resources.go` (satu hunk, aditif). Tidak ada file
lain yang disentuh: `git diff` hanya berisi blok yang dipulihkan.

**Bukti.** `TestValidateAppSpec_PublicEntities_Scope` hijau (sebelumnya 3 sub-kasus
gagal) · `go test ./...` **seluruhnya hijau** · `make lint` **0 issues**
(diverifikasi juga dengan `--max-same-issues 0 --max-issues-per-linter 0`, jadi
tidak ada temuan yang tersembunyi di balik batas default).

**Konteks.** Fiturnya sendiri selesai 2026-09-15 (plan
`docs_internal/plan/public-grant-row-scope.md`, changelog `2026-09-15-011`,
kafe TODO 2.2 / gap #45); changelog ini hanya mengembalikan bagian yang terhapus,
bukan mengubah kontraknya.
