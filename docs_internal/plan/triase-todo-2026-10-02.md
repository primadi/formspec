# Triase ulang todo.md — 2026-10-02

Plan kerja untuk permintaan: _"Triase ulang + sinkronkan angka header todo."_

## Masalah yang dikoreksi

Header `docs_internal/plan/todo.md` menyatakan **"Sisa 37 item deferred"** dengan
catatan bahwa angka itu "batas bawah". Catatan itu benar tetapi tidak
menyelamatkan pembaca: angka 37 tidak bisa direproduksi dengan satu perintah,
karena penandaan deferred di file ini **tiga bentuk**:

| Bentuk                                | Jumlah (terukur)           | Terbaca oleh recipe lama? |
| ------------------------------------- | -------------------------- | ------------------------- |
| `- [⏸️] <id> …`                       | 39                         | ya                        |
| `- [ ] <id> ⏸️ …` (marker setelah id) | 30                         | **tidak**                 |
| `- [ ] ⏸️ …` (marker setelah box)     | 8 (bagian dari 30 di atas) | ya                        |

Recipe lama di header hanya menyebut `^\s*- \[⏸️\]` (39) + `^\s*- \[ \] ⏸️` (8),
sehingga bentuk kedua (30 item) tidak pernah terhitung — itulah kenapa muncul
angka "37" lalu "batas bawah".

## Prinsip

1. **Satu bentuk penandaan.** Semua item yang teksnya sudah menyatakan deferred
   diubah checkbox-nya menjadi `- [⏸️]`, dan marker inline yang menjadi
   redundan dihapus. Sesudah itu satu perintah menjadi otoritatif.
2. **Jangan mengubah semantik.** Item `[ ]` yang memang "belum mulai, tidak
   deferred" (mis. 10.5.x MCP server) **tidak** diubah menjadi `[⏸️]`.
3. **Konsolidasikan pelacakan ganda.** Satu masalah di dua item membuat hitungan
   tidak bisa dipertanggungjawabkan.

## Temuan verifikasi (sampel item "belum ada")

Diperiksa langsung ke kode; semuanya **akurat dan tetap terbuka**:

- **3.7.8** `logs` filter lanjutan — `cmd/formspec/logs.go` hanya punya jalur
  unknown-flag.
- **3.7.9** `archive restore-batch` — `cmd/formspec/archive.go:37` mencetak
  `not implemented yet`.
- **2.11.7a** `/health` tidak ada di `renderers/react-shadcn/vite.config.ts`.
- **3.6.8** `RebuildSpec.Window`/`Since` dideklarasikan + disalin ke
  `plan.Window/Since` (`internal/summary/rebuild.go:43,137`) tetapi tidak dipakai
  untuk memfilter — jadi belum benar-benar parsial.
- **5.18.3** tidak ada pembacaan `link` di jalur tabel.
- **2.1.5** tidak ada `X-FormSpec-Scope-Id` di `sdk/` mana pun.
- **4.2.8** tidak ada pembaca `.star` di `internal/validation`/`check`.
- **7.19.1/7.19.2** `Mockup` hanya muncul di `pkg/spec`, loader, dan generator
  schema/dokumen — tidak ada konsumen runtime.
- **2.11.10** `schemas/kinds/Workspace.schema.json` **sudah ada secara lokal**;
  yang terbuka hanya publish ke `schemas.formspec.dev` (item tetap sah, tetapi
  teks "404 untuk kind Workspace" perlu dibaca sebagai "schema lokal ada, belum
  ter-publish").
- **9.3.1** sudah punya catatan status 2026-09-22 yang menyebut
  `renderers/web/` **sudah tidak ada** dan `make generate` masih stub → item ini
  menunggu keputusan (mirror hand-written vs generated), jadi masuk deferred.

## Tindakan

1. **Normalisasi marker** — 34 item: 30 `- [ ] <id> ⏸️` → `- [⏸️] <id>`
   (marker inline redundan dihapus) + 4 item yang prosanya menyatakan deferred
   tanpa marker (`3.6.6`, `5.6.7`, `9.3.1`, `13.3.9`). Teks lain tidak disentuh.
2. **Konsolidasi duplikat**:
   - `3.7.5` dan `4.8.6` melacak masalah yang sama (`backup create
--incremental`). Tutup `3.7.5` sebagai `[x]`, arahkan ke `4.8.6`.
   - `3.7.6` adalah item pengalih ("lihat 4.8.6/4.8.7") — sisa isinya sudah
     ditutup atau dilacak di tempat lain. Tutup sebagai `[x]`, arahkan ke 4.8.6.
3. **Tulis ulang blok angka di header** dengan angka terukur + satu recipe yang
   benar-benar mereproduksinya.
4. **Changelog** `docs_internal/changelog/2026-10-02-010-sinkronisasi-angka-todo.md`.

## File yang disentuh

- `docs_internal/plan/todo.md` (marker + header + konsolidasi)
- `docs_internal/changelog/2026-10-02-010-sinkronisasi-angka-todo.md`

## Efek ke hitungan

|                        | Sebelum | Sesudah                                    |
| ---------------------- | ------- | ------------------------------------------ |
| `[x]`                  | 583     | **585**                                    |
| `[⏸️]`                 | 39      | **71** (39 − 2 ditutup + 34 dinormalisasi) |
| `[ ]` (belum deferred) | 47      | **13**                                     |
| **Total terbuka**      | **86**  | **84**                                     |

Total terbuka turun 2 karena duplikasi dihapus — triase ini **tidak** mengklaim
ada pekerjaan yang selesai; ia membuat angkanya bisa direproduksi dan menghapus
pelacakan ganda. Recipe hasil akhir:

```bash
grep -cE '^\s*- \[⏸️\]' docs_internal/plan/todo.md   # 71
grep -cE '^\s*- \[ \]'   docs_internal/plan/todo.md   # 13
grep -cE '^\s*- \[x\]'   docs_internal/plan/todo.md   # 585
```

## Effort

Small (dokumen saja, tanpa perubahan kode).
