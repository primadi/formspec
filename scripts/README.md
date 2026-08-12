# FormSpec Scripts

## docs-serve — Tampilkan `docs/` di Browser

Server HTTP ringan (Python stdlib + `markdown` + `Pygments`) yang merender
dokumentasi Markdown menjadi halaman HTML yang nyaman dibaca:

- **Sidebar navigasi** — tree otomatis dari struktur folder `docs/`
- **Pencarian** — filter judul/path dokumen di sidebar (desktop)
- **Syntax highlighting** — server-side via Pygments
- **Tabel konten** — TOC per halaman dengan anchor links
- **Responsive** — layout menyesuaikan mobile (tombol ☰ + backdrop)

### Cara pakai

```bash
# Instal dependensi (sekali)
pip install -r scripts/requirements.txt

# Jalankan (default port 8000)
python3 scripts/docs-serve.py
# atau via Makefile
make docs-serve
# port kustom
make docs-serve PORT=9000
```

Buka di browser: <http://localhost:8000/docs/>

### Opsi CLI

| Opsi | Default | Keterangan |
|---|---|---|
| `--port` | `8000` | Port server |
| `--host` | `0.0.0.0` | Bind host |
| `--dir` | `../docs` | Root direktori dokumen |

### Catatan

- Setiap request memicu render; untuk docs besar pertimbangkan cache.
- Markdown didukung via ekstensi `extra`, `admonition`, `fenced_code`,
  `tables`, `toc`, dll. — setara CommonMark plus ekstensi umum.
