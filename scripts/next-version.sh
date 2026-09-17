#!/usr/bin/env bash
#
# next-version.sh — hitung versi rilis berikutnya: patch-bump dari tag semver
# tertinggi di repo.
#
#   tag semver tertinggi v0.0.8 → v0.0.9
#   tag semver tertinggi v0.2.9 → v0.2.10
#   tidak ada tag semver        → error (rilis pertama harus dipilih sadar)
#
# Dipakai `make release` / `make release-upload` bila VERSION= tidak diisi —
# lihat docs_internal/plan/release-version-auto.md. Bump minor/major tetap harus
# eksplisit (`make release VERSION=v0.1.0`), karena itu keputusan semantik, bukan
# mekanis.
#
# Hanya tag semver murni yang dihitung: `docs-pre-restructure-…` dan string
# git-describe (`v0.0.8-4-gceaaf2a`) diabaikan — tag describe BUKAN versi rilis.
#
# stdout = versi (dipakai `$(shell …)` di Makefile), penjelasan ke stderr.
set -euo pipefail

cd "$(git rev-parse --show-toplevel 2>/dev/null || echo .)"

latest="$(git tag --list | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | sort -V | tail -n 1 || true)"

if [ -z "$latest" ]; then
  echo "❌ Tidak ada tag semver di repo — versi rilis harus ditentukan eksplisit." >&2
  echo "   Contoh: make release VERSION=v0.1.0 (lihat docs/guides/releasing.md §2)." >&2
  exit 1
fi

core="${latest#v}"
major="${core%%.*}"
rest="${core#*.}"
minor="${rest%%.*}"
patch="${rest##*.}"
next="v${major}.${minor}.$((patch + 1))"

echo "ℹ️  VERSION tidak diisi → auto-bump dari tag semver tertinggi ${latest}: ${next}" >&2
echo "   (bump minor/major eksplisit: make release VERSION=v${major}.$((minor + 1)).0)" >&2

echo "$next"
