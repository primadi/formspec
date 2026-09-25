# Plan — Field `help`: warisan `description` entity + tutup situs bolong

Sumber: pertanyaan pengguna pada `promo-form.yaml` — "di entity field ada
`description`, di form field ada `help`, apa yg ditampilkan di ui?"

**Status**: In progress · **Tanggal**: 2026-09-25
**Referensi**: `docs/spec/frontend/06-page-kinds.md` §2,
`docs/renderers/shadcn-shell/02-derivation-engine.md` §2,
`renderers/react-shadcn/src/engine/derive.ts`

## Masalah

Dua kosakata untuk hal yang sama — `Field.description` (Entity) dan
`FormField.help` (Form) — **tidak terhubung di jalur authored**. Hanya form
hasil _derivasi_ yang membaca `description`:

```
renderers/react-shadcn/src/engine/derive.ts:493
  if (field.description) ff.help = field.description
```

Baris itu hidup di dalam `formField()`, yang hanya dipanggil `deriveForm()`.
Form authored tidak pernah lewat sana: `resolveForm()` mengembalikan
`named.spec` apa adanya (hanya label yang di-enrich lewat
`withEntityFieldLabels()`).

Akibatnya, begitu entity punya `kind: Form` — biasanya demi urutan, section,
atau `visible_when`, **bukan** demi help — setiap `description` entity hilang.
Penulis spec harus menulis ulang info yang sama dua kali, dan itu memang yang
terjadi. Terukur di `examples/kafe`:

| Form                     | Field          | `help:` di form                                                              | `description:` di entity                                                                            |
| ------------------------ | -------------- | ---------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------- |
| `promo-form.yaml:19`     | `branch_id`    | "Kosongkan untuk berlaku di semua cabang."                                   | `promo/entity.yaml:64` "Kosong = berlaku di semua cabang"                                           |
| `promo-form.yaml:20`     | `priority`     | "Dipakai memilih satu promo terbaik bila beberapa cocok (aturan bisnis #7)." | `promo/entity.yaml:108` "Dipakai memilih satu promo terbaik bila beberapa cocok (aturan bisnis #7)" |
| `menu-item-form.yaml:20` | `prep_station` | "Menentukan tiket muncul di bar atau dapur."                                 | `menu-item/entity.yaml:63` "Dasar routing tiket di layar dapur (KDS)"                               |
| `menu-item-form.yaml:29` | `is_available` | "Matikan cepat saat bahan habis."                                            | `menu-item/entity.yaml:68` "Saklar cepat kasir saat bahan habis"                                    |

Dua baris pertama **identik** verbatim — salinan manual yang tinggal menunggu
drift. Skala: `examples/kafe` punya **81** `description:` pada field entity dan
**35** `help:` pada form authored.

Kelas yang sama sudah ditutup untuk `label`/`title` (plan
`field-label-fallback-title.md`, changelog `2026-09-24-008`). Ini sisi
`help`-nya, plus tiga lubang yang tidak bisa ditutup oleh warisan saja.

## Lubang tambahan (independen dari warisan)

`help` hanya dirender di **tiga** tempat, dan satu di antaranya bolong di lima
dari enam cabangnya:

| Situs                                                        | Perilaku hari ini                                                             |
| ------------------------------------------------------------ | ----------------------------------------------------------------------------- |
| `kinds/form/FormRenderer.tsx:754`                            | `<p class="text-xs text-muted-foreground">` di bawah widget, digate `!isView` |
| `kinds/form/AuthFormRenderer.tsx:255`                        | selalu tampil                                                                 |
| `kinds/wizard/WizardFormStep.tsx:178`                        | **hanya di cabang field `relation`**                                          |
| `kinds/wizard/WizardRenderer.tsx:322-339`                    | jalur `steps[].fields` inline — **tidak ada help sama sekali**                |
| `Table` / `Listing` / `DetailPage` / `Kanban` / `ChildTable` | **tidak ada konsumsi** `help` maupun `description`                            |

`WizardFormStep.renderField` punya enam cabang — `relation` (:150), `enum`
(:211), `boolean` (:233), `date`/`datetime` (:253), `integer`/`decimal` (:267),
default string (:287). Hanya yang pertama merender `help`, sehingga
`close-shift-wizard.yaml` menampilkan help untuk `supervisor_id` (relation)
tetapi **tidak** untuk `counted_cash`/`note` (money/string) di step yang sama —
kontradiksi yang terlihat pengguna dalam satu layar.

Satu lagi: `entity.description` juga hanya dibaca jalur derivasi
(`derive.ts:166`), sementara `OverlayHost.tsx:123/146` memakai
`sections[0].description ?? "Fill in the details for this <entity>."` — jadi
drawer/dialog form authored selalu jatuh ke string Inggris generik meski
entity sudah punya deskripsi (`promo` mendeklarasikan "ATURAN promo (bukan
pemakaian) — …" yang tidak pernah tampil).

## Keputusan (dikonfirmasi pengguna)

1. **Cakupan: perbaiki situs bolong + warisan. Tanpa permukaan baru** —
   `help` **tidak** ditambahkan ke Table/Listing/DetailPage/Kanban.
2. **`metadata.description` entity ikut diwariskan** ke `sections[0]`, sama
   seperti jalur derivasi.
3. **Anotasi `// @schema {...}` dilengkapi** pada `FormField.Help` dan
   `Field.Description`, lalu `schemas/` + `docs/kind/` di-regenerate.

## Konstruk

**Satu resolver, di fungsi resolusi.** Pelajaran repo ini berulang kali (kafe
10.25/10.26, `entityActionPermission` 5.12.4): "satu kosakata, banyak tempat,
satu bolong". Menaruh resolusi di tiap renderer akan mengulang bentuk itu —
`FormRenderer` punya satu jalur, `WizardFormStep` punya jalur lain
(`useMetaStore.getForm()` → raw bundle entry), `WizardRenderer` jalur ketiga.

`src/engine/derive.ts`:

```ts
// Menggantikan withEntityFieldLabels().
export function withEntityFieldDefaults(
  spec: FormSpec,
  entity: EntitySchema,
): FormSpec
```

- Tiap field: isi `label` yang kosong (`entityFieldLabel` — perilaku sekarang)
  **dan** `help` yang kosong dari `entityField.description`.
- `sections[0].description` yang kosong diisi `entity.description`.
- Mengembalikan salinan (entry bundle dibagi lewat zustand; satu Page bisa
  menyematkan Form yang sama dua kali).
- `formField()` (baris 493) memakai aturan yang sama supaya vocab-nya tidak
  punya dua ejaan.

Dipasang di **keempat** `return` `resolveForm()` (explicitRef, mode-specific,
generic, derive), sehingga `FormRenderer`/`OverlayHost`/`PageRenderer` tidak
perlu tahu derived vs authored — invarian §1 `02-derivation-engine.md`.

Untuk Wizard, resolusi dilakukan di titik baca form: `WizardFormStep.tsx:46`
dan jalur inline `WizardRenderer.tsx`. Bentuknya: helper baru
`resolveFormFields(spec, entity)` (atau memakai `withEntityFieldDefaults`
langsung pada spec wizard) yang dipakai kedua situs.

| File                                                                    | Perubahan                                                                                                                                                           | Effort |
| ----------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------ |
| `engine/derive.ts`                                                      | `withEntityFieldDefaults()` menggantikan `withEntityFieldLabels()`; `resolveForm()` ens enrich help + section-0 description; `formField()` memakai aturan yang sama | medium |
| `engine/derive.test.ts`                                                 | Test warisan help + description + non-mutasi + presedensi                                                                                                           | small  |
| `kinds/wizard/WizardFormStep.tsx`                                       | Resolve help dari entity; render `field.help` di **keenam** cabang                                                                                                  | small  |
| `kinds/wizard/WizardRenderer.tsx`                                       | Jalur `steps[].fields` inline merender help                                                                                                                         | small  |
| `examples/kafe/.../promo-form.yaml`, `menu-item-form.yaml`              | Hapus `help:` yang menduplikasi `description` entity                                                                                                                | small  |
| `pkg/spec/frontend.go`, `pkg/spec/entity.go`                            | `@schema {description: ...}` pada `FormField.Help` + `Field.Description`                                                                                            | small  |
| `schemas/`, `docs/kind/`                                                | Regenerate (`make generate-schema`, `make generate-kind-docs`)                                                                                                      | small  |
| `docs/spec/frontend/06-page-kinds.md` §2                                | Paragraf "Help field (normatif)" — mirror aturan caption                                                                                                            | small  |
| `docs/renderers/shadcn-shell/02-derivation-engine.md` §2                | Perluas blok "Caption field" jadi "Caption & help"                                                                                                                  | small  |
| `docs/kind/ui/Form.md`                                                  | Satu bullet Gotchas (di luar blok generated)                                                                                                                        | small  |
| `ai_skills/entity-authoring/SKILL.md`, `ai_skills/form-layout/SKILL.md` | Nyatakan `description` = teks user-facing                                                                                                                           | small  |

## Urutan

1. Resolver + `resolveForm` (blocking — semua fase lain bergantung).
2. Wizard (paralel dengan 3, setelah 1).
3. De-redundansi contoh kafe (paralel dengan 2).
4. Anotasi schema + regenerate (setelah 1; tidak bergantung 2/3).
5. Kontrak docs + skill (setelah 1–2 terkunci).

## Verifikasi

- Test baru di `engine/derive.test.ts`, pola blok `withEntityFieldLabels`
  (baris 209–245): (a) authored field tanpa `help` mewarisi `description`
  entity; (b) `help` eksplisit menang; (c) entity tanpa `description` → tidak
  ada `help`; (d) `sections[0].description` kosong mewarisi
  `entity.description`; (e) `resolveForm` tidak memutasi `entry.spec`.
  Dibuktikan gagal sebelum patch.
- `npx vitest run` hijau (baseline 403 lulus) · `npx tsc -b` bersih.
- **Browser** (`:8099`, `manajer`), drawer promo: subtitle drawer menampilkan
  deskripsi entity `promo`, bukan `"Fill in the details for this promo."`;
  `branch_id`/`priority` tetap menampilkan help walau deklarasinya sudah
  dihapus dari YAML.
- **Browser** wizard close-shift, step "Hitung Uang Fisik": help
  `counted_cash` **dan** `note` muncul (dua cabang berbeda: money + string).
- `go test ./...` hijau · `formspec check -f examples/kafe/spec` → 0 error.
- `git diff schemas/ docs/kind/` → kolom Deskripsi `help`/`description` terisi.

## Risiko / keputusan terbuka

1. **Kualitas teks `description`.** Warisan menyeluruh akan menampilkan
   deskripsi yang ditulis sebagai catatan developer, bukan kalimat pengguna.
   Terukur di `examples/kafe`:
   - `cafe-order/transaction/order/entity.yaml:79` — "Denormalisasi untuk
     tampilan cepat"
   - `order/entity.yaml:137` — "compute dari branch.service_charge_percent"
   - `gl/entities/gl-balance.yaml:34` — "computed — opening + debit - credit
     (asset) atau opening + credit - debit (liability/revenue)"
   - `promo/entity.yaml:77` — "Array angka 1=Senin..7=Minggu, mis. [1,2,3,4,5]"

   Rencana: fitur **landing**, lalu sisa teks menjadi item `⏸️` bernomor
   dengan contoh-contoh ini sebagai bukti — bukan klaim bahwa semuanya sudah
   bersih. Penegakan jangka panjang lewat `@schema` description + skill
   (fase 4/5) yang menyatakan `description` = teks user-facing.

2. **Subtitle form authored multi-section.** Mengisi `sections[0].description`
   membuat `OverlayHost` otomatis benar, tetapi untuk form 4-section seperti
   `promo-form` deskripsi entity menjadi satu-satunya teks pengantar sebelum
   "Identitas Promo". Keputusan: pakai satu aturan (isi `sections[0]`,
   tidak menambah field bundel); bila terbaca menyempil, perbaiki di form
   yang bersangkutan.
3. **`!isView` pada `FormRenderer.tsx:754`.** Help tetap disembunyikan di mode
   view (argumen: help adalah alat input), dan itu **didokumentasikan
   eksplisit** di fase 5 — mengubahnya menyentuh permukaan di luar lingkup.

## Bukan bagian plan ini

- `TableColumn.description` — tidak ada; atribut baru butuh keputusan kontrak.
- Tooltip/ikon `?` pada permukaan read-only.
- Validasi baru di `formspec validate` yang menolak `help` duplikat.
