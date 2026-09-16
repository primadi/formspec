# Plan — Natural key ber-scope (kafe TODO 3.6 / gap #9)

Sumber: `examples/kafe/gaps_found/TODO.md` 3.6. _Accept:_ `ORD-` mulai dari 1 di
tiap cabang.

## Masalah

`natural_key_rule.scope_field` sudah membuat deret **terpisah per cabang** pada
jalur otomatis `create`, tetapi:

- jalur script (`ctx.next_key`) mengirim scope kosong → deret global;
- index unik natural key adalah `(tenant_id, _number)` — **tanpa cabang** → begitu
  deret cabang kedua mulai dari 1, insert-nya menabrak nomor cabang pertama;
- scope field belum tentu punya kolom turunan, sehingga index yang menyebut
  `_branch_id` gagal dibuat.

Ketiganya harus dibereskan bersama; memperbaiki salah satu saja menghasilkan
"deret benar, jaminan salah".

## Yang ditetapkan

| Aturan                                       | Alasan                                                                                |
| -------------------------------------------- | ------------------------------------------------------------------------------------- |
| `ctx.next_key(field, scope=<nilai>)`         | script-lah yang tahu scope yang sedang ditulis; jalur otomatis membacanya dari record |
| Rule ber-scope + scope kosong → **error**    | deret global terlihat benar sampai dua cabang bertabrakan                             |
| Index unik = `(tenant_id, <scope>, <field>)` | deret ber-scope tanpa jaminan ber-scope = tabrakan saat cabang kedua mulai dari 1     |
| Scope field otomatis dapat kolom turunan     | index merujuk kolom fisik; field-nya hidup di payload JSONB                           |

## File

- `internal/starlark/context.go`, `executor.go`, `internal/action/script.go`,
  `resource/formspec.go` — scope pada jalur script.
- `internal/entity/registry.go` — `GenerateNaturalKey(..., scope)` + tolakan scope kosong.
- `renderers/jsonb-persist/ddl.go` — index unik ber-scope + kolom turunan untuk scope field.
- `renderers/jsonb-persist/persist_backend.go`, `migrate.go` — kontrak `NextKey(..., scope)`.
- `internal/entity/natural_key_scope_test.go` + fixture `registry_fixtures/scoped-counter/spec`.
- `internal/starlark/next_key_scope_test.go`.
- `docs/spec/backend/04-persist-backend.md` §2.

## Bukti

Runtime: empat create anonim (B1, B2, B1, B2) → `00001`/`00001`/`00002`/`00002`,
semua `201`; nomor yang sama hidup di dua cabang. Sebelum: B2 → `500 UNIQUE`.
Unit: 3 test counter + 2 test Starlark. `go test ./...` hijau · kafe `validate` 0.

## Sisa

`scope_field` bertipe `relation` dipetakan ke kolom `text` berisi id referensi —
keunikan mengikuti id, bukan kode cabang.

## Estimasi: **medium** (melebar dari perkiraan: tiga lapis, bukan satu)
