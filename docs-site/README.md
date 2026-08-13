# FormSpec Docs Site — docs.formspec.dev

Docs site berbasis **VitePress** yang membaca langsung dari `../docs/`
(single source of truth). Tidak ada salinan konten.

## Setup (sekali)

VitePress tidak bisa memetakan `README.md` → index folder secara otomatis,
dan `srcDir` yang menunjuk keluar project membuat resolusi modul Vue gagal.
Solusinya dua hal:

1. **Symlink** `docs-site/docs → ../docs` — konten tetap satu source
   (`docs/`), tapi path terlihat dari dalam `docs-site/` sehingga
   `node_modules` (vue, vue/server-renderer) bisa di-resolve
   (`vite.resolve.preserveSymlinks: true` di config).
2. **`rewrites`** memetakan tiap `README.md` → `index.md` agar jadi index
   folder (home `/`, `/spec/`, `/spec/platform/`, dst).

```bash
cd docs-site
ln -sfn ../docs docs   # buat symlink
npm install
```

## Development

```bash
cd docs-site
npm run dev            # http://localhost:5173
```

## Build

```bash
npm run build          # output: docs-site/dist/
npm run preview        # preview build lokal
```

## Deploy (Cloudflare Pages)

Project Pages terhubung ke repo ini (folder root `docs-site`, build command
`npm run build`, output directory `dist`). Custom domain `docs.formspec.dev`.

**Catatan CI:** symlink `docs-site/docs` di-ignore di `.gitignore` — pipeline
harus membuatnya dulu (`ln -sfn ../docs docs`) sebelum `npm run build`.

Redirect & header dikelola lewat file di `public/`.

## Konten yang dieksklusi dari docs publik

Folder internal (work-in-progress) di-exclude lewat `srcExclude` di
`.vitepress/config.mts`:

- `plan/**` — rencana kerja
- `changelog/**` — changelog internal
- `presentations/**` — materi presentasi
- `technical-notes/**` — catatan teknis internal

Dokumen publik: `spec/`, `renderers/`, `architecture/`, `runtimes/`,
`cli-tools/`, `guides/`, `reference/`, `comparison/`, `kind/`, `ai/`.

Cek dead-link di-toleransi untuk kategori tertentu (folder internal yang
tidak ikut site, konvensi README-as-index, sisa rename forma→formspec,
localhost dev) — lihat `ignoreDeadLinks` di config.

## Struktur

```
docs-site/
  .vitepress/
    config.mts         # srcDir → symlink docs, sidebar, nav, search, rewrites
    theme/             # theme kustom + style (brand overlay)
  docs -> ../docs      # symlink (build-time, di-ignore git)
  public/              # favicon + _redirects + _headers
  dist/                # output build (di-upload ke Pages)
```
