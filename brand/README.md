# Usulan Logo FormSpec

> Status: **proposal** — belum diterapkan ke `site/`, `docs-site/`, maupun
> `renderers/react-shadcn`. Preview visual: buka `brand/preview.html` di browser
> (`"$BROWSER" brand/preview.html`).

## Konteks

Logo saat ini (`site/src/components/Nav.tsx`, `site/public/favicon.svg`,
`docs-site/public/favicon.svg`) adalah **monogram "M"** — path huruf M putih di
atas kotak rounded dengan gradien **indigo `#6366f1` → emerald `#10b981`**.

Masalah: monogram satu huruf tidak menceritakan apa yang FormSpec lakukan
(_spec-first, declarative, form + spec_).

## Tiga konsep

| Konsep             | Mark                        | Makna                                  | Karakter                       |
| ------------------ | --------------------------- | -------------------------------------- | ------------------------------ |
| **A — Spec Stack** | 3 batang horizontal menurun | Bidang _form_ + lapisan YAML bersarang | Abstrak, paling khas, scalable |
| **B — Spec Brace** | Kurung kurawal `{`          | _Spec_ / deklarasi / kontrak           | Simbolis, teknis               |
| **C — F Monogram** | Huruf F geometris           | Evolusi minimal dari "M"               | Literal, familiar              |

### Rekomendasi

**Konsep A — Spec Stack.** Alasan:

1. **Menceritakan produk** — "Form" (baris = field input) sekaligus "Spec"
   (lapisan = struktur deklaratif yang bersarang).
2. **Tetap terbaca kecil** — 3 baris tebal masih jelas di favicon 16px,
   berbeda dengan brace (B) yang mulai kabur.
3. **Khas & modern** — bukan monogram generik, dan bukan ikon "docs/checkmark"
   yang sudah banyak dipakai produk lain.
4. **Konsisten brand** — mempertahankan kotak rounded + gradien yang sudah
   dikenal.

## File

- `concept-a-spec-stack.svg` — Konsep A (direkomendasikan)
- `concept-b-brace.svg` — Konsep B
- `concept-c-f-monogram.svg` — Konsep C
- `preview.html` — preview ketiga konsep + uji ukuran (16/32/96 px)

## Langkah berikutnya (jika salah satu dipilih)

1. Ganti `favicon.svg` di `site/public/`, `docs-site/public/`,
   `renderers/react-shadcn/public/`, dan `cmd/formspec/dist/`.
2. Update mark SVG inline di `site/src/components/Nav.tsx`.
3. Buat versi `icon-only` (tanpa kotak) bila dibutuhkan untuk favicon/print.
