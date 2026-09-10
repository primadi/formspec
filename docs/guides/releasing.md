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
tar -xzf formspec-darwin-arm64.tar.gz -C /tmp && /tmp/formspec version   # → formspec v0.4.2
```

## 4. Upload ke GitHub Releases

```bash
make release-upload VERSION=v0.4.2
```

Membuat **draft** release dengan semua artifact + `SHA256SUMS.txt` + generated
notes. Review di halaman Releases (urutan, notes, checksum), lalu klik
**Publish**.

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

Tag tidak bisa dipakai ulang. Untuk memutar versi user kembali:

1. Hapus/retag artifact bila masih draft — atau rilis patch baru `v0.4.3`.
2. Minta user jalankan installer ulang dengan versi terdahulu:
   `FORMSPEC_VERSION=v0.4.1 sh -c "$(curl -fsSL https://formspec.dev/install.sh)"`.

## Catatan versi di mesin user

- Binary user selalu bernama `formspec` (tanpa versi) di slot yang sama —
  installer upgrade/rollback cukup menimpa.
- Versi aktual dicek via `formspec version`.

## Belum otomatis (deferred)

GitHub Actions workflow yang trigger on tag push (build + release otomatis)
sengaja ditunda — lihat `docs_internal/plan/install-page-plan.md` §Excluded.
Target `make release` di Makefile adalah basis script yang siap diadaptasi.
