#!/usr/bin/env bash
# run-registry.sh — jalankan FormSpec Module Registry (cmd/formspec-registry)
# dalam mode dev, dengan config dari cmd/formspec-registry/formspec-app.yaml.
#
# Config memberi:
#   spec: cmd/formspec-registry/app-spec/spec
#         → disk-backed spec → hot-reload watcher aktif
#                             (edit cmd/formspec-registry/app-spec/spec/*.yaml,
#                              tanpa restart)
#   dsn:  sqlite:.formspec/registry.db
#   jwt-secret              → sesi bertahan antar restart
#
# Usage:
#   scripts/run-registry.sh [opsi formspec-registry...]
#
# Contoh:
#   scripts/run-registry.sh
#   scripts/run-registry.sh --addr :8081
#   scripts/run-registry.sh --prod --dsn "postgres://..."
#
# Opsi CLI menimpa nilai config file.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

CONFIG="cmd/formspec-registry/formspec-app.yaml"
[[ -f "$CONFIG" ]] || { echo "Error: $CONFIG tidak ditemukan" >&2; exit 1; }

echo "▶ FormSpec Module Registry (dev)"
echo "  Config : $CONFIG"
echo

exec go run ./cmd/formspec-registry --config "$CONFIG" "$@"
