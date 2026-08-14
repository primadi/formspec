#!/usr/bin/env bash
#
# publish-schemas.sh — stage & (opsional) upload JSON Schema FormSpec ke
# schemas.formspec.dev (Cloudflare R2 public bucket).
#
# Layout yang dihasilkan (staging di schemas/dist/):
#   schemas/dist/<version>/formspec.schema.json
#   schemas/dist/<version>/kinds/{Kind}.schema.json
#   schemas/dist/latest/            → salinan <version> (alias "latest")
#   schemas/dist/<version>/index.json  → metadata versi
#
# Mode:
#   --stage (default)          generate + susun layout versi lokal di schemas/dist/
#   --upload [--bucket NAME]   setelah stage, upload ke R2 via npx wrangler
#
# Contoh:
#   scripts/publish-schemas.sh                        # stage v1
#   scripts/publish-schemas.sh --version v1           # stage v1 eksplisit
#   scripts/publish-schemas.sh --upload --bucket formspec-schemas
#
# Prasyarat R2 (untuk --upload): bucket public + custom domain
# schemas.formspec.dev sudah dikonfigurasi di Cloudflare; login wrangler
# (CLOUDFLARE_API_TOKEN) sudah disiapkan.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCHEMA_DIR="$REPO_ROOT/schemas"
DIST_DIR="$SCHEMA_DIR/dist"
VERSION="v1"
UPLOAD=false
BUCKET=""
OUT_DIR="$SCHEMA_DIR"  # output generator (schema + kinds di sini)

usage() {
  sed -n '2,22p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) VERSION="$2"; shift 2 ;;
    --upload) UPLOAD=true; shift ;;
    --bucket) BUCKET="$2"; shift 2 ;;
    --out) OUT_DIR="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "❌ argumen tak dikenal: $1"; usage; exit 1 ;;
  esac
done

if [[ "$UPLOAD" == true && -z "$BUCKET" ]]; then
  echo "❌ --upload butuh --bucket <nama-bucket>"
  exit 1
fi

echo "📐 Men-generate JSON Schema dari Go types..."
(cd "$REPO_ROOT" && make generate-schema)
echo "✅ Schema segar di $SCHEMA_DIR"

echo "📦 Menyusun layout versi '$VERSION' di $DIST_DIR/$VERSION ..."
mkdir -p "$DIST_DIR/$VERSION/kinds"
cp "$OUT_DIR/formspec.schema.json" "$DIST_DIR/$VERSION/formspec.schema.json"
for f in "$OUT_DIR"/kinds/*.schema.json; do
  cp "$f" "$DIST_DIR/$VERSION/kinds/$(basename "$f")"
done

# Metadata versi (index.json) — berisi daftar kinds yang dipublish supaya
# `formspec schema fetch` / `formspec init` tahu set schema lengkap dari
# registry itu sendiri (self-describing), bukan dari daftar hardcoded CLI.
KINDS_ARRAY=$(for f in "$OUT_DIR"/kinds/*.schema.json; do basename "$f" .schema.json; done | sort | sed 's/^/"/; s/$/"/' | paste -sd, -)
cat > "$DIST_DIR/$VERSION/index.json" <<EOF
{
  "name": "formspec-schemas",
  "version": "$VERSION",
  "description": "JSON Schema (Draft-07) untuk resource kinds FormSpec.",
  "apiVersion": "formspec.dev/v1",
  "schemaDraft": "http://json-schema.org/draft-07/schema#",
  "files": {
    "root": "https://schemas.formspec.dev/$VERSION/formspec.schema.json",
    "kinds": "https://schemas.formspec.dev/$VERSION/kinds/{Kind}.schema.json"
  },
  "kinds": [$KINDS_ARRAY],
  "publishedAt": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}
EOF

echo "📎 Membuat alias 'latest' → '$VERSION' ..."
rm -rf "$DIST_DIR/latest"
cp -r "$DIST_DIR/$VERSION" "$DIST_DIR/latest"

echo ""
echo "✅ Selesai stage:"
echo "   $DIST_DIR/$VERSION/"
echo "   $DIST_DIR/latest/  (alias)"
echo ""
echo "   URL publik (setelah deploy ke R2/Pages):"
echo "   https://schemas.formspec.dev/$VERSION/formspec.schema.json"
echo "   https://schemas.formspec.dev/$VERSION/kinds/Entity.schema.json"
echo "   https://schemas.formspec.dev/latest/formspec.schema.json"

if [[ "$UPLOAD" == true ]]; then
  echo ""
  echo "🚀 Mengupload ke R2 bucket '$BUCKET' ..."
  command -v npx >/dev/null 2>&1 || { echo "❌ npx tidak ditemukan"; exit 1; }

  # Upload semua file di DIST_DIR (dipertahankan struktur folder & versi).
  (cd "$DIST_DIR" && find . -type f -print0 | while IFS= read -r -d '' f; do
    key="${f#./}"
    case "$key" in
      *.json) ct="application/json" ;;
      *) ct="application/octet-stream" ;;
    esac
    echo "   → $key ($ct)"
    npx --yes wrangler r2 object put "$BUCKET/$key" --file "$f" --content-type "$ct"
  done)
  echo "✅ Upload selesai."
fi

echo ""
echo "ℹ️  Verifikasi: curl -I https://schemas.formspec.dev/$VERSION/formspec.schema.json"
