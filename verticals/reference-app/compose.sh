#!/usr/bin/env bash
# Regenerates verticals/reference-app/spec/modules/ and the per-app UI-extras
# folders by copying from each independent vertical App under verticals/.
#
# This is a DEV CONVENIENCE ONLY (see README.md) — the single source of truth
# for every module's content stays its own verticals/<name>/spec/ folder.
# Nothing under the generated paths below should ever be hand-edited; re-run
# this script instead. Production composition is per-App `formspec apply` into
# one workspace (docs/spec/02-core-basic.md §6.0), not this script.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="$ROOT/reference-app/spec"

rm -rf "$OUT/modules" "$OUT/_app-extras"
mkdir -p "$OUT/modules" "$OUT/_app-extras"

# module-scoped content: verticals/<app>/spec/modules/<module-name>/ -> spec/modules/<module-name>/
for app in company billing inventory gl notifications sales-inventory-integrator sales-gl-integrator; do
  src="$ROOT/$app/spec/modules"
  if [ -d "$src" ]; then
    cp -R "$src/." "$OUT/modules/"
  fi
done

# App-level UI extras that some verticals (inventory, gl) ship outside
# spec/modules/ — namespaced per app to avoid path collisions (both currently
# have a spec/config/app.yaml, for example).
for app in inventory gl; do
  src="$ROOT/$app/spec"
  dest="$OUT/_app-extras/$app"
  mkdir -p "$dest"
  for sub in menus widgets reports tables dashboards; do
    if [ -d "$src/$sub" ]; then
      mkdir -p "$dest/$sub"
      cp -R "$src/$sub/." "$dest/$sub/"
    fi
  done
done

echo "Composed $(find "$OUT/modules" "$OUT/_app-extras" -name '*.yaml' -o -name '*.star' | wc -l) manifest/script files into $OUT"
