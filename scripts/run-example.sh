#!/usr/bin/env bash
# run-example.sh — jalankan dev server untuk contoh program di examples/
#
# Usage:
#   scripts/run-example.sh <nama> [opsi]
#
# Contoh:
#   scripts/run-example.sh klinik
#   scripts/run-example.sh cafe --addr :8081
#   scripts/run-example.sh klinik --dsn "sqlite:/tmp/clinic.db"
#
# Opsi:
#   --dsn <dsn>     DSN database (default: sqlite:.formspec/<name>.db)
#   --addr <addr>   Alamat listen (default: :8080)
#   --spec <dir>    Override path spec secara manual
#   --list          Daftar example yang tersedia
#   -h, --help      Tampilkan bantuan

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

# Alias singkat -> nama folder di examples/
declare -A ALIAS=(
  [klinik]="Clinic-UI-Showcase"
  [clinic]="Clinic-UI-Showcase"
  [cafe]="cafe"
  [arisan]="arisan"
  [crc]="crc-management"
  [midtrans]="Midtrans-Payment-Gateway"
  [payment]="Midtrans-Payment-Gateway"
  [service-demo]="service-demo"
  [storefront]="storefront"
)

usage() { sed -n '2,20p' "$0"; }

list_examples() {
  echo "Example yang tersedia:"
  for d in "$ROOT"/examples/*/; do
    name="$(basename "$d")"
    if [[ -d "$d/spec" ]]; then
      echo "  $name"
    fi
  done
  echo
  echo "Alias:"
  for k in "${!ALIAS[@]}"; do
    printf '  %-14s -> %s\n' "$k" "${ALIAS[$k]}"
  done | sort
}

NAME="${1:-}"
[[ -z "$NAME" || "$NAME" == "-h" || "$NAME" == "--help" ]] && { usage; exit 0; }
[[ "$NAME" == "--list" ]] && { list_examples; exit 0; }
shift || true

# Resolve alias -> folder
DIR_NAME="${ALIAS[$NAME]:-$NAME}"
SPEC_DIR="$ROOT/examples/$DIR_NAME/spec"

if [[ ! -d "$SPEC_DIR" ]]; then
  # fallback: coba langsung sebagai nama folder
  if [[ -d "$ROOT/examples/$NAME/spec" ]]; then
    DIR_NAME="$NAME"
    SPEC_DIR="$ROOT/examples/$NAME/spec"
  else
    echo "Error: example '$NAME' tidak ditemukan (folder: examples/$DIR_NAME/spec)" >&2
    echo "Gunakan --list untuk melihat daftar." >&2
    exit 1
  fi
fi

DSN="sqlite:.formspec/${DIR_NAME}.db"
ADDR=":8080"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dsn)  DSN="$2";  shift 2 ;;
    --addr) ADDR="$2"; shift 2 ;;
    --spec) SPEC_DIR="$2"; shift 2 ;;
    *) echo "Opsi tidak dikenal: $1" >&2; exit 1 ;;
  esac
done

echo "▶ Example : $DIR_NAME"
echo "  Spec    : $SPEC_DIR"
echo "  DSN     : $DSN"
echo "  Addr    : $ADDR"
echo

exec go run ./cmd/formspec/ dev --dev-ui --spec "$SPEC_DIR" --dsn "$DSN" --addr "$ADDR"
