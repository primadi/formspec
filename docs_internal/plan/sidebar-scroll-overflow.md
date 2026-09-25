# Plan — sidebar nav tidak bisa di-scroll (item di bawah tidak terjangkau)

Sumber: laporan runtime pengguna, `/kafe/app/pos/cafe-stock/waste-entries` —
"kalau tidak semua item terlihat seharusnya muncul vertical scroll bar, agar
bisa akses item dibawah". Diukur di browser, bukan disimpulkan dari kode.

## Masalah

`ScrollArea` **sudah** dipasang di `Sidebar.tsx` (`flex-1 py-2`), tetapi ia
tumbuh mengikuti isinya alih-alih dibatasi tinggi `aside`, jadi tidak pernah
ada overflow untuk di-scroll dan `Scrollbar` base-ui tidak ikut ter-mount
(base-ui menyembunyikan track saat `hasOverflowY` false → `shouldRender`
false).

Pengukuran di `aside` (viewport 560px):

```
aside           clientHeight 560   scrollHeight 1383   ← isi melimpah, tak diklip
└ ScrollArea    clientHeight 1354  scrollHeight 1354   ← TUMBUH, bukan dibatasi
  └ viewport     clientHeight 1338  scrollHeight 1338   ← overflow: scroll, tapi tak ada overflow
    └ nav        clientHeight 1334
      item terakhir: top 1076, bottom 1398 (di luar 560)
```

Setelah `min-height: 0` dipaksa lewat devtools pada elemen Root yang sama:

```
ScrollArea root  clientHeight 504   (aside 560 − header 56)
viewport         clientHeight 488   scrollHeight 1338   → scrollable: true
scrollbar        ter-mount, rect 10×504 di x 229.2 (kanan dalam)
```

Akar: `flex-1` = `flex: 1 1 0%`, tetapi `min-height` sebuah flex item tetap
`auto` (content-based) kecuali dimatikan. Jadi `flex-1` tidak membatasi
apa-apa; Root memakai tinggi kontennya. Ini bukan soal `overflow` yang belum
diset — `overflow: scroll` sudah ada di viewport, hanya saja tidak ada yang
perlu di-scroll.

## Perbaikan

`renderers/react-shadcn/src/components/ui/scroll-area.tsx` — Root memakai
`cn("relative min-h-0", className)`.

Kenapa di primitif, bukan di call-site:

- `min-h-0` adalah no-op di luar konteks flex/grid (untuk block box
  `min-height: auto` memang berperilaku `0`), jadi tidak ada efek samping.
- Pola ini menyelesaikan **kelas** cacat, bukan satu instance: `ScrollArea`
  yang dipasang sebagai anak flex column selalu butuh ini. Memperbaiki di dua
  call-site `Sidebar.tsx` saja akan mengulang bentuk 10.25/10.26 (satu
  kosakata, dua tempat, satu bolong) begitu ada pemakaian ketiga.
- Kedua varian sidebar (desktop statis + overlay mobile) memakai struktur yang
  sama, jadi keduanya ikut tertutup.

Tidak ada perubahan pada `Sidebar.tsx`: `flex-1 py-2` adalah niat yang benar.

## Test

`renderers/react-shadcn/src/components/ui/scroll-area.test.tsx` — kunci
invariannya: Root `ScrollArea` membawa `min-h-0` (bisa menyusut di dalam flex
column) sambil tetap mempertahankan `className` pemanggil dan berhenti menjadi
`relative` root. jsdom tidak punya layout, jadi yang diuji adalah kontrak
kelas yang menjadi sebab overflow — sama seperti `tableColumn.test.tsx`.

## File

- `renderers/react-shadcn/src/components/ui/scroll-area.tsx` (akar)
- `renderers/react-shadcn/src/components/ui/scroll-area.test.tsx` (baru)
- `docs/renderers/shadcn-shell/03-kind-renderers.md` §2 (perilaku sidebar-nav)

## Verifikasi

`npm run build` di `renderers/react-shadcn/`, lalu ukur ulang di browser pada
`/kafe/app/pos/cafe-stock/waste-entries`: root 504 vs viewport 488 (scrollable),
`scrollbar` ter-mount, dan item terbawah terjangkau setelah di-scroll.

## Todo

kafe **10.31** (temuan, rinci) + master **5.20.1** (pointer, di bawah section
**5.20** `Shell — sidebar nav harus bisa di-scroll`) — Fase 5, karena cacatnya
ada di shell dan berlaku untuk semua App dengan `sidebar-nav`.
