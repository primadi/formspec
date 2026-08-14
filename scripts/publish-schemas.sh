#!/usr/bin/env bash
#
# publish-schemas.sh — stage & (opsional) upload JSON Schema FormSpec ke
# schemas.formspec.dev (Cloudflare R2 public bucket).
#
# Layout yang dihasilkan (staging di schemas/dist/):
#   schemas/dist/index.html            → landing page (biar root tidak 404)
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
KINDS_HTML=""
for f in "$OUT_DIR"/kinds/*.schema.json; do
  name=$(basename "$f" .schema.json)
  KINDS_HTML+=$(printf '        <li><a href="/%s/kinds/%s.schema.json">%s</a></li>\n' "$VERSION" "$name" "$name")
done
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

# Landing page di root (index.html) — supaya https://schemas.formspec.dev
# (base URL) tidak 404: menampilkan versi + link ke semua schema.
cat > "$DIST_DIR/index.html" <<EOF
<!doctype html>
<html lang="id">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>FormSpec — Registry JSON Schema</title>
<style>
  :root { color-scheme: dark; }
  * { box-sizing: border-box; }
  body { margin: 0; font-family: system-ui, -apple-system, "Segoe UI", sans-serif;
    background: #0b0f1a; color: #e4e4e7; line-height: 1.6; }
  main { max-width: 880px; margin: 0 auto; padding: 56px 24px; }
  h1 { font-size: 28px; margin: 8px 0 4px; }
  h2 { font-size: 18px; margin-top: 40px; color: #a1a1aa; }
  a { color: #818cf8; text-decoration: none; }
  a:hover { text-decoration: underline; }
  .brand { display: flex; align-items: center; gap: 10px; font-weight: 600; }
  .brand svg { border-radius: 8px; }
  .desc { color: #a1a1aa; }
  .box { background: #131a2b; border: 1px solid #232c44; border-radius: 12px; padding: 20px 24px; }
  ul { margin: 0; padding-left: 18px; }
  li { margin: 6px 0; }
  code { background: #1c2740; padding: 2px 6px; border-radius: 6px; font-size: 13px; }
</style>
</head>
<body>
<main>
  <div class="brand"><svg width="28" height="28" viewBox="0 0 64 64" fill="none" aria-hidden="true"><rect width="64" height="64" rx="16" fill="url(#fsg)"/><rect x="13" y="15" width="38" height="8" rx="4" fill="#fff"/><rect x="13" y="28" width="28" height="8" rx="4" fill="#fff" fill-opacity="0.85"/><rect x="13" y="41" width="18" height="8" rx="4" fill="#fff" fill-opacity="0.70"/><defs><linearGradient id="fsg" x1="0" y1="0" x2="64" y2="64" gradientUnits="userSpaceOnUse"><stop stop-color="#6366f1"/><stop offset="1" stop-color="#10b981"/></linearGradient></defs></svg> FormSpec</div>
  <h1>Registry JSON Schema</h1>
  <p class="desc">JSON Schema (Draft-07) untuk semua resource kind FormSpec, di-generate dari <code>pkg/spec</code> (Go types).</p>

  <h2>Root schema</h2>
  <div class="box">
    <ul>
      <li><a href="/$VERSION/formspec.schema.json">/$VERSION/formspec.schema.json</a> — schema root (discriminator)</li>
      <li><a href="/latest/formspec.schema.json">/latest/formspec.schema.json</a> — alias versi terbaru</li>
      <li><a href="/$VERSION/index.json">/$VERSION/index.json</a> — metadata versi + daftar kinds</li>
    </ul>
  </div>

  <h2>Schema per kind ($VERSION)</h2>
  <div class="box">
    <ul>
$KINDS_HTML
    </ul>
  </div>

  <p class="desc" style="margin-top: 32px">Registry di-serve statis dari Cloudflare (R2/Pages). Lihat <code>schemas/README.md</code> untuk cara publish.</p>
</main>
</body>
</html>
EOF

echo ""
echo "✅ Selesai stage:"
echo "   $DIST_DIR/index.html  (landing page)"
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
