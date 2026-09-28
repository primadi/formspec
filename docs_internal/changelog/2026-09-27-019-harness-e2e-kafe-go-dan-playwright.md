# 2026-09-27-019 — Harness E2E kafe: Go in-process + Playwright (harness browser pertama)

**Apa:** Skenario pemilik (tamu scan QR → pilih nasi goreng → bayar QRIS → dapur
→ meja `served` → tambah es teh → bayar → `served` → kasir `release`) kini
**diekskusi**, bukan dibaca. Dua lapis, karena keduanya menjawab pertanyaan
berbeda:

| Lapis         | File                                                | Menjawab                                                                |
| ------------- | --------------------------------------------------- | ----------------------------------------------------------------------- |
| Go in-process | `resource/kafe_table_lifecycle_e2e_test.go`         | apakah **semantik**-nya benar (status meja, idempotensi, keunikan sesi) |
| Playwright    | `renderers/react-shadcn/e2e/kafe-lifecycle.spec.ts` | apakah **tombol yang diklik manusia** benar-benar bekerja               |

Ini harness browser **pertama** di repo ini: sebelumnya semua verifikasi browser
bersifat ad-hoc di luar repo (dicatat di `docs_internal/`), sehingga tidak ada
yang gagal saat build bila jalur UI rusak. Dengan dua bug nyata yang langsung
ditemukannya (`2026-09-27-018`), biaya itu sudah terbukti terbayar sekali.

**Jalankan:** `make e2e-kafe` · `make e2e-deps` (sekali per mesin) ·
`npx vitest run` untuk test jsdom yang sudah ada.

## Rancangan: kenapa hermetic, bukan "pakai dev DB"

Pilihan awal sesi ini adalah memakai dev DB `examples/kafe/.formspec/kafe.db`.
Saat implementasi, itu **terbukti menghasilkan false green**, jadi dibalik dan
alasannya dicatat di sini:

- Dev DB adalah fixture yang **bermutasi** — menumpuk order dan sesi. Sejak
  `table-session` punya partial unique "satu sesi TERBUKA per meja" (10.34c),
  run kedua gagal karena alasan yang tidak ada hubungannya dengan kode.
- Lebih buruk: `reuseExistingServer` mengambil **server yang sudah listen**.
  Terukur dalam sesi ini: server debug dengan DB lama dipakai ulang, sehingga
  daftar meja memuat **dua baris `A-01` di satu cabang** (satu `available`, satu
  `occupied`) — assertion membaca baris yang tak pernah disentuh dan melaporkan
  meja yang **sebenarnya sudah terisi** sebagai `available`. Kegagalan yang
  tampak seperti bug produk, padahal artefak harness.

Karena itu: DB sementara yang di-seed ulang per run (`mkdtempSync` +
`formspec seed`), dan **`reuseExistingServer: false`** di kedua `webServer`.
Keduanya dijaga agar port tetap default (`:8080`/`:5173`) sehingga tidak
berebut dengan dev server yang sedang jalan.

**Bonus temuan:** pencarian meja lewat `code` juga salah **terlepas dari harness** —
`code` hanya unik per cabang (JKT punya `A-01`, BDG juga). Assertion kini
me-resolve lewat `qr_token` yang unik, dan itu sekaligus menguji 10.34b.

## Apa yang sengaja UI dan apa yang sengaja API

Langkah 1–3 dan 5–8 adalah **klik** (itu jalur renderer yang bisa rusak oleh
refactor — alasan file ini ada). Langkah 4 (drag KDS) dan perubahan status meja
dilewatkan API, dengan alasan teknis, bukan demi kemudahan:

- Board KDS adalah board `@dnd-kit`: drag butuh gesture pointer nyata, dan klik
  kartunya **bernavigasi** (bukan membuka dialog), jadi tidak ada affordance
  kedua. Yang distabilkan adalah setengah yang jujur — board **menampilkan**
  pesanan yang sudah lunas (aturan bisnis #1) — lalu transisinya lewat API.
- Tombol transisi meja ada di halaman detail **derived** yang segmen route-nya
  bergantung identitas record; mengkliknya akan menguji derivasi route, bukan
  transisi. Semantik transisinya sudah dipin oleh test Go; di sini yang diassert
  adalah **hasil** yang dilihat kasir.

**Pembagian peran dipatuhi, bukan dilewati.** `mark-served` dijalankan sesi
`pelayan` (POS), `in_kitchen`/`ready` sesi `dapur` (KDS). Versi pertama memakai
kds untuk `served` → **403** `missing permission: cafe-order.orders.mark-served`;
itu benar dan berguna (memvalidasi pemisahan peran dari `-016`), jadi test
menghormatinya alih-alih memakai token serba-bisa.

## Instalasi di dev container (dokumentasikan, karena tidak trivial)

`~/.cache` dimiliki root di container ini, jadi:
`PLAYWRIGHT_BROWSERS_PATH=/tmp/pw-browsers npx playwright install chromium`, lalu
`install-deps chromium` (pustaka sistem — glib/atk/gbm/pango/… — sebelumnya tidak
ada, dan Chromium gagal start tanpa pesan yang jelas). Keduanya dibungkus
`make e2e-deps`. `install-deps` butuh `sudo`; `sudo` mereset `PATH`, jadi target
itu memakai path `node` absolut.

**File:** `resource/kafe_table_lifecycle_e2e_test.go`,
`renderers/react-shadcn/{playwright.config.ts, e2e/kafe-lifecycle.spec.ts,
e2e/scripts/start-backend.mjs, package.json}`, `Makefile` (`e2e-kafe`,
`e2e-deps`)
