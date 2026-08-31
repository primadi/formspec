# Date Input Global Format + Runtime-Editable Settings

**Status**: ✅ Complete (2026-08-24)
**Referensi spec**: `docs/spec/backend/01-core-basic.md` §10 (Config & Global
Settings), `docs/spec/frontend/06-page-kinds.md` §1 (Configuration Page),
`docs/spec/backend/05-field-types.md` §2 (money).

## Masalah

1. **Input tanggal tidak konsisten dengan global `date_format`.** Widget
   `DateInput` memakai native `<input type="date">` yang selalu mengikuti
   locale browser (mm/dd/yyyy), bukan `settings.date_format` (DD/MM/YYYY).
   Tiga input lain bahkan tidak memakai `DateInput` sama sekali:
   `KanbanRenderer` (filter), `TableRenderer` (filter date/date_range),
   `wizard/SearchSelect` (field date).

2. **Global settings tidak bisa diubah dari UI.** `settings:` hanya ada di
   manifest `kind: Config` (YAML). Halaman Pengaturan statis (read-only).
   Tidak ada cara update runtime + auto-apply.

## Solusi

### Fase 1 — DateInput ikut global date_format

Rewrite `DateInput` dengan pola **overlay native picker**:

- Input teks (read-only) menampilkan nilai sesuai `formatter.date()` /
  `formatter.dateTime()` (global `date_format`).
- Native `<input type="date|datetime-local">` di-overlay (opacity 0) di atasnya
  — klik di mana pun membuka kalender native; onChange meng-update nilai ISO.
- Ganti 3 input mentah (`KanbanRenderer`, `TableRenderer`, `SearchSelect`)
  dengan `DateInput`.

### Fase 2 — Runtime-editable settings (Entity-backed)

Mengikuti pola **Configuration Page** spec §1: settings dimodelkan sebagai
Entity `characteristic: reference` dengan record sentinel. Manifest `settings:`
menjadi seed/default; DB menyimpan **running value**.

1. **Entity `app-setting`** (module `formspec.core`, characteristic `reference`)
   — field: `name`, `currency_code`, `currency_decimal_places`,
   `currency_symbol`, `locale`, `timezone`, `date_format`, `decimal_scale`,
   `rounding`.
2. **Seed** — saat startup, find-or-create record sentinel dari manifest
   `settings:` (nilai manifest jadi default awal).
3. **Merge ke meta bundle** — handler `/meta/ui` membaca record `app-setting`
   dan merge di atas manifest defaults → `bundle.settings` mencerminkan
   running value.
4. **Halaman Pengaturan** — `kind: Page` + `kind: Form` `mode: edit` atas
   `app-setting` (id sentinel). Simpan → update DB.
5. **Auto-apply** — setelah simpan, frontend refresh meta bundle → formatter
   baca `bundle.settings` baru → seluruh UI re-render.

## Jawaban: simpan running value di DB?

**Ya.** Ini pola resmi spec (Configuration Page): Entity `reference` + record
sentinel. Manifest `settings:` tetap source of truth deklaratif (seed/default),
DB menyimpan nilai berjalan yang bisa diubah admin via UI. Memberi permission
model + auto-generated admin UI gratis.

## File yang Diubah

| File                                                                 | Perubahan                                              |
| -------------------------------------------------------------------- | ------------------------------------------------------ |
| `renderers/.../widgets/DateInput.tsx`                                | Rewrite: overlay native picker + display global format |
| `renderers/.../kinds/kanban/KanbanRenderer.tsx`                      | Pakai `DateInput`                                      |
| `renderers/.../kinds/table/TableRenderer.tsx`                        | Pakai `DateInput`                                      |
| `renderers/.../kinds/wizard/SearchSelect.tsx`                        | Pakai `DateInput`                                      |
| `examples/cafe/spec/modules/formspec.core/entities/app-setting.yaml` | **Baru** — Entity settings                             |
| `resource/formspec.go`                                               | Seed settings entity dari manifest                     |
| `internal/api/meta.go`                                               | Merge running value ke bundle                          |
| `examples/cafe/spec/modules/formspec.core/pages/settings.yaml`       | Form edit settings                                     |
| `examples/cafe/spec/modules/formspec.core/forms/settings-edit.yaml`  | **Baru** — Form edit                                   |

## Level of Effort

- Fase 1 (DateInput + 3 file): **small**
- Fase 2 (Entity + seed + merge + page): **medium-large**

## Dependensi

- Fase 2 butuh pemahaman entity registry API (GetByID/Insert) untuk seed.
- Setelah `pkg/spec` berubah → `make generate-schema`.
