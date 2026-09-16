# Plan — widget `moneyinput` + `timeinput` (kafe TODO 2.14 / gap #1)

Sumber: `examples/kafe/gaps_found/TODO.md` 2.14, gap **#1** (separuh renderer).
Prasyarat: tidak ada (aritmetika `money` sudah ditetapkan di 1.3/S7).

## Masalah

`widget:` sudah jadi kosakata tertutup sejak 1.4, tetapi field `money` dan
`time` **belum punya widget**: keduanya jatuh ke input teks polos. Akibatnya
kasir mengetik uang sebagai teks bebas — tanpa mata uang, tanpa numpad, tanpa
pratinjau terformat — dan jam (happy hour) sebagai teks bebas juga. Form tutup
shift adalah titik paling terasa.

## Konstruk

| Hal                   | Keputusan                                                                                                                                                  |
| --------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Nama widget           | `moneyinput`, `timeinput` — mengikuti keluarga `fileinput`/`datetimeinput`/`decimalinput` (bukan `money-input`)                                            |
| Nilai money           | bentuk kanonik `{amount, currency}`; angka/string lama tetap diterima lalu di-upgrade saat diedit                                                          |
| Presisi saat mengetik | jumlah disimpan sebagai **teks** (money eksak — tidak pernah float, §2.1)                                                                                  |
| Mata uang/skala       | `settings.currency`/`settings.locale`; override per field (`currency`, `decimal_places`) dihormati, dan pratinjau tidak memakai simbol mata uang lain      |
| Time                  | kontrol `type=time` asli (picker di perangkat sentuh), disimpan `HH:MM:SS`                                                                                 |
| Turunan               | `derive.formWidget`: `money → moneyinput`, `time → timeinput`; manifest form yang tidak menulis `widget:` juga diarahkan ke sana (`implicitWidgetForType`) |
| Validation            | zod: money menerima bentuk kanonik + number + string; time `^\d{2}:\d{2}(:\d{2})?$`                                                                        |

## File

- `pkg/spec/widget.go` (+`WidgetMoneyInput`, `WidgetTimeInput`, list, alias hints).
- Klien: `widgets/MoneyInput.tsx`, `widgets/TimeInput.tsx` (baru), `widgets/index.ts`,
  `widgets/catalog.ts`, `engine/derive.ts`, `kinds/form/FormRenderer.tsx`
  (dispatch + fallback tipe), `lib/zod-schema.ts`, `types/manifest.ts`
  (`currency`, `decimal_places`).
- Schema diregenerasi; marker `GAP-01` di spec kafe ditutup (entitas + form + wizard).
- Docs: `docs/spec/frontend/07-component-kinds.md` §1.2.

## Bukti

`go test ./...` hijau · `vitest` **265** (dari 258; 7 test baru untuk kedua
widget) · `tsc` bersih · kafe `validate` 0 problem · test kosakata tertutup
diperbarui (FormWidget 24).

## Estimasi: **medium** (2 komponen + kosakata + dispatch + zod + marker kafe)
