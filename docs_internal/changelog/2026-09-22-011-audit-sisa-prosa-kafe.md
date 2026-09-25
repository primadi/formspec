# 2026-09-22-011 — Audit sisa prosa kafe: 7 item bernomor + koreksi klaim basi

**Tanggal:** 2026-09-22 · **Trigger:** pertanyaan pemilik — "semua TODO untuk kafe
apakah sudah ditutup semua? `gaps_found` apa juga sudah ditutup semua?"

## Yang ditemukan

Menjawab pertanyaan itu secara jujur menemukan **tiga kelas masalah**, bukan satu:

1. **4 item kafe memang masih terbuka** (2.15 ⏸️ kartu meja QR, 6.3 ⏸️ cross-app
   grant, 6.4 ⏸️ kepemilikan `publishes`, 9.4 🟡 walkthrough) — ini keadaan yang
   benar dan beralasan, bukan cacat pencatatan.
2. **7 pekerjaan terbuka hanya hidup sebagai prosa** `**Sisa (dicatat):**` di
   bawah item `[x]`, tanpa nomor — melanggar `AGENTS.md` §3 (tidak bisa di-grep,
   jadi misinformasi begitu entry lain menutupnya).
3. **Tiga klaim dokumen sudah kedaluwarsa** dan masih mengumumkan sesuatu sebagai
   belum beres padahal sudah jalan (dan sebaliknya).

## Tindakan

**A. Fase 10 baru di `examples/kafe/gaps_found/TODO.md`** — 5 item untuk 7 sisa
prosa (dua di antaranya digabung karena satu paket):

| Item | Isi | Asal prosa |
| --- | --- | --- |
| 10.1 ⏸️ | landed cost belum dimodelkan | 6.5(c) |
| 10.2 ⏸️ | integrasi purchase → jurnal belum dinyatakan | 6.5(b) |
| 10.3 ⏸️ | adopsi konteks sesi kafe parsial (grants + `employee.assignments`→`user.assignments`) | 3.8 |
| 10.4 ⏸️ | verifikasi runtime di browser belum pernah dijalankan | 2.5, 2.14 |
| 10.5 ⏸️ | tiga sisa lain (alur QR dua langkah, kolom turunan `REAL` SQLite, dok kontrak REST tulis tangan) | 2.2, 3.2, 2.7 |

**B. 6 item baru di master todo** (prosa yang **bukan** khas kafe — kelas
engine/DX, karena itu masuk fase masing-masing):

| Item | Isi | Asal |
| --- | --- | --- |
| 4.2.6 ⏸️ | `dml`+`ddl` satu manifest belum satu transaksi (jalur PG belum diverifikasi) | kafe 3.4 |
| 4.4.4 ⏸️ | blokir cross-category di jalur baca masih log+skip, bukan hard error | kafe 3.7 |
| 5.13.6 ⏸️ | wiring runtime `ApprovalInbox` (engine mengisi item dari step) | "Sisa 5.3" |
| 6.5.9 ⏸️ | layar pemilih konteks sesi + pengalih header + `localStorage` | plan tahap 4, kafe 3.8 |
| 7.4.8 ⏸️ | test PATCH untuk interception approval (7.4.7 hanya menutup jalur custom action) | kafe 1.7 |
| 8.1.7 ⏸️ | peringatan workspace aktif belum dipasang di `formspec serve`/`resource` | kafe 2.8 |

Dua kandidat **dibuang** setelah diperiksa: `HookDecl.uses` (sudah ditutup 4.7 —
`pkg/spec/entity.go:1381`) dan pemetaan `scope_field` ke kolom `text` (perilaku
yang **disengaja**, bukan item terbuka). Satu kandidat dialihkan ke item yang
sudah ada (thumbnail `transform` → `7.17.3 ⏸️`) supaya tidak ada dua nomor untuk
satu pekerjaan.

**C. Prosa basi ditandai tertutup** (bukan dihapus — jejak tetap berguna):

- **kafe 1.6** — "GAP-36 tetap: constraint tidak bisa merapikan duplikat lama"
  ditulis **sebelum** `dml` ada (3.4, 2026-09-16); skenario itu justru **diuji
  bisa gagal** sesudahnya (3 baris → 2 baris).
- **kafe 1.7** — "tidak ada test level-API untuk interception" → tertutup 7.4.7
  (2026-09-22), yang sekaligus menemukan bug requester self-approve.

**D. Koreksi klaim kedaluwarsa:**

| Lokasi | Klaim lama | Kenyataan (diukur 2026-09-22) |
| --- | --- | --- |
| `todo.md` 3.6.6 | "semua proyeksi `kafe` digerakkan integrator (`cafe-gl-integrator`), `rebuild` melaporkan `orphaned`" | `grep "^kind: Integrator" examples/kafe/spec` → **0 hasil**; ketiga proyeksi kini `kind: Subscription` (`gl/subscriptions/sales-to-journal.yaml`), jalurnya durabel lewat outbox |
| `todo.md` 3.6.7 | "3 problem untuk ketiga summary kafe" | **78 manifest, 7 problem**, dan **tidak satu pun** pada summary: `unit` (ingredient, recipe), `hooks/0` (stock-movement), `emit` (4 transisi order), `steps[0]` (order-void-approval), `body[5]/[7]` (`qrcode` di Print). `--schema schemas` → 0 problem |
| `kafe/docs/overview.md` | "seluruh keterbatasan mesin yang ditemukan di `gaps_found/` sudah diselesaikan" (ditulis 2026-09-14) | sebagian besar, bukan seluruhnya — daftar terbuka kini menyebut 2.15/6.3/6.4/9.4 + Fase 10 |
| `kafe/gaps_found/README.md` | ringkasan "sisa terbuka" masih memuat #13/#14/#16/#17/#21/#32/#34 | keenamnya **tertutup** (7.3, 7.4, 8.3+8.7, 4.1, 4.4, 4.7, 6.5); tabel status final ditambahkan, #45 sisa diarahkan ke 2.15 |

## File yang tersentuh

- `examples/kafe/gaps_found/TODO.md` — Fase 10 baru, header, 11 rujukan bernomor
- `examples/kafe/gaps_found/README.md` — baris #13/#14/#21/#32/#34/#45 + tabel status final
- `examples/kafe/docs/overview.md` — klaim "sudah diselesaikan" dikoreksi
- `docs_internal/plan/todo.md` — 6 item ⏸️ baru, 2 koreksi klaim, header

## Verifikasi

`grep "Sisa (dicatat)"` di ledger kafe → setiap baris menyebut item bernomornya ·
`formspec validate --spec examples/kafe/spec` → 7 problem (registry ketinggalan,
`--schema schemas` → 0) · `grep -c "\[⏸️\]" docs_internal/plan/todo.md` naik 18 → 24.

## Referensi

Audit metode & lanjutan: `docs_internal/plan/audit-open-items-prosa.md` (audit
pertama, 2026-09-22, master todo) → changelog ini adalah versi untuk ekosistem
kafe. Aturan yang melandasi: `AGENTS.md` §3 (pekerjaan terbuka WAJIB item `[⏸️]`
bernomor).
