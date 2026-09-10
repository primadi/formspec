#!/bin/sh
# ---------------------------------------------------------------------------
# FormSpec installer (POSIX sh) — macOS & Linux.
#
#   curl -fsSL https://formspec.dev/install.sh | sh
#
# Opsi:
#   sh install.sh                       # install versi terbaru (stable)
#   sh install.sh --version v0.0.3     # install versi tertentu
#   FORMSPEC_VERSION=v0.0.3 sh install.sh
#
# Perilaku:
#   - Tanpa sudo: binary dipasang di ~/.local/bin/formspec (user-local).
#   - Deteksi OS/arch otomatis; download dari GitHub Releases.
#   - Kalau ~/.local/bin belum ada di PATH, tampilkan instruksi menambahkannya.
#   - Idempotent: jalankan ulang = upgrade/overwrite versi yang sama slot.
# ---------------------------------------------------------------------------
set -eu

REPO="primadi/formspec"
BIN_NAME="formspec"
INSTALL_DIR="${FORMSPEC_INSTALL_DIR:-$HOME/.local/bin}"

log()  { printf '\033[1;32m✓\033[0m %s\n' "$1"; }
warn() { printf '\033[1;33m!\033[0m %s\n' "$1"; }
fail() { printf '\033[1;31m✗ %s\033[0m\n' "$1" >&2; exit 1; }

# --- Argumen sederhana -------------------------------------------------------
VERSION="${FORMSPEC_VERSION:-}"
while [ $# -gt 0 ]; do
  case "$1" in
    --version) VERSION="$2"; shift 2 ;;
    --version=*) VERSION="${1#--version=}"; shift ;;
    *) fail "Argumen tidak dikenal: $1 (gunakan --version <tag>)" ;;
  esac
done

# --- Deteksi OS --------------------------------------------------------------
case "$(uname -s)" in
  Linux*) OS=linux ;;
  Darwin*) OS=darwin ;;
  *) fail "OS tidak didukung oleh installer ini: $(uname -s). Lihat https://docs.formspec.dev/guides/install.html" ;;
esac

# --- Deteksi Arch ------------------------------------------------------------
case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) fail "Arsitektur tidak didukung: $(uname -m)" ;;
esac

# --- Download helper (curl atau wget) -----------------------------------------
fetch() {
  _url="$1"; _out="$2"
  if command -v curl >/dev/null 2>&1; then
    if [ "$_out" = "-" ]; then curl -fsSL "$_url"; else curl -fsSL "$_url" -o "$_out"; fi
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$_out" "$_url"
  else
    fail "curl atau wget diperlukan untuk download."
  fi
}

# --- Resolusi versi -----------------------------------------------------------
if [ -z "$VERSION" ]; then
  log "Mencari versi terbaru..."
  RELEASE_JSON=$(fetch "https://api.github.com/repos/${REPO}/releases/latest" - || true)
  VERSION=$(printf '%s' "$RELEASE_JSON" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)
  [ -n "$VERSION" ] || fail "Gagal menentukan versi terbaru (cek koneksi / rate-limit GitHub). Coba: sh install.sh --version vX.Y.Z"
fi
log "Versi: ${VERSION}"

# --- Download arsip binary ------------------------------------------------------
# Override untuk testing: FORMSPEC_RELEASE_BASE=file:///path/ke/dist/release
RELEASE_BASE="${FORMSPEC_RELEASE_BASE:-https://github.com/${REPO}/releases/download}"
ARCHIVE="formspec-${OS}-${ARCH}.tar.gz"
URL="${RELEASE_BASE}/${VERSION}/${ARCHIVE}"
TMPDIR_DL=$(mktemp -d)
trap 'rm -rf "$TMPDIR_DL"' EXIT

log "Mengunduh ${ARCHIVE}..."
fetch "$URL" "${TMPDIR_DL}/${ARCHIVE}" || fail "Download gagal: $URL (pastikan tag ${VERSION} punya artifact ${ARCHIVE})"

# --- Verifikasi checksum bila SHA256SUMS tersedia --------------------------------
if fetch "${RELEASE_BASE}/${VERSION}/SHA256SUMS.txt" "${TMPDIR_DL}/SHA256SUMS.txt" 2>/dev/null; then
  EXPECTED=$(grep " ${ARCHIVE}\$" "${TMPDIR_DL}/SHA256SUMS.txt" | awk '{print $1}')
  if [ -n "$EXPECTED" ]; then
    if command -v shasum >/dev/null 2>&1; then
      ACTUAL=$(shasum -a 256 "${TMPDIR_DL}/${ARCHIVE}" | awk '{print $1}')
    else
      ACTUAL=$(sha256sum "${TMPDIR_DL}/${ARCHIVE}" | awk '{print $1}')
    fi
    [ "$ACTUAL" = "$EXPECTED" ] || fail "Checksum tidak cocok!\n  expected: $EXPECTED\n  actual:   $ACTUAL"
    log "Checksum SHA256 OK"
  fi
fi

# --- Ekstrak + install ----------------------------------------------------------
tar -xzf "${TMPDIR_DL}/${ARCHIVE}" -C "$TMPDIR_DL"
[ -f "${TMPDIR_DL}/${BIN_NAME}" ] || fail "Arsip tidak berisi ${BIN_NAME}"

mkdir -p "$INSTALL_DIR"
mv "${TMPDIR_DL}/${BIN_NAME}" "${INSTALL_DIR}/${BIN_NAME}"
chmod +x "${INSTALL_DIR}/${BIN_NAME}"
log "Terpasang di ${INSTALL_DIR}/${BIN_NAME}"

# --- Cek PATH ---------------------------------------------------------------------
case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) ;;
  *) warn "${INSTALL_DIR} belum ada di PATH."
     printf '\nTambahkan ke shell rc Anda, lalu buka terminal baru:\n\n'
     if [ -f "$HOME/.zshrc" ]; then
       printf '  echo "export PATH=\\"$HOME/.local/bin:\$PATH\\"" >> ~/.zshrc\n'
     elif [ -d "$HOME/.config/fish" ]; then
       printf '  fish_add_path ~/.local/bin\n'
     else
       printf '  echo "export PATH=\\"$HOME/.local/bin:\$PATH\\"" >> ~/.bashrc\n'
     fi
     printf '\nAtau untuk sesi ini saja:\n  export PATH="$HOME/.local/bin:$PATH"\n' ;;
esac

# --- Verifikasi -------------------------------------------------------------------
if "${INSTALL_DIR}/${BIN_NAME}" version >/dev/null 2>&1; then
  log "Verifikasi: $(${INSTALL_DIR}/${BIN_NAME} version)"
else
  warn "Tidak bisa menjalankan ${INSTALL_DIR}/${BIN_NAME} version — coba buka terminal baru lalu: formspec version"
fi

printf '\nNext steps: https://docs.formspec.dev/guides/install.html\n'
