# HandleFind: find-or-create untuk reference entity

**Apa yang diubah:**
- `internal/api/handler.go` — `HandleFind` sekarang mendukung find-or-create singleton pattern untuk entity dengan `characteristic: reference`. Saat record tidak ditemukan, handler akan mencari record yang ada (via List limit 1), dan jika tidak ada, auto-create dengan field defaults.
- Method baru `findOrCreateReference()` menampung logika find-or-create agar `HandleFind` tetap terbaca.
- `examples/Clinic-UI-Showcase/spec/modules/clinic/pages/system-settings.yaml` — update komentar untuk merefleksikan bahwa `id: "0"` adalah sentinel dan backend akan auto-create.

**Kenapa diubah:**
Page settings (`/app/settings`) pertama kali diakses selalu error 404 karena GET /clinic/settings/0 tidak menemukan record. Sebelumnya seeding harus manual via POST. Dengan perubahan ini, backend auto-create default record saat pertama kali settings page diakses.

**File terkena:**
- `internal/api/handler.go` (+50 lines)
- `examples/Clinic-UI-Showcase/spec/modules/clinic/pages/system-settings.yaml`
- `docs/spec/backend/01-core-basic.md` — tambah baris find-or-create di tabel characteristic + sub-section
- `docs/spec/frontend/06-page-kinds.md` — tambah catatan find-or-create di Configuration Page pattern
- `docs/spec/platform/02-workspace-app-module.md` — tambah referensi find-or-create

**Referensi:**
- todo: (task baru, tidak tercantum di todo.md sebelumnya)
- spec: §4.1 Characteristic, Configuration Page pattern
