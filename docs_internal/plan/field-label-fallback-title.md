# Plan — Label field fallback ke `title` entity (authored Form/Table/Wizard/Listing/Report)

**Tanggal**: 2026-09-24 · **Status**: In progress
**Referensi**: `docs/spec/frontend/06-page-kinds.md` §2 & §3,
`docs/renderers/shadcn-shell/02-derivation-engine.md` §2–3,
`renderers/react-shadcn/src/engine/derive.ts`

## Masalah

Keputusan pengguna: **fallback ke `title` entity**.

Di form `promo-form` yang terbuka, caption field tampil sebagai **nama mentah**:
`min_purchase`, `menu_item_id`, `start_date`, `max_uses_per_member`, … padahal
entity `promo` sudah mendeklarasikan `title` untuk semuanya (`Minimum Belanja`,
`Menu Spesifik`, `Mulai Berlaku`, `Batas per Member`). Terukur di DOM:

```
labels: ["code*","name*","branch_id","priority","menu_item_id",
         "start_date","end_date","days_of_week","max_uses_per_member","max_uses_total"]
```

Sebabnya adalah **dua jalur yang tidak konsisten**:

| Jalur                                         | Sumber label                                          | Hasil                          |
| --------------------------------------------- | ----------------------------------------------------- | ------------------------------ |
| Form **derivasi** (entity tanpa `kind: Form`) | `deriveForm()` → `fieldLabel()`                       | `field.title` ✅               |
| Form **authored** (21 file)                   | `resolveForm()` mengembalikan `named.spec` apa adanya | `field.label ?? field.name` ❌ |

Dibuktikan dua arah dengan probe `deriveForm` vs `resolveForm` pada entity yang sama:

```
DERIVED:  ["min_purchase=Minimum Belanja","menu_item_id=Menu Spesifik","start_date=Mulai Berlaku"]
AUTHORED: ["min_purchase=(no label)","menu_item_id=(no label)"]
```

Jadi begitu sebuah entity punya `kind: Form` — biasanya demi urutan/section/
`visible_when`, **bukan** demi label — seluruh labelnya mundur dari `title`
entity ke nama mentah. Itu sebabnya `Jam Mulai`/`Jam Selesai` benar (ditulis
`label:` eksplisit) sementara `min_purchase` tidak.

Skala: **111 dari 161** field entry di 21 form authored tanpa `label:`; pada
authored Table/Wizard/Laporan kecil tapi ada (journal-table 1, visit 1,
close-shift-wizard 4/8). Kontrak `docs/renderers/shadcn-shell/02-derivation-engine.md`
§1 menyatakan output derivasi "**sama persis** dengan tipe manifest hasil YAML,
sehingga kind renderer tidak tahu bedanya derived vs authored" — invarian itu
justru _dilanggar_ di sini: authored kehilangan label yang derived dapat.

## Perubahan

| File                                                  | Perubahan                                                                                                                                                                                         | Effort |
| ----------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------ |
| `engine/derive.ts`                                    | Helper bersama `authoredFieldLabel(field, label, entityField)` → `label` → `title` → fallback `fieldLabel`; `resolveForm()` meng-enrich label; `deriveTable()`/`resolveTable` sejenis untuk kolom | medium |
| `kinds/form/FormRenderer.tsx`                         | Pakai label yang sudah teresolusi (4 situs `field.label ?? field.name`)                                                                                                                           | small  |
| `kinds/table/TableRenderer.tsx`                       | Header kolom pakai title entity bila `col.label` kosong (2 situs)                                                                                                                                 | small  |
| `kinds/wizard/*`                                      | `WizardFormStep`, `WizardRenderer`, `SearchSelect` pakai title entity                                                                                                                             | small  |
| `kinds/listing/ListingRenderer.tsx`                   | Kolom + filter pakai title entity                                                                                                                                                                 | small  |
| `engine/derive.test.ts`                               | Test: authored field tanpa `label` mewarisi `title` entity; `label` eksplisit menang; fallback nama bila title kosong                                                                             | small  |
| `docs/spec/frontend/06-page-kinds.md` §2              | Dokumentasikan fallback label yang mengikat (kontrak, bukan perilaku implisit)                                                                                                                    | small  |
| `docs/renderers/shadcn-shell/02-derivation-engine.md` | Catat invarian derived↔authored ditegakkan untuk label                                                                                                                                            | small  |

## Keputusan

- **Resolusi di satu tempat, bukan di setiap renderer.** 4 situs di FormRenderer
  - 2 di TableRenderer + Wizard/Listing masing-masing menulis
    `label ?? name` sendiri — bentuk "satu kosakata, banyak tempat, satu bolong"
    yang sudah pernah kena di repo ini (kafe 10.25/10.26, `entityActionPermission`
    5.12.4). Resolusi ditaruh di fungsi resolusi (`resolveForm`), sehingga renderer
    tetap tidak perlu tahu derived vs authored.
- **`label` eksplisit YAML selalu menang.** Fallback hanya mengisi yang kosong;
  tidak ada deklarasi penulis yang ditimpa.
- **Fallback terakhir tetap `fieldLabel()`/nama field**, bukan string kosong —
  entity tanpa `title` harus tetap tampil.
- **Tidak** menyentuh `pkg/spec`/schema: `ReportColumn.label` sudah `required`,
  `FormField`/`TableColumn`/`FilterSpec` tidak. Ini murni resolusi renderer;
  menambah `required` akan memaksa 111 situs menulis label yang sudah bisa
  diturunkan.

## Verifikasi

- `npx vitest run` hijau (termasuk test baru); `npx tsc -b` bersih.
- Browser `/kafe/app/pos/cafe-master/promos?action=create&form=promo-form&mode=drawer`
  → caption `Minimum Belanja`, `Menu Spesifik`, `Mulai Berlaku`, `Batas per
Member` (bukan `min_purchase`, dst), sementara `Jam Mulai` yang ditulis
  eksplisit tetap utuh.
- `go test ./...` hijau (tidak ada perubahan Go).
