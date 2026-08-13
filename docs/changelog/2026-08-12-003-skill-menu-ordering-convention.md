# 2026-08-12-003-skill-menu-ordering-convention

Tambah konvensi kurasi menu App ke skill pembuatan aplikasi: (1) jika ada
Dashboard, taruh sebagai leaf level-1 pertama (landing) di `spec.menu[0]`;
(2) urutkan module dari paling sering diakses ke paling jarang — default
untuk app transaksional **Transaksi → Laporan → Master → Config/Pengaturan**.
Dibingkai sebagai heuristic + rationale, bukan aturan keras.

## Alasan

Konvensi urutan menu dibutuhkan supaya AI-assisted app development
menghasilkan navigasi yang konsisten dan intuitif — pengguna mendarat di
ringkasan (Dashboard), lalu menu mengikuti alur kerja harian (transaksi →
laporan → master → config). Sekaligus merekonsiliasi klaim lama di skill
"renderer nests orphan leaf" yang sudah usang: resolver
(`internal/app/resolve.go`) menerima leaf level-1, dan renderer
(`renderers/web/src/shell/Sidebar.tsx`) me-render leaf tanpa children
sebagai link standalone — sehingga leaf Dashboard landing di level-1 aman.

## Dampak

| File                                                          | Perubahan                                                                                                             |
| ------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| `ai_skills/formspec-kinds/SKILL.md`                           | Tambah aturan landing Dashboard + heuristic urutan menu; perbaiki klaim "orphan leaf" dan gotcha leaf level 2–3 → 1–3 |
| `ai_skills/formspec-app-workflow/SKILL.md`                    | Pointer konvensi urutan menu di Phase 3 step 4                                                                        |
| `examples/cafe/.agents/skills/formspec-kinds/SKILL.md`        | Mirror edit sumber                                                                                                    |
| `examples/cafe/.agents/skills/formspec-app-workflow/SKILL.md` | Mirror edit sumber                                                                                                    |
