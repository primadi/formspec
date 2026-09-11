# Install FormSpec

FormSpec didistribusikan sebagai satu binary CLI (`formspec`) untuk
Linux, macOS, dan Windows (amd64 + arm64). Ada 3 cara install — pilih sesuai
kondisi:

| Metode                  | Perlu Go? | Cocok untuk                      |
| ----------------------- | --------- | -------------------------------- |
| **1. Installer script** | ❌        | Semua developer (recommended)    |
| **2. `go install`**     | ✅ ≥ 1.26 | CLI-only (Go developer)          |
| **3. Manual download**  | ❌        | Environment tanpa curl/sandboxed |

Binary yang terpasang selalu bernama **`formspec`** (tanpa versi). Versi yang
terinstall dicek dengan `formspec version`.

---

## Metode 1: Installer (recommended)

Satu perintah — tanpa Go, tanpa sudo/admin:

```bash
# macOS / Linux
curl -fsSL https://formspec.dev/install.sh | sh
```

```powershell
# Windows (PowerShell)
irm https://formspec.dev/install.ps1 | iex
```

Yang dilakukan installer:

1. Deteksi OS dan arsitektur otomatis (amd64/arm64)
2. Download binary dari [GitHub Releases](https://github.com/primadi/formspec/releases)
3. Verifikasi checksum SHA256
4. Pasang di folder user-local:
   - macOS/Linux: `~/.local/bin/formspec`
   - Windows: `%LOCALAPPDATA%\Programs\formspec\formspec.exe`
5. Cek PATH — tampilkan instruksi menambahkannya bila belum ada
6. Verifikasi: `formspec version`

### Versi tertentu (pinned install)

```bash
# macOS / Linux
curl -fsSL https://formspec.dev/install.sh | sh -s -- --version v0.0.4
# atau via env: FORMSPEC_VERSION=v0.0.4 sh -c "$(curl -fsSL https://formspec.dev/install.sh)"
```

```powershell
# Windows
$env:FORMSPEC_VERSION = 'v0.0.4'; irm https://formspec.dev/install.ps1 | iex
```

### Catatan shell sandbox

Bila kebijakan sekuriti melarang `curl | sh`, download installer dulu lalu
jalankan sebagai file:

```bash
curl -fsSL https://formspec.dev/install.sh -o install.sh
sh install.sh
```

---

## Metode 2: `go install` (CLI-only)

Untuk developer yang sudah punya Go ≥ 1.26 dan bekerja di level CLI/impl:

```bash
go install github.com/primadi/formspec/cmd/formspec@latest
```

Catatan: `go install` mendownload seluruh dependency dan meng-compile dari
source — pertama kali butuh beberapa menit. Jalur ini memasang binary ke
`$(go env GOPATH)/bin` — pastikan folder itu ada di PATH.

### Batasan: tanpa embedded UI

`go install` hanya menjalankan compiler Go — tidak bisa menjalankan npm —
sehingga binary ini **tidak memuat embedded SPA**. Semua perintah CLI tetap
lengkap (`apply`, `validate`, `check`, `diff`, dll.), tapi UI menampilkan
placeholder. Untuk UI lengkap:

1. **`formspec spa install`** (recommended) — download & cache SPA artifact
   dari GitHub Releases versi yang sama dengan binary (checksum terverifikasi
   terhadap `SHA256SUMS.txt`), lalu `formspec dev` memakainya otomatis.
2. Installer (Metode 1) — binary release sudah embed SPA penuh.
3. Dari repo checkout: `make build` / `--dev-ui` / `--web-dir`.

### `formspec spa` — UI untuk binary tanpa embedded SPA

```bash
formspec spa install    # download spa-<versi-binary>.tar.gz + verifikasi SHA256
formspec spa path       # path cache (untuk scripting / --web-dir)
formspec spa remove     # hapus cache versi ini (--all: semua versi)
```

- Cache: `~/.formspec/spa/<versi>/`. Idempotent — `--force` untuk download ulang.
- Download selalu dari release **dengan versi yang sama** dengan binary
  (`formspec version`), bukan `latest` — mencegah mismatch SPA ↔ CLI.
- Binary build `dev` (go run / tanpa ldflags) akan menolak `spa install` —
  pakai auto-detect repo atau `--dev-ui`.
- Base URL bisa dioverride via `FORMSPEC_SPA_URL` (proxy/enterprise).

---

## Metode 3: Manual download

Semua binary ada di
[GitHub Releases](https://github.com/primadi/formspec/releases) beserta
`SHA256SUMS.txt`:

| OS      | Arch  | File                                           |
| ------- | ----- | ---------------------------------------------- |
| linux   | amd64 | `formspec-linux-amd64.tar.gz`                  |
| linux   | arm64 | `formspec-linux-arm64.tar.gz`                  |
| darwin  | amd64 | `formspec-darwin-amd64.tar.gz` (Intel Mac)     |
| darwin  | arm64 | `formspec-darwin-arm64.tar.gz` (Apple Silicon) |
| windows | amd64 | `formspec-windows-amd64.zip`                   |
| windows | arm64 | `formspec-windows-arm64.zip`                   |

### macOS / Linux

```bash
# contoh: darwin arm64 (Apple Silicon)
curl -fsSL -o formspec.tar.gz \
  https://github.com/primadi/formspec/releases/download/v0.0.4/formspec-darwin-arm64.tar.gz
tar -xzf formspec-darwin-arm64.tar.gz
mkdir -p ~/.local/bin && mv formspec ~/.local/bin/
```

Pastikan `~/.local/bin` (atau folder lain pilihan Anda) ada di PATH:

```bash
# bash / zsh
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc   # atau ~/.zshrc
```

### Windows

1. Download `formspec-windows-amd64.zip` (atau `formspec-windows-arm64.zip`
   untuk Windows on ARM) dari GitHub Releases
2. Ekstrak → `formspec.exe`
3. Pindahkan ke `%LOCALAPPDATA%\Programs\formspec\`
4. Tambahkan folder itu ke user PATH: **Settings → System → About → Advanced
   system settings → Environment Variables → Path → Edit → New**
5. Buka terminal baru, jalankan `formspec version`

---

## Verifikasi

```bash
formspec version
# formspec v0.0.4
```

Lanjutkan ke [How to Run](how-to-run.md) untuk menjalankan aplikasi pertama.

---

## Upgrade

Jalankan ulang installer (idempotent — menimpa binary lama di slot yang sama):

```bash
# macOS / Linux
curl -fsSL https://formspec.dev/install.sh | sh

# Windows
irm https://formspec.dev/install.ps1 | iex
```

Atau untuk Metode 2: `go install github.com/primadi/formspec/cmd/formspec@latest` lagi.

## Rollback ke versi sebelumnya

```bash
# macOS / Linux
curl -fsSL https://formspec.dev/install.sh | sh -s -- --version v0.4.0
```

```powershell
$env:FORMSPEC_VERSION = 'v0.4.0'; irm https://formspec.dev/install.ps1 | iex
```

## Uninstall

```bash
# macOS / Linux
rm ~/.local/bin/formspec

# Windows — PowerShell
Remove-Item "$env:LOCALAPPDATA\Programs\formspec\formspec.exe"
# (opsional) hapus folder dari user PATH via Environment Variables
```
