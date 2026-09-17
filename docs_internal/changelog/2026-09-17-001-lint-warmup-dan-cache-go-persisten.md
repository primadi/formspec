# 2026-09-17-001 — `make lint` tidak lagi tampak freeze: warm-up modul + cache Go persisten

Plan: `docs_internal/plan/devcontainer-go-cache-lint.md`.

**Gejala.** `make lint` diam beberapa menit, tanpa output, CPU nyaris nol — dibaca sebagai
freeze dan di-Ctrl+C (exit 130). Ternyata bukan hang: `golangci-lint run ./...` harus
me-load seluruh package graph lewat `go list` sebelum menganalisis, dan langkah itu menunggu
modul Go yang belum ada di cache container. Terukur: `go list ./...` **3m03s** dengan
`user 0m0.563s` (murni menunggu I/O) sambil mencetak `go: downloading …`, sementara
golangci-lint sendiri tidak mencetak apa pun sepanjang fase tersebut. Yang memperparah:
beberapa unduhan **stall** — file `.tmp` di `/go/pkg/mod/cache/download` berukuran 0 byte
dan mtime-nya tidak berubah >90 detik.

**Sebab berulang.** `GOMODCACHE=/go/pkg/mod` dan `GOCACHE=~/.cache/go-build` hidup di layer
container, bukan named volume, dan `.devcontainer/Dockerfile` tidak pre-warm dependensi —
jadi setiap *Rebuild Container* mengulang cold start (~200 MB zip modul, ≈1 GB setelah
diekstrak, + 1.3 GB cache build). Ekstensi Go VS Code juga menginstal tools
(golines/dupl/staticcheck) ke cache yang sama, ikut berebut.

**Perubahan.** (a) `Makefile`: target baru `deps-warm` (`go mod download`, idempoten dan
menampilkan progress `go: downloading …`) sebagai prerequisite `lint`, plus
`--timeout 10m` — di golangci-lint v2 `--timeout` **disabled by default**, sehingga hang
tidak pernah gagal, hanya diam. `deps-warm` sengaja dipisah dari `deps` yang sudah ada
(`go mod tidy` mengubah `go.mod`/`go.sum`, tidak layak jadi gate). (b)
`.devcontainer/compose.yaml`: named volume `go-mod-cache` → `/go/pkg/mod` dan
`go-build-cache` → `/home/vscode/.cache/go-build` supaya cache selamat dari Rebuild
Container; aktif setelah *Rebuild and Reopen in Container*.

**Bukti.** Setelah cache dihangatkan (`go build ./...`), `make lint` selesai **1m43s**
dengan `user 6m19s` — CPU terpakai penuh, artinya fase download sudah lewat dan waktunya
kini benar-benar analisis. Percobaan pertama via `make lint` masih menerima *Interrupt*
sebelum ada perubahan ini, dan `go mod download all` sempat macet >9 menit. Sisa kegagalan
lint **bukan** soal cache: `golangci-lint` melaporkan `typecheck: 1`, dan penelusuran
lanjut menunjukkan **`go build ./...` sendiri gagal** — di luar urusan `lint`. Penyebabnya
perubahan yang belum di-commit di `pkg/spec` (`resources.go` +5/−233): API yang masih
dipakai hilang — `internal/manifest/loader.go:395,406` memanggil `ValidateWorkflowSpec`/
`ValidateModuleSpec`, `internal/workflow/registry.go:72,81,84,172` memakai
`WorkflowTransitionRef.Name`/`.ByName()` — sementara pemakainya tidak ikut diubah. Berkas
test `pkg/spec/frontend_test.go` & `workflow_test.go` **tidak** tersentuh WIP dan menguji
kontrak HEAD, jadi keduanya ikut gagal; test itu **bukan** usang dan tidak boleh dihapus.
Di HEAD API itu semua ada (`pkg/spec/resources.go:834,844,1135`) dan build sehat. Dicatat
sebagai 16.3 di `docs_internal/plan/todo.md`.
