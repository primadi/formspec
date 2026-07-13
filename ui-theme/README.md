# Forma UI Themes

Kumpulan tema untuk Forma web. Setiap tema adalah **module standalone** yang bisa dipasang di workspace mana pun.

## Cara Pakai

```bash
# 1. Copy folder tema ke spec/modules/ project
cp -r ui-theme/<nama-theme> path/to/spec/modules/

# 2. Tambahkan nama module ke spec/forma.yaml
#    modules: [..., ocean-blue]

# 3. Jalankan dev server
cd path/to && go run cmd/forma/ dev
```

> **Catatan:** Symlink (`ln -s`) tidak didukung. Go's `filepath.Walk` tidak follow symlinks. Selalu gunakan `cp -r`.

## Daftar Tema

### 🎨 Batik Nusantara
| | |
|---|---|
| **Module** | `batik-theme` |
| **Inspirasi** | Motif batik tradisional Indonesia (Parang, Kawung, Mega Mendung) |
| **Palet** | Cokelat emas, krem alami, biru gelap |
| **Keunikan** | Dark mode dengan pattern titik halus, warm earthy tones |

### 🌊 Ocean Blue
| | |
|---|---|
| **Module** | `ocean-blue` |
| **Inspirasi** | Laut dalam — profesional, tenang, berwibawa |
| **Palet** | Navy, biru terang, slate |
| **Keunikan** | Gradient background, wave SVG pattern, glass card effect |

### 🌙 Midnight Pro
| | |
|---|---|
| **Module** | `midnight-pro` |
| **Inspirasi** | Dashboard SaaS premium |
| **Palet** | Deep slate, violet, neon accent |
| **Keunikan** | **Dark-first**, frosted glass (`backdrop-filter: blur`), glow effects |

### 🌴 Tropical
| | |
|---|---|
| **Module** | `tropical` |
| **Inspirasi** | Pantai tropis — energik, friendly, playful |
| **Palet** | Coral, lime, gold, teal |
| **Keunikan** | **Animated gradient**, bubble pattern, gradient scrollbar |

### 🌲 Forest Mist
| | |
|---|---|
| **Module** | `forest-mist` |
| **Inspirasi** | Hutan kabut — semi-transparent background image showcase |
| **Palet** | Forest green, sage, warm cream |
| **Keunikan** | **Multiple semi-transparent SVG overlays**: pine forest silhouette (opacity 0.04–0.10) + animated mist/fog (opacity 0.08) di atas gradient. Menunjukkan bahwa theme bisa pakai background image asli via data URI di `stylesheet` |

## Membuat Theme Sendiri

Buat folder dengan struktur:

```
my-custom-theme/
├── module.yaml              # kind: Module
├── themes/
│   └── my-theme.yaml        # kind: Theme (Frontend Spec §10)
└── assets/
    └── my-theme.css          # Optional extra CSS
```

### Referensi

- `tokens` → CSS custom properties (`color.primary`, `radius.md`, `font.family`)
- `stylesheet` → CSS bebas (background, gradient, animasi, scrollbar, dll)
- Dark mode → tulis `.dark { ... }` di `stylesheet`
- Light mode override → tulis `:root:not(.dark) { ... }` di `stylesheet`

## Security Policy

- **Hanya YAML manifest** yang bisa di-load dari `spec/modules/`
- `module.yaml` + `kind: Theme` → divalidasi oleh `forma validate`
- Image/asset eksternal → referensi URL absolut di `stylesheet`
- Tidak ada loading dari path di luar workspace root
