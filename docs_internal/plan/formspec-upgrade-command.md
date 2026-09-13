# Plan — `formspec upgrade` (self-update)

Status: **Implemented** — fase 1–5 selesai 2026-09-13 (changelog
`2026-09-13-004-formspec-upgrade-command.md`)
Tanggal: 2026-09-13
LoE: Medium
Referensi: `site/public/install.sh`, `site/public/install.ps1`,
`docs/guides/install.md` §Upgrade, `docs/guides/releasing.md`,
`cmd/formspec/spa.go` (pola download+verify+cache),
`docs_internal/plan/install-page-plan.md` (§Excluded — auto-update deferred),
`docs_internal/plan/spa-install-command.md` (prinsip eksplisit, bukan silent-magic)

---

## Latar Belakang

Saat ini upgrade hanya bisa lewat **jalankan ulang installer**:

```bash
curl -fsSL https://formspec.dev/install.sh | sh
```

Itu idempotent (menimpa slot yang sama), tapi punya gap:

1. **Butuh jaringan ke `formspec.dev` + shell pipe** — tidak nyaman saat
   dipanggil dari dalam skrip/CI, dan tertahan di environment yang melarang
   `curl | sh` (lihat `docs/guides/install.md` §Catatan shell sandbox).
2. **Tidak tahu apakah sudah versi terbaru** — user harus cek GitHub Releases
   manual atau download ulang buta-buta.
3. **User `go install` punya jalur berbeda** (`go install ...@latest`) — dua
   jalur upgrade yang harus diingat user.
4. **Tidak ada jalur untuk tahu versi tersedia** tanpa install (tidak ada
   `--check`).

Tujuan: satu perintah self-contained `formspec upgrade` yang tahu binary-nya
sendiri, tahu platform, download artifact resmi dari GitHub Releases, verifikasi
checksum, dan mengganti binary di tempat — **tanpa install ulang manual**.

Prinsip desain (konsisten dengan `spa install`):

- **Eksplisit, bukan silent-magic** — tidak ada auto-update/background check.
  Download hanya saat user menjalankan `formspec upgrade` (security by default).
- **Rilis resmi saja** — HTTPS ke GitHub Releases, wajib verify `SHA256SUMS.txt`
  sebelum eksekusi. Checksum mismatch = abort, binary lama tidak disentuh.
- **Reuse pipeline release yang ada** — artifact `formspec-<os>-<arch>.tar.gz|.zip`
  - `SHA256SUMS.txt` sudah diproduksi `make release`; tidak ada artifact baru.
- **Fail-safe** — tulis ke temp di direktori yang sama, verifikasi binary baru
  bisa jalan, baru swap. Gagal di langkah mana pun sebelum swap = tidak ada
  perubahan.

---

## Keputusan Desain

| Aspek                | Keputusan                                                                                                                              |
| -------------------- | -------------------------------------------------------------------------------------------------------------------------------------- |
| Nama command         | `formspec upgrade` (verb di dispatcher `main.go`, satu binary)                                                                         |
| Sumber versi         | GitHub API `releases/latest`; `--version <tag>` untuk pin/rollback                                                                     |
| Sumber artifact      | `https://github.com/primadi/formspec/releases/download/<tag>/formspec-<os>-<arch>.{tar.gz,zip}` + `SHA256SUMS.txt` dari tag sama       |
| Override URL         | env `FORMSPEC_RELEASE_BASE` (paritas dengan installer script) — enterprise/proxy/mirror                                                |
| Target yang didukung | `{linux,darwin} × {amd64,arm64}` + `{windows} × {amd64,arm64}` (sama dengan matriks release)                                           |
| Lokasi self-replace  | `os.Executable()` + `filepath.EvalSymlinks` (bukan hardcode `~/.local/bin`) — jalur installer, `go install`, dan manual semua jalan    |
| Metode swap          | Download → extract ke temp **di direktori binary** → chmod 0755 → smoke test `version` → rename atomik; Windows pakai rename-ke-`.old` |
| Semver               | Comparator mini in-repo (tanpa dependency baru; `golang.org/x/mod` tidak ada di `go.mod`)                                              |
| Aman bila read-only  | Deteksi write-permission sebelum download; error jelas + fallback perintah installer. **Tidak pernah auto-`sudo`**                     |
| Dev build            | `version == "dev"` → fatal dengan pesan ("build dev tidak punya tag rilis"), sama seperti `spa install`                                |
| SPA cache            | Upgrade sukses → hint `formspec spa install` bila versi lama punya cache SPA; tidak auto-download di Fase 1–4                          |
| Auto-update          | Di luar scope — tetap deferred (lihat `install-page-plan.md` §Excluded)                                                                |

### Surface command

```
formspec upgrade                     # cek latest → upgrade bila lebih baru
formspec upgrade --version v0.0.7    # pin/rollback ke tag tertentu (idempotent)
formspec upgrade --check             # hanya cek & laporkan; tidak menyentuh binary
formspec upgrade --dry-run           # tampilkan rencana (versi, URL, path) tanpa download
formspec upgrade --force             # paksa walau versi sama (re-install binary)
formspec upgrade --yes               # non-interaktif (default: konfirmasi 1 baris bila TTY)
```

Catatan: tanpa `--yes`, konfirmasi hanya muncul saat stdin adalah TTY —
pemanggilan dari skrip/CI tidak akan hang menunggu input.

---

## Fakta Teknis (hasil riset)

- `version` di-stamp `-ldflags "-X main.version=<tag>"` (`cmd/formspec/main.go`);
  build lokal (`go run`/`go build` tanpa tag) = `"dev"`.
- Matriks artifact (`Makefile` `RELEASE_TARGETS`): `formspec-<os>-<arch>.tar.gz`
  (linux/darwin) & `.zip` (windows); `SHA256SUMS.txt` digenerate sekali di
  `$(RELEASE_DIR)`. SPA artifact `spa-<version>.tar.gz` platform-agnostic.
- `cmd/formspec/spa.go` sudah punya primitif yang bisa dipakai ulang:
  `spaHTTPGet` (timeout 5 menit, mapping 404 → pesan "release tidak punya
  artifact") dan `spaChecksumFromSums` (parse baris `<hex>  <name>`).
  Keduanya **namanya SPA-spesifik** → perlu di-generalisasi (Fase 1).
- `cmd/formspec/schema.go` + `internal/schemaregistry/` = pola lain untuk
  "network + cache + subcommand" (env override → config → default).
- Direktori install installer script: `~/.local/bin` (Linux/macOS),
  `%LOCALAPPDATA%\Programs\formspec` (Windows). Tapi **tidak boleh** di-hardcode:
  `go install` menaruh binary di `$(go env GOPATH)/bin`.
- Cache SPA: `~/.formspec/spa/<version>/` (via `os.UserHomeDir`). Cache schema
  version-independent (spec version `v1`) → **tidak terpengaruh upgrade**.
- Windows: `.exe` yang sedang berjalan **tidak bisa di-overwrite/di-delete**,
  tapi **bisa di-rename**. Karena itu strategi swap-nya berbeda dari Unix.
- `go.mod`: tidak ada `golang.org/x/mod` maupun library semver → jangan tambah
  dependency; tulis comparator ~40 baris + test.
- `docs/guides/install.md` §Upgrade saat ini hanya mengarahkan ke installer →
  harus diperbarui agar `formspec upgrade` jadi jalur utama.

---

## Fase & File

### Fase 1 — Ekstraksi helper release (small)

Tujuan: satu tempat untuk "download + verify checksum + extract artifact rilis",
dipakai `spa install` dan `upgrade` — hindari duplikasi logika verifikasi.

File:

- `cmd/formspec/release.go` (baru) — pindahkan/rename dari `spa.go`:
  `releaseGet(url)` (dari `spaHTTPGet`), `checksumFromSums(sums, name)` (dari
  `spaChecksumFromSums`), `verifyArtifact(data []byte, sums string, name string) error`,
  `extractTarGz(data, dest, stripPrefix)`, `extractZip(data, dest)`,
  `assetName(os, arch)` → `formspec-<os>-<arch>.tar.gz|.zip`,
  `releaseBaseURL()` (env `FORMSPEC_RELEASE_BASE` → default GitHub),
  `latestTag()` (GitHub API `releases/latest`, parse `tag_name`).
- `cmd/formspec/release_test.go` (baru) — test `assetName`, `checksumFromSums`,
  `verifyArtifact` (mismatch → error), `extractTarGz`/`extractZip` (path-traversal
  guard), `latestTag` via `httptest`.
- `cmd/formspec/spa.go` — ganti pemanggilan ke helper baru (perilaku tidak berubah;
  `spa install` tetap version-locked ke `main.version`).

Verifikasi: `go build ./cmd/formspec && go test ./cmd/formspec` hijau; `formspec spa install`
masih berperilaku sama (manual/E2E).

### Fase 2 — Semver compare + resolusi target (small)

File:

- `cmd/formspec/semver.go` (baru) — `compareVersions(a, b string) int` untuk
  `vX.Y.Z[-prerelease]`. Aturan minimal & deterministik: bandingkan
  `major/minor/patch` numerik; prerelease < release; prerelease dibanding
  lexically per identifier numerik/alfa (subset semver yang cukup untuk tag
  FormSpec). Tag non-semver → error terkontrol.
- `cmd/formspec/semver_test.go` (baru) — tabel kasus: sama, lebih baru, lebih
  lama, prerelease, `v` prefix hilang, tag invalid.

Verifikasi: `go test ./cmd/formspec -run Semver`.

### Fase 3 — Command `formspec upgrade` (medium)

File:

- `cmd/formspec/upgrade.go` (baru) — `runUpgrade(args []string)`.
- `cmd/formspec/main.go` — tambah `case "upgrade": runUpgrade(os.Args[2:])` +
  baris di `usage()`.

Alur:

- [ ] 3.1. Guard awal: `version == "dev"` → fatal (pesan pola `spa install`).
- [ ] 3.2. Parse flags (`--version`, `--check`, `--dry-run`, `--force`,
      `--yes`) via `flag.NewFlagSet` (pola `schema.go`).
- [ ] 3.3. Resolve target tag: `--version` bila ada; jika tidak → `latestTag()`.
      Bandingkan dengan `main.version`: - sama & bukan `--force` → "sudah versi terbaru" (exit 0) - lebih lama (downgrade) → konfirmasi ekstra/label jelas "rollback bila
      versi lebih lama" (pola `install.sh --version` sudah mendukung rollback)
- [ ] 3.4. `--check` → cetak `current` vs `latest` + exit, tanpa menyentuh apa pun.
- [ ] 3.5. Resolve `exePath` (`os.Executable` + `EvalSymlinks`) dan cek direktori
      writable **sebelum** download (fail cepat, pesan actionable).
- [ ] 3.6. `--dry-run` → cetak tag target, `assetName(runtime.GOOS, runtime.GOARCH)`,
      URL artifact + URL SHA256SUMS, `exePath`, tanpa download.
- [ ] 3.7. Konfirmasi interaktif 1 baris bila TTY dan bukan `--yes`
      (`golang.org/x/term` tidak ada di modul → deteksi TTY via `os.Stdin.Stat()`
      `ModeCharDevice`, tanpa dependency baru).
- [ ] 3.8. Download artifact + `SHA256SUMS.txt`, `verifyArtifact` → mismatch =
      abort (binary lama utuh).
- [ ] 3.9. Extract ke file temp **di direktori `exePath`** (bukan `os.TempDir()`
      — agar rename satu filesystem), chmod 0755.

### Fase 4 — Self-replace atomik + Windows (medium)

File: `cmd/formspec/upgrade.go` (+ `upgrade_windows.go` bila perlu build tag).

- [ ] 4.1. Smoke test binary baru sebelum swap: jalankan `<tmp> version` dan
      cocokkan output dengan tag target. Gagal → abort, hapus temp, binary lama
      tetap jalan.
- [ ] 4.2. Unix: `os.Rename(tmp, exePath)` — atomik; proses yang sedang jalan
      tetap memegang inode lama sampai exit.
- [ ] 4.3. Windows (guard `runtime.GOOS`): `os.Rename(exePath, exePath+".old")`
      → `os.Rename(tmp, exePath)` → `os.Remove(exePath+".old")` (gagal dihapus =
      abaikan, jangan jadi error). Bersihkan sisa `*.old` di awal `upgrade`
      berikutnya.
- [ ] 4.4. `--force` pada versi sama → tetap lakukan download+swap (re-install
      binary, berguna saat binary korup).
- [ ] 4.5. Setelah sukses: cetak `dari <lama> → <baru>` + path; bila
      `~/.formspec/spa/<lama>/` ada → hint `formspec spa install` (SPA cache
      version-locked, belum otomatis di fase ini).
- [ ] 4.6. Test: `upgrade_test.go` — self-replace di `t.TempDir()` dengan binary
      dummy (assert isi + mode `0755` + binary lama tergantikan); helper
      `runtimeOS`/`runtimeArch` dibuat injectable agar jalur Windows bisa diuji
      di Linux (logika rename-`.old` di unit-test terpisah).

Verifikasi: `go test ./cmd/formspec`; E2E manual: install tag lama → `formspec upgrade`
→ `formspec version` == tag baru.

### Fase 5 — Docs, DX & tracing (small)

- [ ] 5.1. `docs/cli-tools/02-formspec-cli.md` §2 Deployment → tambah
      `### formspec upgrade` (sinopsis flag + contoh + exit code).
- [ ] 5.2. `docs/guides/install.md` §Upgrade → `formspec upgrade` jadi jalur
      utama; installer re-run tetap didokumentasikan sebagai fallback
      (pertama kali install, instalasi dikelola package manager, atau
      direktori read-only).
- [ ] 5.3. `docs/guides/releasing.md` → catat kontrak yang harus dijaga:
      nama artifact + `SHA256SUMS.txt` + `releases/latest` adalah **API publik**
      yang dipakai `formspec upgrade` (jangan di-rename tanpa bump major).
- [ ] 5.4. `.github/skills/formspec-cli/SKILL.md` — tambah `upgrade` ke tabel
      Command Status + `cmd/formspec/upgrade.go` + `release.go` ke Key paths.
- [ ] 5.5. `site/src/components/Install.tsx` — tambah catatan singkat
      "upgrade: `formspec upgrade`" di tab installer.
- [ ] 5.6. `docs_internal/plan/todo.md` — task baru (lihat Fase 3.9 di bawah),
      update `Last Updated`.
- [ ] 5.7. `docs_internal/changelog/2026-09-13-NNN-formspec-upgrade-command.md`
      — ditulis saat implementasi (bukan saat plan), sesuai Workflow Discipline.

---

## Dependensi antar fase

```
F1 (helper share) ─┬─→ F3 (command) ─→ F4 (self-replace) ─→ F5 (docs)
F2 (semver)       ─┘
```

F2 bisa paralel dengan F1. F4 bergantung F3 (butuh artifact sudah ter-extract).

---

## Risks / Catatan Desain

- **Instalasi dikelola package manager** (apt/brew/scoop): self-replace adalah
  tindakan salah (paket manager kehilangan jejak). Deteksi tidak mungkin 100%
  tanpa metadata; mitigasi: (a) cek direktori writable, (b) pesan error
  menyebut opsi "install ulang lewat package manager Anda", (c) catat di docs
  bahwa `formspec upgrade` untuk instalasi installer/`go install`/manual.
- **Rollback `--version` lebih lama**: bukan bug, fitur (paritas dengan
  `install.sh --version`). Beri label jelas + konfirmasi.
- **Windows file lock**: rename `.old` bisa gagal dihapus (antivirus/AV scan
  menahan handle) → jangan pernah jadikan hard error; sapu `.old` saat upgrade
  berikutnya.
- **Binary readonly FS / container**: `os.Rename` gagal → pesan error harus
  menyertakan perintah installer sebagai fallback, bukan stack trace.
- **GitHub API rate-limit** (unauth 60/jam): `--version <tag>` tidak memanggil
  API sama sekali (langsung ke URL download) — sediakan sebagai jalan keluar
  saat rate-limited; pesan error `latestTag` harus menyebut ini.
- **Symlink** (mis. `~/.local/bin/formspec` → `/opt/...`): `EvalSymlinks`
  memastikan yang diganti targetnya, bukan symlink-nya.
- **SPA cache mismatch**: binary baru + SPA cache versi lama tidak dipakai
  (`dev.go` match `main.version`) → aman, hanya perlu `spa install` ulang bila
  user pakai binary tanpa embedded SPA (`go install`).
- **Downgrade/forward compatibility data**: upgrade binary tidak menyentuh DB
  atau spec; migrasi schema tetap tanggung jawab user (`formspec migrate`).
  Catat di docs upgrade.

---

## Out of Scope (deferred)

- **Auto-update / background version check** — tetap deferred
  (`install-page-plan.md` §Excluded). `--check` eksplisit saja.
- **Upgrade binary lain** — `formspec-registry`, `formspec-operator` tidak
  termasuk artifact release saat ini → di luar scope.
- **Distribution via brew/scoop/apt** — akan membuat `upgrade` mengikuti
  package manager; defer sampai formula ada.
- **`formspec upgrade --spa`** (auto-install SPA cache baru) — kandidat
  lanjutan setelah Fase 1–5 stabil; untuk sekarang hanya hint.
- **Self-update untuk `formspec-ctl` / Operator** — control-plane phase.

---

## Verifikasi / Test Matrix

| Level       | Cakupan                                                                                                                                                                                                       |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Unit        | semver compare; `assetName` (6 target + unsupported); `checksumFromSums`; `verifyArtifact` mismatch; extract tar.gz/zip + traversal guard; `latestTag` (httptest)                                             |
| Integration | self-replace di `t.TempDir()` (binary dummy): isi, mode, `.old` cleanup; write-deny → error terkontrol                                                                                                        |
| Manual E2E  | (1) install tag lama → `upgrade` → `version` naik; (2) `upgrade --check` tanpa efek; (3) `upgrade --version` rollback; (4) `upgrade` di direktori read-only → pesan fallback; (5) `--dry-run` tidak mengunduh |
| Docs        | `formspec validate` tidak terpengaruh; tidak ada placeholder `wget`/URL lama yang tertinggal di `docs/guides/install.md`                                                                                      |

---

## Checklist Traceability

- Plan ini → todo: `docs_internal/plan/todo.md` Fase 3.9 (`formspec upgrade`).
- Implementasi → changelog: `docs_internal/changelog/2026-09-13-NNN-formspec-upgrade-command.md`.
- Kontrak rilis yang dipakai: `docs/guides/releasing.md`, `Makefile` (`release`, `release-upload`).
