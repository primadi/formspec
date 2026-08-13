# FormSpec Site — Landing Page

Landing page untuk `formspec.dev` — Vite + React + TypeScript + Tailwind CSS v4.

## Development

```bash
cd site
npm install
npm run dev          # http://localhost:5173
```

## Build

```bash
npm run build        # output: site/dist/
npm run preview      # preview build lokal
```

## Deploy (Cloudflare Pages)

Project Pages terhubung ke repo ini:

- **Root directory**: `site`
- **Build command**: `npm install && npm run build`
- **Deploy command**: `npx wrangler deploy` (default)
- **Output**: dibaca dari `wrangler.toml` (`[assets] directory = "./dist"`)
- Custom domain `formspec.dev` (+ `www.formspec.dev`, lalu redirect ke apex)

Redirect & header dikelola lewat file di `public/`:

- `_redirects` — `formspec.dev/schemas/* → schemas.formspec.dev/*` (redirect
  `www → apex` di-handle lewat **Redirect Rule** dashboard Cloudflare, bukan
  `_redirects` — domain redirect tidak didukung di `_redirects`)
- `_headers` — security headers (CSP, X-Frame-Options, dsb.)
- `.well-known/security.txt` — kontak keamanan

## Struktur

```
site/
  public/            # aset statis + _redirects + _headers
  src/
    App.tsx          # komposisi section
    components/      # Nav, Hero, Primitives, Architecture, ImplTypes,
                     # Derived, Marketplace, Quickstart, CTA, Footer
    index.css        # Tailwind v4 + design tokens
```

## Verifikasi produksi

```bash
curl -I https://formspec.dev                          # → 200
curl -I https://www.formspec.dev                      # → 301 ke apex
curl -I https://formspec.dev/schemas/formspec.schema.json  # → redirect ke schemas.formspec.dev
```
