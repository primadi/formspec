# Plan — `formspec spa install` + Demote `go install` ke CLI-only

Status: **Implemented** — fase 1–4 selesai 2026-09-10 (changelog
`2026-09-10-013-formspec-spa-install.md`)
Tanggal: 2026-09-10
LoE: Medium
Referensi: `docs/guides/install.md`, `docs/guides/releasing.md`,
`docs_internal/changelog/2026-09-10-012-untrack-embedded-spa-build-tag.md`

---

## Latar Belakang

Sejak `spa_stub` (changelog 012), binary hasil `go install` berjalan penuh
sebagai CLI tapi tanpa embedded SPA — UI menampilkan placeholder. Gap yang
tersisa: user binary `go install` di luar repo tidak punya jalur mudah untuk
mendapat UI lengkap (harus installer atau clone repo). Solusi yang disepakati:

1. **Jangan hapus `go install`** — jalurnya valid untuk CLI/Go developer.
   Masalahnya posisi di docs; demote ke "CLI-only".
2. **`formspec spa install`** — command eksplisit untuk download SPA
   artifact dari GitHub Releases **versi yang sama dengan binary** (bukan
   `latest`), verify checksum, cache lokal, dan dipakai oleh `formspec dev`
   sebagai fallback sebelum embedded FS.

Prinsip desain:

- **Eksplisit, tidak silent-magic** — download hanya saat user menjalankan
  `formspec spa install`, bukan saat `formspec dev` jalan (security by
  default).
- **Version-locked** — SPA artifact harus dari tag yang sama dengan binary
  (`formspec version`); tidak ada mismatch.
- **Cache + offline-first** — sekali install, dipakai ulang; tanpa jaringan
  tetap jalan; placeholder + pesan jelas bila belum install.
- **Reuse pipeline release** — artifact di-upload oleh `make release-upload`
  yang sudah ada; checksum via `SHA256SUMS.txt` yang sudah di-generate.

---

## Fase 1 — Artifact SPA di release (small)

File yang diubah: `Makefile`

- [ ] 1.1. Di target `release`, setelah packaging binary, tambahkan artifact
      `spa-<version>.tar.gz`: rename isi `cmd/formspec/dist/` → `spa/`
      (layout: `spa/index.html`, `spa/assets/*`, ...) + `spa/manifest.json`
      berisi `{"version": "<VERSION>", "builtin": "<hash-or-empty>"}`.
- [ ] 1.2. Pastikan `spa-*.tar.gz` ikut masuk `SHA256SUMS.txt` (sudah otomatis
      bila ada di `$(RELEASE_DIR)` saat `shasum` dijalankan).
- [ ] 1.3. `make release-upload` — tidak perlu diubah (meng-upload semua file
      di `$(RELEASE_DIR)`).

Verifikasi: `make release VERSION=test && tar -tzf dist/release/spa-test.tar.gz`

- checksum masuk SHA256SUMS.

## Fase 2 — Command `formspec spa` (medium)

File baru:

- `cmd/formspec/spa.go` — subcommand `formspec spa install|path|remove`.

Perilaku:

- [ ] 2.1. `formspec spa install` - Resolve versi target: `main.version` (bila `dev`, fatal dengan pesan
      jelas — dev build harus pakai auto-detect repo). - Download
      `https://github.com/primadi/formspec/releases/download/<ver>/spa-<ver>.tar.gz` + `SHA256SUMS.txt`. - Verify checksum (baris `spa-<ver>.tar.gz`), lalu extract ke
      `~/.formspec/spa/<ver>/` (os.UserHomeDir, bukan git-based path). - Idempotent: bila sudah ada + checksum OK → print "already installed". - Progress & error message jelas (404 → "release <ver> belum punya
      spa artifact").
- [ ] 2.2. `formspec spa path` — print path cache versi binary (untuk
      scripting / `--web-dir $(formspec spa path)`).
- [ ] 2.3. `formspec spa remove [--all]` — hapus cache versi ini / semua.
- [ ] 2.4. Semua operasi via `net/http` standar + `os` — tanpa dependency baru.
      Download tidak pernah berjalan di background; no auto-update.

## Fase 3 — Wiring fallback chain di `formspec dev` (small)

File yang diubah: `cmd/formspec/dev.go`

- [ ] 3.1. Urutan SPA resolution menjadi: 1. `--dev-ui` (Vite, repo) — tidak berubah 2. `--web-dir` — tidak berubah 3. auto-detect `renderers/react-shadcn/dist` (repo) — tidak berubah 4. **BARU**: cache `~/.formspec/spa/<version>/` (hasil `spa install`),
      hanya jika versinya == `main.version` 5. embedded FS (`spa_embed.go` / placeholder `spa_stub.go`) — tidak
      berubah
- [ ] 3.2. Log tiap sumber yang terpakai: `SPA from: <sumber>` — termasuk
      `~/.formspec/spa cache`.
- [ ] 3.3. Placeholder stub page: ganti daftar opsi, opsi pertama jadi
      `formspec spa install` (file: `cmd/formspec/spa_stub/index.html`,
      `cmd/formspec-registry/web/stub/index.html`).

Verifikasi: `go build ./cmd/formspec && go test ./cmd/formspec`; simulasi
manual: install → dev tanpa flag → UI dari cache; hapus cache → kembali ke
placeholder.

## Fase 4 — Docs & DX (small)

- [ ] 4.1. `docs/guides/install.md` — demote Metode 2 `go install`:
      heading "CLI-only (Go developer)", note UI lengkap = installer /
      `formspec spa install`.
- [ ] 4.2. `docs/guides/install.md` / `docs/cli-tools/` — dokumentasi
      `formspec spa install|path|remove` (jika ada file CLI reference,
      tambah di sana; verifikasi struktur `docs/cli-tools/`).
- [ ] 4.3. `scripts/git-push-and-tag.sh` — tidak berubah (spa artifact ikut
      alur release yang ada). Verifikasi saja end-to-end pada release
      berikutnya.
- [ ] 4.4. Update `docs_internal/plan/install-page-plan.md` §Excluded bila
      menyinggung deferred CI workflow — catat keterkaitan.

## Dependensi antar fase

```
F1 (artifact) ──→ F2 (command download) ──→ F3 (fallback wiring) ──→ F4 (docs)
```

F3 bisa dikerjakan paralel dengan F2 (butuh hanya path convention),
tapi verifikasi end-to-end menunggu F1+F2.

## Risks / catatan desain

- **Versi dev binary**: `formspec version` = "dev" → `spa install` fatal
  dengan pesan "gunakan auto-detect repo atau build via make" — hindari
  ambiguity download "dev" release.
- **Platform**: spa artifact platform-agnostic (JS build) — satu artifact
  untuk semua OS/arch; nama file `spa-<ver>.tar.gz` tanpa os/arch.
- **Disk**: cache per-versi bisa menumpuk → `spa remove --all`; tidak ada
  auto-prune (eksplisit saja).
- **Proxy/enterprise**: user dengan jaringan terbatas bisa set
  `FORMSPEC_SPA_URL` base override (opsional, fase 2 stretch).
