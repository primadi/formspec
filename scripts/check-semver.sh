#!/usr/bin/env bash
#
# check-semver.sh — guard versi untuk target rilis Makefile (`release`,
# `release-upload`). Hanya semver murni `v<major>.<minor>.<patch>` yang lolos.
#
# Kenapa strict: `VERSION` bocor dari `git describe --tags` menghasilkan string
# seperti `v0.0.8-4-gceaaf2a`. String itu BUKAN versi rilis — `formspec upgrade`
# (komparator semver) membacanya sebagai *prerelease dari v0.0.8*, sehingga
# release yang isinya 4 commit lebih baru tampak sebagai rollback ke user.
# Kejadian nyata: rilis `v0.0.8-4-gceaaf2a` (2026-09-17) —
# docs_internal/plan/release-version-auto.md.
#
# Usage: scripts/check-semver.sh <versi>
set -euo pipefail

v="${1:-}"

if [ -z "$v" ]; then
  echo "❌ Versi rilis kosong — VERSION= tidak diisi dan tidak ada tag semver di repo." >&2
  echo "   Tentukan eksplisit: make release VERSION=v0.1.0" >&2
  exit 1
fi

if printf '%s' "$v" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$'; then
  exit 0
fi

echo "❌ VERSION='$v' bukan semver murni (butuh v<major>.<minor>.<patch>)." >&2
if printf '%s' "$v" | grep -Eq -- '-[0-9]+-g[0-9a-f]+(-dirty)?$'; then
  echo "   Itu string git-describe, bukan versi rilis. Tag describe tidak boleh di-publish:" >&2
  echo "   'formspec upgrade' membacanya sebagai prerelease v0.0.8 → user dapat prompt rollback." >&2
fi
echo "   Pakai semver murni (mis. v0.0.9, atau patch-bump dari tag tertinggi)." >&2
echo "   Di target Makefile rilis, VERSION= boleh dikosongkan agar di-auto-bump." >&2
echo "   Lihat docs/guides/releasing.md §2 & §4." >&2
exit 1
