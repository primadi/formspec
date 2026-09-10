#!/usr/bin/env bash
#
# git-push-and-tag.sh — jalur cepat release FormSpec dalam satu perintah.
#
# Menggabungkan langkah 2–4 dari docs/guides/releasing.md:
#   1. Validasi: semver, working tree bersih, tag belum dipakai (lokal & remote)
#   2. git tag <VERSION> + git push origin main --tags
#   3. make release VERSION=<VERSION>        (cross-compile 6 target + packaging)
#   4. make release-upload VERSION=<VERSION> (draft release via gh → review → Publish)
#
# Setelah selesai, release masih DRAFT. Review di halaman Releases lalu klik
# Publish — installer user hanya melihat release yang sudah published.
#
# Contoh:
#   scripts/git-push-and-tag.sh v0.0.2              # jalur normal
#   scripts/git-push-and-tag.sh v0.0.2 --skip-tests # lewati go test (harus sudah hijau)
#
# Semua guard yang sama tetap berlaku — "satu tag = satu release" di-enforce
# oleh Makefile guard `release-upload`. Prosedur manual & rollback tetap di
# docs/guides/releasing.md.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

usage() {
  echo "usage: $0 <vX.Y.Z> [--skip-tests]" >&2
  echo "  contoh: $0 v0.0.2" >&2
  exit 1
}

VERSION=""
SKIP_TESTS=false
for arg in "$@"; do
  case "$arg" in
    --skip-tests) SKIP_TESTS=true ;;
    -h|--help) usage ;;
    -*) echo "❌ Flag tidak dikenal: $arg" >&2; usage ;;
    *)
      if [ -n "$VERSION" ]; then echo "❌ VERSION diberikan dua kali ('${VERSION}' dan '${arg}')" >&2; usage; fi
      VERSION="$arg"
      ;;
  esac
done

# --- Validasi (fail cepat sebelum menyentuh apapun) ---------------------------
if [ -z "$VERSION" ]; then usage; fi
printf '%s' "$VERSION" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$' || {
  echo "❌ VERSION='$VERSION' bukan semver (format: v<major>.<minor>.patch>)" >&2
  exit 1
}

if [ -n "$(git status --porcelain)" ]; then
  echo "❌ Working tree tidak bersih — commit dulu semua perubahan." >&2
  echo "   Tag harus menunjuk commit yang ter-push; source belum masuk tag = artifact tidak konsisten dengan repo." >&2
  git status --short | head -10 >&2
  exit 1
fi

if git rev-parse "${VERSION}^{commit}" >/dev/null 2>&1; then
  echo "❌ Tag $VERSION sudah ada lokal — satu tag = satu release. Pakai versi baru." >&2
  exit 1
fi
if git ls-remote --tags origin | grep -q "refs/tags/${VERSION}$"; then
  echo "❌ Tag $VERSION sudah ada di remote — satu tag = satu release. Pakai versi baru." >&2
  exit 1
fi

command -v gh >/dev/null 2>&1 || {
  echo "❌ 'gh' CLI tidak ditemukan — ikuti prosedur manual di docs/guides/releasing.md §4" >&2
  exit 1
}
gh auth status >/dev/null 2>&1 || {
  echo "❌ gh belum ter-auth — jalankan 'gh auth login' dulu (lihat docs/guides/releasing.md)" >&2
  exit 1
}

BRANCH="$(git branch --show-current)"
if [ "$BRANCH" != "main" ]; then
  echo "⚠️  Bukan di branch main (sekarang: '$BRANCH'). Tag rilis harus menunjuk main." >&2
  exit 1
fi

echo "▶️  Release $VERSION dari branch $BRANCH"

# --- Langkah 1: test -----------------------------------------------------------
if [ "$SKIP_TESTS" = false ]; then
  echo "🧪 go test ./..."
  go test ./...
else
  echo "⏭️  --skip-tests: melewati go test (pastikan sudah dijalankan manual)"
fi

# --- Langkah 2: tag + push -----------------------------------------------------
echo "🏷️  Tag $VERSION + push origin $BRANCH --tags"
git tag "$VERSION"
git push origin "$BRANCH" --tags

# --- Langkah 3: build semua artifact -------------------------------------------
echo "🏗️  make release VERSION=$VERSION"
make release VERSION="$VERSION"

# --- Langkah 4: upload draft release -------------------------------------------
echo "📦 make release-upload VERSION=$VERSION"
make release-upload VERSION="$VERSION"

echo
echo "✅ Draft release $VERSION dibuat."
echo "   → Review & Publish: $(gh repo view --json url --jq .url)/releases"
echo "   → Setelah publish, verifikasi: curl -fsSL https://formspec.dev/install.sh | sh && formspec version"
