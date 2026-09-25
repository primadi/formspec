# 2026-09-24-008 — Caption field: fallback ke `title` entity (label → title → nama)

## Apa yang diubah

Pertanyaan pengguna pada form promo: "`min_purchase`, `menu_item_id` dll itu
apakah sudah berupa label?" Jawabannya **belum** — caption-nya masih nama field
mentah, padahal entity `promo` sudah mendeklarasikan `title` untuk semuanya
(`Minimum Belanja`, `Menu Spesifik`, `Mulai Berlaku`, …). Keputusan pengguna:
**fallback ke `title` entity**.

**Akar — dua jalur yang tidak konsisten.** `deriveForm()`/`deriveTableColumns()`
memakai `fieldLabel()` yang membaca `field.title`; sedangkan `resolveForm()`
mengembalikan `named.spec` **apa adanya**, sehingga renderer jatuh ke
`field.label ?? field.name`. Dibuktikan dua arah dengan probe pada entity yang
sama: `DERIVED: min_purchase=Minimum Belanja` vs `AUTHORED: min_purchase=(no label)`.
Jadi mendeklarasikan `kind: Form` — biasanya demi urutan, section, atau
`visible_when`, **bukan** demi label — menurunkan seluruh caption dari title
entity menjadi nama mentah. Skala: **111 dari 161** field entry di 21 form
authored tanpa `label:`.

**Perbaikan — satu kosakata, bukan lima.** `engine/derive.ts` kini punya
`entityFieldLabel()` + `humanizeFieldName()` dengan presedensi
`label` manifest → `title` entity → nama di-humanise, plus
`withEntityFieldLabels()`/`withEntityColumnLabels()` yang mengisi yang kosong.
`label` eksplisit selalu menang. Dipasang di **fungsi resolusi**
(`resolveForm`, `resolveTable`) agar renderer tetap tidak perlu tahu derived vs
authored — dan karena lima ejaan berbeda sudah hidup berdampingan:
`fieldLabel()` (derivasi), `field.label ?? field.name` (authored Form/Table/
Wizard/SearchSelect), `label ?? description ?? name` (WizardFormStep — jatuh ke
**deskripsi** saat tanpa title), dan `field.name.replace(/_/g," ")` (DetailPage —
mengabaikan title entity sepenuhnya, bahkan tanpa manifest authored).
Konsumen yang diselaraskan: `FormRenderer`, `TableRenderer`, `WizardRenderer`,
`WizardFormStep`, `SearchSelect`, `ListingRenderer` (kolom + filter),
`ReportRenderer`, `DetailPage`.

Helper mengembalikan **salinan**, tidak memutasi `entry.spec` — entry bundle
dibagi lewat zustand dan satu Page bisa menyematkan Form yang sama dua kali.

## Verifikasi

- **Bukti browser** (kafe `:8099`, login `manajer`), drawer promo:
  `Kode Promo`, `Nama Promo`, `Cabang`, `Prioritas`, `Persentase (%)`,
  `Minimum Belanja`, `Menu Spesifik`, `Mulai Berlaku`, `Selesai Berlaku`,
  `Hari Berlaku`, `Jam Mulai`, `Jam Selesai`, `Batas per Member`, `Batas Total`
  — regex nama mentah (`min_purchase|menu_item_id|applies_to|…`) → **false**.
- Tabel promo: header `Kode Promo` … `Berlaku Untuk`. Detail cabang:
  `Kode Cabang`, `Nama Cabang`, `Cabang Induk`, `Pajak (%)`,
  `Service Charge (%)`, `Header Struk` — sebelumnya `branch_id`/
  `service_charge_percent` gaya mentah.
- `vitest` **377 lulus** (25 file, +12 baru di `derive.test.ts`), `tsc -b`
  bersih, `go test ./...` hijau, `formspec check -f examples/kafe/spec` → 0 error.
- Kontrak: `06-page-kinds.md` §2 (presedensi caption, normatif) +
  `docs/renderers/shadcn-shell/02-derivation-engine.md` §2.

## Catatan

- `ReportColumn.label` sudah `required` di schema, jadi jalur Report bersifat
  defensif; FilterSpec/TableColumn/FormField memang boleh kosong — itulah alasan
  fallback ini kontrak, bukan pilihan gaya.
- Tidak menyentuh `pkg/spec`/JSON Schema: menambah `required` akan memaksa 111
  situs menulis label yang sudah bisa diturunkan.
