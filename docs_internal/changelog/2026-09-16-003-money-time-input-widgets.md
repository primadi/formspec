# 2026-09-16-003 — Widget `moneyinput` + `timeinput` (gap #1 / 2.14)

Item `examples/kafe/gaps_found/TODO.md` **2.14** — separuh renderer dari gap #1.
Plan: `docs_internal/plan/money-time-input-widgets.md`.

**Masalah.** `widget:` sudah tertutup sejak 1.4, tetapi `money` dan `time` belum
punya widget: keduanya jatuh ke input teks polos. Kasir mengetik uang sebagai
teks bebas (tanpa mata uang, tanpa numpad, tanpa pratinjau), jam happy hour juga.

**Konstruk.** `moneyinput` dan `timeinput` masuk kosakata tertutup (S10) —
nama mengikuti keluarga yang ada (`fileinput`, `datetimeinput`, `decimalinput`).
Nilai money yang dikirim adalah bentuk kanonik `{amount, currency}`, jumlah
disimpan sebagai **teks** selama mengetik (uang eksak, tidak pernah float —
§2.1), dan tampilannya mengikuti `settings.currency`/`settings.locale` dengan
`inputMode: decimal` (numpad di perangkat sentuh). Override per field
(`currency`, `decimal_places`) dihormati — pratinjau tidak memakai simbol mata
uangkan lain. `timeinput` memakai kontrol `type=time` asli dan menyimpan
`HH:MM:SS`.

**Yang membuatnya benar-benar sampai ke kasir.** Satu detail ikut ditutup:
manifest form tidak menulis `widget:` untuk field uang, dan `FormFieldWidget`
memakai `field.widget ?? entityField.type` — jadi `money` akan tetap jatuh ke
input teks walau widget-nya ada. Fallback itu kini melewati
`implicitWidgetForType` (`money → moneyinput`, `time → timeinput`), sengaja
sempit: memperluasnya ke semua tipe (enum → select, relation → relation-picker)
adalah perubahan tersendiri, dan form turunan sudah lewat `derive.formWidget`.

**Adopsi kafe.** Marker `GAP-01` di seluruh spec kafe ditutup (5 entitas + 4 form

- 1 wizard) — dan dibersihkan dari komentar gabungan `GAP-01/GAP-02`, sehingga
  yang tersisa benar-benar hanya GAP-02 (indeks money).

**Bukti.** `go test ./...` hijau · `vitest` **265** (dari 258 — 7 test baru:
bentuk kanonik, presisi `12.345` tidak dibulatkan, field dikosongkan → `null`,
mode read-only; time menyimpan detik, mempertahankan detik, dan clear ≠ midnight)
· `tsc` bersih · kafe `validate` **0 problem** (69 manifest) · test kosakata
tertutup diperbarui (FormWidget 24 nilai).

**Sisa.** Belum ada verifikasi runtime di browser (numpad/pratinjau hanya diuji
lewat jsdom); `Print` belum memformat money dengan widget yang sama.
