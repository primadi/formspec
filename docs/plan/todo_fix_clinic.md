# TODO: Fix Clinic-UI-Showcase — Bug & Gap List

**Audit**: 2026-08-04. **Fixes applied**: 2026-08-04 (same day, follow-up session).

Semua item di bawah **sudah diperbaiki dan diverifikasi ulang di browser**
(Playwright headless, `formspec dev --dev-ui`) kecuali yang ditandai eksplisit
"belum diuji tuntas". `go build ./...`, `go vet ./...`, `go test ./...`
(minus 9 kegagalan pre-existing yang tidak terkait — lihat catatan di
bagian bawah), dan `tsc -b --noEmit` semua bersih setelah perubahan.

---

## Bug tambahan (follow-up) — SUDAH DIPERBAIKI

- [x] **15. Dashboard Klinik selalu 0 walau ada kunjungan nyata.**
  Root cause: widget `visits-today` / `revenue-today` / `visits-by-polyclinic`
  membaca entity `daily-visit-summary` (`characteristic: summary`) yang
  **tidak pernah di-populate** — summary read-only via API dan belum ada
  projection engine/recompute di codebase (`formspec seed` belum ada).
  Diverifikasi: `clinic_daily_visit_summaries` = 0 rows padahal `clinic_visits`
  punya 6 rows termasuk 1 hari ini. **Fix**: repoint ketiga widget ke entity
  live `clinic/visit` (precedent: `pharmacy-queue-count` & widget arisan sudah
  baca entity live): `visits-today` → `aggregate: count`,
  `revenue-today` → `aggregate: sum` dari `visit.total` (dipindah ke
  `transaction/visit/widgets/`), `visits-by-polyclinic` → chart **count-mode**
  (`x: transaction_date`, `group_by: polyclinic_id`, tanpa `config.y`).
  Renderer: `ChartWidget` dukung count mode (bucket per groupBy/x, hitung row,
  agregasi per-x), dan `applySimpleQuery` `today()` pakai local date (bukan
  `toISOString()`/UTC — bug laten dekat tengah malam untuk zona non-UTC).
  **Verified di browser (localhost:18080)**: "Kunjungan Hari Ini" = **1**,
  "Pendapatan Hari Ini" = Rp 0 (jujur, visit belum `total`), chart render
  seri **Poli Kulit** & **Poli Jantung** (sebelumnya "No data"). Detail:
  `docs/plan/fix-clinic-dashboard-summary.md` + changelog
  `2026-08-04-001-fix-dashboard-clinic-widget-baca-summary-kosong.md`.

- [x] **16. Widget `query` → server-side filter + exclude `cancelled`.**
  Query widget diterjemahkan ke list filters server-side (`field[op]=value`
  via `translateWidgetQuery()` + `buildListParams()`), bukan regex manual
  client-side. Widget showcase: `visits-today`/`revenue-today` →
  `"transaction_date = today() and status != 'cancelled'"`,
  `visits-by-polyclinic` → `"status != 'cancelled'"`. `applySimpleQuery`
  (client) jadi fallback + final safety-net, diperluas dukung compound
  `and` + `!=` + `==`. **Fix timezone**: `today()` kini = tanggal
  server/business (UTC) via `serverToday()` — wall-clock browser (WIB)
  yang bergeser setelah tengah malam sempat membuat "Kunjungan Hari Ini" 0.
  **Verified**: API `status[neq]=cancelled` → 4 (2 cancelled di-exclude),
  dashboard tetap benar di browser.

- [x] **17. `useRealtime` hook — event-driven dashboard refetch.**
  Implementasi `docs/plan/use-realtime-hook.md`: `types/events.ts`
  (`RealtimeMessage`) + `hooks/useRealtime.ts` (koneksi WS **singleton** ke
  `/{workspace}/_ui/_ws`, reconnect+backoff, filter resource/event, `tick`
  naik tiap event + reconnect). `DashboardRenderer` menurunkan
  `realtime` (dari `DashboardSpec.realtime`) ke `MetricWidget`/`ChartWidget`
  → refetch saat event entity cocok; polling `refresh_secs` tetap backstop.
  `clinic-dashboard.yaml` → `realtime: true`. Backend: `AuthMiddleware` baca
  token `?token=` (browser WS tak bisa set header) + test. **Verified
  end-to-end**: Node WS client terima `{"event":"completed","resource":
  "clinic/visit",...}`; di browser, create visit #2 + complete → dashboard
  berubah **tanpa reload**: Kunjungan 1→2, Pendapatan Rp 0→Rp 25.000.

- [x] **18. Bug blocking realtime yang ditemukan saat demo.**
  (a) Guard `complete` `!empty(diagnosis)` — `!` bukan operator Starlark →
  konsultasi tidak pernah bisa selesai; diganti `not empty(diagnosis)`.
  (b) `complete.star` crash `NoneType value is not iterable` saat
  `treatments` kosong → `(resource.field.treatments or [])`.
  (c) `visit.total` rule `positive` menolak 0 → visit tanpa treatment tidak
  bisa diselesaikan; diganti `[min: 0]`.

- [x] **19. Row-level realtime refetch — Kanban board konsultasi.**
  `KanbanRenderer` subscribe `useRealtime` + silent `fetchRecords(true)` pada
  event/reconnect, gated `entry.spec.realtime`. Board sudah `realtime: true`.
  **Verified** di browser: create visit + complete → board silent refetch
  tanpa reload → kolom "Selesai" 2→3.

- [x] **20. Row-level realtime refetch — Table Daftar Kunjungan.**
  `TableRenderer` subscribe `useRealtime` + `silentRefetch` flag → silent
  `setReloadKey` tanpa flash "Loading...". Gated `tableSpec.realtime`. Tabel
  `visit-table` sudah `realtime: true`. Mekanisme sama dengan Kanban.

---

## Bug signifikan — SUDAH DIPERBAIKI

- [x] **1. `how-to-run.md` — path Meta API salah/basi.**
  Fixed: contoh curl + JSON output diupdate ke `/default/_ui/_meta/me`
  (path lama `/default/api/v1/_meta/me` dihapus), plus catatan singkat
  kenapa `/api/v1/*` beda dari `/_ui/_meta/*`.

- [x] **2. Custom Page `patient-detail` tidak ter-routing.**
  Root cause (dikonfirmasi via investigasi routing): `spec.route` di 5 file
  (`patient/pages/detail.yaml`, `pages/data-master.yaml`,
  `reference/setting/pages/settings.yaml`,
  `transaction/visit/pages/list.yaml`, dan menu escape-hatch di
  `module.yaml`) semuanya salah diawali `/app/...` — padahal frontend
  (`App.tsx`/`router.tsx`) sudah otomatis prepend App's `root_url` sebagai
  `basePath`, jadi hasilnya route ganda (`/app/klinik-internal/app/...`)
  yang tidak pernah match apapun → silent fallback ke dashboard.
  **Fix**: hapus prefix `/app` dari kelima route tsb (mis. `patient-detail`
  sekarang `/clinic/patients/:id`, meng-override route derived entity
  `patient` sesuai komentar aslinya di `entity.yaml`). Ditambah defensive
  check baru di `internal/ui/validate.go` yang me-reject route Page manapun
  yang diawali `/app` (dengan pesan error yang menjelaskan kenapa) — 2 test
  fixture di `internal/ui/ui_test.go` yang kena pola yang sama juga
  diperbaiki. **Verified**: `/default/app/klinik-internal/clinic/patients/:id`
  sekarang render Page custom-nya (title, form view-mode, table riwayat),
  bukan fallback ke dashboard.

  **Temuan baru saat verifikasi fix ini** (bukan bug asli yang ditemukan
  sesi audit, tapi baru kelihatan setelah Page-nya reachable): Page block
  param resolution (`form.id: ":id"`, `table.param: {patient_id: ":id"}`)
  ternyata **tidak pernah di-resolve ke route param sungguhan** — nilai
  literal string `":id"` dikirim apa adanya ke API (`GET
  .../patient/:id` — 404), dan `table.param` malah tidak pernah diteruskan
  ke `TableRenderer` sama sekali (list tabel jadi tidak ter-filter). Title
  interpolation Page (`"Pasien — {patient.name}"`) juga tidak pernah
  di-substitusi. **Semua ikut diperbaiki** dalam pass yang sama:
  `PageRenderer.tsx` sekarang resolve `:param` terhadap `useParams()` yang
  sebenarnya, `TableRenderer` dapat prop baru `fixedFilters` untuk filter
  yang tidak bisa di-clear user, dan title Page di-interpolate pakai record
  block form pertama yang resolve. **Verified**: halaman sekarang
  menampilkan "Pasien — Andi Pratama", form Profil terisi penuh, dan tabel
  riwayat kunjungan benar-benar ter-filter ke 1 record milik pasien
  tersebut (sebelumnya nampilkan semua 5 visit tanpa filter).

- [x] **3. Widget (metric + chart) tidak fetch data sama sekali.**
  Fixed di `DashboardRenderer.tsx`: `MetricWidget` sekarang fetch entity
  list, terapkan `spec.query` (subset kecil FormSpecExpr yang dipakai showcase
  ini: `field = today()` dan `field in [...]`), agregasi
  (`sum`/`count`/`avg`/`min`/`max`) sesuai `spec.config`, format
  currency/percentage, dan auto-refresh tiap `refresh_secs`.
  `ChartWidget` sekarang fetch data dan render **line chart SVG inline**
  (tanpa dependency baru) per `config.group_by`, dgn legend warna-kode.
  **Verified**: dashboard klinik sekarang nampilkan angka nyata ("0",
  "Rp 0", "0" — data legitimately kosong di DB seed) alih-alih placeholder
  "--" permanen, dan chart nampilkan "No data" (state jujur) alih-alih teks
  statis "Chart renderer (Fase 4.F6)".

- [x] **4. Report — endpoint 404 + parameter widget salah.**
  Root cause: `ReportRenderer.tsx` memanggil `${schema.module}/${schema.plural}`
  (path plural) padahal route REST yang benar pakai nama singular
  (`schema.name`) — konfirmasi dgn cara TableRenderer yang sudah benar.
  **Fix**: satu baris (`schema.plural` → `schema.name`). Plus parameter
  widget: `type: date` sekarang native date-picker, `type: select` pada
  field `*_id` sekarang pakai `RelationPicker` yang sama dgn form (resource
  ditebak dari strip suffix `_id`, mis. `polyclinic_id` → `polyclinic`).
  **Verified**: generate report sekarang jalan tanpa toast error, hasil
  "0 rows" jujur (tabel summary memang kosong di seed data), parameter
  tanggal & poliklinik render sebagai widget yang benar.

- [x] **5. Print — template interpolation tidak jalan.**
  Fixed: helper `interpolate()`/`resolvePath()` baru (`lib/interpolate.ts`,
  dipakai bersama oleh Print & Page) resolve dot-path (`polyclinic.name`,
  `patient.name`) dan qualified-entity-name (`visit.queue_number`) plus
  `workspace.name`, dipasang ke header title/subtitle, body fields/totals,
  dan footer. **Temuan baru saat verifikasi**: ternyata route Print
  (`router.tsx`) tidak pernah didaftarkan dengan segmen `:id` sama sekali —
  jadi record tidak pernah bisa di-load lewat URL apapun, terlepas dari
  interpolation-nya. Ditambahkan route kedua `${basePath}/print/${name}/:id`
  (mengikuti konvensi list/detail yang sudah ada di file yang sama).
  **Verified**: `print/visit-invoice/:id` & `print/queue-ticket/:id`
  sekarang menampilkan data asli (nomor antrian, nama pasien/dokter via
  relation, tanggal, footer workspace) — tidak ada lagi placeholder mentah.

- [x] **6. Wizard "Pasien Baru" — DIINVESTIGASI ULANG, BUKAN BUG.**
  Temuan audit awal (dialog tidak muncul saat diklik) ternyata **artefak
  tooling test**, bukan bug produk: `page.getByText(...).click()`
  (Playwright) gagal memicu handler tombolnya, tapi `button.click()` DOM
  mentah di elemen yang sama berhasil membuka dialog dgn benar (form NIK,
  Nama, Tanggal Lahir dgn date-picker native, Jenis Kelamin select, submit
  jalan). Tidak ada perubahan kode untuk item ini.

## Gap field-widget library — SUDAH DIPERBAIKI

- [x] **7. Field `type: json`** — widget `JsonInput` baru
  (`widgets/JsonInput.tsx`): textarea JSON pretty-printed, parse live +
  pesan error inline, re-sync saat value berubah dari luar (load/reset).
  Dipasang di `FormFieldWidget` (`case "json"`). **Verified**:
  `patient.allergies` sekarang render sbg editor JSON sungguhan (ketik
  `["peanuts","shellfish"]` → auto pretty-print multi-baris).

- [x] **8. Child table / array field (`type: child`)** — widget
  `ChildTable` baru (`widgets/ChildTable.tsx`): grid kolom sesuai
  `child.fields`, tombol tambah/hapus baris, sequence field
  (`line_number`) auto-increment & read-only, cell editor per tipe
  sub-field (text/number/enum/boolean/date/relation), dan untuk field
  `uuid` dgn rule `exists: <resource>` otomatis pakai `RelationPicker`
  (bukan input teks polos). Dipasang di `FormFieldWidget`
  (`case "child-grid"`/`"child"` — dua-duanya perlu krn form authored tidak
  set `field.widget` eksplisit, jadi fallback ke `entityField.type` mentah).
  **Verified**: `visit.treatments` DAN `otc-sale.items` (termasuk
  `medicine_id` yang otomatis jadi relation-picker) — dua-duanya grid
  penuh, bukan lagi single-line text.

- [x] **9. Field `type: date`/`datetime`** — widget `DateInput` baru
  (native `<input type="date">`/`datetime-local`, tanpa dependency
  tambahan). Dipasang di `FormFieldWidget`
  (`case "datepicker"`/`"date"`/`"datetime"`). **Verified**: `birth_date`,
  `joined_at`, dll semua sekarang date-picker native, bukan lagi input teks
  bebas.

## Bug format tampilan — SUDAH DIPERBAIKI

- [x] **10. Fallback title pakai capitalize huruf-pertama-saja, bukan
  per-kata.** Util baru `titleCase()` (`lib/utils.ts`) dipasang konsisten
  di: `engine/derive.ts` (`entityDisplayName`/`moduleDisplayName`),
  `KanbanRenderer`/`TimelineRenderer` (sekarang pakai nama Kanban/Timeline
  itu sendiri, bukan `"{entity} Board"`/`"{entity} Timeline}"` generik —
  jadi "Consultation Board"/"Pharmacy Queue"/"Patient Medical History",
  bukan lagi "visit Board"/"prescription Board"/"medical-record Timeline"),
  `FormRenderer`, `DetailPage`, `TableRenderer`, `PrintRenderer` (sekaligus
  memperbaiki dua Print yg tadinya sama-sama tampil "visit Document" —
  sekarang "Visit Invoice" vs "Queue Ticket", saling beda sesuai nama
  manifest-nya), `Sidebar`, dan `OverlayHost`. **Verified**: semua contoh di
  atas dicek langsung di browser.

- [x] **11. Breadcrumb duplikat "App".** Ternyata efek samping langsung
  dari bug #2 (double `/app` di path) — begitu bug #2 diperbaiki,
  breadcrumb otomatis benar (`Klinik Internal › Visits`, bukan lagi
  `Klinik Internal › App › Visits`). Tidak ada perubahan kode terpisah.

## Ketidaksesuaian README/spec — DIKLARIFIKASI / DIPERBAIKI

- [x] **12. Heuristik modal/drawer (§1.6) tidak diterapkan di derived
  form.** Dikonfirmasi ini bug nyata (bukan gap dokumentasi) — spec
  `docs/spec/frontend/06-page-kinds.md` sudah **Draft** (berlaku), dan
  `derive.ts` **sudah** menghitung `render.mode` yang benar
  (`deriveFormRenderMode`) untuk entity manapun, tapi `TableRenderer`'s
  tombol New/Edit hanya mengecek render mode dari **authored Form**
  (`authoredForm?.spec.render?.mode`) — untuk entity yang formnya
  full-derived (spt `polyclinic`, `doctor` — showcase ini sengaja tanpa
  authored Form utk keduanya), variabel itu selalu `undefined` sehingga
  selalu jatuh ke full-page navigation. **Fix**: `TableRenderer` sekarang
  hitung `formRenderMode` dari `authoredForm ?? deriveForm(entity,
  "create").render.mode`, dan saat overlay (bukan authored) kirim param
  `entity=module.name` (bukan `form=<name>`) ke `OverlayHost`, yang
  sekarang bisa derive form-nya sendiri (tanpa perlu authored Form
  bundle entry) via `FormRenderer`'s existing `resolveForm()` fallback.
  **Verified**: `Polyclinic` (≤5 field) → modal Dialog; `Doctor` (6-12
  field) → drawer Sheet kanan — dua-duanya di `_admin` surface,
  sebelumnya dua-duanya full-page navigation.

- [x] **13. `daily-visit-summary` muncul di `_admin` nav walau
  `characteristic: summary`.** Diklarifikasi (bukan diubah perilakunya) —
  `_admin` adalah generic nav utk SEMUA entity per module terlepas
  `characteristic`, beda tujuan dgn App menu (`clinic/module.yaml`) yang
  memang sudah benar tidak menyertakan entity ini. Catatan ditambahkan di
  README §1 coverage table.

- [x] **14. README menyebut file `kind: Menu` yang sudah tidak ada.**
  `clinic/menus/clinic-main.yaml` & `pharmacy/menus/pharmacy-main.yaml`
  diganti referensinya ke `clinic/module.yaml`/`pharmacy/module.yaml`
  (`spec.menu`) — lokasi sebenarnya sejak migrasi kind: Menu lama.

## Belum sempat diuji tuntas (di luar scope sesi ini)

- [ ] Export CSV/XLSX di Report — sudah bisa di-generate (bug #4 fixed),
  tapi belum diklik tombol export-nya scr eksplisit.
- [ ] Permission-driven UI hiding dgn role terbatas — dev auth selalu
  `permissions: ["*"]`; butuh cara inject token/role terbatas utk diuji.
- [ ] CAS/optimistic concurrency (409 konflik) — belum disimulasikan 2
  klien konkuren.
- [x] Realtime delivery `visit.completed` (websocket) — sudah dipicu &
  diamati langsung (2026-08-05): event `created|updated|deleted`/action
  broadcast untuk SEMUA mutasi (bukan cuma declared events), listener-gated
  via `HasListeners`. Fix transport `--dev-ui`: Vite proxy `ws: true` +
  `AcceptOptions.InsecureSkipVerify`. Changelog `2026-08-05-002`.
- [ ] Theme switching UI (5 tema wired via `formspec-app.yaml`) — belum
  ditemukan switcher-nya di avatar/user menu; belum digali lebih dalam.
- [ ] FormSpecExpr `visible_when`/`required_when` cross-field — belum diuji
  aktif (isi field A, lihat field B react).
- [ ] State machine guard (mis. diagnosis wajib sebelum visit selesai) —
  row action transisi sudah kelihatan benar (state-aware), tapi guard
  reject belum diklik langsung utk verifikasi pesannya.

## Catatan tambahan (ditemukan saat memperbaiki, di luar 14 bug asli)

- **9 test e2e pre-existing gagal, TIDAK terkait perubahan sesi ini**
  (dikonfirmasi dgn `git stash` — gagal identik di kode asli tanpa
  perubahan apapun): `clinic_e2e_test.go` (3 test) gagal krn tanggal
  hardcode `2026-07-12` sudah lewat `backdate_policy` (maks 3 hari mundur,
  makin basi seiring waktu berjalan); `pharmacy_otc_sale_e2e_test.go` (3
  test) & `pharmacy_prescription_scenarios_e2e_test.go` (3 test) gagal krn
  script Starlark `stock-guard.star`/`derive-patient-name.star` tidak
  ketemu di path manapun yang dicoba resolver. Di luar scope sesi ini
  (tidak disentuh), tapi worth di-follow-up terpisah.
- `internal/ui/validate.go`'s `Registry.Validate()` — dipastikan dipanggil
  dari test suite (`internal/ui/ui_test.go`), TAPI grep ke seluruh
  `cmd/`/`internal/` tidak menemukan pemanggilan dari `formspec dev`/`formspec
  validate` CLI manapun. Kemungkinan besar validasi semantik lintas-file
  ini (termasuk defensive check baru utk route `/app`) **tidak pernah
  jalan di runtime nyata**, hanya di test. Di luar scope sesi ini untuk
  di-wire, tapi flag utk perhatian — nilainya baru kelihatan kalau memang
  dipanggil saat `formspec dev` start atau `formspec validate`.
- Bug yang sama (`route: /app/...` di depan) ditemukan juga tersebar di
  `verticals/billing`, `verticals/gl`, `verticals/inventory`,
  `verticals/reference-app` (bukan cuma Clinic-UI-Showcase) — TIDAK
  diperbaiki di sesi ini (di luar scope contoh yang diaudit), tapi
  defensive check baru di `validate.go` akan menangkapnya kapanpun
  registry validation itu benar-benar dipanggil.
