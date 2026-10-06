# Synkronisasi angka todo — marker deferred diseragamkan

**Tanggal:** 2026-10-02 · **Plan:** `docs_internal/plan/triase-todo-2026-10-02.md`

## Apa yang diubah

`docs_internal/plan/todo.md` — **dokumen saja**, tidak ada perubahan kode.

1. **Normalisasi marker (34 item).** Item yang teksnya sudah menyatakan deferred
   tetapi masih `- [ ] <id> ⏸️` (30) atau `- [ ]` tanpa marker padahal prosanya
   deferred (4: `3.6.6`, `5.6.7`, `9.3.1`, `13.3.9`) diubah menjadi `- [⏸️]`.
   Marker inline yang menjadi redundan dihapus. Tidak ada teks item yang diubah
   maknanya.
2. **Konsolidasi pelacakan ganda.** `3.7.5` dan baris pengalih `3.7.6` sama-sama
   melacak `backup create --incremental` yang sudah punya pelacak `4.8.6 ⏸️`.
   Keduanya ditutup `[x]` dengan catatan arah, dan rujukan lama di `3.7.1`
   diperbarui supaya tidak ada dua dokumen yang berbeda status.
3. **Blok angka header ditulis ulang** dengan angka terukur + satu recipe.

## Kenapa

Header menyatakan "Sisa **37** item deferred" dengan pengakuan bahwa itu "batas
bawah". Pengakuan itu tidak menyelamatkan pembaca: angka 37 **tidak bisa
direproduksi**, karena deferred hidup di tiga bentuk penandaan sekaligus
(`- [⏸️] <id>`, `- [ ] <id> ⏸️`, `- [ ] ⏸️`) sementara recipe lama hanya membaca
bentuk pertama dan ketiga. Bentuk kedua (30 item) tidak pernah terhitung —
itulah sebabnya angkanya menjadi batas bawah.

## Angka (terukur sebelum → sesudah)

|                        | Sebelum | Sesudah |
| ---------------------- | ------- | ------- |
| `[x]`                  | 583     | 585     |
| `[⏸️]`                 | 39      | 71      |
| `[ ]` (belum deferred) | 47      | 13      |
| **Total terbuka**      | **86**  | **84**  |

Total terbuka turun 2 hanya karena duplikasi dihapus — **tidak ada item yang
diklaim selesai tanpa kode**. Recipe otoritatif kini satu perintah:

```bash
grep -cE '^\s*- \[⏸️\]' docs_internal/plan/todo.md   # 71
grep -cE '^\s*- \[ \]'   docs_internal/plan/todo.md   # 13
grep -cE '^\s*- \[x\]'   docs_internal/plan/todo.md   # 585
```

## Verifikasi yang dilakukan (sampel item "belum ada")

Sebelum menutup/menormalkan, klaim sampel diperiksa ke kode dan **semuanya
akurat**: `3.7.8` (`cmd/formspec/logs.go` tanpa flag lanjutan), `3.7.9`
(`archive.go:37` → `not implemented yet`), `2.11.7a` (tidak ada `/health` di
`vite.config.ts`), `3.6.8` (`RebuildSpec.Window`/`Since` disalin ke plan tetapi
tidak memfilter), `5.18.3` (tidak ada pembacaan `link` di jalur tabel),
`2.1.5` (tidak ada `X-FormSpec-Scope-Id` di `sdk/`), `4.2.8` (tidak ada pembaca
`.star` di validate/check), `7.19.x` (`Mockup` hanya di `pkg/spec` + loader +
generator, tanpa konsumen runtime). Satu koreksi teks: `2.11.10` menyebut
"404 untuk kind Workspace", padahal `schemas/kinds/Workspace.schema.json` sudah
ada lokal — yang terbuka hanya publish ke registry online.

## Sisa / belum tertutup

Triase ini **hanya** menyatukan penandaan dan menghapus duplikasi; 84 item
terbuka tetap terbuka. Re-verifikasi penuh terhadap kode untuk seluruh 84 item
tidak dilakukan — sampel yang diperiksa semuanya masih sah, jadi tidak ada
indikasi penutupan massal yang tertunda. Bila re-verifikasi penuh diinginkan,
ia pekerjaan tersendiri dan belum punya item bernomor.
