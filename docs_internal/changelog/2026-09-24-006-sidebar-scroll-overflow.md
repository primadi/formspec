# 2026-09-24-006 — Sidebar nav akhirnya bisa di-scroll (menu panjang terjangkau)

**Plan**: `docs_internal/plan/sidebar-scroll-overflow.md`
**Todo**: kafe **10.31**, master **5.20.1**.

## Konteks

Dilaporkan pengguna: "kalau tidak semua item terlihat seharusnya muncul vertical
scroll bar, agar bisa akses item dibawah". Diukur di browser pada App `kafe-pos`
(`/kafe/app/pos/cafe-stock/waste-entries`), bukan disimpulkan dari kode.

`ScrollArea` **sudah** dipasang di `Sidebar.tsx` sejak awal, jadi setiap
deklarasi **tampak** benar — `flex-1` (`flex: 1 1 0%`) meminta sisa tinggi, dan
Viewport base-ui sudah membawa `overflow: scroll`. Pengukuran menunjukkan dua
duanya tidak berarti:

```
aside           clientHeight  560   scrollHeight 1383
└ ScrollArea    clientHeight 1354   ← TUMBUH mengikuti isi, bukan diklip
  └ viewport    clientHeight 1338   scrollHeight 1338   → tak ada yang di-scroll
    item terakhir: bottom 1398 (viewport 560px)
```

Akarnya bukan `overflow` yang lupa diset: `min-height` sebuah flex item tetap
`auto` (content-based) kecuali dimatikan, jadi `flex-1` tidak meng-clamp apa
pun — Root memakai tinggi kontennya, `aside` (`overflow: visible`) ikut meluber,
dan Scrollbar tidak di-mount karena `hasOverflowY` false
(`shouldRender` → `null`). Item di bawah y=560 tak punya jalan dijangkau.

## Yang diubah

- **`src/components/ui/scroll-area.tsx`** — Root memakai `min-h-0`. Diperbaiki di
  **primitif**, bukan di dua call-site: `min-h-0` no-op di luar konteks
  flex/grid (`min-height: auto` sudah berperilaku `0` untuk block box), dan ini
  menutup **kelas** cacatnya — `ScrollArea` yang dipasang sebagai anak flex
  column selalu butuh ini. Memperbaiki `Sidebar.tsx` saja akan mengulang bentuk
  10.25/10.26 begitu ada pemakaian ketiga. `Sidebar.tsx` tidak disentuh:
  `flex-1 py-2` adalah niat yang sudah benar.
- **`src/components/ui/scroll-area.test.tsx`** (baru) — mengunci invarian untuk
  **kedua** varian sidebar (desktop + overlay mobile), plus bahwa Viewport tetap
  `size-full` dan nav benar-benar berada di dalam ScrollArea.
- **`docs/spec/frontend/05-app-kinds.md` §2** + **`docs/renderers/shadcn-shell/03-kind-renderers.md`**
  — kontraknya kini tertulis: tree menu yang lebih tinggi dari viewport harus
  bisa dijangkau seluruhnya.

## Bukti terukur (sesudah, browser yang sama)

| Ukur                  | Sebelum                            | Sesudah                                  |
| --------------------- | ---------------------------------- | ---------------------------------------- |
| ScrollArea root       | clientHeight **1354**              | clientHeight **504** (= 560 − header 56) |
| Viewport              | 1338 / 1338 → **tidak** scrollable | 488 / 1338 → **scrollable**              |
| Scrollbar base-ui     | tidak ter-mount                    | ter-mount, **10×504** di kanan dalam     |
| `aside.scrollHeight`  | 1383 (meluber)                     | **560** (diklip)                         |
| Item terbawah         | bottom **1398** (di luar 560)      | bottom **548** setelah scroll            |
| Wheel di atas sidebar | —                                  | `scrollTop` 0 → **850** (= maxScroll)    |

Test: `npx vitest run src/components/ui/scroll-area.test.tsx` → 4 lulus; dengan
`min-h-0` dilepas kembali, 2 case **gagal** (test mengunci regresinya).
`npm run build` hijau; dist dibangun ulang.
