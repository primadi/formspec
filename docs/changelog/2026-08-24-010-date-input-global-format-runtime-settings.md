# Date Input Global Format + Runtime-Editable Settings

**Tanggal**: 2026-08-24 · **Plan**: `docs/plan/date-input-global-format-runtime-settings.md`

## Apa yang diubah

### Fase 1 — Input tanggal ikut global `date_format`

- **`DateInput.tsx`** di-rewrite mendukung **dua metode input**:
  - **Ketik manual**: text input editable — ketik tanggal sesuai global
    `date_format` (mis. `24/08/2026` untuk DD/MM/YYYY), di-parse ke ISO via
    `parseDateByPattern()` (baru di `lib/format.ts`) dan tersimpan; blur
    mengembalikan ke tampilan format kanonik. **Auto-format separator**: ketik
    `24082026` otomatis jadi `24/08/2026` (separator `/` di-insert bertahap
    via `formatDateInput()` baru di `lib/format.ts`).
  - **Kalender**: tombol ikon kalender (atau Space/Enter) memanggil
    `showPicker()` pada native `<input type="date|datetime-local">` tersembunyi
    (sr-only) → membuka kalender browser; onChange meng-update nilai ISO.
    Fallback `native.focus()+click()` bila `showPicker()` tidak didukung.
  - Tambah prop `className`.
  - Catatan: pendekatan awal (overlay native input `opacity-0` di atas text
    input) tidak andal membuka picker di semua browser — diganti `showPicker()`.
- **`lib/format.ts`**: tambah `parseDateByPattern(input, pattern)` — parse
  string tanggal per token pattern (YYYY/MM/DD/HH/mm/ss) → ISO; tolak input
  tidak lengkap / tanggal mustahil (31/02). Tambah `formatDateInput(raw,
pattern)` — auto-insert separator saat mengetik (`24082026` →
  `24/08/2026`).
- **3 input tanggal mentah diganti `DateInput`**: `KanbanRenderer` (filter),
  `TableRenderer` (filter date/date_range), `wizard/SearchSelect` (field date).

### Fase 2 — Runtime-editable settings (Entity-backed, Configuration Page)

- **Entity `app-setting`** (baru, `examples/cafe/spec/modules/formspec.core/
entities/app-setting.yaml`) — `characteristic: reference`, natural key `name`
  ("global"), field: `currency_code`, `currency_decimal_places`,
  `currency_symbol`, `locale`, `timezone`, `date_format`, `decimal_scale`,
  `rounding`.
- **Form `settings-edit`** (baru) — `mode: edit` atas `app-setting`.
- **Page `settings`** diubah dari HTML statis → Configuration Page (form block
  `{ ref: settings-edit, id: "global", mode: edit }`).
- **Backend merge** (`internal/api/meta.go` `mergeRunningSettings`) — handler
  `/meta/ui` membaca record `app-setting` (find-or-create, natural key
  "global") dan merge di atas manifest `settings:` → `bundle.settings`
  mencerminkan **running value** dari DB. Field kosong fallback ke manifest.
- **Auto-apply** (`FormRenderer.tsx`) — setelah simpan `app-setting`, refresh
  meta bundle → seluruh UI re-render dengan settings baru.
- **Fix bug** (`FormRenderer.tsx`) — widget resolution `field.widget ??
entityField.type` tidak handle `integer`/`decimal` (hanya `number`), sehingga
  field integer di authored form tanpa `widget: number` dirender sebagai text
  input → zod `z.number()` gagal ("Must be a number"). Ditambah
  `case "integer": case "decimal":` → NumberInput.

## Kenapa

- Input tanggal native selalu ikut locale browser (mm/dd/yyyy), tidak konsisten
  dengan global `date_format` (DD/MM/YYYY).
- Global settings sebelumnya hanya di manifest YAML (tidak bisa diubah dari UI).
  Sesuai pola Configuration Page spec §1: settings dimodelkan sebagai Entity
  `reference` + record sentinel; manifest `settings:` jadi seed/default, DB
  menyimpan running value yang bisa diubah admin.

## Jawaban: simpan running value di DB?

**Ya.** Ini pola resmi spec (Configuration Page): Entity `reference` + record
sentinel. Manifest `settings:` tetap source of truth deklaratif (default), DB
menyimpan nilai berjalan yang bisa diubah admin via UI — dengan permission
model + auto-generated admin UI gratis.

## File terdampak

- `renderers/react-shadcn/src/widgets/DateInput.tsx`
- `renderers/react-shadcn/src/kinds/{kanban/KanbanRenderer,table/TableRenderer,wizard/SearchSelect,form/FormRenderer}.tsx`
- `internal/api/meta.go` (mergeRunningSettings)
- `examples/cafe/spec/modules/formspec.core/{entities/app-setting.yaml,forms/settings-edit.yaml,pages/settings.yaml}`

## Test

- `go test ./...` — semua pass.
- `npx vitest run` — 113 pass.
- `formspec validate --schema schemas` — 22 manifest cafe pass.
- Verified via browser: form Pengaturan edit → save → DB ter-update → meta
  bundle auto-apply → tabel Orders berubah format tanggal (DD/MM/YYYY →
  YYYY-MM-DD saat diubah).
