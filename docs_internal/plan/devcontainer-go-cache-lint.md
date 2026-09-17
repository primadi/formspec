# Plan: `make lint` tidak lagi tampak freeze — warm-up modul + cache Go persisten

**Status**: ✅ selesai 2026-09-17
**Changelog**: `docs_internal/changelog/2026-09-17-001-lint-warmup-dan-cache-go-persisten.md`
**Pemicu**: laporan "`make lint` kok freeze?" (exit 130 — di-Ctrl+C setelah menunggu lama tanpa output)

## Gejala

`make lint` (`Makefile:52-53` → `golangci-lint run ./...`) berhenti tanpa output apa pun
selama beberapa menit. Tidak ada pesan error, tidak ada progress, CPU nyaris nol.

## Akar masalah (hasil investigasi)

Golangci-lint tidak menganalisis apa pun sebelum ia **me-load seluruh package graph**
(`go list`). Langkah itu menunggu modul Go yang belum ada di cache:

| Bukti | Data terukur |
| --- | --- |
| `go list ./...` | **3m03s** lalu timeout 180s; `user 0m0.563s` → ~0 CPU, murni menunggu I/O |
| Output yang terlihat | `go: downloading github.com/mailru/easyjson …` dst. |
| Cache modul saat itu | 278M dan **tidak lengkap** (k8s.io, google.golang.org/grpc, genproto, honnef.co/go/tools belum ada) |
| Stall nyata | file `.tmp` di `cache/download` berukuran **0 byte** dan mtime-nya tidak berubah >90 detik (mis. `honnef.co/go/tools/@v/v0.7.0.zip*.tmp`) — koneksi stall, bukan sekadar lambat |

Selama fase load ini golangci-lint **mencetak apa-apa**, jadi download (lambat atau stall)
tampak persis seperti hang.

**Mengapa berulang.** `.devcontainer/compose.yaml` hanya mount `../..:/workspaces:cached`.
`GOMODCACHE=/go/pkg/mod` dan `GOCACHE=/home/vscode/.cache/go-build` hidup di layer container,
bukan named volume, dan `.devcontainer/Dockerfile` tidak pre-warm dependensi. Setiap
*Rebuild Container* mengulang cold start: ~200 MB zip modul (≈1 GB setelah diekstrak) +
cache build 1.3 GB. Ekstensi Go di VS Code juga menginstal tools sendiri
(golines/dupl/staticcheck) ke cache yang sama, jadi ikut berebut.

## Perubahan

| File | Perubahan |
| --- | --- |
| `Makefile` | target baru `deps-warm` (`go mod download`), dipakai sebagai prerequisite `lint`; `lint` memakai `--timeout 10m` |
| `.devcontainer/compose.yaml` | named volume `go-mod-cache` → `/go/pkg/mod` dan `go-build-cache` → `/home/vscode/.cache/go-build` |

Alasan `deps-warm` dipisah dari `deps` yang sudah ada: `deps` menjalankan `go mod tidy`
(mengubah `go.mod`/`go.sum` dan butuh seluruh module graph) — tidak cocok sebagai
prerequisite gate lint. `go mod download` idempoten dan hampir no-op kalau cache hangat.

Alasan `--timeout 10m`: di golangci-lint v2 `--timeout` **disabled by default**, sehingga
hang tidak pernah gagal — hanya diam.

## Verifikasi

- Sebelum: `make lint` >3 menit tanpa output (user Ctrl+C, exit 130).
- Sesudah cache hangat: `make lint` selesai **1m43s** (`user 6m19s` — CPU terpakai penuh,
  artinya fase download memang sudah lewat) dan mencetak hasil analisis.
- Sisa kegagalan lint **bukan** soal cache: `typecheck: 1` di `pkg/spec/*_test.go`
  (test usang terhadap API yang sudah dihapus) → dicatat sebagai task terpisah di `todo.md`.

## Efek samping / batasan

- Volume `go-mod-cache` di-seed dari isi `/go/pkg/mod` di image saat pertama kali dibuat
  (Docker menyalin konten + ownership `vscode:golang`), jadi cache hangat sekarang tidak hilang.
- Perubahan `compose.yaml` baru aktif setelah **Rebuild and Reopen in Container**.
- Disk bertambah: cache modul + build cache kini menetap antar-rebuild (~2–3 GB).
  Bersihkan dengan `docker volume rm formspec_go-mod-cache formspec_go-build-cache`
  (nama persisnya mengandung prefix project) bila perlu.
