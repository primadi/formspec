# Plan: Versi Rilis Otomatis (anti git-describe)

**Status**: 🚧 in progress
**Tanggal**: 2026-09-17
**Referensi**: `docs/guides/releasing.md`, `docs_internal/plan/formspec-upgrade-command.md` §Fase 2

## Masalah

`make release` / `make release-upload` tanpa `VERSION=` memakai default Makefile
`VERSION ?= $(shell git describe --tags --always --dirty)` → menghasilkan
`v0.0.8-4-gceaaf2a` (string git, bukan semver). Tag itu ter-publish dan menjadi
`releases/latest`; komparator semver `formspec upgrade` membacanya sebagai
*prerelease dari v0.0.8* → user di v0.0.8 melihat prompt "rollback" padahal
isinya 4 commit **lebih baru** (rilis `v0.0.8-4-gceaaf2a`, 2026-09-17).

Guard `release-upload` tidak menangkapnya karena regex-nya tidak di-anchor `$`
(`^v[0-9]+\.[0-9]+\.[0-9]+` cocok sebagai prefix).

## Solusi

1. **Versi rilis otomatis** — `VERSION=` eksplisit menang; kalau tidak diisi,
   versi diturunkan dari tag semver tertinggi (patch-bump) via
   `scripts/next-version.sh`. Default `git describe` tetap dipakai hanya untuk
   *build dev* (`build-formspec`), bukan untuk rilis.
2. **Guard semver murni** — `scripts/check-semver.sh` dipakai target `release`
   & `release-upload` (fail sebelum build mahal); menolak string git-describe
   dengan pesan yang menjelaskan akibatnya di `formspec upgrade`.
3. **Comparator toleran legacy** — `cmd/formspec/semver.go` mengenali suffix
   git-describe `-<N>-g<hash>[-dirty]` sebagai *post-release snapshot*
   (lebih baru dari rilis core yang sama, lebih lama dari patch berikutnya),
   bukan prerelease. Ini menjaga binary/tag describe yang sudah beredar
   (`v0.0.8-4-gceaaf2a`) tidak lagi memicu prompt rollback palsu.

## File yang diubah/dibuat

| File                                | Aksi    | Isi                                                                        | Effort |
| ----------------------------------- | ------- | -------------------------------------------------------------------------- | ------ |
| `scripts/next-version.sh`           | baru    | Patch-bump dari tag semver tertinggi; error bila belum ada tag semver       | small  |
| `scripts/check-semver.sh`           | baru    | Guard semver murni + pesan khusus git-describe                              | small  |
| `Makefile`                          | ubah    | `RELEASE_VERSION`, target `release-version`, guard `check-release-version`  | small  |
| `cmd/formspec/semver.go`            | ubah    | Deteksi + ordering post-release (git describe)                              | small  |
| `cmd/formspec/semver_test.go`       | ubah    | Kasus describe: `v0.0.8-4-gceaaf2a` vs `v0.0.8` / `v0.0.9` / `-dirty`       | small  |
| `scripts/git-push-and-tag.sh`       | ubah    | Usulan versi dari `next-version.sh` saat argumen kosong                     | small  |
| `docs/guides/releasing.md`          | ubah    | §0 + jalur cepat: VERSION opsional, auto-bump, larangan describe            | small  |

## Keputusan

- **Tag tetap dipilih sadar**: `release-upload` masih menuntut tag sudah ada
  lokal (`git rev-parse`), jadi auto-version tidak pernah membuat tag sendiri.
  Alur: `make release` (auto) → `git tag <versi>` → `make release-upload`.
- **Rilis pertama** (tanpa tag semver sama sekali) → `next-version.sh` gagal
  dengan pesan "tentukan VERSION= eksplisit", mempertahankan aturan lama.
- **Prerelease (`v0.1.0-rc.1`) ditolak** oleh guard rilis — paritas dengan
  `scripts/git-push-and-tag.sh` yang memang sudah strict sejak awal.
- **Tidak** mengubah nama artifact/`SHA256SUMS.txt`/endpoint `releases/latest`
  (kontrak publik, lihat `docs/guides/releasing.md` §3).

## Verifikasi

- `go test ./cmd/formspec/...` (kasus baru di `semver_test.go`).
- `make release-version` → `v0.0.9`; `make release-version VERSION=v0.0.8-4-gceaaf2a`
  → ditolak guard; `make release-version VERSION=v0.2.0` → dipakai apa adanya.

## Catatan lanjutan (out of scope)

- Release `v0.0.8-4-gceaaf2a` yang sudah published tidak dihapus/di-retag
  (satu tag = satu release). Rilis patch semver berikutnya (`v0.0.9`) akan
  mengambil alih label "Latest" di GitHub.

## Lanjutan 2026-09-18 — langkah tag tidak terbaca di `make release`

**Gejala**: `make release` sukses (auto-bump `v0.0.9` + 8 artifact di
`dist/release/`), lalu `make release-upload` gagal `Tag v0.0.9 belum ada lokal`.
Guard-nya benar (keputusan di atas: tag tidak pernah dibuat otomatis), tapi
`make release` tidak menyebut langkah tag sama sekali — kebutuhannya baru terasa
setelah SPA build + 6 cross-compile selesai. Temuan menyertai: `docs/guides/
releasing.md` §2 menaruh tag **sebelum** build, sedangkan keputusan plan menaruh
`git tag` **sesudah** `make release`.

**Perbaikan (opsi A — guard tidak diubah, tag tetap manual)**:

| File                        | Aksi | Isi                                                                                  | Effort |
| --------------------------- | ---- | ------------------------------------------------------------------------------------ | ------ |
| `scripts/release-tag-status.sh` | baru | Status tag: belum ada / ada tapi bukan di HEAD / OK — selalu exit 0 (info, bukan guard) | small  |
| `Makefile`                  | ubah | Target `check-release-tag` sebagai prereq **sebelum** `build-spa` + dipanggil lagi di ringkasan akhir | small  |
| `docs/guides/releasing.md`  | ubah | §2: urutan tag ↔ build direkonsiliasi (dua-duanya sah, syaratnya tag di commit yang dibangun) | small  |

**Keputusan**: gate tag tetap manual dan `release` tetap tidak menyentuh git ref;
yang diperbaiki hanya **visibilitas** (status tag terbaca sebelum build mahal)
dan **konsistensi dokumen**. Opsi yang ditolak: `release` auto-create tag (menyimpang
dari §Keputusan di atas) dan `release-upload` push tag sendiri (mutasi publik
diam-diam di dalam target upload).

**Verifikasi**: `./scripts/release-tag-status.sh` untuk tiga kasus (tag di HEAD,
tag belum ada, tag di commit lain) + versi kosong; `bash -n` script; `make -n release`.
Changelog `docs_internal/changelog/2026-09-18-002-visibilitas-langkah-tag-rilis.md`.
