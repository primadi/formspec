# 2026-09-09-002 — TagsInput value-type-aware (JSON array) + aktif di form user

## Apa

- `renderers/react-shadcn/src/widgets/TagsInput.tsx`: value kini polimorfik —
  menerima `string | string[]`. Array masuk → array keluar (entity field
  `type: json`), string masuk → comma-joined string keluar (perilaku lama
  tidak berubah). Normalisasi via `parseTags`/`serializeTags`.
- `FormRenderer.tsx` case `tags`: oper value mentah; field `json` dipaksa
  shape array walau kosong (mode create) agar string tidak bocor ke kolom JSON.
- `internal/auth/module/master/user/forms/{create,edit}.yaml`: field `roles`
  dan `permissions` kini `widget: tags` — input modern ketik → Enter → badge
  dengan tombol X (bukan JSON textarea lagi).

## Kenapa

Form manajemen user (Roles & Permissions) sebelumnya menampilkan JSON array
mentah via JsonInput — tidak ergonomis. Widget TagsInput sudah ada sebelumnya
tetapi hanya mendukung string comma-separated sehingga tidak bisa dipakai untuk
field json.

## File terdampak

- `renderers/react-shadcn/src/widgets/TagsInput.tsx`
- `renderers/react-shadcn/src/kinds/form/FormRenderer.tsx`
- `internal/auth/module/master/user/forms/create.yaml`
- `internal/auth/module/master/user/forms/edit.yaml`

## Catatan

- Widget `tags` diaktifkan via `widget:` pada FormField (Form kind), karena
  Entity Field tidak punya attr widget override.
- Perlu restart dev server (formspec.core embedded) agar YAML form baru terload.
