#!/usr/bin/env bash
#
# git-push-and-tag.sh — jalur cepat release FormSpec dalam satu perintah.
#
# Menggabungkan langkah 2–4 dari docs/guides/releasing.md:
#   0. Sinkronisasi versi contoh di docs + site (Install.tsx, install.md, dsb.)
#      — auto replace versi lama → VERSION, commit, lalu tag menunjuk commit itu
#   1. Validasi: semver, working tree bersih, tag belum dipakai (lokal & remote)
#   2. git tag <VERSION> + git push origin main --tags
#   3. make release VERSION=<VERSION>        (cross-compile 6 target + packaging)
#   4. make release-upload VERSION=<VERSION> (draft release via gh → review → Publish)
#
# Setelah selesai, release masih DRAFT. Review di halaman Releases lalu klik
# Publish — installer user hanya melihat release yang sudah published.
#
# Catatan: file install.sh/install.ps1 sendiri resolve versi terbaru via GitHub
# API saat runtime — yang perlu di-replace hanya teks contoh/preview hardcoded.
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

BRANCH="$(git branch --show-current)"
if [ "$BRANCH" != "main" ]; then
  echo "⚠️  Bukan di branch main (sekarang: '$BRANCH'). Tag rilis harus menunjuk main." >&2
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

# Guard versi mundur: VERSION harus lebih tinggi dari tag tertinggi yang ada.
# Tanpa ini, release semver lebih rendah (mis. v0.0.10 setelah v0.1.0) tetap
# terbuat dan — karena GitHub 'latest' = release terakhir di-publish — installer
# akan men-downgrade user.
LATEST_TAG="$(git tag --sort=-v:refname | head -n 1)"
if [ -n "$LATEST_TAG" ]; then
  HIGHEST="$(printf '%s\n%s\n' "$LATEST_TAG" "$VERSION" | sort -V | tail -n 1)"
  if [ "$HIGHEST" = "$LATEST_TAG" ]; then
    echo "❌ VERSION=$VERSION tidak lebih tinggi dari tag terakhir $LATEST_TAG — versi tidak boleh mundur." >&2
    echo "   Pilih versi yang lebih tinggi, mis. bump patch/minor dari $LATEST_TAG." >&2
    exit 1
  fi
fi

command -v gh >/dev/null 2>&1 || {
  echo "❌ 'gh' CLI tidak ditemukan — ikuti prosedur manual di docs/guides/releasing.md §4" >&2
  exit 1
}
gh auth status >/dev/null 2>&1 || {
  echo "❌ gh belum ter-auth — jalankan 'gh auth login' dulu (lihat docs/guides/releasing.md)" >&2
  exit 1
}

echo "▶️  Release $VERSION dari branch $BRANCH"

# --- Langkah 0: sinkronisasi versi contoh di docs + site ---------------------
# File berisi contoh versi installer yang hardcoded (preview UI & docs).
# Diganti + di-commit SEBELUM tag agar formspec.dev tidak menampilkan versi lama.
VERSION_REF_FILES=(
  "site/src/components/Install.tsx"
  "docs/guides/install.md"
  "site/public/install.sh"
  "site/public/install.ps1"
)

sync_version_refs() {
  local old_version
  old_version="$(grep -hoE 'v[0-9]+\.[0-9]+\.[0-9]+' "site/src/components/Install.tsx" | head -n 1)"
  if [ -z "$old_version" ]; then
    echo "❌ Tidak bisa mendeteksi versi contoh di site/src/components/Install.tsx" >&2
    exit 1
  fi
  if [ "$old_version" = "$VERSION" ]; then
    echo "🔖 Versi contoh sudah $VERSION — tidak ada yang perlu diganti"
    return
  fi
  echo "🔖 Ganti versi contoh $old_version → $VERSION di docs + site"
  local f changed=()
  for f in "${VERSION_REF_FILES[@]}"; do
    if [ -f "$f" ]; then
      # -i.bak + rm: portable untuk GNU sed (Linux) dan BSD sed (macOS)
      sed -i.bak "s/${old_version//./\\.}/${VERSION//./\\.}/g" "$f"
      rm -f "$f.bak"
      changed+=("$f")
    else
      echo "⚠️  File versi-ref tidak ditemukan (skip): $f" >&2
    fi
  done
  git add "${changed[@]}"
  git commit -m "chore: bump contoh versi installer ${old_version} → ${VERSION} (docs + site)"
}
sync_version_refs

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
