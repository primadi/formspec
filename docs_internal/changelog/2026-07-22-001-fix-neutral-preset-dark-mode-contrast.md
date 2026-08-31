# Fix: Neutral preset dark mode contrast — invisible text on Wizard stepper

## Perubahan

### Masalah
Saat menggunakan **neutral (black) preset** di **dark mode**, teks pada step yang sudah selesai (completed step) di Wizard stepper menjadi tidak terbaca. Ini terjadi di form pendaftaran pasien (Wizard 3 langkah).

### Root cause
`useTheme` hook meng-override `--primary` CSS variable via inline style (`oklch(0.205 0 0)` — near-black) di semua mode, termasuk dark mode. Inline style memiliki specificity lebih tinggi dari `.dark` class, sehingga `--primary` yang seharusnya `oklch(0.922 0 0)` (light gray) di dark mode tertimpa menjadi near-black. Akibatnya:
- Completed step: `bg-primary/10 text-primary` → teks near-black di background near-dark → **invisible**
- Preset lain (blue, green, dll) tidak terpengaruh karena hue jenuh tetap kontras di background gelap

### Solusi
Di `renderers/web/src/hooks/useTheme.ts`, tambahkan guard: ketika dark mode + neutral preset, jangan override `--primary` dan `--primary-foreground` — biarkan `.dark` class di `index.css` yang menangani (sudah memiliki nilai kontras-optimal).

### File terkena
- `renderers/web/src/hooks/useTheme.ts` — guard untuk neutral preset di dark mode

### Referensi
- Plan: docs/plan/todo.md
- Spec: `reff_docs/FormSpec-Technical-Note-DX-dan-Entity-Extension.md`
- Issue: Wizard stepper completed step tidak terbaca di dark mode + neutral preset
