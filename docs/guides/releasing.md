# Releasing FormSpec

Prosedur manual untuk maintainer memproduksi dan mempublikasikan release CLI
`formspec` (binary multi-OS). Panduan untuk **mengunduh** binary ada di
[install.md](install.md).

## Prasyarat (di mesin release)

| Tool                               | Kegunaan                                                           |
| ---------------------------------- | ------------------------------------------------------------------ |
| Go ≥ 1.26                          | Compile binary `formspec`                                          |
| Node.js + npm                      | Build SPA yang di-embed ke binary                                  |
| `gh` CLI (ter-auth)                | Upload artifact ke GitHub Releases — opsional, bisa manual via web |
| `tar`, `zip`, `shasum`/`sha256sum` | Packaging + checksums                                              |

```bash
gh auth status   # pastikan sudah login ke github.com
```

## Cara cepat — script satu perintah

Langkah 2–4 di bawah ini (tag → push → build → upload draft) bisa digabung
dalam satu perintah:

```bash
scripts/git-push-and-tag.sh v0.4.2                # test + tag + push + build + upload draft
scripts/git-push-and-tag.sh v0.4.2 --skip-tests   # lewati go test (harus sudah dijalankan manual)
```

Sebelum tagging, script juga otomatis menyinkronkan versi contoh yang hardcoded
di `site/src/components/Install.tsx` (bagian Install di formspec.dev) dan
`docs/guides/install.md` — versi lama di-replace ke versi release lalu
di-commit, sehingga tag selalu berisi site/docs dengan versi yang benar.
Installer (`install.sh`/`install.ps1`) sendiri tidak perlu diubah karena
resolve versi terbaru via GitHub API saat runtime.

Setelah itu script menjalankan **preflight generated artifacts**: regenerate
`make generate-schema` + `make generate-kind-docs` lalu fail-fast bila
`schemas/` atau `docs/kind/` berubah (artefak basi — biasanya `pkg/spec` baru
diubah tapi generator lupa dijalankan). Bila preflight gagal, hasil regenerate
dibiarkan di tree — commit dulu, lalu jalankan ulang script.

Script yang sama menerapkan semua guard prosedur manual sebelum menyentuh
apapun: `VERSION` harus semver, working tree harus bersih, tag belum dipakai
(lokal & remote), `gh` ter-auth, dan sedang di branch `main`. Selain itu,
`VERSION` harus **lebih tinggi** dari tag tertinggi yang sudah ada — versi
tidak boleh mundur, karena GitHub menentukan release "latest" berdasarkan
yang terakhir di-publish (bukan semver tertinggi); release semver lebih rendah
akan membuat installer men-downgrade user. Output akhirnya adalah **draft
release** — tetap harus di-review lalu Publish (langkah yang sama seperti §4).

> Script memerlukan working tree bersih — commit semua perubahan dulu.
> Untuk kondisi khusus (mis. release dari commit tertentu, atau setelah gagal
> di tengah jalan), ikuti langkah manual §2–§4 di bawah yang lebih bisa
> diputus per langkah.

## 0. Cek versi — tidak perlu diingat-ingat

Versi yang pernah di-release tersimpan di 3 tempat; versi berikutnya selalu
kelihatan dari sini, bukan dari catatan manual:

```bash
# Tag terakhir yang pernah dibuat (lokal)
git tag -l | sort -V | tail -1

# Tag yang SUDAH ter-push ke remote (ini yang menentukan: satu tag = satu release)
git ls-remote --tags origin | grep -o 'refs/tags/.*' | sort -V | tail -1

# Versi terbaru yang live di GitHub Releases (dipakai installer user)
curl -fsSL https://api.github.com/repos/primadi/formspec/releases/latest \
  | grep tag_name

# Versi yang terpasang di mesin ini
formspec version
```

Catatan: repo tidak menyimpan file `VERSION` — angka versi di-stamp saat build
dari tag (`-ldflags -X main.version=`), jadi tidak ada angka versi yang bisa
stale di source code. `make release-upload` tanpa `VERSION=` pun otomatis pakai
`git describe --tags`, dan guard-nya akan gagal cepat bila tag sudah dipakai
(satu tag = satu release).

> **Status repo saat ini**: belum ada tag semver — satu-satunya tag adalah
> `docs-pre-restructure-2026-07-15` (marker internal restrukturisasi docs,
> bukan rilis). Jadi: belum ada versi yang pernah di-release. **Rilis pertama
> harus memilih tag semver secara sadar** (mis. `v0.4.1`) — jangan jalankan
> `make release-upload` tanpa `VERSION=` sebelum tag semver pertama ada, karena
> `git describe` akan jatuh ke tag marker dan version stamp/URL download jadi
> memakai nama itu. Setelah tag semver pertama dibuat, semua perintah di atas
> otomatis benar (`sort -V` selalu menempatkan `v*` setelah `docs-*`).

## 1. Pastikan state siap rilis

```bash
git checkout main
git pull origin main
go test ./...            # semua hijau
```

## 2. Tentukan & push tag versi

Tag meng-embed ke URL download (`.../download/<tag>/formspec-<os>-<arch>.tar.gz`),
sehingga **satu tag = satu release** dan tidak bisa dipakai ulang:

```bash
git tag v0.4.2
git push origin main --tags
```

## 3. Build semua artifact

```bash
make release VERSION=v0.4.2
```

Yang dilakukan target ini:

1. Build SPA (`build-spa`) sekali — identik untuk semua target
2. Cross-compile `{linux,darwin,windows} × {amd64,arm64}` dengan
   `CGO_ENABLED=0 -trimpath` dan versi di-stamp via `-ldflags "-X main.version=..."`
3. Packaging: `tar.gz` (linux/darwin), `zip` (windows), + `SHA256SUMS.txt`

Output: `dist/release/`. Verifikasi cepat sebelum upload:

```bash
cd dist/release
shasum -a 256 -c SHA256SUMS.txt
ls -lh                # semua arsip harus besar (belasan–dua puluh MB); ukuran
                      # < 1 MB berarti binary di dalamnya kosong — jangan upload
tar -xzf formspec-darwin-arm64.tar.gz -C /tmp && /tmp/formspec version   # → formspec v0.4.2
```

> Catatan: `SHA256SUMS.txt` di-generate dari file yang ada — checksum "OK"
> hanya membuktikan konsistensi, bukan bahwa artifact valid. Ukuran file dan
> cek `formspec version` yang mendeteksi artifact rusak. Makefile guard
> `release` juga fail-fast bila binary hasil build kosong.

## 4. Upload ke GitHub Releases

```bash
make release-upload VERSION=v0.4.2
```

Membuat **draft** release dengan semua artifact + `SHA256SUMS.txt` + generated
notes. Review di halaman Releases (urutan, notes, checksum), lalu klik
**Publish**.

Guard `release-upload` mengecek apakah **release** dengan tag tersebut sudah
ada di GitHub (bukan apakah tag sudah di-push — push tag di langkah 2 memang
mendahului upload). Jadi tag yang sudah di-push tapi belum punya release tetap
bisa di-upload ulang; yang diblokir adalah membuat release kedua untuk tag yang
sama.

Tanpa `gh`: upload `dist/release/*` manual di
`https://github.com/primadi/formspec/releases/new` — pilih tag di langkah 2.

## 5. Verifikasi pasca-publish (public sanity check)

```bash
curl -fsSL https://formspec.dev/install.sh | sh     # default → latest
formspec version                                     # → formspec v0.4.2
```

Installer meresolve `releases/latest`, jadi publish release menentukan versi
yang di-install user baru.

## Rollback rilis

Tag **published** tidak bisa dipakai ulang — installer user bisa saja sudah
mengunduh artifact dari tag tersebut (URL download meng-embed tag), sehingga
retag membuat artifact yang sudah disebar tidak lagi cocok. Untuk memutar versi
user kembali:

1. Rilis patch baru `v0.4.3`, atau
2. Minta user jalankan installer ulang dengan versi terdahulu:
   `FORMSPEC_VERSION=v0.4.1 sh -c "$(curl -fsSL https://formspec.dev/install.sh)"`.

## Hapus tag pada release yang masih DRAFT

Berbeda dengan published, tag pada release yang masih **draft** boleh dihapus
dan dipakai ulang — draft tidak bisa diunduh user, jadi tidak ada artifact yang
pernah tersebar dengan stamp versi itu:

```bash
gh release delete v0.0.5 --cleanup-tag --yes   # hapus draft release + tag remote

git tag -d v0.0.5                             # hapus tag lokal
```

Setelah itu tag bisa dibuat ulang menunjuk commit mana pun dan di-upload ulang
(`scripts/git-push-and-tag.sh` / `make release-upload`). Guard "versi tidak
boleh mundur" tetap berlaku terhadap tag published tertinggi — tag yang
dihapus tidak dihitung lagi karena sudah tidak ada di remote.

Catatan: `gh release delete` tanpa `--cleanup-tag` hanya menghapus release-nya;
tag git tetap ada dan harus dihapus terpisah (tag draft release tetap ter-push
ke remote karena dibuat lewat `git push --tags` sebelum upload).

### Kalau release yang dihapus sudah PUBLISHED

Jangan. Konsekuensinya:

1. **URL download tag itu 404** — user yang install dengan
   `FORMSPEC_VERSION=<tag>` gagal. User yang sudah ter-install tidak terdampak
   (binary lokal tetap jalan, stamp versinya tetap).
2. **Label "Latest" berpindah bila yang dihapus adalah latest** — GitHub
   memilih release published yang _terakhir di-publish_ (bukan semver
   tertinggi), jadi user baru bisa ter-downgrade.
3. **Nomor versi jadi tidak bisa dipercaya** — bila tag ikut dihapus dan
   dipakai ulang, dua artifact berbeda pernah beredar dengan stamp versi sama;
   guard `release-upload` juga lolos lagi sehingga upload ulang tidak
   terdeteksi. Anggap nomor versi yang pernah published "terbakar": jika
   artifact-nya bermasalah, hapus tag+release lalu rilis versi **lebih
   tinggi** — jangan pakai ulang nomornya.

## Catatan versi di mesin user

- Binary user selalu bernama `formspec` (tanpa versi) di slot yang sama —
  installer upgrade/rollback cukup menimpa.
- Versi aktual dicek via `formspec version`.

## Belum otomatis (deferred)

GitHub Actions workflow yang trigger on tag push (build + release otomatis)
sengaja ditunda — lihat `docs_internal/plan/install-page-plan.md` §Excluded.
Target `make release` di Makefile adalah basis script yang siap diadaptasi.

## Yang sengaja di luar script

Dua pipeline deploy berjalan dengan kadensi tersendiri dan **tidak** diikat ke
release CLI:

- **schemas.formspec.dev** — JSON Schema untuk YAML editor. Generator
  (`make generate-schema`) di-commit ke repo dan diverifikasi fresh oleh
  preflight script; pen-deployan-nya lewat jalur git-based terpisah
  (`make publish-schemas` men-stage `schemas/dist/<version>`, commit, push →
  Cloudflare auto-build — lihat `schemas/README.md`).
- **docs-site** (`docs/`) — auto-build oleh Cloudflare saat main ter-push;
  tidak perlu langkah release khusus.
