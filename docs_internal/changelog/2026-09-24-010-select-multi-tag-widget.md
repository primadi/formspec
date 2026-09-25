# Widget `select-multi-tag` + atribut `Field.options`

**Tanggal:** 2026-09-24 · **Seq:** 010

## Apa

Field `promo.days_of_week` (`type: json`, nilai `[1,2,3,4,5]`) dirender
**editor JSON mentah**: penulis spec harus mengetik array angka, dan `1..7`
tidak pernah terbaca sebagai hari. Sudah ada `widget: tags`, tetapi `tags`
menerima **ketikan bebas** — himpunan yang ditetapkan spec tidak bisa
ditegakkan lewatnya (`9` tetap tersimpan). Permintaan pengguna: "ubah widget
menjadi SelectMultiTag, jadi seperti tag input, tapi dari selection, bukan dari
ketikan; item yg sudah diselect tidak bisa diselect lagi."

Ditutup dengan **dua** kontrak, bukan satu workaround:

1. **Atribut baru `Field.options`** (`[]FieldOption{value,label}`) di Entity —
   `enum_values` hanya membawa nilai tanpa caption, jadi `[1,2]` tampil sebagai
   `1, 2`. `value` mempertahankan tipe skalar (`value: 1` → angka `1`), `label`
   opsional (default humanise). Ditaruh di **Entity** karena "nilai mana yang
   sah" adalah properti **data**, bukan satu form — deklarasi yang sama dipakai
   form, sel tabel, dan halaman detail.
2. **Widget baru `select-multi-tag`** (katalog form 24 → 25) — tag yang
   sumbernya deklarasi, bukan ketikan. Opsi terpilih tidak ditawarkan lagi; chip
   diurutkan menurut **deklarasi**; nilai di luar deklarasi tetap ditampilkan
   (ditandai) dan tidak dibuang saat save; nilai non-daftar → **error terlihat**;
   bentuk nilai dipertahankan (`json` array, `string` comma-separated).
   Field `json` ber-`options` menurunkan widget ini; tanpa `options` tetap
   editor JSON (tidak ada spec lama yang berubah perilaku).

Satu resolver bersama (`lib/field-options.ts`, `renderCell.tsx`) dipakai form,
DetailPage, dan sel tabel/listing — bentuk "satu kosakata, beberapa call-site,
satu bolong" yang menghasilkan 10.25/10.26/10.28 sebelumnya.

## Kenapa bukan sekadar mengubah manifest

Memperbaiki `promo-form.yaml` saja (mis. `widget: tags`) tidak menutup apa pun:
`tags` tetap menerima `9`, dan tidak ada tempat untuk mendeklarasikan bahwa `1`
berarti Senin. Yang dibutuhkan adalah tempat deklarasi + widget yang
membacanya — dan begitu kosakata widget tertutup (S10), nama baru harus
didaftarkan di ketiga sisi yang dijaga `catalog.test.tsx`.

## File

**Backend/spec:** `pkg/spec/entity.go` (`FieldOption`, `Field.Options`,
`validateFieldOptions`, `optionValueKey`), `pkg/spec/widget.go`
(`WidgetSelectMultiTag`), `pkg/spec/field_options_test.go` (baru),
`pkg/spec/widget_test.go`, `internal/genjsonschema/generator.go` (`FieldOption`
ke `sharedTypes` — tanpa itu setiap `Entity.schema.json` menunjuk definisi yang
tidak ada).

**Frontend:** `types/manifest.ts`, `lib/field-options.ts` (baru),
`widgets/SelectMultiTag.tsx` (baru, + `OptionChips`), `widgets/{index.ts,catalog.ts}`,
`kinds/form/FormRenderer.tsx`, `engine/derive.ts`, `lib/renderCell.tsx`,
`kinds/page/DetailPage.tsx`, `widgets/select-multi-tag.test.tsx` (baru, 20 test).

**Contoh/docs:** `examples/kafe/.../promo/entity.yaml` (options 1..7 + label
hari), `.../forms/promo-form.yaml` (`widget: select-multi-tag`),
`docs/spec/backend/05-field-types.md` §1.1.1,
`docs/spec/frontend/07-component-kinds.md` §1 + §1.3,
`docs/renderers/shadcn-shell/03-kind-renderers.md` (24 → 25),
`schemas/*` + `docs/kind/*` (regenerasi).

## Bukti

- Validasi **bukan konvensi**: duplikat nilai ditolak engine —
  `field "days_of_week": options[1] duplicates the value "1" from options[0] — a
duplicate choice can never be selected twice` (diuji dengan spec probe);
  `1` dan `"1"` dianggap sama.
- `formspec validate` kafe: **85 manifest, 0 problem**.
- Browser (`:8099`, login `manajer`/`kafe123`): pilih **Senin** → daftar
  berikutnya `[Selasa…Minggu]` (Senin **hilang**); chip tampil urut deklarasi
  (`Senin, Selasa, Jumat`) meski urutan klik berbeda; detail page menampilkan
  chip berlabel tanpa `<pre>` JSON (`preCount: 0`).
- Bentuk nilai tersimpan terukur: simpan dari UI → API membaca
  `days_of_week: [1, 5, 2]` dengan `types: [int, int, int]` — **angka**, bukan
  `"1"`. Membuka lalu menyimpan ulang **tidak** menulis ulang urutan array
  (urutan hanya diubah saat tampilan).
- `go test ./...` **39 paket hijau** (exit 0) · `vitest` **403 lulus** (27 file,
  +20) · `tsc -b` bersih · `oxlint` tidak ada temuan baru.
- Test dibuktikan **gagal** saat filter "sudah dipilih" dilepas (1 failed).

## Sisa

Kandidat `⏸️` (bukan blocker) didaftarkan sebagai item — lihat
`docs_internal/plan/todo.md` 5.10.17/5.10.18 dan `examples/kafe/gaps_found/TODO.md` 10.33.
