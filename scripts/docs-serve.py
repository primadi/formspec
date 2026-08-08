#!/usr/bin/env python3
"""Forma Docs Server — tampilkan docs/ dengan nyaman di browser.

Server HTTP ringan yang merender Markdown menjadi halaman HTML yang rapi
dengan navigasi sidebar, pencarian, dan syntax highlighting (server-side).

Penggunaan:
    python3 scripts/docs-serve.py                # serve docs/ di :8000
    python3 scripts/docs-serve.py --port 9000    # port kustom
    python3 scripts/docs-serve.py --dir ../foo   # root dokumen kustom

Atau via Makefile:
    make docs-serve
"""

from __future__ import annotations

import argparse
import html
import os
import posixpath
import re
import sys
import urllib.parse
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

# ---------------------------------------------------------------------------
# Konfigurasi
# ---------------------------------------------------------------------------
ROOT_DIR = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "docs"))
PORT = 8000
INDEX_NAME = "README.md"

# ---------------------------------------------------------------------------
# Markdown -> HTML
# ---------------------------------------------------------------------------
import markdown  # type: ignore
from markdown.extensions.toc import TocExtension  # type: ignore

# Handler kode: gunakan Pygments bila tersedia, fallback ke <pre><code>.
try:
    from pygments import highlight
    from pygments.formatters import HtmlFormatter
    from pygments.lexers import get_lexer_by_name, guess_lexer
    from pygments.util import ClassNotFound

    PYGM = True
except Exception:  # pragma: no cover
    PYGM = False

PYGM_STYLE_CSS = HtmlFormatter(style="github-dark").get_style_defs(".codehilite") if PYGM else ""


def _code_formatter(src: str, language: str, css_class: str) -> str:
    if not language:
        language = ""
    if PYGM:
        try:
            lexer = get_lexer_by_name(language.strip().lower()) if language else guess_lexer(src)
            return highlight(src, lexer, HtmlFormatter(nowrap=True))
        except ClassNotFound:
            pass
    return html.escape(src)


# Custom markdown extension untuk fenced code + pygments.
from markdown.extensions.codehilite import CodeHiliteExtension  # type: ignore


def render_markdown(text: str) -> str:
    """Konversi Markdown menjadi HTML."""
    md = markdown.Markdown(
        extensions=[
            "extra",
            "admonition",
            "tables",
            "fenced_code",
            "sane_lists",
            "attr_list",
            "md_in_html",
            "footnotes",
            CodeHiliteExtension(use_pygments=PYGM, noclasses=False, guess_lang=False),
            TocExtension(permalink=True, slugify=lambda v, s: re.sub(r"[^\w\-]+", "-", v).strip("-").lower()),
        ],
        output_format="html5",
    )
    body = md.convert(text)
    toc = md.toc if hasattr(md, "toc") else ""
    return body, toc


# ---------------------------------------------------------------------------
# Navigasi tree
# ---------------------------------------------------------------------------
_ORDER_PREFIX = re.compile(r"^\d{2}-")


def _display_name(name: str) -> str:
    """'01-core-basic.md' -> '01. Core basic'."""
    base = name[:-3] if name.endswith(".md") else name
    base = _ORDER_PREFIX.sub("", base)
    return base.replace("-", " ").replace("_", " ")


def _walk_tree(abs_dir: str, rel_dir: str) -> list[dict]:
    """Bangun tree dokumen sebagai list of dict."""
    items: list[dict] = []
    try:
        entries = sorted(os.listdir(abs_dir))
    except OSError:
        return items
    for entry in entries:
        if entry.startswith("."):
            continue
        abs_path = os.path.join(abs_dir, entry)
        rel_path = posixpath.join(rel_dir, entry) if rel_dir else entry
        if os.path.isdir(abs_path):
            children = _walk_tree(abs_path, rel_path)
            items.append(
                {
                    "name": entry,
                    "rel": rel_path,
                    "kind": "dir",
                    "title": _display_name(entry),
                    "children": children,
                }
            )
        elif entry.endswith(".md"):
            items.append(
                {
                    "name": entry,
                    "rel": rel_path,
                    "kind": "file",
                    "title": _display_name(entry),
                }
            )
    # Sort: folder dulu, lalu file. Pertahankan urutan nama (prefix 2-digit).
    items.sort(key=lambda i: (0 if i["kind"] == "dir" else 1, i["name"].lower()))
    return items


def _build_nav(abs_dir: str) -> list[dict]:
    return _walk_tree(abs_dir, "")


def _nav_item(item: dict, current: str, depth: int) -> str:
    """Render satu entri navigasi (file)."""
    link = "/docs/" + urllib.parse.quote(item["rel"])
    cls = "nav-item active" if item["rel"] == current else "nav-item"
    pad = 8 + depth * 14
    return (
        f'<li><a class="{cls}" style="padding-left:{pad}px" '
        f'href="{link}">📄 {html.escape(item["title"])}</a></li>'
    )


def _render_nav(tree: list[dict], current: str, depth: int = 0) -> str:
    """Render tree menjadi HTML <ul> sidebar yang valid & bersarang."""
    parts: list[str] = ['<ul class="nav-children">']
    for item in tree:
        if item["kind"] == "dir":
            parts.append(f'<li class="nav-dir"><span class="nav-dir-label">{html.escape(item["title"])}</span>')
            parts.append('<ul class="nav-children">')
            # README dir ditampilkan sebagai entri pertama folder itu sendiri
            for child in item["children"]:
                if child["kind"] == "file" and child["name"].lower() == INDEX_NAME.lower():
                    parts.append(_nav_item(child, current, depth + 1))
            parts.append(_render_nav(item["children"], current, depth + 1))
            parts.append("</ul></li>")
        else:
            if item["name"].lower() == INDEX_NAME.lower():
                continue  # sudah dirender oleh parent dir
            parts.append(_nav_item(item, current, depth))
    parts.append("</ul>")
    return "".join(parts)


# ---------------------------------------------------------------------------
# Template HTML
# ---------------------------------------------------------------------------
CSS = r"""
:root {
  --bg: #ffffff;
  --bg-soft: #f6f8fa;
  --bg-nav: #fafbfc;
  --border: #d0d7de;
  --text: #1f2328;
  --text-muted: #57606a;
  --accent: #0969da;
  --accent-soft: #ddf4ff;
  --code-bg: #f6f8fa;
  --radius: 8px;
  --sidebar-w: 320px;
  --header-h: 56px;
  --mono: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace;
}
@media (prefers-color-scheme: dark) {
  :root {
    --bg: #0d1117;
    --bg-soft: #161b22;
    --bg-nav: #010409;
    --border: #30363d;
    --text: #e6edf3;
    --text-muted: #8b949e;
    --accent: #4493f8;
    --accent-soft: #1f3d5c;
    --code-bg: #161b22;
  }
}
* { box-sizing: border-box; }
html { scroll-behavior: smooth; }
body {
  margin: 0;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", "Noto Sans",
    Helvetica, Arial, sans-serif;
  font-size: 16px;
  line-height: 1.65;
  color: var(--text);
  background: var(--bg);
}
a { color: var(--accent); text-decoration: none; }
a:hover { text-decoration: underline; }

/* ---------- Header ---------- */
.site-header {
  position: sticky; top: 0; z-index: 50;
  height: var(--header-h);
  display: flex; align-items: center; gap: 12px;
  padding: 0 16px;
  background: var(--bg);
  border-bottom: 1px solid var(--border);
}
.site-header .brand { font-weight: 700; font-size: 15px; display: flex; gap: 8px; align-items: center; }
.site-header .brand .dot { color: var(--accent); }
.burger {
  display: none; background: none; border: 1px solid var(--border);
  color: var(--text); border-radius: 6px; padding: 6px 10px; cursor: pointer;
  font-size: 14px;
}
.search-box { margin-left: auto; width: min(320px, 40vw); position: relative; }
.search-box input {
  width: 100%; padding: 7px 12px; border: 1px solid var(--border);
  border-radius: 6px; background: var(--bg-soft); color: var(--text); font-size: 14px;
}
.search-box input:focus { outline: 2px solid var(--accent); outline-offset: -1px; }
.search-clear {
  position: absolute; right: 8px; top: 50%; transform: translateY(-50%);
  background: none; border: none; color: var(--text-muted); cursor: pointer; font-size: 14px; display: none;
}
.search-clear.show { display: block; }

/* ---------- Layout ---------- */
.layout { display: flex; min-height: calc(100vh - var(--header-h)); }
.sidebar {
  width: var(--sidebar-w); flex: 0 0 var(--sidebar-w);
  background: var(--bg-nav); border-right: 1px solid var(--border);
  overflow-y: auto; padding: 16px 8px 40px;
  position: sticky; top: var(--header-h); height: calc(100vh - var(--header-h));
  font-size: 14px;
}
.sidebar.collapsed { display: none; }
.content {
  flex: 1; min-width: 0; padding: 32px 48px 80px;
  max-width: 1000px; margin: 0 auto;
}
.content-inner { max-width: 860px; margin: 0 auto; }

/* ---------- Nav ---------- */
.nav-tree, .nav-children { list-style: none; margin: 0; padding: 0; }
.nav-tree { padding-left: 0; }
.nav-children { padding-left: 14px; }
.nav-dir-label {
  display: block; font-weight: 600; color: var(--text-muted);
  padding: 6px 8px; margin-top: 8px; font-size: 12px; text-transform: uppercase;
  letter-spacing: 0.03em;
}
.nav-item {
  display: block; color: var(--text); padding: 4px 8px;
  border-radius: 6px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.nav-item:hover { background: var(--bg-soft); text-decoration: none; }
.nav-item.active { background: var(--accent-soft); color: var(--accent); font-weight: 600; }
.nav-item[hidden] { display: none; }

/* ---------- Breadcrumb ---------- */
.breadcrumb { font-size: 13px; color: var(--text-muted); margin-bottom: 8px; }
.breadcrumb a { color: var(--text-muted); }
.breadcrumb span.sep { margin: 0 6px; opacity: 0.6; }

/* ---------- Prose / Markdown ---------- */
.prose h1 {
  font-size: 30px; line-height: 1.3; border-bottom: 1px solid var(--border);
  padding-bottom: 12px; margin: 0 0 20px; font-weight: 700;
}
.prose h2 {
  font-size: 23px; margin-top: 40px; border-bottom: 1px solid var(--border);
  padding-bottom: 8px;
}
.prose h3 { font-size: 19px; margin-top: 32px; }
.prose h4 { font-size: 16px; margin-top: 24px; }
.prose p { margin: 14px 0; }
.prose ul, .prose ol { padding-left: 24px; margin: 14px 0; }
.prose li { margin: 4px 0; }
.prose li > ul, .prose li > ol { margin: 4px 0; }
.prose blockquote {
  margin: 16px 0; padding: 4px 16px; border-left: 4px solid var(--accent);
  background: var(--bg-soft); border-radius: 0 6px 6px 0; color: var(--text-muted);
}
.prose blockquote p { margin: 8px 0; }
.prose code {
  font-family: var(--mono); font-size: 0.88em;
  background: var(--bg-soft); padding: 2px 5px; border-radius: 4px;
}
.prose pre {
  background: var(--code-bg); border: 1px solid var(--border);
  border-radius: var(--radius); padding: 16px; overflow-x: auto; line-height: 1.5;
}
.prose pre code { background: none; padding: 0; font-size: 13.5px; }
.prose table {
  border-collapse: collapse; width: 100%; margin: 18px 0;
  font-size: 14.5px; display: block; overflow-x: auto;
}
.prose th, .prose td { border: 1px solid var(--border); padding: 8px 12px; text-align: left; }
.prose th { background: var(--bg-soft); font-weight: 600; }
.prose tr:nth-child(2n) td { background: var(--bg-soft); }
.prose hr { border: none; border-top: 1px solid var(--border); margin: 32px 0; }
.prose img { max-width: 100%; border-radius: var(--radius); }
.prose .toc { background: var(--bg-soft); border: 1px solid var(--border); border-radius: var(--radius); padding: 12px 20px; margin: 20px 0; font-size: 14px; }
.prose .toc ul { padding-left: 20px; margin: 6px 0; }
.prose .toc a { color: var(--text-muted); }
.prose .headerlink { opacity: 0; transition: opacity .15s; font-weight: 400; margin-left: 6px; color: var(--accent); }
.prose h1:hover .headerlink, .prose h2:hover .headerlink,
.prose h3:hover .headerlink, .prose h4:hover .headerlink { opacity: 1; }

/* Admonition (extra) */
.prose .admonition {
  border: 1px solid var(--border); border-left-width: 4px; border-radius: 6px;
  padding: 8px 16px; margin: 16px 0; font-size: 15px;
}
.prose .admonition-title { font-weight: 700; margin-bottom: 4px; }
.prose .admonition.note { border-left-color: var(--accent); background: var(--accent-soft); }
.prose .admonition.warning { border-left-color: #d29922; background: color-mix(in srgb, #d29922 12%, transparent); }
.prose .admonition.tip { border-left-color: #1a7f37; background: color-mix(in srgb, #1a7f37 12%, transparent); }
.prose .admonition.danger { border-left-color: #cf222e; background: color-mix(in srgb, #cf222e 12%, transparent); }

/* Pygments blocks */
.prose .codehilite { background: var(--code-bg); border: 1px solid var(--border); border-radius: var(--radius); padding: 14px 16px; overflow-x: auto; }
.prose .codehilite pre { background: none; border: none; padding: 0; margin: 0; }

/* ---------- File listing (dir tanpa README) ---------- */
.file-list { list-style: none; padding: 0; }
.file-list li { margin: 6px 0; }
.file-list a { display: flex; align-items: center; gap: 10px; padding: 8px 12px; border-radius: 6px; }
.file-list a:hover { background: var(--bg-soft); text-decoration: none; }
.file-list .ic { font-size: 18px; }
.file-list .size { margin-left: auto; color: var(--text-muted); font-size: 12px; }

/* ---------- Footer ---------- */
.page-footer {
  margin-top: 48px; padding-top: 16px; border-top: 1px solid var(--border);
  font-size: 13px; color: var(--text-muted);
  display: flex; justify-content: space-between; flex-wrap: wrap; gap: 8px;
}

/* ---------- Responsive ---------- */
.backdrop { display: none; position: fixed; inset: 0; background: rgba(0,0,0,.4); z-index: 40; }
@media (max-width: 900px) {
  .burger { display: inline-block; }
  .search-box { display: none; }
  .sidebar {
    position: fixed; left: 0; top: var(--header-h); bottom: 0;
    z-index: 45; width: 290px; box-shadow: 2px 0 12px rgba(0,0,0,.15);
  }
  .sidebar.collapsed { display: none; }
  .backdrop.show { display: block; }
  .content { padding: 20px 16px 60px; }
  .prose h1 { font-size: 24px; }
}
"""

JS = r"""
(() => {
  const sidebar = document.getElementById('sidebar');
  const backdrop = document.getElementById('backdrop');
  const burger = document.getElementById('burger');
  const search = document.getElementById('search');
  const clear = document.getElementById('search-clear');

  const openSidebar = () => { sidebar.classList.remove('collapsed'); backdrop.classList.add('show'); };
  const closeSidebar = () => { sidebar.classList.add('collapsed'); backdrop.classList.remove('show'); };

  burger.addEventListener('click', () => {
    sidebar.classList.contains('collapsed') ? openSidebar() : closeSidebar();
  });
  backdrop.addEventListener('click', closeSidebar);

  // Search filter pada navigasi (match title & path)
  const items = Array.from(document.querySelectorAll('.nav-item'));
  const labels = items.map(el => el.textContent + ' ' + el.getAttribute('href'));
  search.addEventListener('input', () => {
    const q = search.value.trim().toLowerCase();
    clear.classList.toggle('show', !!q);
    let visible = 0;
    items.forEach((el, i) => {
      const ok = !q || labels[i].toLowerCase().includes(q);
      el.hidden = !ok;
      if (ok) visible++;
    });
    document.querySelectorAll('.nav-dir').forEach(dir => {
      const childHas = Array.from(dir.querySelectorAll('.nav-item')).some(c => !c.hidden);
      dir.hidden = !childHas;
    });
    document.getElementById('nav-empty').hidden = visible !== 0;
  });
  clear.addEventListener('click', () => { search.value = ''; search.dispatchEvent(new Event('input')); });

  // Klik tautan di mobile -> tutup sidebar
  items.forEach(el => el.addEventListener('click', () => { if (window.innerWidth <= 900) closeSidebar(); }));

  // Highlight baris pada url fragment untuk headerlink
  const hl = (id) => {
    document.querySelectorAll('.prose .headerlink').forEach(a => {
      a.classList.toggle('hl-active', a.getAttribute('href') === '#' + id);
    });
  };
  const updateHash = () => hl(decodeURIComponent(location.hash.slice(1)));
  window.addEventListener('hashchange', updateHash);
  if (location.hash) setTimeout(updateHash, 0);
})();
"""

PAGE_TEMPLATE = """<!DOCTYPE html>
<html lang="id">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{title}</title>
<style>{css}</style>
</head>
<body>
<header class="site-header">
  <button class="burger" id="burger" aria-label="Menu">☰</button>
  <div class="brand"><span class="dot">◆</span> Forma Docs</div>
  <div class="search-box">
    <input id="search" type="search" placeholder="Cari di dokumentasi…" autocomplete="off">
    <button class="search-clear" id="search-clear" aria-label="Bersihkan">✕</button>
  </div>
</header>
<div class="layout">
  <aside class="sidebar" id="sidebar">
    <div style="padding:4px 8px">
      <div id="nav-empty" style="color:var(--text-muted);font-size:13px;display:none">Tidak ada hasil.</div>
    </div>
    {nav}
  </aside>
  <div class="backdrop" id="backdrop"></div>
  <main class="content">
    <div class="content-inner">
      {breadcrumb}
      {content}
      {footer}
    </div>
  </main>
</div>
<script>{js}</script>
</body>
</html>
"""


def _escape_path_segment(s: str) -> str:
    return urllib.parse.quote(s)


def _breadcrumb(rel: str) -> str:
    parts = [('<a href="/docs/">Docs</a>')]
    segs = [s for s in rel.split("/") if s]
    for i, seg in enumerate(segs):
        if i == len(segs) - 1:
            parts.append(f'<span class="sep">›</span> <span>{html.escape(_display_name(seg))}</span>')
        else:
            href = "/docs/" + _escape_path_segment("/".join(segs[: i + 1])) + "/"
            parts.append(f'<span class="sep">›</span> <a href="{href}">{html.escape(_display_name(seg))}</a>')
    return '<div class="breadcrumb">' + "".join(parts) + "</div>"


def _footer(rel: str) -> str:
    rel_esc = html.escape(rel)
    return (
        '<div class="page-footer">'
        f"<span>Dokumentasi Forma · {rel_esc}</span>"
        "<span>Rendered by docs-serve</span>"
        "</div>"
    )


def _render_page(title: str, content_html: str, rel: str, nav: str) -> bytes:
    html_out = PAGE_TEMPLATE.format(
        title=html.escape(title),
        css=CSS,
        js=JS,
        nav=nav,
        breadcrumb=_breadcrumb(rel),
        content=content_html,
        footer=_footer(rel),
    )
    return html_out.encode("utf-8")


# ---------------------------------------------------------------------------
# HTTP handler
# ---------------------------------------------------------------------------
class DocsHandler(BaseHTTPRequestHandler):
    server_version = "FormaDocs/1.0"
    NAV_CACHE = None
    ROOT = ROOT_DIR

    # ---- helpers ----
    @classmethod
    def nav(cls) -> str:
        if cls.NAV_CACHE is None:
            cls.NAV_CACHE = _render_nav(_build_nav(cls.ROOT), "")
        return cls.NAV_CACHE

    def _send_bytes(self, data: bytes, mime: str, status: int = 200) -> None:
        self.send_response(status)
        self.send_header("Content-Type", mime)
        self.send_header("Content-Length", str(len(data)))
        self.send_header("Cache-Control", "no-store")
        self.end_headers()
        self.wfile.write(data)

    def _send_html(self, page: bytes) -> None:
        self._send_bytes(page, "text/html; charset=utf-8")

    def _redirect(self, path: str) -> None:
        self.send_response(302)
        self.send_header("Location", path)
        self.end_headers()

    # ---- routing ----
    def do_GET(self):  # noqa: N802
        parsed = urllib.parse.urlparse(self.path)
        path = urllib.parse.unquote(parsed.path)

        if path == "/":
            self._redirect("/docs/")
            return
        if not path.startswith("/docs/"):
            self._send_html(_render_page("404", "<h1>404</h1><p>Halaman tidak ditemukan.</p>", "", self.nav()))
            return

        rel = path[len("/docs/"):].strip("/")
        fs_path = os.path.abspath(os.path.join(self.ROOT, rel))

        # Proteksi path traversal
        if not (fs_path == self.ROOT or fs_path.startswith(self.ROOT + os.sep)):
            self.send_error(403, "Forbidden")
            return

        if os.path.isdir(fs_path):
            # Coba index README
            readme = os.path.join(fs_path, INDEX_NAME)
            if os.path.isfile(readme):
                self._serve_markdown(readme, posixpath.join(rel, INDEX_NAME))
            else:
                self._serve_dir(fs_path, rel)
            return

        if os.path.isfile(fs_path):
            if fs_path.lower().endswith(".md"):
                self._serve_markdown(fs_path, rel)
            else:
                self._serve_static(fs_path)
            return

        self.send_error(404, "Not Found")

    # ---- renderers ----
    def _serve_markdown(self, abs_path: str, rel: str) -> None:
        try:
            with open(abs_path, "r", encoding="utf-8") as f:
                text = f.read()
        except OSError:
            self.send_error(500, "Cannot read file")
            return

        body, toc = render_markdown(text)
        # Title dari H1 pertama
        m = re.search(r"<h1[^>]*>(.*?)</h1>", body, re.S)
        title = re.sub(r"<[^>]+>", "", m.group(1)).strip() if m else _display_name(os.path.basename(abs_path))
        if toc:
            toc_html = '<div class="toc">' + toc + "</div>"
            body = toc_html + body

        page = _render_page(title, body, rel, self.nav())
        self._send_html(page)

    def _serve_dir(self, abs_dir: str, rel: str) -> None:
        entries = sorted(os.listdir(abs_dir))
        rows = []
        for e in entries:
            if e.startswith("."):
                continue
            full = os.path.join(abs_dir, e)
            is_dir = os.path.isdir(full)
            href = "/docs/" + _escape_path_segment(posixpath.join(rel, e))
            if is_dir:
                href += "/"
                ic = "📁"
                size = ""
            else:
                ic = "📄" if e.endswith(".md") else "🗎"
                try:
                    size = f"{os.path.getsize(full):,} B"
                except OSError:
                    size = ""
            rows.append(
                f'<li><a href="{href}"><span class="ic">{ic}</span>'
                f"{html.escape(_display_name(e) if is_dir else e)}"
                f'<span class="size">{size}</span></a></li>'
            )
        title = _display_name(rel.split("/")[-1]) if rel else "Docs"
        body = (
            f"<h1>{html.escape(title)}</h1>"
            '<ul class="file-list">' + "".join(rows) + "</ul>"
        )
        page = _render_page(title, body, rel, self.nav())
        self._send_html(page)

    def _serve_static(self, abs_path: str) -> None:
        ext = os.path.splitext(abs_path)[1].lower()
        mime = {
            ".png": "image/png",
            ".jpg": "image/jpeg",
            ".jpeg": "image/jpeg",
            ".gif": "image/gif",
            ".svg": "image/svg+xml",
            ".webp": "image/webp",
            ".ico": "image/x-icon",
            ".pdf": "application/pdf",
            ".css": "text/css; charset=utf-8",
            ".js": "text/javascript; charset=utf-8",
            ".json": "application/json; charset=utf-8",
            ".woff2": "font/woff2",
            ".txt": "text/plain; charset=utf-8",
        }.get(ext, "application/octet-stream")
        try:
            with open(abs_path, "rb") as f:
                data = f.read()
        except OSError:
            self.send_error(404, "Not Found")
            return
        self.send_response(200)
        self.send_header("Content-Type", mime)
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def log_message(self, fmt, *args):  # noqa: N802
        sys.stderr.write("[docs] %s - %s\n" % (self.address_string(), fmt % args))


def main() -> None:
    global ROOT_DIR, PORT
    parser = argparse.ArgumentParser(description="Forma Docs Server")
    parser.add_argument("--port", type=int, default=PORT, help="port (default %d)" % PORT)
    parser.add_argument("--dir", default=ROOT_DIR, help="root direktori dokumen")
    parser.add_argument("--host", default="0.0.0.0", help="bind host (default 0.0.0.0)")
    args = parser.parse_args()

    ROOT_DIR = os.path.abspath(args.dir)
    DocsHandler.ROOT = ROOT_DIR
    DocsHandler.NAV_CACHE = None

    if not os.path.isdir(ROOT_DIR):
        print(f"❌ Direktori tidak ditemukan: {ROOT_DIR}", file=sys.stderr)
        sys.exit(1)

    server = ThreadingHTTPServer((args.host, args.port), DocsHandler)
    url = f"http://localhost:{args.port}/docs/"
    print("=" * 56)
    print("  Forma Docs Server")
    print("  Buka di browser:  " + url)
    print(f"  Root dokumen:     {ROOT_DIR}")
    print("  Tekan Ctrl+C untuk berhenti")
    print("=" * 56)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nBerhenti.")
        server.server_close()


if __name__ == "__main__":
    main()
