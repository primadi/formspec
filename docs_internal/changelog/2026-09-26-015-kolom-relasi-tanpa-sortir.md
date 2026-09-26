# Kolom relasi tidak menawarkan sortir yang ditolak server (todo 5.18.5)

## Apa yang diubah

`TableColumn.sortable: true` pada kolom relasi menjanjikan urutan yang **tidak
bisa** diberikan API. Dua kegagalan sekaligus, keduanya terukur:

1. kolomnya merender nama target (`branch.name`) sedangkan nilai tersimpannya
   adalah **UUID** foreign key — jadi "urutkan" berarti mengurutkan UUID;
2. bentuk dot-path-nya **ditolak**: `?sort=branch.name` → `422 unknown field`,
   karena `checkField` (`internal/api/handler.go:543`) hanya menerima field
   entity dan kolom normatif.

Manifest nyata memang mendeklarasikannya — `cafe-stock/stock-level-table.yaml`
(`{ field: branch_id, sortable: true }`, `ingredient_id`), dan tabel kunjungan
klinik (`{ field: patient.name, sortable: true }`) — jadi tombol sortirnya ada,
tampak bekerja, dan gagal saat diklik.

**Keputusan yang dipakai adalah opsi (b) yang item ini sendiri sebutkan**:
matikan tombol sortir untuk kolom relasi, alih-alih membiarkannya berbohong.
Dua tempat:

- `engine/derive.ts` `isSortable` — sudah mengecualikan `relation`, kini dengan
  komentar **mengapa** (sebelumnya hanya daftar tipe; pembaca tidak bisa tahu
  ini disengaja atau kelalaian).
- `kinds/table/TableRenderer.tsx` — guard `isRelationColumn(entity, col.field)`
  pada `enableSorting`. **Ini yang sebenarnya menutup bug**, karena jalur
  derivasi sudah benar: yang salah adalah tabel **authored** yang menulis
  `sortable: true` sendiri. Helper-nya me-resolve **akar** nama kolom
  (`branch.name` → `branch`/`branch_id`) terhadap entity, jadi alias dan
  dot-path sama-sama tertangkap.

**Opsi (a) — sortir relasi yang sungguh bekerja** — tidak diambil: ia butuh JOIN
di `PersistBackend` (skala medium–large, dan tidak ada jalur JOIN di sana hari
ini). Dicatat sebagai item terbuka, bukan diaku selesai.

## Kenapa

Item 5.18.5 memberi dua pilihan dan meminta keputusan kontrak; memilih (b) berarti
pekerjaan kecil dan jujur, memilih (a) berarti membuka pekerjaan besar. Memilih
(b) **tanpa** mencatat (a) sebagai terbuka akan menjadi klaim palsu — jadi
keduanya dilakukan.

## File terdampak

- `renderers/react-shadcn/src/engine/derive.ts` — komentar `isSortable`
- `renderers/react-shadcn/src/kinds/table/TableRenderer.tsx` —
  `isRelationColumn` + `enableSorting`
- `renderers/react-shadcn/src/engine/derive.test.ts` — +1 test
- `renderers/react-shadcn/src/kinds/table/relation-sort.test.ts` — **baru** (3 test)

## Bukti

- `npx tsc -b` bersih; `npx vitest run` **519 lulus** / 38 file (baseline sesi ini
  516; +3 dari file ini, +1 di derive.test.ts).
- **Dibuktikan gagal:** guard `enableSorting` dikembalikan ke `col.sortable ?? true`
  → test `applies the guard to enableSorting` gagal, hijau sesudah dikembalikan.
- **Catatan kalibrasi (kesalahan saya sendiri, dibiarkan tercatat):** percobaan
  pertama saya menambahkan `if (field.type === "relation") return false` di
  `isSortable` dan menulis test untuk itu — tetapi test-nya **lulus tanpa patch**,
  karena daftar tipe di fungsi itu memang sudah tidak memuat `relation`. Guard itu
  no-op. Bug-nya ada di jalur **authored**, dan test yang benar menargetkan
  `TableRenderer`, bukan `derive`. Test pertama dihapus, yang kedua menggantikannya.

## Rujukan

Todo **5.18.5** (opsi b selesai; opsi a → item terbuka di bawah) ·
`docs/spec/frontend/06-page-kinds.md` §3.1/§3.3 (filter `field` "boleh dot-path
relation" — sortir tidak, dan kini renderer setuju) · 5.18.4 (kolom relasi
sebelumnya menampilkan UUID).
