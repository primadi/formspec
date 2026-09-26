# `formspec generate`: `options` menjadi literal union (todo 5.10.22)

## Apa yang diubah

Field skalar ber-`options` kini digenerate sebagai **literal union**, bukan tipe
terbuka. Sebelumnya `tsFieldType` (`cmd/formspec/generate.go`) membaca
`EnumValues` saja dan tidak pernah melihat `Options`, sehingga

```yaml
- name: days_of_week
  type: json
  multiple: true
  options:
    [
      { value: 1, label: Senin },
      { value: 2, label: Selasa },
      { value: 7, label: Minggu },
    ]
- name: channel
  type: string
  multiple: false
  options: [{ value: qris }, { value: cash }]
```

digenerate menjadi `unknown` dan `string` — pemanggil boleh menulis nilai yang
tidak dideklarasikan server, padahal `options` ada justru untuk mencegahnya
(5.10.19 menetapkannya sebagai kontrak **data**, level Entity).

**Cardinality menentukan bentuk**, karena ia properti data (`Field.Multiple`),
bukan properti satu form:

| Deklarasi                                | Digenerate                               |
| ---------------------------------------- | ---------------------------------------- |
| skalar dengan `options` bernilai tunggal | `1 \| 2 \| 7`, `"qris" \| "cash"`        |
| `type: json` + `multiple: true`          | `Array<1 \| 2 \| 7>`                     |
| `type: string` + `multiple: true`        | `string` — wire form-nya comma-separated |

Baris terakhir adalah **penolakan yang disengaja**: bentuk di wire adalah string
berisi daftar, dan tidak ada tipe string TypeScript yang bisa mempersempitnya ke
anggota himpunan. Menghasilkan union di sana akan menjadi janji palsu — kode
tidak akan menolak `"9"` yang masuk. Karena itu syaratnya diperketat: union
hanya untuk nilai tunggal, atau `Array<>` untuk `json` yang multi.

Label **tidak pernah** ikut digenerate: union adalah tipe, bukan tempat caption.
Renderer membacanya dari manifest saat runtime.

## Kenapa

Menutup item 5.10.22 (sisa dari 5.10.19). Kalimat rincinya sudah menyebut akarnya
(`tsFieldType` "tidak ada rujukan `Options`"), jadi ini pekerjaan yang terukur —
bukan keputusan desain baru.

## File terdampak

- `cmd/formspec/generate.go` — `tsOptionType` + `tsOptionUnion` (baru), cabang
  `string`/`integer`/`json`; import `strconv`
- `cmd/formspec/generate_options_test.go` — **baru** (4 test)
- `docs/cli-tools/03-formspec-generate.md` — tabel pemetaan + section `options`

## Bukti

- **Terukur pada spec sungguhan** (spec dua-field sementara, `formspec generate
--lang typescript`):
  ```ts
  "days_of_week"?: Array<1 | 2 | 7> | null;
  "channel"?: "qris" | "cash" | null;
  ```
- `TestTsFieldType_ScalarOptionsBecomeLiteralUnion` dan
  `TestTsFieldType_OptionCardinality` **dibuktikan gagal** saat cabang union
  dikembalikan ke tipe terbuka, hijau sesudahnya.
- `TestTsOptionUnion_MixedAndUnrepresentable` mem-pin **fallback**: opsi nil /
  non-skalar menghasilkan `""` (tipe terbuka), bukan union rusak yang tidak bisa
  dikompilasi.
- `go build ./...` bersih; `gofmt -l` bersih; `go test ./cmd/formspec/` hijau.

## Rujukan

Todo **5.10.22** (tertutup) — sisa dari **5.10.19** (cardinality `options`).
