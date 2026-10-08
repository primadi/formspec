# Plan — Pemicu cetak deklaratif (`row_action.view`) + kartu nomor meja kafe

**Status:** disetujui pemilik (bentuk **A** terkunci, 2026-10-08).
**Menutup:** todo **5.25.9 ⏸️** (kind `Print` tanpa pemicu UI) · kafe **2.15 ⏸️** (kartu meja QR).
**Bentuk A:** table authored + row action eksplisit. Alternatif B (derive.ts
auto-menambahkan row action "Cetak" untuk tiap `Print` yang entity-nya cocok)
ditolak — ia mengawinkan engine `derive` ↔ `prints` dan mengubah perilaku
derived **semua** entity.

## Masalah

`kind: Print` bisa dideklarasikan dan rutenya hidup
(`router.tsx` → `{basePath}/print/{name}` + `.../print/{name}/:id`), tetapi
**tidak ada satu pun** tempat di klien yang menavigasi ke sana
(`grep -rn 'print/' src/kinds src/engine` → 0). Jadi kasir harus mengetik
`/kafe/app/pos/print/receipt-thermal/<order-id>` sendiri.

Sisi server **sudah menyediakan** pintu masuknya: `internal/ui/validate.go`
`builtinRowActions` memuat `print` (dan `export`) sejak awal — action yang
"renderer-provided, tidak butuh backing entity action". Yang belum ada hanya
**sisi klien** + cara menyatakan **ke Print yang mana**.

## Solusi

Tambah satu field deklaratif generik pada `TableAction`:

```yaml
row_actions:
  - {
      action: print,
      label: "Cetak Kartu",
      icon: printer,
      view: "print:table-tent-card",
    }
```

- `action: print` — penanda builtin yang **sudah** di-whitelist validator.
- `view: "<kind>:<name>"` — target view resource; memakai kosakata
  `{kind}:{name}` yang sudah dipakai `roles.yaml`, `registered_views`, dan
  `GrantsEditor`.
- Klien: bila `action.view` ada, **navigasi** (bukan POST). Record id
  ditambahkan untuk kind yang rutenya menerima `:id` (`print`).

Mengapa generik, bukan `print:` khusus: kosakata domain tidak boleh masuk
`pkg/spec` (aturan pemilik 2026-10-07). `view:` juga melayani laporan/kanban/
timeline yang sama-sama routable.

## Fase 1 — Framework (medium)

| #   | File                                                                                           | Perubahan                                                                                                                                                                                                                           |
| --- | ---------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | `pkg/spec/frontend.go`                                                                         | `TableAction.View string` (`view,omitempty`) + `@schema`. Struct dipakai `Table.row_actions`/`bulk_actions` + `Kanban.row_actions` → satu perubahan melayani tiga situs.                                                            |
| 2   | `internal/ui/validate.go`                                                                      | Bila `View != ""` → parse `kind:name`, pastikan entri ada di registry & kind routable (himpunan sama dengan `cmd/formspec/validate_dangling.go` `viewKinds`). Gagal → error. Cek nama action tetap jalan (`print` builtin → lolos). |
| 3   | `renderers/react-shadcn/src/types/manifest.ts`                                                 | `TableAction.view?: string`.                                                                                                                                                                                                        |
| 4   | **baru** `renderers/react-shadcn/src/engine/viewTarget.ts`                                     | `parseView`, `resolveViewRoute(bundle, view)`, `viewTakesId(kind)`, `canDoViewTarget`.                                                                                                                                              |
| 5   | `renderers/react-shadcn/src/kinds/table/TableRenderer.tsx` + `kinds/kanban/KanbanRenderer.tsx` | Cabang `action.view` **sebelum** guard `canDoEntityAction` (action `print` bukan entity action).                                                                                                                                    |
| 6   | `make generate-schema` + `make generate-kind-docs`                                             | Diff = atribut `view`.                                                                                                                                                                                                              |

## Fase 2 — Spec kafe (medium, depends Fase 1)

| #   | File                                                               | Isi                                                                                                                                                                            |
| --- | ------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 7   | **baru** `spec/modules/cafe-master/prints/table-tent-card.yaml`    | `entity: cafe-master.dining-table`, `output: {format: html, paper: {size: A5}}`, header `MEJA {code}`, body fields + `qrcode` payload `/kafe/t/{qr_token}` (`absolute: true`). |
| 8   | **baru** `spec/modules/cafe-master/tables/dining-table-table.yaml` | Table authored Meja + row action `print`. **Wajib** mereproduksi row action transisi derived (kalau tidak, tombol transisi hilang).                                            |
| 9   | `apps/kafe-pos.yaml` / `formspec.core/seeds/roles.yaml`            | Verifikasi apakah print ikut bundle + grant; tambah bila perlu.                                                                                                                |

## Fase 3 — Workflow (small)

Plan ini · changelog `2026-10-08-005` · tutup todo `5.25.9` & kafe `2.15` ·
rekonsiliasi dengan `docs_internal/plan/kafe-qr-table-session-flow.md`.

## Verifikasi

1. `formspec validate --spec examples/kafe/spec --schema schemas` = baseline 89/0.
2. `formspec check -f examples/kafe/spec`.
3. `gofmt -l` · `go test ./pkg/... ./internal/ui/...`.
4. `npx tsc -b && npx vitest run` — test pemicu **dibuktikan gagal** tanpa cabang `view`.
5. Live: Meja → Cetak Kartu → A5 `MEJA A-01` + QR → `/kafe/t/<token>`.

## Di luar cakupan (tetap ⏸️)

Generator `qr_token` · QR ber-scope sesi (10.35a, PIN tamu 10.38/10.39) ·
`table-qr-page` berbasis `asset` · varian thermal · landing 10.20.
