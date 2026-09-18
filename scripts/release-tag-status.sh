#!/usr/bin/env bash
#
# release-tag-status.sh — cetak status tag rilis untuk versi yang akan dibangun.
#
# INFO saja: selalu exit 0. Guard yang sebenarnya (menolak upload tanpa tag) tetap
# di target `release-upload`.
#
# Kenapa ada: `make release` hanya build artifact dan tidak pernah menyentuh git
# ref — tag adalah keputusan sadar maintainer
# (docs_internal/plan/release-version-auto.md §Keputusan). Akibatnya kebutuhan tag
# baru terasa di `release-upload`, yaitu *setelah* SPA build + 6 cross-compile
# selesai: `make release` sukses dengan `v0.0.9`, lalu `make release-upload`
# menolak "Tag v0.0.9 belum ada lokal". Script ini dipanggil `make release`
# SEBELUM build mahal itu (dan sekali lagi di ringkasan akhir), supaya langkah
# berikutnya terbaca di depan, bukan jadi kejutan.
#
# Usage: scripts/release-tag-status.sh <versi>
set -euo pipefail

v="${1:-}"
cd "$(git rev-parse --show-toplevel 2>/dev/null || echo .)"

if [ -z "$v" ]; then
  echo "⚠️  Versi rilis tidak terbaca — status tag tidak bisa diperiksa." >&2
  exit 0
fi

head_sha="$(git rev-parse HEAD 2>/dev/null || echo '')"

# Tag belum ada → build tetap jalan, tapi upload akan menolak.
if ! tag_sha="$(git rev-parse --verify --quiet "refs/tags/${v}^{commit}")"; then
  echo "🏷️  Tag ${v} belum ada."
  echo "    'make release' tidak membuat tag. Setelah build selesai:"
  echo "      git tag ${v} && git push origin ${v}"
  echo "      make release-upload"
  exit 0
fi

# Tag ada tapi bukan di commit yang dibangun → artifact tidak mewakili isi tag.
if [ -n "$head_sha" ] && [ "$tag_sha" != "$head_sha" ]; then
  echo "⚠️  Tag ${v} menunjuk ${tag_sha:0:7}, sedangkan HEAD sekarang ${head_sha:0:7}."
  echo "    Artifact di dist/release/ dibangun dari worktree saat ini, jadi release"
  echo "    yang di-publish tidak akan cocok dengan isi tag. Pindahkan tag ke commit"
  echo "    yang dibangun, atau ulangi build dari commit yang di-tag."
  exit 0
fi

echo "🏷️  Tag ${v} ada di commit yang dibangun → lanjut: make release-upload"
