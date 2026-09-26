# TODO — Menutup Gap agar Aplikasi Kafe Berjalan

**Tujuan:** menutup **seluruh** gap yang didokumentasikan di folder ini sehingga
`examples/kafe` (3 App: `kafe-qr` publik, `kafe-pos` privat, `kafe-kds` no-nav;
6 module; 24 entity) dapat dijalankan **end-to-end dengan benar**.

**Cakupan:** #1–#48 (ledger temuan), S1–S16 (kelengkapan bahasa spec,
`13-kelengkapan-spec-untuk-kafe.md`), D1–D7 (semantik yang belum ditetapkan),
plus **Fase 10** (sisa non-blocker aplikasi kafe, ditambahkan 2026-09-22 dari
audit prosa-terbuka).

**Update 2026-09-22:** audit sisa prosa menemukan 7 pekerjaan terbuka yang hanya
hidup sebagai "**Sisa (dicatat)**" di bawah item `[x]` — kini semuanya item
bernomor di **Fase 10**; dua prosa yang sudah kedaluwarsa (1.6/GAP-36, 1.7) juga
ditandai tertutup. Tidak ada gap #1–#53/S1–S16 yang berubah status.

**Aturan yang disepakati:**

| Aturan                           | Detail                                                                                                                                                                                                 |
| -------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Spec kafe tidak di-degradasi** | Tidak ada workaround yang mengubah desain ideal kafe. Semua gap ditutup di sisi **engine / bahasa spec**. `# GAP-nn` marker di `spec/` tetap sampai gap-nya benar-benar tertutup.                      |
| **Fase 0 wajib**                 | Ledger ini dibuat dari workspace lain dan memuat ≥4 klaim yang sudah terbukti salah (mis. #7, #24, #26, separuh #2, separuh #18). Tidak ada item yang dikerjakan sebelum statusnya diverifikasi ulang. |
| **Bukti, bukan inferensi**       | Setiap item Fase 0 ditutup dengan **perintah yang bisa gagal** + outputnya, bukan pembacaan kode.                                                                                                      |
| **Urutan tidak dilompati**       | Fase 1 membuka sebagian besar fase lain. Kerjakan item Fase 1 sesuai urutan prioritas.                                                                                                                 |

**Legenda status:**

| Status          | Arti                                                       |
| --------------- | ---------------------------------------------------------- |
| 🔴 `OPEN`       | Terverifikasi masih terbuka pada kode terkini              |
| 🟡 `PARTIAL`    | Sebagian sudah ada; sisa pekerjaan tercatat                |
| ⚪ `UNVERIFIED` | Belum diuji ulang di Fase 0                                |
| ✅ `CLOSED`     | Sudah tertutup; ledger lama (hapus/arsipkan entri terkait) |
| ⛔ `RETIRED`    | Klaim gugur / tidak diberlakukan                           |

---

## Fase 0 — Re-triage ledger

> **Deliverable:** kolom status terisi di `README.md` + `validate-baseline.md`.

- [x] **0.1 — Verifikasi ulang #1–#48** terhadap binary & kode terkini.
      Setiap gap: jalankan perintah yang bisa gagal (curl ke `/_ui/`, `formspec migrate plan`,
      `formspec validate`, baca DDL hasil generate). Catat: status + bukti + file:line.
      _Accept:_ 48 entri punya bukti; yang `CLOSED`/`RETIRED` diberi alasan.
      **Progres 2026-09-14:** #7 ✅`CLOSED`; #22, #27, #44, #46, #10, #17, #21, S10 🔴`OPEN`
      (bukti: `14-temuan-fase-0.md`); **#52, #53 🔴 baru ditemukan**; sisanya belum diuji ulang.
      ✅ **2026-09-18 — SELESAI.** Seluruh #1–#53 (termasuk #49–#53) diberi status final +
      bukti di `14-temuan-fase-0.md` §7. Hasil: **24 CLOSED · 3 PARTIAL (#1, #3, #21) ·
      22 OPEN · 1 RETIRED (#24)**. Re-verifikasi ini menemukan **nol koreksi status** —
      semua ✅ memang tertutup, semua 🔴 memang terbuka (berbeda dari Fase 0 pertama yang
      membatalkan #7/#24/separuh #2/separuh #18). Satu koreksi cakupan: #19/#20 menyebut
      file yang sudah tidak ada (`realtime.md`) / bukan kind manifest (`formspec-app.yaml`)
      → target diperbarui saat 8.2. Baseline suite: `validate` 0 problem · `go test ./...`
      hijau · `vitest` 265 · `make lint` 0 issues.
- [x] **0.2 — Verifikasi ulang S1–S16** terhadap `schemas/formspec.schema.json` &
      `pkg/spec/*.go` terkini (bukan cache `v0.0.8`).
      _Accept:_ tiap S-item ditandai `OPEN`/`PARTIAL`/`CLOSED` + kutipan schema.
      **Progres 2026-09-14:** S4 🔴OPEN (tak ada widget/field `qrcode`); S9 🔴OPEN
      (`WorkflowTransitionRef.From string` tunggal, `pkg/spec/resources.go:632`);
      S10 🔴OPEN (`widget` = string bebas di schema); S13 🔴OPEN (`TransitionDecl`
      tanpa field `emit`, `pkg/spec/entity.go:708`); S16 🔴OPEN (`ReportColumn` tanpa `widget`).
      Sisa S-item belum diuji satu per satu.
      ✅ **2026-09-18 — SELESAI.** Seluruh S1–S16 diberi status + kutipan di
      `14-temuan-fase-0.md` §8. Hasil: **10 CLOSED** (S1, S2, S3, S5, S7, S8, S9,
      S10, S12, S14) · **2 PARTIAL** (S4 widget ✅/cetak+adopsi belum; S11 tipe
      `percent` ✅/model pajak penuh belum) · **4 OPEN** (S6, S13, S15, S16).
      **Koreksi:** S9 & S10 ternyata **sudah tertutup** (1.7 & 1.4), S4 bergeser ke
      PARTIAL (widget `qrcode` ada di kedua kosakata tertutup) — jadi yang benar-benar
      OPEN tinggal S6/S13/S15/S16, semuanya item Fase 5–7 yang belum dikerjakan.
- [x] **0.3 — Putuskan D1–D7** (semantik): baca sumber kode, tetapkan jawaban kanonik.
      Rujukan: `renderers/jsonb-persist/crud.go` (`doc_status`), `internal/api/generator.go`,
      registrasi permission (`internal/entity/registry.go`), `ValidateEventNaming`.
      _Accept:_ `decisions-needed.md` berisi jawaban + siapa yang memutuskan.
      ✅ 2026-09-14 — `decisions-needed.md`. **Koreksi penting:** D6 salah di ledger lama
      (`submit` punya permission sendiri, bukan `update`); D4 kanonik = prefix `on_*`.
- [x] **0.4 — Buat `validate-baseline.md`** — daftar problem `formspec validate` yang
      **diharapkan** (dengan GAP id). Dijanjikan `docs/architecture.md` §0 tapi belum ada.
      _Accept:_ `formspec validate` → problem hanya yang ada di baseline.
      ✅ 2026-09-14 — baseline = **0 problem** (69 manifest); lihat `validate-baseline.md`.
- [x] **0.5 — Klasifikasi ulang ledger** ke SPEC / ENGINE / DOC / PERILAKU sesuai
      hasil Fase 0; hapus entri yang gugur dari `README.md`.
      ✅ **2026-09-18 — SELESAI.** Klasifikasi final ada di `README.md`
      §"Klasifikasi final (Fase 0.5)". Hasil: **SPEC 5** (S6/#41, S13/#40, S15/#39,
      S16/#29, #37) · **ENGINE 12** (#10, #13, #14, #15, #16, #17, #30–#34, #42) ·
      **DOC 6** (#19, #20, #21, #24, #25, #43) · **PERILAKU 0** (D1–D7 ✅ ditetapkan
      1.9; #26/#46 ✅) · **PARTIAL 5** (#1, #3, #21, S4, S11). Entri gugur (#7, #24
      klaim awal, separuh #2, #18, separuh #26) sudah dibuang dari hitungan.
- [x] **0.6 — Catat gap baru #49 & #50** ke `README.md` (✅ 2026-09-14) dan tindak
      lanjuti di `2.9` (perbaikan script + dokumen dialek) & `8.7` (validator).
      ✅ 2026-09-14 — plus **#51** ditemukan & dicatat saat mengerjakan 2.9.
      _Accept:_ ledger memuat #49/#50/#51.

---

## Fase 1 — Bahasa spec: fondasi transaksional

> Prioritas dari `13-...md` §F. Membuka sebagian besar fase lain. **Urutan penting.**

- [x] **1.1 — S2: filter bernilai dari sesi/route** (prioritas §F #1).
      Konstruksi: `fixed_filters: [{field, op, from: session|route, attr|param}]`.
      Target: `pkg/spec/frontend.go` (`FilterSpec`/`fixed_filters`), `pkg/spec/entity.go`,
      resolver di `internal/ui/` + `internal/api/` (merge server-side),
      schema regeneration (`internal/genjsonschema`).
      Membuka: **#6**, **#9**, isolasi multi-outlet, akses pelanggan per-record.
      _Accept:_ kasir cabang A tidak melihat pesanan cabang B; pelanggan melihat
      pesanannya sendiri via token route — diuji lewat `/_ui/` list.
      ✅ 2026-09-14 — mekanisme selesai & terverifikasi; **adopsi di spec kafe masih
      terblokir** (lihat di bawah). Yang dikerjakan:
      (a) `FilterSpec` dapat `from: session|route` + `attr`/`param`
      (`pkg/spec/frontend.go`), dan **`EntitySpec.Scope []FilterSpec`** baru
      (`pkg/spec/entity.go`) — **otoritatif di entity, bukan di kind**: `fixed_filters`
      di kind di-merge browser dan bisa dihilangkan klien mana pun, jadi ia tidak
      pernah bisa jadi kontrol otorisasi.
      (b) Resolver server-side `internal/api/scope.go` (`applyRowScope` + `sessionAttr`),
      dipanggil di `HandleList`. `from: session` mengambil atribut identitas dan
      **meng-override** nilai klien pada field yang sama; `from: route` mengambil
      parameter query yang dideklarasikan. Atribut/parameter yang tidak bisa
      diselesaikan **fail closed (403)**, tidak pernah degrade jadi "tanpa filter".
      `internal/auth`: `Identity.Attributes` + klaim JWT `attrs`.
      (c) `parseListQuery` melewati parameter scope yang dideklarasikan (kalau tidak,
      `?branch=A` diparse sebagai filter field `branch` → 422).
      (d) Validasi dua lapis: engine (`scope[0] (branch_id): from must be "session"
or "route", got "cookie"`) + schema (`/spec/scope/0/from: validation failed`).
      Bukti runtime (binary SUDAH di-rebuild, raw JSON, spec uji dengan dua baris A/B):

                                                  | Permintaan | Hasil |
                                                  | --- | --- |
                                                  | (tanpa param) | `403 row scope on branch_id: missing "branch" request parameter` |
                                                  | `?branch=A` | `total: 1`, hanya `branch_id":"A"` |
                                                  | `?branch=A&branch_id[eq]=B` | **`total: 1`, hanya A** — klien tidak bisa melebarkan |
                                                  | `?branch=B` | `total: 1`, hanya B |
                                                  | `?branch_id[notnull]=1` (op lain) | `403` — tidak bisa dilewati |

                                                  Test: 7 case `internal/api/scope_test.go` + `TestValidateEntitySpec_Scope`.
                                                  (e) **Gerbang adopsi (temuan baru):** `formspec validate` tanpa `--schema`
                                                  memakai **schema registry cache**, yang menolak properti baru →
                                                  `additional properties 'scope' not allowed`. Schema lokal sudah diregenerasi
                                                  (`make generate-schema`), tetapi **schema yang dipublikasikan di registry harus
                                                  di-refresh lebih dulu** sebelum aplikasi mana pun memakai `scope:`.
                                                  **Catatan 2026-09-15:** gerbang yang sama kini menghadang `1.2` —
                                                  `formspec validate` (tanpa `--schema`) pada spec kafe melaporkan
                                                  `additional properties 'public_entities' not allowed` karena schema **App**
                                                  di registry masih versi sebelum `public_entities`; dengan schema lokal
                                                  (`--schema ../../schemas`) hasilnya **0 problem**. Jadi ini murni staleness
                                                  registry, bukan regresi spec. Satu tiket refresh menutup `scope` +
                                                  `public_entities` sekaligus.
                                                  **Kenapa kafe belum memakai `scope:`** (keputusan sadar, bukan lupa):
                                                  - `from: session, attr: branch_id` butuh **atribut sesi yang terisi** — mekanisme
                                                    penugasan (employee → cabang) baru ada di **1.8/3.5 (S5 `assignments`)**.
                                                    Kalau dipasang sekarang, seluruh list kasir akan **403** (fail-closed benar,
                                                    tapi aplikasi tak terpakai).
                                                  - `from: route` pada `table-session` akan menuntut `?token=` pada **semua** surface,
                                                    termasuk POS kasir yang tidak punya token → merusak surface POS. Scope
                                                    per-permukaan adalah **S3 (1.2)**, bukan scope entitas menyeluruh.
                                                  Jadi adopsi di spec kafe menunggu **1.8** (session) dan **1.2** (per-surface).

- [x] **1.2 — S3: akses publik per-entity** (prioritas §F #2).
      Konstruksi: `spec.public_entities: [{entity, actions}]` pada App.
      Target: `pkg/spec/` (AppSpec) → `internal/api/router.go` (`publicEntities`, :222),
      `internal/api/meta.go`. Konfirmasi #7 (`exclude` sudah ditegakkan) & tegakkan di
      **semua** surface (`internal/api/fieldsec.go`).
      _Accept:_ App publik hanya membuka entity & aksi yang dideklarasikan; `shift`/
      `cash-movement`/`member` → 401 untuk anonim.
      ✅ 2026-09-14 — `AppSpec.PublicEntities *[]PublicEntityDecl` (pointer: membedakan
      **absen** vs **kosong**), `PublicEntityActions` (closed set),
      `NormalizeEntityRef` menerima `module.entity` **dan** `module/entity`
      (last-separator, jadi `formspec.core.workspace` benar). Router:
      `publicGrants()` + `isPublicAction(module, entity, action)` menggantikan
      pengecekan per-module. Tiga keadaan: **absen** = legacy module-wide
      list/find/create; **`[]`** = tidak ada yang anonim; **daftar** = hanya pasangan itu.
      Validasi menolak: `access` bukan `public`, module yang tidak di-mount,
      aksi di luar closed set, `actions` kosong, ref malformed, duplikat.
      Spec kafe `kafe-qr` kini **memakai** allowlist (komentar “bentuk ideal yang belum
      bisa dinyatakan” dihapus — sudah bisa). Bukti runtime (anonim, spec kafe):

      | Entity/aksi | Hasil |
      | --- | --- |
      | `menu-category` list, `menu-item` list, `menu-item-price` list | **200** |
      | `order` create | **201** |
      | `member` list & create | **401** (sebelumnya terbuka — nomor HP) |
      | `employee` list | **401** |
      | `shift` list, `cash-movement` list | **401** (sebelumnya terbuka — data kas) |
      | `menu-item-price` create (aksi di luar izin) | **401** |
      | `order` list (aksi di luar izin) | **401** |

      Test: `public_entities_test.go` (api, 4 case) + `public_entities_test.go` (spec,
      4 case + `TestNormalizeEntityRef`). #6 **tertutup**.
      **#45 sebagian:** izin per-entity ✅ dan penyelesaian oleh kasir ✅ (rute lifecycle
      sudah ada sejak #52; `order` lifecycle-free sejak #44, jadi record anonim langsung
      referenceable oleh `payment`). **Sisa #45:** klaim kepemilikan record oleh tamu
      (`table-session.guest_token`) — butuh scope **per-permukaan**, sebab `scope`
      menyeluruh di `table-session` akan menuntut `?token=` juga di POS kasir (lihat 1.1).

- [x] **1.3 — S7: semantik aritmetika & agregasi `money`** (prioritas §F #3).
      Tetapkan bentuk kanonik (`amount(x)`, `money: {compare, result}`), implementasikan di
      evaluator FormSpecExpr, `Aggregate`/`Window` (`renderers/jsonb-persist/crud.go`).
      Menutup: **#28**. Prasyarat **#2** (renderer) & **#46** (batas API).
      _Accept:_ `SUM(money)`, `money - money`, kembalian & selisih kas benar; field
      non-numerik **ditolak dengan error**, bukan diam-diam salah.
      ✅ 2026-09-15 — bentuk kanonik: **uang beroperasi langsung** (tidak ada sintaks
      pembungkus baru), operand diklasifikasikan money|skalar:
      `m ± m` → money (securrency; beda → error) · `m × / n` → money · `m1 / m2` → rasio
      number · `m <op> m` → boolean · `m == m` → kesetaraan nilai (deep) ·
      `amount(x)`/`currency(x)` untuk ekstraksi eksplisit · `sum([m…])` → money.
      **Operand tidak sah = error**, bukan `0` (objek non-money, list, string non-numerik),
      termasuk `money` vs angka mentah → `amount(m)` untuk membandingkan dengan skalar.
      Diimplementasikan di **empat** jalur agar konsisten:
      (a) **server** — `internal/starlark/money.go` (tipe Starlark `moneyValue`:
      `HasBinary` `+ - * /`, `HasUnary` `-`, `Comparable`; presisi eksak via `math/big.Rat`,
      bukan float64) + `amount()`/`currency()` (+ alias `money_amount`/`money_currency`
      karena nama env menang atas builtin) di `EvalExpr` **dan** executor script;
      `toStarlark` mengenali `spec.Money`/objek JSON money (jalur baca-DB) dan
      `fromStarlark` mengembalikannya sebagai `{amount, currency}`;
      (b) **klien** — `lib/formspec-expr/eval.ts` (tabel yang sama), `==`/`!=` jadi
      `deepEqual` (sebelumnya identitas objek → dua money yang sama dianggap tidak sama);
      (c) **agregasi** — `renderers/jsonb-persist/crud.go`: `columnRefExpr` memetakan
      `money` ke sub-path `.amount` (`data->'f'->>'amount'` / `json_extract(data,'$.f.amount')`),
      `requireNumericAggregateField` menolak `sum|avg|min|max` atas field non-numerik
      (dan field tak dikenal), berlaku juga untuk `Window running_total`;
      (d) **klien laporan/widget** — `src/lib/aggregate.ts` baru (shared oleh Report
      `computeTotals`, Widget metric, dan **chart** `y` yang sebelumnya `Number(obj) || 0`
      → memplot 0), plus gerbang statis `formspec check` (`checkAggregates`: verb di closed
      set, field ada, non-numerik ditolak).
      **Bukti runtime** (spec kafe, HTTP):

      | Kasus | Hasil |
      | --- | --- |
      | `payment` amount 75000, tendered 100000 | `change = {amount: "25000", currency: "IDR"}` (sebelumnya field **absen**) |
      | `payment` GET ulang (jalur baca-DB) | `change` tetap `{25000 IDR}` |
      | `shift` counted 500000, expected 512500 | `difference = {-12500 IDR}` (kas kurang, negatif) |
      | `formspec check -f examples/kafe/spec` | 0 error (semua agregat memang numerik/money) |

      Test: `internal/starlark/money_test.go` (11 case termasuk guard + money vs angka),
      `renderers/jsonb-persist/aggregate_money_test.go` (agregat money, penolakan,
      window, computed bertingkat `quantity * unit_price` → 62500),
      `renderers/react-shadcn/src/lib/formspec-expr/money.test.ts` (paritas klien),
      `src/lib/aggregate.test.ts`, `cmd/formspec/check_aggregate_test.go`.
      Normatif: `docs/spec/backend/05-field-types.md` §2.1, `docs/spec/frontend/08-formspec-expr.md` §5,
      `docs/reference/primitives.md` (dialek Starlark), `ai_skills/entity-authoring`.
      Marker `# GAP-28` dihapus dari `spec/**`.
      _Sisa (dicatat, bukan bagian #28):_ perbandingan `money` dengan **angka mentah**
      tidak bisa diizinkan karena Starlark tidak memberi hook pada perbandingan lintas tipe
      → bentuknya `amount(m) <op> n`; perbandingan dengan nol pun mengikuti aturan itu.

- [x] **1.4 — S10: kosakata `widget` menjadi `enum`** (prioritas §F #4).
      Generate katalog widget dari barrel renderer (`renderers/react-shadcn/src/widgets/`,
      `src/renderer/kinds/**`), ekspor ke `pkg/spec/frontend.go` + `internal/genjsonschema`.
      Menutup akar **#1** ("salah ketik == fitur belum ada").
      _Accept:_ `widget: relaion-picker` **gagal** validasi; editor autocomplete.
      ✅ 2026-09-15 — **dua** himpunan tertutup, satu per permukaan (bukan satu daftar
      gabungan — widget form pada kolom tabel diabaikan renderer sel dan nilainya
      tercetak mentah): `FormWidget` **20 nilai** (`input`, `textarea`, `richtext`,
      `number`, `decimalinput`, `select`, `switch`, `radio-group`, `combobox`,
      `password`, `slider`, `tags`, `uuid`, `json`, `fileinput`, `relation-picker`,
      `datepicker`, `datetimeinput`, `child-grid`, `grants-editor`) dan
      `TableCellWidget` **2 nilai** (`badge`, `boolean`).
      Sumber kebenaran `pkg/spec/widget.go` (tipe bernama + blok `const`) —
      `internal/genjsonschema.enrichEnumValues` sudah mengumpulkan nilai `const`
      menjadi enum, jadi **tidak ada pipeline baru**; enum masuk schema sebagai
      `$defs/FormWidget` / `$defs/TableCellWidget` dan `FormField.widget` /
      `TableColumn.widget` `$ref` ke sana.
      **Bukti accept** (`formspec validate --schema schemas`):

      | Spec | Hasil |
      | --- | --- |
      | `widget: select` (kanonik) | **0 problem** |
      | `widget: relaion-picker` | **1 problem** — *schema* (`/spec/sections/0/fields/1/widget: validation failed`) **dan** *engine* (`unknown widget "relaion-picker" (allowed: input, textarea, …)`) |
      | `widget: relation` (nama tipe) | gagal + petunjuk: `"relation" is a field type; the widget is "relation-picker"` |
      | `widget: select` pada kolom **tabel** | gagal: `unknown table cell widget "select" (allowed: badge, boolean) — "select" is a form widget, not a table cell widget` |
      | `widget: money-input` | gagal (akar #1 tertutup: bukan lagi "lolos lalu tidak melakukan apa pun") |

      Enum di schema juga yang memberi **autocomplete** di editor YAML
      (`schemas/kinds/*.json` + `formspec.schema.json` ter-regenerasi).
      **Anti-drift** (inti S10 — schema dan implementasi tidak boleh berbeda):
      `src/widgets/catalog.ts` (katalog runtime) + `src/widgets/catalog.test.tsx`
      (14 test, jsdom) yang gagal bila (a) enum schema ≠ katalog, (b) ada nama
      katalog tanpa `case` di `FormFieldWidget` (scan label `case`), (c) ada nama
      katalog tanpa cabang di `renderCellValue`, (d) `derive.formWidget()`
      mengembalikan nama di luar katalog. Gerbang ini **diuji bisa gagal**:
      menambahkan `money-input` ke katalog → 3 test merah.
      Di runtime, `widget:` eksplisit yang tidak dikenal kini merender **error yang
      terlihat** (`UnknownWidget`, `role="alert"`), bukan `TextInput` senyap;
      turunan tipe field tanpa widget (`money`, `time`) tetap `input` seperti
      sebelumnya (gap #1, kini item **2.14**).
      Validasi dua lapis: `ValidateFormSpec`/`ValidateFormSections` (engine) dan
      `ValidateTableColumns` (Table + Listing, dipanggil dari `manifest.Loader`).
      Regresi spec: kafe 0 problem, cafe 0 problem, registry app-spec 0 problem,
      Clinic/crc/reference-app **jumlah problem identik sebelum-sesudah** (10/1/32 —
      drift schema lama, bukan dari enum; diverifikasi dengan schema pra-perubahan).
      Dokumen: `docs/spec/frontend/07-component-kinds.md` §1 ditulis ulang (nama
      dokumen sebelumnya `textinput`/`numberinput`/`dateinput`/`toggle`/`json-editor`
      **tidak pernah ada** di renderer) + `ai_skills/form-layout` (klaim `MoneyInput`
      dihapus, diganti `money → input` + catatan gap).

- [x] **1.5 — S1: blok transaksional (cart)** (prioritas §F #5).
      Konstruksi: blok `order_builder` **atau** `kind: OrderBuilder` — grid katalog +
      keranjang + checkout. Target: `pkg/spec/frontend.go` (`PageBlock`),
      renderer baru di `renderers/react-shadcn/src/kinds/`.
      Menutup: **#5**.
      _Accept:_ halaman QR = grid menu + keranjang + kirim pesanan (bukan Form+ChildTable).
      ✅ 2026-09-15 — **implementasi digeneralisasi** (lihat catatan di bawah):
      konstruknya bukan blok Page khusus, melainkan `picker` pada **child field**
      (`child.picker`), sehingga berlaku di Form mana pun. Menggantikan blok
      `order_builder` yang lebih sempit (changelog `2026-09-15-003` → digantikan
      `2026-09-15-004`). Bukti E2E tidak berubah: pesanan QR terkirim dengan
      `line_total`/`subtotal` terhitung server.
      **Kenapa diganti:** blok itu (a) memakai kosakata spesifik pesanan
      (`catalog`/`lines`/`checkout`) untuk pola yang umum — bukti: PO, hitung stok,
      perpindahan stok, jurnal, resep, checklist semuanya "pilih baris dari sumber +
      qty + snapshot"; dan (b) menduplikasi jalur tulis Form (POST sendiri →
      kehilangan validasi `rules`, permission, idempotency, `action:` lifecycle,
      redirect, event). Sekarang baris yang dipilih adalah baris child biasa di
      state Form, jadi submit tetap jalur Form.
      **Bentuk akhir** (`pkg/spec/picker.go`): `picker.entity` + `filter` +
      `display{name/image/description/category/price_entity+price_match_field+
price_field+price_filter/columns/search}` + `map{ref_field, name_field,
price_field, quantity_field, note_field, max_quantity}`. Aturan: `ref_field`
      wajib; field `map` wajib ada di `child.fields`; `quantity_field` wajib
      disertai `max_quantity`; `price_entity` wajib disertai match+price field;
      baris tanpa harga **tampil tapi tidak bisa dipilih**; picker tidak pernah
      menghitung total (itu `computed` entity).
      **Primitif pelengkap:** `FormField.default_from` (seed dari render context,
      pengganti `checkout.defaults`), `widget: hidden` (masuk kosakata S10), dan
      `FormRender.picker_panel: inline|aside`.
      **Adopsi (kafe + inventory/gl sekaligus):** `cafe-order.order` +
      `order-form-qr` (baru), `cafe-stock.purchase-order` + `purchase-order-form`
      (baru), `cafe-stock.stock-opname` (`quantity_field: counted_qty`),
      `inventory.stock-movement` + `stock-movement-form` (baru),
      `gl.journal-entry` + `journal-entry-form` (baru, tanpa `quantity_field` —
      satu akun per baris). Halaman QR kini Page biasa + blok `form`; **blok
      transaksional di `PageBlock` dihapus lagi** (closed set kembali seperti
      semula).
      Test: `lib/picker.test.ts` (25), parity widget S10 (termasuk `hidden`),
      `go test ./...` 35 paket ok, `vitest` 250 lulus. `formspec validate` kafe
      **0 problem** (72 manifest); inventory/gl jumlah problem **identik
      sebelum-sesudah** (drift schema lama, tidak ada error menyebut `picker`).
      ✅ 2026-09-15 — dipilih **blok Page**, bukan kind baru: Page sudah
      mengomposisi blok, sedangkan kind baru menuntut registrasi kind, rute,
      permission, docs-kind, dan dispatcher SPA. `Listing` juga tidak diperluas —
      kontraknya sendiri (katalog publik read-only) akan rusak oleh aksi baris.
      Bentuknya: `catalog` (entity + nama/harga/gambar/kategori + `filter` +
      `search` + `columns`), `lines` (pemetaan ke child entity: item, qty,
      snapshot harga/nama, catatan, `max_quantity`), `checkout` (entity +
      `fields` FormField — jadi kosakata widget S10 berlaku + `defaults` +
      label/sukses/reset).
      **Join harga per cabang**: kafe menyimpan harga di entity terpisah
      (`menu-item-price`), jadi blok mendukung
      `price_entity`/`price_match_field`/`price_filter` — join client-side, dan
      item tanpa harga tampil **tidak bisa dipesan** (bukan dikirim sebagai `0`).
      **Interpolasi `defaults`** memakai `{dotted.path}` yang sudah dipakai
      `ContextDecl.id` (scope = render context halaman + token blok-lokal
      `{now}`/`{today}`); token yang tak bisa diselesaikan dibiarkan **verbatim**
      supaya terlihat di payload, bukan jadi field wajib yang tampak terisi.
      **Blok tidak menghitung total pesanan** — itu kontrak Entity (`computed`).
      Renderer: `kinds/page/blocks/OrderBuilderBlock.tsx`, logika murni di
      `lib/orderBuilder.ts` (25 test), dispatch di `PageRenderer.tsx`.
      **Bukti runtime** (dev server, spec kafe, satu DB segar — lihat catatan
      nomor pesanan di bawah):

                                              | Langkah | Hasil |
                                              | --- | --- |
                                              | Anonim baca katalog (`is_available=true`) | `['Kopi Susu','Roti Bakar']` |
                                              | Anonim baca baris harga cabang | 2 baris `{amount,currency}` |
                                              | Context halaman (sesi dari route) | `session.id`, `branch_id`, `table_id` |
                                              | POST payload hasil `buildSubmitPayload` | **201** — `number=ORD-2026-00001`, `channel=qr_table`, `guest_note` terbawa, `note` per baris terbawa, `line_total` **{50000,12500}** dan `subtotal` **{62500}** dihitung server (S7) |

                                              Halaman `cafe-order/pages/menu-catalog.yaml` (route `/menu/:session_id`,
                                              `public: true`). Adopsi: `line_total` + `subtotal` kini `computed` di
                                              entity order (komentar GAP-02 yang kedaluwarsa dihapus);
                                              `total_amount` **belum** diturunkan karena rantai diskon/pajak butuh nilai
                                              Config cabang — dicatat di entity, bukan dikira-kira.
                                              _Sisa (dicatat, bukan disembunyikan):_
                                              - **Token QR → sesi** masih dua langkah (aplikasi membuat/menemukan sesi
                                                dulu, halaman mengambil ID-nya): `GET /entity/{id}` me-resolve ID dan
                                                natural key, sedangkan `guest_token` bukan keduanya. Menjadikan token
                                                kunci milik tamu = **2.2** (sisa #45). Halaman **tidak** berpura-pura
                                                sudah bisa.
                                              - **Sisi kasir belum tersentuh**: numpad uang & kembalian menunggu widget
                                                uang (**2.14**), layar POS sebagai kind tersendiri menunggu `kind: Pos`.
                                                Marker GAP-05 di `pos-workbench.yaml` diperbarui: pelanggan tertutup,
                                                kasir masih terbuka.
                                              - **3 bug engine ditemukan & diperbaiki** selama mengerjakan ini (kelas
                                                "diam-diam salah", jadi bagian dari pekerjaan, bukan deferred):
                                                (a) filter boolean `?flag=true` **mencocokkan nol baris** (nilai string
                                                "true" dibandingkan dengan kolom hasil cast numerik) → katalog QR akan
                                                kosong tanpa gejala; kini `true/false/1/0/yes/no` diterima untuk field
                                                boolean (`coerceFilterValue`, dipakai List/Aggregate/Window);
                                                (b) `created_by`/`updated_by` NULL (baris hasil seed/migrasi/operator)
                                                membuat **setiap** pembacaan entity 500 (`converting NULL to string is
                                                unsupported`) → `scanEntityRecord` memakai `sql.NullString`;
                                                (c) gerbang permission `source: entity` di render context memakai nama
                                                **singular** (`{module}.{entity}.view`) padahal permission terdaftar
                                                `{module}.{plural}.view` → deklarasi `context` entity **tidak pernah**
                                                resolve kecuali pemanggil punya `*` (seed dev), dan permukaan publik
                                                mustahil; kini plural dari metadata + permukaan `public: true`
                                                melewati pra-cek (server tetap otoritas).
                                              - **Dua gap lama tetap terbuka** (bukan bagian 1.5): menulis harga lewat
                                                API gagal karena `guard_menu_item_price_unique.star` memakai SQL mentah
                                                dengan nama kolom yang tidak ada (#30/#31, item 4.5) — seed harga di
                                                verifikasi memakai SQL langsung; dan nomor pesanan `ORD-2026-00001`
                                                bertabrakan saat cabang kedua membuat pesanan di DB yang sama
                                                (`scope_field` per cabang + index unik global) → item **1.6**/#9/3.6,
                                                verifikasi memakai DB segar.

- [x] **1.6 — S8: unique parsial + index atas relasi** (prioritas §F #6).
      Target: `pkg/spec/entity.go` (`IndexDecl.where`), `renderers/jsonb-persist/ddl.go`
      (§4 indexes :242, kolom turunan relasi), `renderers/jsonb-persist/migrate.go`.
      Menutup akar: **#22**, **#23**.
      _Accept:_ `formspec migrate plan` menghasilkan `CREATE UNIQUE INDEX`
      untuk `(branch_id, menu_item_id)` **dan** partial `WHERE status='open'`
      untuk shift.
      ✅ 2026-09-15. **Relasi sebagian sudah tertutup** oleh 3.1/#22; yang benar-benar
      sisa adalah **predikat parsial**. Yang dikerjakan:
      (a) `IndexDecl.Where` + parser grammar **tertutup** di `pkg/spec/indexwhere.go`
      (`<field> <op> <literal>`, `<field> IS [NOT] NULL`, digabung `AND`) —
      predikat ditulis dengan **nama field**, divalidasi `ValidateEntitySpec` di
      kedua situs deklarasi (`indexes` dan `persist.indexes`), dan menerjemahkan
      nama field → kolom turunan saat render (`_status`, sama seperti `fields:`).
      Grammar tertutup dipilih sadar: teks ini masuk ke DDL, jadi string bebas =
      injeksi lewat manifest + tidak bisa divalidasi.
      (b) **Dua bug nyata ikut ketemu** (keduanya kelas gagal-senyap):
      — `diffExistingTable` hanya merekonsiliasi **kolom**, tidak pernah membuat
      **index**. Jadi `indexes:` yang ditambahkan setelah tabel ada **tidak pernah**
      terpasang di DB yang sudah jalan. Kini index dijalur alter juga dibuat
      (intropeksi nama, `IF NOT EXISTS`, dan plan tetap 0 saat sudah konvergen).
      — Field yang hanya disebut di **predikat** tidak pernah dapat kolom turunan →
      `CREATE INDEX ... WHERE _status = ...` gagal `no such column: _status`.
      Kini `indexDeclFields` mengumpulkan field dari index **dan** predikat.
      (c) **Adopsi kafe — workaround DDL mentah dihapus.** Ketiga aturan kini
      dinyatakan di manifest dan `kind: Migration` mentahnya dihapus
      (`menu-item-price-unique`, `stock-level-unique`, `shift-open-unique`) —
      sekaligus menutup GAP-35 untuk kasus ini (partial index portabel).
      Bukti:
      | Perintah | Hasil |
      | --- | --- |
      | `formspec migrate plan` | `CREATE UNIQUE INDEX ...menu_item_prices (…, _menu_item_id);` **dan** `CREATE UNIQUE INDEX ...shifts (…, _cashier_id) WHERE _status = 'open';` |
      | `formspec migrate apply` (DB segar) | 24 structural + 0 custom — index benar-benar ada di `sqlite_master` |
      | INSERT 2 shift `open` (B1,C1) | **REJECTED** `UNIQUE constraint failed` |
      | INSERT shift `closed` (B1,C1) ×2 | **OK** — parsial, hanya `open` yang dibatasi |
      | INSERT shift `open` (B2,C1) | **OK** — per cabang |
      | `go test ./...` | hijau (test baru: parser 2, validation 1, DDL 2, migrate 1) |
      Test: `pkg/spec/indexwhere_test.go`, `ddl_test.go` (`TestGenerateEntityDDL_PartialIndex*`),
      `migrate_test.go` (`TestMigrationRunner_NewDeclaredIndexReachesExistingTable`).
      Normatif: `docs/spec/backend/01-core-basic.md` §3 (contoh `indexes:` sebelumnya
      mendokumentasikan bentuk yang **tidak ada** — `{field, type}`; dikoreksi).
      Plan: `docs_internal/plan/s8-partial-index.md` · changelog: `2026-09-15-008`.
      **Sisa (bukan bagian 1.6) — ✅ TERTUTUP 2026-09-16 oleh 3.4.** Catatan
      "GAP-36 tetap: constraint tidak bisa merapikan duplikat lama" ditulis
      **sebelum** `dml` ada. Sejak 3.4 (`kind: Migration` dicabut, `dml` +
      `reason` wajib masuk jalur migrasi otomatis), duplikat lama memang bisa
      dirapikan — dan skenario itu **diuji bisa gagal**: `CREATE UNIQUE INDEX`
      gagal dengan 3 baris duplikat, berhasil setelah `dml` merapikannya
      (3 → 2 baris). Prosa ini dipertahankan hanya sebagai jejak; tidak ada
      pekerjaan terbuka yang tersisa di sini.
- [x] **1.7 — S9: workflow merujuk nama transisi** (prioritas §F #7).
      Konstruksi: `on: { transition: { name: <via> } }` + `from`/`to` sebagai
      bentuk lama (saling eksklusif).
      Target: `pkg/spec/` (`WorkflowTransitionRef.Name` + `ValidateWorkflowSpec`),
      `internal/workflow/` (index `byName` + `ForTransition(entity, transition,
from, to)`), `cmd/formspec/validate_workflow.go` (Layer 1.5 cross-manifest),
      `internal/manifest/loader.go`, dua call site `internal/api/handler.go`.
      Menutup: **#38**. ✅ 2026-09-15 (`docs_internal/changelog/2026-09-15-009-*`).
      _Accept:_ void dari `paid`/`in_kitchen`/`ready`/`served` **semua** butuh approval.
      **Bukti:** `TestRegistry_ForTransitionByName` menguji keempat state asal
      (lolos) + `cancel-order` (tidak lolos) + entity lain (tidak lolos);
      delegasi runtime `actionName` (= `via`) diteruskan di kedua call site;
      `order-void-approval.yaml` kini `name: void-order`.
      **Lubang yang ditutup validator (bukan hanya dilaporkan):** pasangan
      `from`/`to` pada transisi multi-asal kini **ditolak** `formspec validate`
      dengan pesan yang menyebut `name:` — sebelumnya lolos hijau sambil tidak
      menegakkan apa pun. Nama transisi yang salah juga ditolak, dengan daftar
      `via` yang tersedia.
      **Catatan bentuk:** ledger mengusulkan `transition: <string>`. Itu union
      string-atau-objek pada satu field; generator schema hanya bisa
      mengekspresikan bentuk objek, jadi menambahkannya berarti menambah satu
      lagi kelas "lolos engine, ditolak schema" (preseden `guard:`/`render:`).
      Dipilih `transition: { name: ... }` — maksud yang sama, tanpa divergensi.
      **Sisa (bukan bagian 1.7):** tidak ada test level-API untuk interception
      approval (harness auth+seed belum ada) — lihat TODO master Fase 7.4;
      bukti saat ini unit-level (registry/engine/validator).
      ✅ **TERTUTUP 2026-09-22** — `7.4.7` mendaratkan harness itu
      (`internal/api/workflow_approval_api_test.go`, 5 test HTTP lewat
      `HandleCustomAction`), dan justru menemukan bug nyata: requester bisa
      menyetujui request-nya sendiri. Prosa di atas dipertahankan sebagai jejak
      sejarah; tidak ada pekerjaan tersisa.
- [x] **1.8 — S5/S11/S12/S14: konstruk pelengkap** (prioritas §F #9–#10, versi minimal).
      `spec.scope {dimension, field}` + `assignments` (S5), field type `percent`
      (S11 minimal), deklarasi satuan `unit: {base, convertible}` (S12 minimal),
      `maintained_by` + `invariants` pada `characteristic: summary` (S14).
      _Accept:_ spec kafe dapat menyatakan scope cabang, resep multi-satuan, dan
      kontrak pemelihara `stock-level` tanpa komentar "tidak bisa dinyatakan".
      ✅ 2026-09-15 — plan `docs_internal/plan/s5-s11-s12-s14-scope-percent-unit-summary.md`
      · changelog `2026-09-15-010`. Ringkas:
      (a) **S5** — tiga konstruk, tiga pertanyaan: `scope: {dimension, field,
required}` (fakta partisi, **tidak** memfilter), `row_scope` (**nama baru**
      untuk filter yang ditegakkan — dulu bernama `scope`; tidak ada spec yang
      memakainya sehingga rename-nya murah, dan satu nama tidak bisa dua bentuk),
      `assignments: [{dimension, field, principal_field}]` (**dari mana** nilai
      seorang principal berasal). Nilai `from: session` kini **diselesaikan**
      dari `assignments` kalau token tidak membawanya (`internal/api/scope.go` + `entity.FindAssignmentValue`, memo 30 detik) — inilah bagian S5 yang
      sebelumnya tidak ada sama sekali: "pengguna ini bertugas di cabang X".
      (b) **Gerbang validator (inti kejujuran):** `formspec validate` menolak
      `row_scope` `from: session` tanpa `attr` yang atributnya tak punya sumber
      — bentuk yang kalau dibiarkan akan **403 selamanya** dengan manifest
      terlihat benar (kelas yang sama dengan #52/#53/GAP-33).
      (c) **S11** `percent`: numerik = `decimal` (`10` = 10%), berbeda di
      interpretasi/rendering; klien memakai `decimalinput` + `format: percent`.
      (d) **S12** `unit: {base, convertible}` pada field satuan; `base` dan
      `convertible` wajib ada di `enum_values`; satuan dimensi lain (`pcs` vs
      `gram`) sengaja tidak dikonversi. Konversi engine-nya = **4.6**.
      (e) **S14** `maintained_by` (wajib menunjuk script yang ada **dan bisa
      dikompilasi**, dicek bersama `impl.ref`) + `invariants[{unique, message}]`
      (wajib ditopang **unique index** yang benar-benar dideklarasikan) — jadi
      invarian summary ditegakkan **database**, bukan disiplin script, dan
      hook yang tak pernah jalan tidak lagi bisa terlihat sebagai perlindungan.
      **Adopsi kafe (bukti accept):** 15 entity menyatakan `scope`; `employee`
      menyatakan `assignments`; `branch.tax_percent`/`service_charge_percent`
      dan `promo.percent` menjadi `percent`; `ingredient.unit` +
      `recipe.lines[].unit` menyatakan gram/kg; ketiga summary menyatakan
      `invariants`, dan `stock_level_apply.star` (rename dari
      `guard_stock_level_unique.star`) menjadi `maintained_by` `stock-level`.
      `formspec validate` kafe **0 problem** (69 manifest); `go test ./...`
      hijau; `vitest` 250 lulus; `tsc` bersih.
      **Sisa (dicatat, bukan disembunyikan):** - **Adopsi `row_scope` di kafe ditunda ke 3.5** — sesuai urutan ledger
      ("3.5: entity ter-scope difilter otomatis"). Menyalakannya sekarang
      mem-403 seluruh list kasir **dan** identitas dev yang tidak punya baris
      `employee` (fail closed yang benar, aplikasi tak terpakai). Yang sudah
      ada sekarang: konstruk, sumber nilai, dan gerbang validatornya. - **Pajak belum punya model** — S11 versi minimal hanya menambah tipe
      `percent`; dasar pengenaan, harga-termasuk-pajak, pembulatan pajak, dan
      pelaporannya masih urusan spec aplikasi (bukan bagian 1.8). - Konversi satuan (S12) belum dihitung engine — **4.6**.
- [x] **1.9 — D1–D7 ditulis normatif** ke `docs/spec/backend/01-core-basic.md` dkk.
      _Accept:_ tidak ada lagi "belum ditetapkan" untuk D1–D7.
      ✅ 2026-09-15 — ditulis di tempat masing-masing, bukan satu lampiran: - **D1/D2** `01-core-basic.md` §1.2 — `create` → `draft`, kecuali
      lifecycle-free; **`lifecycle:` adalah hint UI**, penentunya aksi mana
      yang aktif (menutup D2 persis seperti jawaban `decisions-needed.md`). - **D3** §7 — transisi **tidak** memancarkan event otomatis
      (`TransitionDecl` tanpa `emit`); integrasi tidak boleh mengandalkan
      "nama event = nama state". - **D4** §7 — `before_*` sync / `on_*` async; **nama state polos bukan
      konvensi** yang ditegakkan. - **D5/D6** §8.6 (baru) — kanonik `{module}.{plural}.{action}`; bentuk
      singular tidak setara dan tidak pernah cocok; **`submit` punya
      permission sendiri**, bukan `update`. - **D7** §11 — `settings` hidup di `kind: Config` level App dan dibaca
      **framework**, bukan hanya renderer (mata uang `money`; ragu = tolak,
      bukan tebak).

---

## Fase 2 — Unblock loop pesanan QR

> **Prasyarat:** 1.2, 1.3, 1.4, 1.5. Ini yang membuat app **bisa dipakai**.

- [x] **2.1 — #44: `lifecycle: none` nyata + auto-submit anonim.**
      Record master langsung _referenceable_; order anonim masuk `submitted`.
      Target: `renderers/jsonb-persist/crud.go` (:510 `doc_status`), `internal/api/generator.go`
      (:135), `pkg/spec/entity.go`.
      _Accept:_ `menu-item` dapat mereferensikan `menu-category` hasil `create` tanpa
      langkah `submit` manual.
      ✅ 2026-09-14 — **tanpa menambah nilai enum baru**: aturan tunggal
      `EntitySpec.LifecycleFree()` → `lifecycle: plain_crud` (atau alias `none`), **atau**
      tanpa lifecycle eksplisit dengan `characteristic: master`/`reference`. Data katalog
      tidak punya alur draft→submit; lifecycle untuk dokumen. Dipakai `crud.go`
      (`submitEnabled := !entity.LifecycleFree()`), `internal/api/generator.go`
      (`disabledActions()` → rute submit/cancel/amend tidak dibuat), dan
      `internal/entity/registry.go` (permission tidak diregistrasi). **Verifikasi runtime:**
      `menu-item` → **HTTP 201** (sebelumnya 422 `is draft`). Test:
      `TestGenerateUIRoutes_LifecycleActions` (katalog tanpa rute lifecycle; transaksi punya),
      `TestEntityStore_ResolveRelations_NoDeadlockUnderTxScope` (target langsung valid).
      _Catatan:_ ini menyelaraskan server dengan semantik frontend (`plain_crud` = tanpa
      Submit) — akar #44. Pertanyaan “default apa untuk entity tanpa characteristic” dicatat
      di `decisions-needed.md` (D2).
- [x] **2.2 — #45: `create` anonim tidak menghasilkan sampah.**
      Izin publik per-entity + pernyataan langkah penyelesaian + kepemilikan token tamu.
      _Accept:_ order anonim bisa dilanjutkan kasir (bayar) & tidak menumpuk `draft`.
      ✅ 2026-09-15 — plan `docs_internal/plan/public-grant-row-scope.md` · changelog
      `2026-09-15-011`. **Konstruk baru: `public_entities[].scope`** — filter baris
      per-permukaan untuk pembacaan anonim. Grant mengatakan entity MANA yang boleh
      dibaca anonim; scope mengatakan BARIS mana. Nilainya dibaca server dari
      parameter request (token tamu), jadi klien tidak bisa melebarkannya, dan
      parameter yang tidak ada **menolak permintaan** — bukan berarti "tanpa filter".
      **Bukti runtime** (dev server, kafe, 2 pesanan dengan token berbeda):

      | Permintaan anonim | Hasil |
      | --- | --- |
      | `list order` (tanpa token) | **403** `readable anonymously only together with a "guest_token" request parameter` |
      | `list order?guest_token=TOKENA` | `total: 1`, hanya TOKENA |
      | `list order?guest_token=TOKENB` | `total: 1`, hanya TOKENB |
      | `list order?guest_token=TOKENA&guest_token[eq]=TOKENB` | **`total: 1`, hanya TOKENA** — klien tidak bisa melebarkan |
      | `list order?guest_token=NOPE` | `total: 0` (bukan error, bukan semua baris) |
      | `list order?status=paid` (tanpa token) | **403** — tidak bisa dilewati |
      | `create order` (anonim) | **422** validation (mencapai handler → jalur pesan QR utuh) |
      | `list menu-item` (grant tanpa scope) | **200** |
      Test: `pkg/spec` (3 case scope grant + 4 reject), `internal/api` (5 case)
      termasuk `TestRequirePermissionOrAnonymous`.
      **Tiga hal yang ikut diperbaiki karena jalur ini menyentuhnya:**
      (a) **Grant publik bukan lagi bypass permission.** Dulu grant menyetel
      permission rute menjadi `"public"`, sehingga siapa pun yang sudah login
      melewati `cafe-order.orders.list` di route itu — dan karena `/_ui/entity`
      dipakai bersama, memberi anonim `list` pada `order` akan **mencabut** gerbang
      permission dari table POS & kanban KDS. Kini grant hanya mengizinkan
      **anonim**; pemanggil terautentikasi tetap wajib punya permission, dan scope
      anonim tidak diterapkan padanya (kalau diterapkan, table POS yang tidak
      membawa token akan 403).
      (b) **Parameter scope grant tidak lagi diparse sebagai filter field**
      (422 `unknown field`) — sama seperti perbaikan 1.1 untuk `row_scope`.
      (c) `order` kini punya field `guest_token` (disalin dari sesi meja oleh
      `order-form-qr`), dan `order-status-page` memakai token itu sebagai kunci —
      komentar "GAP-06: bergantung pada pengaman aplikasi" di halaman itu
      **dihapus** karena engine sekarang yang menegakkannya.
      **Bagian "tidak menumpuk `draft`" & "kasir bisa melanjutkan"** sudah benar
      sejak #44/#52 (katalog & order lifecycle-free → record anonim langsung
      referenceable oleh `payment`), dan diverifikasi ulang di 1.2.
      **Sisa (dicatat):** alur scan QR masih dua langkah (token → sesi → ID sesi)
      karena `find` tidak bisa di-scope; menjadikan token satu-satunya kunci
      menuntut kind/halaman yang men-resolve sesi dari token, bukan dari ID
      → satu paket dengan **2.15 ⏸️** (lihat juga **10.5(a) ⏸️**).

- [x] **2.3 — #46 + #26: `money` divalidasi & dinormalisasi di batas API.**
      Satu bentuk kanonik `{amount, currency}`; `currency` dari `settings.currency`;
      tolak bila tetap tak bisa ditentukan. Target: `renderers/jsonb-persist/crud.go`,
      `pkg/spec/money.go` (`ResolveMoneyCurrency`).
      _Accept:_ kirim `25000` dan `{amount:"15000"}` → tersimpan ternormalisasi;
      tanpa `settings.currency` → **error**, bukan data tanpa mata uang.
      ✅ 2026-09-14 — `spec.NormalizeMoneyValue()` baru (number | numeric string |
      `{amount[, currency]}` → selalu `Money{amount, currency}`) dipanggil dari
      `HandlerFactory.normalizeMoneyFields()` di `HandleCreate` **dan** `HandleUpdate`;
      `NormalizeMoneyValue` → dilanjutkan `spec.ValidateMoneyValue()` (kontrak lama yang
      **tidak pernah dipanggil** di luar test) untuk cek currency-mismatch & scale.
      **Verifikasi runtime:** `25000` & `{amount:"15000"}` & `"7000"` → semua tersimpan
      `{"amount":"…","currency":"IDR"}`; `"Rp25.000"` → **422** dengan pesan menuntun.
      Test: `TestNormalizeMoneyValue`.
      _Sisa:_ `settings.currency` masih dari Config App (diverifikasi berfungsi di dev);
      penolakan keras bila currency tetap tak bisa ditentukan sudah ada.
- [x] **2.4 — #2: renderer `money` menerima bentuk objek.**
      Target: `renderers/react-shadcn/src/lib/renderCell.tsx` (:29),
      `src/lib/format.ts`, `DetailPage.tsx`, `ReportRenderer.tsx`, `DashboardRenderer.tsx`.
      _Accept:_ harga tampil `Rp25.000`, bukan `{"amount":...}`.
      ✅ 2026-09-14 — helper `moneyAmount()` di `lib/format.ts` menerima number, numeric
      string, dan `{amount, currency}`; dipakai `renderCell.tsx`, `DetailPage.tsx`
      (type `money` sebelumnya jatuh ke cabang JSON!), `ReportRenderer.tsx`,
      `DashboardRenderer.tsx`. Test: 5 case baru di `format.test.ts`; `vitest run`
      **171 test lulus**, `tsc -p tsconfig.app.json --noEmit` bersih.
- [x] **2.5 — #4 + #4b: gambar produk tampil; `storage.allowed_types` konsisten.**
      Render gambar di Table/Listing/Detail; samakan format `jpg` vs `.jpg` vs `image/jpeg`.
      Target: `src/renderer/components/`, `pkg/spec/` (StorageSpec) + validator.
      _Accept:_ foto menu tampil di katalog publik.
      ✅ 2026-09-16 — plan `docs_internal/plan/media-image-cells.md` · changelog
      `2026-09-16-001`. **#4b (akar masalahnya lebih dulu).** Bentuk kanonik
      `allowed_types` = **ekstensi tanpa titik** (`[jpg, png, webp]`), dan itulah
      satu-satunya bentuk yang **tidak cocok dengan apa pun** sebelumnya: ia bukan
      `.jpg`, bukan MIME, bukan wildcard — jadi foto menu yang sah ditolak
      "File type not allowed" di klien **dan** server. Kini keempat ejaan
      (`jpg`, `.jpg`, `image/jpeg`, `image/*`) diperlakukan sama oleh matcher
      klien (`src/lib/media.ts`, dipakai bersama `FileInput`) dan server
      (`internal/api/file.go`), dan `formspec validate` menolak entri di luar
      keempat bentuk itu (`JPG`, `*.jpg`, `"jpg, png"`). `menu-item.photo` tidak
      lagi menuliskan dua bentuk sekaligus. "Banyak file" = `max_count > 1`
      (bukan tipe field terpisah) — didokumentasikan di `05-field-types.md` §1.3.
      **#4: gambar kini dirender sebagai gambar.** `renderCellValue` mendapat
      cabang `widget: image` (nilai `file` = object key → `<img src>` dari route
      unduh), dan `image` masuk **kosakata tertutup** `TableCellWidget` (S10) +
      katalog klien — paritas schema↔katalog↔renderer tetap dijaga
      `catalog.test.tsx`. Turunan otomatis: field file yang `allowed_types`-nya
      memuat gambar mendapat `widget: image` tanpa ditulis di manifest; file
      non-gambar tetap tautan unduh (perilaku lama). Table & Listing meneruskan
      URL unduh lewat `CellRenderOpts.imageUrl` (satu helper `fileDownloadUrl`,
      sekarang juga dipakai `PickerPanel` yang sebelumnya menyusun URL sendiri),
      dan DetailPage merender `<img>` sebelum tautan unduh.
      **Bukti:** `pkg/spec` (5 test: 4 bentuk diterima, 5 bentuk ditolak,
      `StorageAllowsImage`, entity-level), `internal/api` (`TestAllowedFileType`,
      8 case termasuk regresi bentuk kanonik), klien
      `src/lib/media.test.ts` (8 case) + parity widget; `go test ./...` hijau,
      `vitest` **258** lulus (dari 250), `tsc` bersih, kafe `validate`
      **0 problem**.
      **Sisa (dicatat, bukan disembunyikan):** - **Belum ada verifikasi runtime gambar di browser.** Yang terbukti:
      helper, cabang renderer, paritas kosakata, dan route unduh yang sudah
      dipakai `PickerPanel`/`FileInput` (terverifikasi sejak 1.5/7.17). Yang
      **belum**: mengunggah foto sungguhan lalu melihatnya di katalog — butuh
      sesi terautentikasi (upload = permission `update`) yang tidak tersedia di
      dev server tanpa seed user. Kafe belum punya kolom tabel ber-`photo`,
      jadi jalur Table/Listing diuji di level unit, bukan di aplikasi. - `Print` belum ikut: `resolveCellValue()` masih mencetak key sebagai teks,
      dan cetak gambar butuh URL absolut (bukan key) — bagian dari 7.1/2.6. - **Thumbnail `transform` belum diverifikasi** (item 5 di gap doc):
      kolom tabel memakai gambar penuh, bukan hasil resize. Perlu dicek terpisah
      apakah transform benar-benar digenerate saat upload → tercakup item
      **7.17.3 ⏸️** (Transform server-side belum dikerjakan; selama itu, kolom
      memang memakai gambar penuh). Bukan item kafe tersendiri.
- [x] **2.6 — #3/S4: QR code.** ✅ **2026-09-20 — SELESAI (jalur cetak + adopsi struk)**
      Field type/widget `qrcode` read-only (`derived_from`) untuk QR meja & QR struk.
      Target: `pkg/spec/frontend.go` (`FieldType` closed set), widget baru di
      `src/widgets/`, barrel katalog (§1.4).
      _Accept:_ QR meja bisa dirender & dicetak dari spec.
      _(Catatan 2026-09-15: 1.4 sudah menutup kosakata widget — begitu widget
      `qrcode` diimplementasikan, ia masuk `pkg/spec/widget.go` (FormWidget +
      `IsFormWidget`) dan enum schema ikut ter-regenerate; test paritas
      `src/widgets/catalog.test.tsx` akan gagal bila lupa.)_
      ✅ 2026-09-16 (🟡 **sebagian** — sengaja TIDAK ditandai selesai, lihat "Sisa":
      accept-nya menuntut "dirender **& dicetak**", dan jalur cetak belum ada).
      **Jalur termurah dipilih (keputusan pemilik proyek): widget
      read-only + dependency klien**, bukan field type baru dan bukan Service
      engine. `qrcode.react` (MIT, SVG — tajam saat dicetak, tanpa canvas/DPR).
      **Yang dikerjakan:** widget `qrcode` masuk **dua** kosakata tertutup (S10)
      dengan nama yang sama — `FormWidget` (form: menggantikan input, karena
      nilainya ADALAH payload dan tidak ada yang bisa diketik) dan
      `TableCellWidget` (sel tabel/listing: menggantikan teks) — plus komponen
      `src/widgets/QrCode.tsx` (SVG, `level: M`, teks pengganti bila kosong),
      cabang di `FormFieldWidget` **dan** `renderCellValue` (paritas dijaga
      `catalog.test.tsx`), barrel widget, dan enum schema ter-regenerasi
      (`$defs/FormWidget` + `$defs/TableCellWidget` kini memuat `qrcode`).
      **Bukti:** `go test ./...` hijau (test kosakata tertutup diperbarui:
      FormWidget 22, TableCellWidget 4), `vitest` 258 lulus, `tsc` bersih, enum
      schema terverifikasi memuat `qrcode` di kedua `$defs`.
      **Sisa (dicatat — inilah sebabnya item ini masih terbuka):** - **Jalur cetak belum ada.** `Print` memakai `resolveCellValue()` yang
      mengubah nilai jadi teks, jadi `kind: Print` struk/kartu meja belum bisa
      memuat QR — padahal "QR meja untuk dicetak dan ditempel" itu inti
      kebutuhannya. Ini bagian dari **7.1** (Print), bukan lagi kosakata widget. - **Belum diadopsi di spec kafe**, dan penghalangnya konkret: QR yang bisa
      dipindai ponsel butuh **URL absolut**, sedangkan `dining-table` hanya
      menyimpan kode meja (`A-01`) — origin aplikasi tidak diketahui entity.
      Menyusun URL absolut (dan menyuntikkan origin saat cetak) adalah
      pekerjaan pemanggil, bukan widget. Sampai ada tempat untuk itu, menaruh
      `widget: qrcode` di spec kafe hanya akan menghasilkan QR berisi "A-01"
      yang tidak menuju apa pun — jadi sengaja **tidak** dipasang, bukan
      dipasang supaya terlihat tertutup. - Jalur termurah berikutnya: field `qr_url` pada `dining-table` yang diisi
      URL absolut (dari Config `settings.*` atau komposisi print-time), lalu
      `widget: qrcode` pada kolom tabel/kartu meja + `kind: Print` struk. - QR di struk digital (`receipt-digital.yaml`, GAP-03) menunggu hal yang
      sama: URL struk absolut. - **Scanning** (barcode/QRIS) tetap di luar cakupan item ini.
- [x] **2.14 — #1 (separuh renderer): widget `MoneyInput` + `TimeInput`.**
      Separuh lain gap #1. 1.4 menutup **akar**-nya (kosakata tertutup: menulis
      `widget: money-input` sekarang **gagal validasi** dengan pesan jelas alih-alih
      diam-diam jadi input teks), tetapi widget-nya sendiri belum ada: field `money`
      dan `time` masih jatuh ke `input` (`derive.formWidget()` default).
      Target: `renderers/react-shadcn/src/widgets/MoneyInput.tsx` + `TimeInput.tsx`
      (numpad-friendly, format & pembulatan ikut `settings` — `format.money`,
      `ResolveRoundingMode`), `derive.formWidget()` (`money → moneyinput`,
      `time → timeinput`), `buildZodField()`, `pkg/spec/widget.go`
      (`WidgetMoneyInput`, `WidgetTimeInput`), `make generate-schema`, dan
      `src/widgets/catalog.test.tsx`.
      _Accept:_ `menu-item.price` & `payment.tendered` dirender dengan format mata uang
      saat mengetik; `widget: money-input` **valid** (bukan lagi error) dan dipakai
      kasir tanpa keluar dari numpad.
      _Prasyarat:_ tidak ada (S7 sudah menetapkan aritmetika `money`).
      ✅ 2026-09-16 — plan `docs_internal/plan/money-time-input-widgets.md` ·
      changelog `2026-09-16-003`.
      (a) **Nama kanonik `moneyinput`/`timeinput`**, bukan `money-input` —
      mengikuti keluarga yang sudah ada (`fileinput`, `datetimeinput`,
      `decimalinput`). Jadi bunyi accept di atas dipenuhi dalam roh (widget uang
      ada, valid, dipakai kasir), dengan ejaan yang konsisten dengan katalog;
      `money-input` tetap ditolak validator seperti nama lain di luar himpunan.
      (b) **Nilai money tetap eksak.** Jumlah disimpan sebagai **teks** selama
      mengetik (`12.345` tidak dibulatkan menjadi `12.35` sebelum selesai), dan
      yang dikirim adalah bentuk kanonik `{amount, currency}` — mata uang tidak
      pernah ikut hilang. Angka/string telanjang (payload lama) tetap diterima
      dan di-upgrade saat diedit; field dikosongkan → `null`, bukan `0`.
      (c) **Mata uang & skala dari settings**, dengan override per field
      dihormati: `settings.currency`/`settings.locale` untuk tampilan, dan bila
      field mendeklarasikan mata uangnya sendiri, pratinjau memakai skala field
      itu (`decimal_places`) + kodenya — bukan simbol mata uang global.
      `inputMode: decimal` memberi numpad di perangkat sentuh.
      (d) **`timeinput`** memakai kontrol `type=time` asli (picker di tablet) dan
      menyimpan `HH:MM:SS`, sehingga kontrak tipe field `time` tetap terjaga.
      (e) **Yang membuatnya benar-benar sampai ke kasir:** manifest form tidak
      menulis `widget:`, dan `FormFieldWidget` memakai `field.widget ??
entityField.type` — jadi `money` akan tetap jadi input teks walau widget
      ada. Fallback itu kini lewat `implicitWidgetForType` (`money → moneyinput`,
      `time → timeinput`), sengaja sempit: memperluasnya ke semua tipe
      (`enum → select`, `relation → relation-picker`) adalah perubahan
      tersendiri, dan form turunan sudah lewat `derive.formWidget`.
      **Adopsi kafe:** marker `GAP-01` ditutup di seluruh spec (5 entitas + 4 form + 1 wizard), termasuk dibersihkan dari komentar gabungan `GAP-01/GAP-02`
      sehingga yang tersisa benar-benar hanya GAP-02 (indeks money).
      **Bukti:** `vitest` **265** (dari 258; 7 test baru — bentuk kanonik,
      presisi tidak dibulatkan, clear → `null`, read-only; time: detik disimpan,
      detik dipertahankan, clear ≠ midnight) · `tsc` bersih · `go test ./...`
      hijau · kafe `validate` **0 problem** · test kosakata tertutup diperbarui
      (FormWidget **24**).
      **Sisa (dicatat):** belum ada verifikasi runtime di browser (numpad dan
      pratinjau diuji lewat jsdom, bukan di perangkat); `Print` belum memformat
      money lewat widget yang sama — kategori yang sama dengan **10.4 ⏸️**
      (verifikasi runtime di browser belum pernah dijalankan untuk jalur UI kafe).
      Catatan: bagian "`Print` belum memformat money" sudah **tertutup** di 2.6.
- [x] **2.7 — #47: dokumentasi kontrak REST `/_ui/`** + `formspec describe` mencetak
      kontrak HTTP. Target: `docs/runtimes/`, `cmd/formspec/get.go`.
      ✅ 2026-09-16 — plan `docs_internal/plan/ui-rest-contract.md` · changelog
      `2026-09-16-005`. Dua bagian, dan yang kedua yang membuatnya tidak bisa
      basi:
      (a) **Halaman `docs/runtimes/06-ui-rest-contract.md`** — bentuk path
      (`/{workspace}/_ui/entity/{module}/{entity}` — perhatikan: **nama entity
      singular**, plural hanya untuk permission & `api/v1`), tabel method/aksi/
      permission, **body flat** (envelope `{"data": …}` ditolak `400`), ketiga
      envelope respons, query `list` (`per_page` max 100, 13 operator filter,
      sort type-aware), catatan bahwa **parameter `row_scope` bukan filter**,
      dan ringkasan surface publik (`public_entities[].scope`).
      (b) **`formspec describe entity <name>` kini mencetak kontrak HTTP-nya**,
      dan route-nya **digenerate** dari generator yang sama dengan server
      (`api.UIRoutesForEntity` + `api.UICustomActionRoutesForEntity`, diekstrak
      dari `GenerateUIRoutes`/`GenerateUICustomActionRoutes` sehingga keduanya
      tidak bisa berbeda) — jadi ia tidak bisa menyebut endpoint yang tidak ada
      atau melewatkan yang ada. Ini penting karena kontraknya penuh pengecualian
      yang mudah salah ditulis tangan: aksi `disabled: true` tidak punya route,
      entity lifecycle-free tidak punya `submit`/`cancel`/`amend`, `summary`
      hanya `list`+`find`, dan **transisi state machine tanpa `impl` tidak punya
      endpoint sendiri** (diterapkan lewat `update`, guard transisi yang
      memvalidasi) — klaim terakhir itu kini juga tercetak, supaya tidak
      disalahartikan sebagai route yang hilang.
      **Bukti:** `formspec describe entity order` (kafe) mencetak tepat 4 route
      list/find/create/update — `delete` + `submit` memang `disabled: true` di
      spec kafe, jadi kontraknya sesuai perilaku runtime, bukan tebakan;
      `TestUIRoutesForEntity_LifecycleAndDisabled` (lifecycle penuh vs kafe
      `order` vs `summary`) dan `TestUICustomActionRoutesForEntity_OnlyActionsWithImpl`
      (transisi tanpa `impl` tidak menghasilkan route; permission berbentuk
      kanonik) · `go test ./...` hijau · kafe `validate` 0 problem.
      **Sisa (dicatat):** halaman itu belum digenerate dari `pkg/spec` seperti
      usulan #47 poin 2 — ia ditulis tangan, sedangkan bagian per-entity
      dilayani `formspec describe`. Report/Print/dashboard masih di luar cakupan
      halaman, dan kontrak `api/v1` tetap terpisah (§8.2/§8.4) → **10.5(c) ⏸️**.
- [x] **2.8 — #48: peringatan workspace aktif** saat startup bila
      `spec/workspaces/*` ada tapi workspace aktif berbeda; atau jadikan satu-satunya
      workspace sebagai default. Target: `cmd/formspec/dev.go`, `internal/api`.
      _(Catatan Fase 0: flag `--workspace-id kafe` terbukti dihormati → `tenant_id: "kafe"`;
      yang kurang hanya peringatan bila flag tidak diberikan.)_
      ✅ 2026-09-16 — plan `docs_internal/plan/active-workspace-resolution.md` ·
      changelog `2026-09-16-006`. **Keduanya dikerjakan** (peringatan + default),
      karena keduanya menutup gejala yang sama dari dua arah, dan aturannya
      dipilih supaya tidak ada kejutan: - **tidak ada flag + tepat satu workspace dideklarasikan → dipakai**, dan
      diumumkan: `workspace: kafe (the only one declared under spec/workspaces;
override with --workspace-id)`. - **tidak ada flag + lebih dari satu** → tetap `default`, dengan
      peringatan yang menyebut daftarnya (memilih salah satu diam-diam justru
      akan mengejutkan). - **`--workspace-id` yang tidak dideklarasikan** → diperingatkan: salah
      ketik akan mengirim semua tulisan ke tenant yang tidak pernah
      dideklarasikan siapa pun.
      Pembedaan "diberikan pengguna" vs "nilai default" ditambahkan sebagai
      `DevConfig.WorkspaceIDExplicit` (diisi dari flag; config file juga
      dianggap eksplisit) — tanpa itu, aturan "tepat satu → pakai" tidak bisa
      dibedakan dari pengguna yang memang menulis `--workspace-id default`.
      **Bukti runtime:** `formspec dev` pada spec kafe (yang mendeklarasikan
      `workspaces/kafe.yaml`) kini mencetak baris workspace itu, bukan lagi
      diam-diam memakai `default`. **Bukti unit:**
      `TestResolveActiveWorkspace` — 4 kasus (satu workspace diadopsi, flag
      eksplisit menang, tree tanpa workspace tetap `default`, dua workspace tetap
      `default` + peringatan). `go test ./...` hijau · kafe `validate` 0 problem.
      Dokumen: `docs/spec/platform/02-workspace-app-module.md` §1 kini menyatakan
      eksplisit bahwa manifest Workspace **mendaftarkan, bukan memilih** —
      kalimat yang dulu mudah dibaca sebaliknya.
      **Sisa (dicatat):** peringatan yang sama belum dipasang di `formspec serve`
      / `resource` (jalur non-dev) → item **8.1.7 ⏸️** di master todo; pengaruh
      ke `tenant_id` diverifikasi lewat flag pada catatan Fase 0, bukan lewat
      penulisan record di sesi ini.
- [x] **2.9 — #49: guard script kafe bisa dikompilasi.** Ganti implicit string-literal
      concatenation (tidak didukung Starlark) di 3 script: `cafe-master/.../guard_menu_item_price_unique.star`,
      `cafe-order/.../guard_shift_open_unique.star`, `cafe-stock/.../guard_stock_level_unique.star`,
      plus pesan `fail()` multi-baris.
      ✅ 2026-09-14 — konkatenasi implisit → `+`; guard `menu-item-price` kini **berjalan**.
      Sisa (dipindah ke `2.10` & `8.7`): dokumentasi dialek Starlark di `ai_skills/**` belum.
- [x] **2.10 — #51 (DIPERBAIKI) — `ctx.db().query(sql, args...)` menerima bind parameter.**
      Kontrak `Querier` (`internal/starlark/primitive.go:21`) menyatakan `query(sql, args...)`
      tetapi `builtinQuery` hanya menerima `sql` → `query: got 2 arguments, want at most 1`.
      ✅ Engine diperbaiki 2026-09-14 (`builtinQuery` + `fromStarlark` + `q.Query(ctx, sql, params...)`;
      test regresi `TestCtxDBQuery_BindArgs`). ✅ Dokumentasi 2026-09-14 — `docs/reference/primitives.md`
      "Dialek Starlark" + `ai_skills/entity-authoring`.
- [x] **2.11 — Dokumentasi dialek Starlark** (dari #49/#51): catat bahwa
      (a) **tidak ada** implicit adjacent string-literal concatenation — pakai `+`;
      (b) `ctx.db().query(sql, [args])` adalah bentuk resmi untuk query ber-parameter.
      Target: `ai_skills/**`, `docs/runtimes/04-formspec-sidecar.md`, `docs/spec/backend/`.
      _Accept:_ skill tidak lagi menghasilkan script gaya Python yang gagal kompilasi.
      ✅ 2026-09-14 — tabel batasan ditambahkan di `docs/reference/primitives.md`
      (§"Dialek Starlark", termasuk contoh salah/benar) dan section "Starlark — Batasan
      Dialek" di `ai_skills/entity-authoring/SKILL.md`, plus catatan bahwa `formspec validate`
      mengompilasi script `impl.ref`/`hooks:`.
- [x] **2.12 — #52 (BLOCKER): rute aksi lifecycle di surface UI.** `GenerateUIRoutes`
      hanya mengirim `[list, find, create, update, delete]` ke `generateRESTRoutes`, sehingga
      `submit`/`cancel`/`amend` tidak punya rute; rute wildcard file `/{id}/{field}`
      (`internal/api/router.go:451-452`) menelan `/{id}/<apa pun>` dan membalas 403 menyesatkan.
      Target: `internal/api/generator.go` (`GenerateUIRoutes`), `internal/api/file.go`
      (wildcard file → 404 bila `{field}` bukan field `file`/`attachment`).
      _Accept:_ `POST /{ws}/_ui/entity/{module}/{entity}/{id}/{action}` mencapai handler aksi;
      aksi yang tidak ada → **404** (bukan 403).
      ✅ 2026-09-14 — `submit`/`cancel`/`amend` ditambahkan ke `uiActions` (di-skip untuk
      `summary` dan untuk aksi ber-`impl:` yang sudah ditangani `GenerateUICustomActionRoutes`,
      agar tidak ada rute ganda). Verifikasi runtime: `POST …/menu-category/{id}/submit` →
      **401 authentication required** (rute ada; sebelumnya 403 `.update` menyesatkan);
      `POST …/zzz` → **404**. Test: `TestGenerateUIRoutes_LifecycleActions`,
      `TestGenerateUIRoutes_SummaryNoLifecycle`. Regresi clinic `TestOTCSale_Cancel` tertangani.
- [x] **2.13 — #53: permission rute file memakai plural.** `internal/api/file.go:451`
      (`module+"."+entity+"."+action`) + enam pesan error memakai nama entity **singular**,
      sementara registry (`registry.go:218`) & generator (`generator.go:173`) memakai **plural**
      → pengguna yang diberi `…menu-categories.update` tetap ditolak di rute upload/download.
      Target: `internal/api/file.go` (resolve spec entity → `Plural`).
      _Accept:_ permission yang dipakai rute file identik dengan yang diregistrasi (plural).
      ✅ 2026-09-14 — `HandlerFactory.permName()` (plural, fallback `{entity}s`) dipakai `can()`
      dan pesan error upload/download. Test `internal/api/file_test.go` & `link_test.go`
      diselaraskan ke bentuk plural (perilaku lama yang ter-enkode di test memang yang salah).

- [ ] **2.15 — ⏸️ Kartu meja QR: halaman masuk token → sesi (DEFERRED).**
      Sisa satu-satunya dari 2.6. Yang sudah ada: `kind: Print` bisa memuat QR
      di ketiga pipeline (2.6) dan `dining-table.qr_token` sudah tersedia;
      struk digital/thermal membuktikannya hidup.
      **Yang menghalangi:** pindai QR meja harus (1) men-resolve meja dari
      `qr_token` di route, lalu (2) membuat `table-session`, lalu (3)
      mengarahkan ke `/menu/{session_id}`. Ketiganya belum bisa dinyatakan:
      `context.source: entity` me-resolve lewat **id** (bukan filter token), dan
      `submit.redirect` tidak membawa id record yang baru dibuat.
      **Kenapa tidak dipaksakan sekarang:** memasang `qrcode` di spec kafe
      dengan payload yang tidak bisa di-resolve hanya menghasilkan QR yang
      menuju ke mana-mana — persis yang dilarang aturan ledger ("spec kafe
      tidak di-degradasi"). Ini satu paket dengan sisa **2.2/#45** (token tamu
      sebagai kunci record) dan keputusan desain "halaman masuk" baru.
      **Jalan termurah bila dikerjakan:** `context` source yang me-resolve entity
      dari token route (find by token), atau field `natural_key` untuk
      `qr_token` sehingga `find` bisa me-resolve-nya.

---

## Fase 3 — Multi-outlet & integritas data

## Fase 3 — Multi-outlet & integritas data

- [x] **3.1 — #22: `indexes:` dihormati.** `ddl.go` §4 hanya membaca
      `entity.Persist.Indexes`; spec kafe menaruh di root (`EntitySpec.Indexes`,
      `pkg/spec/entity.go:87`) → harus dibaca dari kedua lokasi (atau disatukan).
      _Accept:_ `formspec migrate plan` pada `menu-item-price` menghasilkan satu index.
      ✅ 2026-09-14 — `ddl.go` sekarang membaca **kedua** lokasi dan **membuat kolom turunan
      untuk field yang disebut index** (termasuk `relation`, yang sebelumnya tidak dapat
      kolom turunan sama sekali). Bukti `migrate plan`:
      `CREATE UNIQUE INDEX idx_cafe_master_menu_item_prices_branch_id_menu_item_id ON … (_branch_id, _menu_item_id)`;
      ikut benar: `dining_tables(_branch_id,_code)`, `stock_levels(_branch_id,_ingredient_id)`,
      `menu_costs(_branch_id,_menu_item_id)`, `shifts(_branch_id,_transaction_date)`.
      `migrate.go` (`diffExistingTable`) juga menambah kolom turunan untuk tabel yang sudah ada.
      Test: `TestGenerateEntityDDL_DeclaredIndexes`, `TestGenerateEntityDDL_PersistIndexesStillWorked`.
      _Catatan:_ index baru terpasang pada **database baru**; DB lama perlu recreate atau
      `kind: Migration` (lihat 3.4).
- [x] **3.2 — #23: kolom turunan `money` bertipe numerik** (bukan `text`) agar
      sortir/rentang & agregasi benar. Target: `ddl.go` (`generateGeneratedColumn`),
      driver-aware (#27).
      ✅ 2026-09-16 — plan `docs_internal/plan/money-derived-column-numeric.md` ·
      changelog `2026-09-16-007`. Akarnya bukan sekadar tipe kolom: kolom turunan
      menyimpan **objek** `{amount, currency}` sebagai teks JSON, sehingga
      `"9000"` dianggap **lebih besar** dari `"10000"` — sortir harga, filter
      rentang, dan laporan margin salah tanpa gejala, dan index di atasnya hanya
      mempercepat jawaban yang salah. Yang dikerjakan:
      (a) `generateGeneratedColumn` kini menerima tipe field; untuk `money`
      ekspresinya membaca **`.amount`** (`json_extract(data,'$.x.amount')` di
      SQLite, `data->'x'->>'amount'` di Postgres) dan tipenya `numeric(20,8)`
      (`fieldTypeToSQL` juga mendapat `case spec.FieldMoney` — sebelumnya jatuh
      ke `default: text`, yang justru akar #23). Diterapkan di ketiga situs
      (field ber-indeks, kolom turunan dari `indexes:`, dan jalur ALTER di
      `migrate.go`).
      (b) `columnRefExpr` **didahulukan** ke kolom turunan sebelum ekspresi
      `.amount`: sekarang kolomnya numerik, jadi memakai ekspresi justru membuat
      index tidak terpakai pada setiap sort/filter money.
      **Bukti runtime (DDL nyata dari `migrate apply` pada DB segar):**
      `_price numeric(20,8) GENERATED ALWAYS AS (CAST(json_extract(data,
'$.price.amount') AS REAL)) STORED` + `CREATE INDEX … (_price)`, dan
      `idx_cafe_order_orders_total_amount`.
      **Bukti perilaku:** `TestEntityStore_MoneySortAndRangeAreNumeric` (insert
      9000 & 10000 → naik: **9000 dulu**; `price >= 9500` → hanya 10000) dan
      `TestGenerateEntityDDL_MoneyDerivedColumnReadsAmount` (kedua driver:
      ekspresi `.amount`, tipe numerik, field string tetap text) ·
      `go test ./...` hijau · kafe `validate` 0 problem.
      **Adopsi kafe — workaround-nya dihapus:** `menu-item-price.price` dan
      `order.total_amount` kini `index: true` (sebelumnya sengaja tidak, karena
      index di atas teks JSON lebih buruk daripada tidak ada), dan table POS
      menandai kolom Total `sortable: true`. Komentar GAP-02/GAP-23 di dua file
      itu diganti catatan tertutup.
      Normatif: `docs/spec/backend/05-field-types.md` §2.2 (baru).
      **Sisa (dicatat):** pada SQLite kolom turunan memakai cast `REAL`, jadi
      presisi eksak tetap milik payload JSON — kolom turunan adalah **proyeksi
      untuk sortir/agregasi**, bukan sumber kebenaran nilai uang; dan DB lama
      perlu recreate/`kind: Migration` agar kolomnya ikut berubah tipe
      → **10.5(b) ⏸️**.
- [x] **3.3 — #27: pemetaan tipe SQL driver-aware** — `timestamptz` tidak bocor ke SQLite.
      ✅ 2026-09-14 — `fieldTypeToSQLFor(ft, enum, driver)` memetakan `timestamptz`/`jsonb`/`uuid`
      → `text` dan `bigint` → `integer` pada SQLite; PostgreSQL tetap native. Dipakai di
      `ddl.go` (kolom turunan, index deklaratif, extension) + `migrate.go`. Bukti:
      `grep -c timestamptz` pada `migrate plan` SQLite = **0**. Test
      `TestFieldTypeToSQLFor_NoPostgresTypesOnSQLite`.
- [x] **3.4 — #35 + #36: `MigrationSpec.ddl` multi-dialek** (bukan satu string) dan
      jalur aman untuk `CREATE UNIQUE INDEX`/backfill.
      ✅ 2026-09-16 — plan `docs_internal/plan/migration-dialect-and-dml.md` ·
      changelog `2026-09-16-008`. **Dua celah, dua penutup yang dijaga tetap
      terpisah** (mencampurnya justru yang membuat keduanya tidak bisa dipercaya):
      (a) **#35 — DDL yang tidak portabel.** `ddl` tetap satu statement untuk
      semua driver; **`ddl_by`** memuat varian per driver (`sqlite`, `postgres` —
      himpunan tertutup, salah ketik ditolak, bukan dilewati diam-diam). Menulis
      **keduanya** ditolak supaya maksudnya tidak ambigu, dan driver yang tidak
      punya varian melewati migration itu **dengan peringatan** — bukan
      menjalankan SQL driver lain, yang justru kegagalan yang #35 khawatirkan
      (benar di dev, salah di produksi, baru ketahuan saat deploy).
      (b) **#36 — perbaikan data.** Constraint hanya bisa ditambahkan setelah
      datanya memenuhi syarat, dan duplikat penghalang itu muncul justru **karena**
      constraint-nya belum ada. Menolak DML membuat perbaikannya manual di luar
      spec tanpa jejak. Kini **`dml`** dinyatakan bersama **`reason` wajib**
      (audit: kenapa, bukan hanya apa), hanya boleh INSERT/UPDATE/DELETE/WITH
      (perubahan skema tetap milik `ddl`), dijalankan **sebelum** DDL dalam
      manifest yang sama (urutan itulah yang membuat "rapikan lalu batasi" bisa
      dinyatakan sekali jalan), dan **diumumkan** saat `migrate apply` serta
      dicetak `migrate plan`. Backfill besar tetap milik `kind: DataMigration`.
      **Bukti:** `TestApplyCustomMigrations_DataRepairRunsBeforeDDL` menjalankan
      skenario gap-nya utuh — tabel berisi duplikat, `CREATE UNIQUE INDEX`
      **gagal** tanpa perbaikan, lalu berhasil setelah `dml` merapikan (3 baris →
      2 baris, dan insert duplikat berikutnya ditolak index) ·
      `TestLoadCustomMigrations_PicksDialect` (varian dipilih per driver; `ddl`
      portabel berlaku di keduanya) · `TestValidateMigrationSpec` (6 bentuk
      ditolak: tanpa DDL, dua bentuk sekaligus, dialek tak dikenal, varian kosong,
      `dml` tanpa `reason`, DDL menyusup di `dml`) · `go test ./...` hijau · kafe
      `validate` 0 problem. Normatif: `01-core-basic.md` §4.1 (baru).
      **Sisa (dicatat):** `dml`+`ddl` dalam satu manifest belum dibungkus satu
      transaksi eksplisit — pada SQLite keduanya efektif atomik, tetapi jalur
      Postgres belum diverifikasi → item **4.2.6 ⏸️** di master todo.
      **Catatan cakupan (temuan saat mengerjakan ini):** yang **tidak** ada lagi
      adalah _manifest_ `kind: Migration` di spec kafe — ketiga filenya
      (`menu-item-price-unique`, `shift-open-unique`, `stock-level-unique`)
      **dihapus di 1.6**, karena aturan keunikan itu kini dinyatakan deklaratif
      lewat `indexes[].where`. Kind-nya sendiri tetap ada dan baru saja diperluas
      di item ini. Akibatnya, dan ini berlaku lebih luas: **tidak ada satu pun
      example di repo ini yang masih memakai `kind: Migration`** (diperiksa:
      `find examples -path "*/migrations/*" -name "*.yaml"` kosong), sehingga
      bukti untuk 3.4 hanya bisa datang dari test — bukan dari aplikasi. Kind
      yang tidak dipakai example mana pun adalah kind yang bisa membusuk tanpa
      ketahuan; karena itu dicatat sebagai item **8.8**.
- [x] **3.5 — #8/S5: scope cabang ditegakkan engine** (setelah 1.1 & 1.8): entity
      ter-scope difilter otomatis; `TenantDecl` diganti deskriptor dimensi.
      **Verifikasi 2026-09-18 (checkbox tertinggal `[ ]`; isinya sudah ✅ 2026-09-16):**
      ketiga klaim dicek ulang dan cocok dengan kode/spec terkini — (a) 10 entity
      benar-benar mendeklarasikan `row_scope` (`grep -rl '^\s*row_scope:' examples/kafe/spec --include=entity.yaml`
      → 10 baris, tepat himpunan yang disebut); (b) aturan skip ada di
      `internal/api/scope.go` (`applyRowScope`: exemption `read_all` lebih dulu,
      lalu `IdentityFromContext == nil && len(publicScopeFromContext) > 0` →
      dilewati, sisanya fail closed); (c) grant publik `menu-item-price` di
      `apps/kafe-qr.yaml` ber-scope `{field: branch_id, from: route}`.
      Perintah yang bisa gagal: `go test ./cmd/formspec/ -run
      'TestKafeRowScopeSpec_ScopeAndSource|TestKafeAssignmentSources_EmployeeMapsUsernameToBranch|TestKafePublicGrants_ScopedWhereRowsMatter' -v`
      → ketiganya **PASS** (test itu menolak tepat kasus-kasus ini: entity
      ter-scope di luar himpunan, `menu-item-price` di-scope di sesi, grant
      ber-scope tanpa sumber nilai `branch`, grant ber-scope yang juga memberi
      `find`).
      **Keputusan yang sebelumnya menghambat ini sudah diambil 2026-09-16:**
      "boleh lihat semua cabang" dinyatakan sebagai **permission eksplisit**
      `{module}.{plural}.read_all` (bukan bypass `*` implisit, bukan wildcard di
      atribut). Mekanismenya sudah ada: pemegang permission itu dilewati dari
      `row_scope` entity tersebut, permission-nya didaftarkan bersama permission
      standar sehingga bisa diberikan dan terlihat di audit, `*` tetap memenuhi
      (identitas dev aman), dan ruang lingkupnya per entity. Normatif:
      `docs/spec/backend/01-core-basic.md` §1.7 + §8.6 · changelog
      `2026-09-16-004`. Dengan begitu menyalakan `row_scope` di spec kafe tidak
      lagi mematikan aplikasi bagi pemilik maupun `formspec dev`.
      ✅ 2026-09-16 — **penyaringan dinyalakan.** `row_scope: [{field: branch_id,
op: eq, from: session}]` dipasang di 10 entity yang dibaca **hanya** lewat
      permukaan terautentikasi: `order`, `payment`, `shift`, `cash-movement`,
      `stock-level`, `stock-movement`, `purchase-order`, `stock-opname`,
      `waste-entry`, `menu-cost`.
      **Aturan yang harus ditambahkan agar ini tidak mematikan permukaan publik**
      (temuan saat mengerjakan): `row_scope` menyeluruh mem-403 anonim — pemanggil
      anonim tidak punya atribut sesi, dan `from: session` memang fail closed.
      Karena itu `applyRowScope` kini **melewati** permintaan anonim yang datang
      lewat grant publik yang **punya scope sendiri** (yang membatasinya adalah
      `applyPublicScope`). Entity tanpa grant publik tetap fail closed untuk
      anonim — jawaban yang benar untuk entity yang memang tidak dimaksudkan
      terbuka.
      **Adopsi tambahan yang menutup celah terakhir:** grant publik
      `menu-item-price` (katalog QR) kini **ber-scope** `{field: branch_id, from:
route}` — sebelumnya daftar harga anonim terbuka tanpa syarat, sehingga satu
      permintaan tanpa parameter mengembalikan harga **seluruh cabang**.
      **Bukti runtime** (dev server, DB segar, dua pesanan B1/B2, token dev
      ditandatangani dengan secret dev sehingga klaim sesi nyata):

                              | Permintaan | Hasil |
                              | --- | --- |
                              | kasir B1 (`attrs.branch_id=B1`, perm `.list`) | `200`, `total: 1`, hanya `B1-1` |
                              | kasir B1 + `?branch_id[eq]=B2` | `200`, **tetap hanya `B1-1`** — klien tidak bisa melebarkan |
                              | kasir B2 | `200`, hanya `B2-1` |
                              | pemilik (`.list` + `.read_all`, **tanpa** atribut cabang) | `200`, `total: 2` — kedua cabang |
                              | identitas `.list` tanpa atribut cabang | **403** `row scope on branch_id: caller has no "branch_id" session attribute` |
                              | anonim `list order` | **401** (allowlist; `order` butuh token tamu) |
                              | anonim `list menu-item-price` tanpa `?branch_id` | **403** |
                              | anonim `list menu-item-price?branch_id=B1` | `200` |

                              **Nuansa yang tercatat:** pemilik yang **hanya** memegang `read_all` (tanpa
                              `.list`) mendapat **404** — itu gerbang permission UI-surface, bukan scope:
                              `read_all` mengecualikan penyaringan, bukan memberikan hak baca.
                              Test: `TestKafeRowScopeSpec_ScopeAndSource`, `TestKafeAssignmentSources_EmployeeMapsUsernameToBranch`,
                              `TestKafePublicGrants_ScopedWhereRowsMatter`. `go test ./...` hijau · kafe
                              `validate` **0 problem**.
                              **Sisa (dicatat):** ~~supervisor pemegang **dua** cabang belum bisa dinyatakan
                              (`assignments` masih satu nilai per dimensi; butuh daftar nilai + `op: in`)~~
                              — **digantikan oleh 3.8**: bukan daftar nilai, melainkan pilihan konteks
                              sesi `(role, cabang)` yang **selalu tunggal**. Yang tetap tersisa:
                              `dining-table`/`table-session` sengaja belum di-scope sesi karena masih
                              dibaca permukaan tamu (`find` by id) → **10.5(a) ⏸️** (satu paket dengan
                              **2.15 ⏸️**).

- [x] **3.8 — Konteks sesi: (principal, role, cabang).** ✅ **2026-09-20 — SELESAI (inti + API; UI pemilih menyusul)**
      Usulan pemilik proyek 2026-09-16: sesi selalu spesifik — siapa, sebagai
      **role apa**, di **cabang mana**. Bukan "user punya daftar role + daftar
      cabang", melainkan satu daftar **pasangan (role, cabang)**:
      `admin → {role: sales, branch: A}`, `{role: admin, branch: B}`. Login
      memilih satu; bila hanya satu, otomatis. Login dengan OAuth (tidak ada
      langkah memilih) memakai pilihan **terakhir** yang disimpan
      **per-device di klien** (`localStorage`), dan pengguna bisa pindah kapan
      saja. Permission = grant **role yang dipilih** (bukan union), sehingga
      boundary-nya selalu spesifik dan audit menjawab "sebagai role apa, di
      cabang mana" untuk setiap aksi.
      **Kenapa ini menggantikan rencana lama:** sisa 3.5 semula akan
      diselesaikan dengan memperluas `assignments` menjadi daftar nilai +
      filter `op: in`. Pilihan konteks lebih baik: boundary-nya **tunggal**
      (tidak ada pelebaran otomatis), dan ia menutup dua celah sekaligus —
      multi-cabang **dan** multi-role. Efek sampingnya menyederhanakan 1.8:
      nilai cabang dibawa sesi, jadi `row_scope: from session` tidak lagi
      menempuh jalur kritis resolusi `assignments` tiap request (jalur itu turun
      jadi fallback).
      Desain lengkap (model, alur login/OAuth/switch, apa yang disentuh, 5
      tahapan, bukti yang harus ada):
      `docs_internal/plan/session-context-role-branch.md`.
      _Accept:_ kasir dengan dua assignment (sales@A, admin@B) → login tanpa
      memilih membalas daftar pilihan (bukan token); memilih sales@A hanya
      memberi data **dan** permission cabang/A role sales; pindah konteks tanpa
      logout; OAuth di device yang sudah pernah memilih langsung memakai pilihan
      itu; akun tanpa assignment (pemilik) tetap lintas cabang lewat `read_all`;
      assignment yang dicabut → minta pilih ulang, bukan lanjut dengan konteks lama.
      ✅ **2026-09-20 — SELESAI (inti + API; UI pemilih belum).** Changelog
      `2026-09-20-012`.
      **(a) Model:** `formspec.core.user.assignments` = daftar `{role, dimension,
value}` (semua required); `dimension` = **nama field** yang dibandingkan
      `row_scope.field` (`branch_id`), bukan nama dimensi dekoratif. Entity
      `session` menyimpan role + dimensi + nilai. `User.Context` transient.
      **(b) Login:** 0 assignment → perilaku lama (union role, tanpa boundary);
      1 → otomatis; >1 tanpa pilihan → **409 `CONTEXT_REQUIRED` + choices**
      (id = `<role>@<value>`); id dicabut → 409 juga (fail closed).
      **(c) Token:** klaim `role` (tunggal) + `attrs{dimension: value}`;
      validator memakai `role` dan mengabaikan `roles` bila keduanya ada —
      daftar role basi tidak bisa melebarkan sesi.
      **(d) Permission:** materialisasi memakai role terpilih **saja**
      (`issuePair` mengganti `User.Roles` selama resolusi).
      **(e) Refresh:** `contextStillValid` (assignment masih ada **dan** role
      masih ada) → gagal = 409 minta pilih ulang, bukan lanjut boundary lama.
      **(f) `POST /_ui/auth/switch`:** sesi lama di-revoke, pair baru; tidak
      pernah dua konteks hidup. `SetAssignments` untuk admin.
      **Bukti E2E** (dev server `:8099`, user `kasir2` dengan `sales@cabangA` +
      `admin@cabangB`, dua cabang nyata): login tanpa `assignment` → **409 +
      choices**; login `sales@<A>` → klaim `role=sales`, `roles=["sales"]`
      (bukan `["sales","admin"]`), `attrs={branch_id: A}`; list order dengan
      token itu → `total: 2` **hanya cabang A**; login `admin@<B>` → `total: 0`.
      Test `internal/auth/context_test.go` (11 kasus, termasuk akun tanpa
      assignment tetap union = aditif). `go test ./...` hijau · `make lint`
      **0 issues** · kafe `validate` **0 problem**.
      **Sisa (tahap 4 plan):** layar pemilih konteks + pengalih di header +
      `localStorage` (pilihan terakhir untuk OAuth) → item **6.5.9 ⏸️** di master
      todo. Respons 409 sudah membawa `choices` yang dibutuhkan layar itu.
      Adopsi kafe juga masih parsial: role kafe belum punya grant tersimpan, dan
      pemetaan `employee.assignments` → `user.assignments` (data/seed) belum
      ditulis → item **10.3 ⏸️** di bawah.

- [x] **3.9 — Index yang definisinya berubah tidak direkonsiliasi (temuan E2E 3.8).**
      Ditemukan saat verifikasi 3.8: membuat order di **cabang kedua** pada DB
      kafe yang sudah ada gagal `UNIQUE constraint failed:
cafe_order_orders.tenant_id, cafe_order_orders._number`. Terbukti dengan
      membandingkan dua DB: DB **segar** punya index
      `idx_uq_cafe_order_orders_number ON cafe_order_orders (tenant_id, _branch_id, _number)`
      (spec kafe memang mendeklarasikan `natural_key_rule.scope_field: branch_id`),
      DB **lama** masih `(tenant_id, _number)`. Jadi ini murni storage yang
      tertinggal, bukan bug loader/spec.
      **Yang sudah dikerjakan** (`renderers/jsonb-persist/migrate.go`):
      (a) diff index tidak lagi membandingkan **nama** saja, tapi **bentuk**
      (`indexShape`: unique + daftar kolom + predikat parsial) — index yang ada
      dengan definisi berbeda kini di-`DROP` + `CREATE` ulang;
      (b) `driftedIndexes` menjalankan pemeriksaan itu juga di jalur
      **"checksum sama"**, karena checksum mem-fingerprint _manifest_, bukan
      storage — tanpa ini DB yang tertinggal tidak akan pernah diperbaiki;
      (c) normalisasi bentuk menoleransi perbedaan kosmetik (quoting,
      `ASC/DESC`, `USING btree`, predikat dalam tanda kurung, cast `::text`)
      supaya plan tetap konvergen.
      **Bukti:** `TestMigrationRunner_ChangedIndexDefinitionIsRebuilt` (diff
      path: nama sama, kolom berbeda → DROP+CREATE → konvergen) dan
      `TestMigrationRunner_DriftedIndexIsDriftIsRepairedWithoutManifestChange`
      (checksum sama, storage drift → repair → konvergen) · `go test ./...`
      hijau · `make lint` 0 issues.
      **✅ 2026-09-20 — SELESAI.** Empat sub-masalah ditutup + satu jebakan DX
      ditemukan (penyebab "plan senyap" yang menyesatkan):
      (i) diff index membandingkan **nama** saja → `indexShape` (unique + kolom + predikat parsial, dinormalisasi agar tetap konvergen);
      (ii) pemeriksaan drift tidak jalan di jalur **"checksum sama"** →
      `driftedIndexes` di cabang itu;
      (iii) DDL dibangun **hanya** dari `DiffShapes(snapshot, manifest)` di
      `planEntityChange` — snapshot merekam _niat_, bukan storage → drift
      storage kini ikut diperiksa di jalur itu;
      (iv) **kolom turunan scope field natural key** tidak dibuat di jalur alter
      (hanya create) → `derivedColumnFields` menandainya juga.
      **Jebakan DX:** `formspec migrate` default DSN-nya `.formspec/data.db`,
      sedangkan `formspec dev` memakai `dsn:` dari `formspec-app.yaml`
      (`.formspec/kafe.db`). Semua pemeriksaan awal saya menunjuk **DB yang
      salah**, sehingga plan tampak "senyap" padahal DB dev memang tertinggal.
      **Bukti:** tiga test pengunci (`ChangedIndexDefinitionIsRebuilt`,
      `DriftedIndexIsRepairedWithoutManifestChange`,
      `DriftedIndexIsRepairedOnSnapshotDiffPath`) · plan pada DB dev memuat
      `[derived] index_changed: declared index definition drifted from storage —
rebuilt` + `field_projection_changed branch_id` · setelah bootstrap:
      `Applied 24 migration(s)`, index jadi
      `(tenant_id, _branch_id, _number)`, kolom `_branch_id` ada · **E2E**: order
      cabang B → `ORD-2026-00002`, cabang A → `ORD-2026-00004` (keduanya 201;
      sebelumnya cabang B 500 `UNIQUE constraint failed`).

- [x] **3.12 — Hook `after create` tidak menghasilkan proyeksi summary (temuan walkthrough 9.4).**
      ✅ **2026-09-20 — SELESAI. Akarnya bukan hook-nya, tapi script-nya.**
      **Reproduksi (dev server, DB kafe):** `POST /_ui/entity/cafe-stock/stock-movement`
      dua kali (in, 1000@50 lalu 500@80) → **201** keduanya; lalu
      `select count(*) from cafe_stock_stock_levels` → **0**. Proyeksi
      `stock-level` (saldo + moving average) tidak pernah ditulis, jadi HPP/margin
      tidak punya dasar.
      **Wiring terlihat benar** (jadi ini bukan salah spec):
      `stock-movement.hooks: [{on: after, action: create, impl: {type: script_ref, ref: cafe-stock/stock_level_apply}}]`,
      `stock-level.maintained_by: cafe-stock/stock_level_apply` (sama), dan
      script-nya memang memanggil `resource.upsert(...)` di jalur pertama
      (`current == None` → `return ok(...)`). `formspec validate` tetap
      **0 problem**.
      **Kandidat penyebab (belum diverifikasi):** (a) hook `after` tidak
      dijalankan pada jalur create API (atau hanya untuk action tertentu);
      (b) error di hook `after` ditelan (post-commit best-effort) sehingga
      kegagalan `resource.upsert` tidak terlihat di respons 201; (c)
      `resource.find`/`upsert` gagal pada `summary` karena aturan pemanggil
      (4.1) dievaluasi terhadap ref yang berbeda dari `maintained_by`.
      **Bukti 4.1 yang ada bersifat unit-level**
      (`TestSummaryUpsert_MaintainerWritesProjection`) — jalur hook-nya sendiri
      tidak pernah diuji lewat API, dan walkthrough inilah yang pertama
      menyentuhnya.
      _Accept:_ dua `stock-movement` in di atas menghasilkan satu baris
      `stock-level` dengan `quantity_on_hand 1500` dan `moving_avg_cost 60`
      (= (1000×50 + 500×80) ÷ 1500), plus test yang menutup jalur hook-nya
      (bukan hanya `resource.upsert` langsung).
      **Langkah diagnostik berikutnya:** jalankan ulang satu create dengan log
      server terlihat (apakah hook dipanggil?), lalu uji `after`-hook error:
      apakah kegagalannya membatalkan transaksi atau hanya tercatat.
      ✅ **2026-09-20 — SELESAI. Akarnya bukan hook-nya, tapi script-nya.**
      **Akar (satu baris log sementara di `RunAfterPhase`):**
      `[hook-debug] after action="create" hooks=1 selected=1` lalu
      `dispatch … err=action cafe-stock.create (script_ref): script failed:
script runtime error: float got money, want number or string`.
      Hook **dipilih dan dijalankan**; script-nya yang gagal karena
      `float(resource.field.unit_cost)` — `unit_cost` adalah nilai **money**
      (objek `{amount, currency}`), dan `float()` menolaknya (S7/1.3: operand
      tidak sah = error, bukan 0). Kegagalannya **senyap bagi pemanggil**
      (after-hook tidak membatalkan respons, hanya tercatat di log engine —
      itulah sebabnya create tetap 201).
      **Perbaikan (`scripts/stock_level_apply.star`):** baca uang lewat
      `amount()` (helper `money_amount`) dan tulis lewat bentuk kanonik
      (helper `money_like`, mata uang ikut sumber, bilangan bulat ditulis tanpa
      `.0`). Berlaku untuk `unit_cost` **dan** `moving_avg_cost` yang dibaca
      dari baris proyeksi — keduanya money.
      **Bukti:** `validate` **0 problem** (script dikompilasi) · dua movement in
      (1000@50 lalu 500@80) → **201** · `cafe_stock_stock_levels` → **1 baris**:
      `quantity_on_hand 1500`, `moving_avg_cost {"amount":"60","currency":"IDR"}`
      — persis (1000×50 + 500×80) ÷ 1500 · `go test ./...` hijau · `make lint`
      0 issues.
      **Sisa kecil — ✅ DITUNTASKAN 2026-09-20:** tiga kolom itu kini diisi
      `stock_level_apply.star`: `stock_value` (`qty × avg`, money kanonik),
      `last_movement_at` (dari `transaction_date` pergerakan), dan
      `is_below_min` (dibandingkan dengan `ingredient.min_stock`, dibaca lewat
      `resource.fetch`). Dua catatan teknis: `resource.fetch` hanya tersedia
      DI DALAM `execute()` (di level modul ia tidak terdefinisi — kelas error
      compile script yang senyap bagi pemanggil hook `after`), dan `created_at`
      engine tidak diekspos ke script. Akses `cafe-stock.ingredient` lintas
      entity kini DIDEKLARASIKAN (`hooks[].uses.resources`) — honesty check
      menolaknya tanpa itu (USES_VIOLATION), persis footprint konsen yang
      diminta item 4.7/#34. **Bukti:** in 1000@50 + 500@80 → `qty 1500, avg
60` → in 5@60 → `value 90300` (= 1505×60) → out 600 → `qty 905`,
      `value 54300` (= 905×60), `below_min = True` (min_stock 1000) — semua
      benar; `validate` 0 problem · `go test ./...` hijau · `make lint` 0
      issues.

- [x] **3.11 — Kolom turunan hasil ALTER tidak terisi → aturan bisnis lolos (temuan walkthrough 9.4).**
      ✅ **2026-09-20 — DIPERBAIKI.** SQLite menolak `ALTER TABLE ADD COLUMN …
GENERATED ALWAYS … STORED`, tetapi **menerima** varian **VIRTUAL** —
      kolom yang dihitung saat baca, jadi benar untuk baris yang sudah ada
      **dan** untuk setiap baris baru (jalur INSERT hanya menulis
      `(id, tenant_id, version, data)` dan mengandalkan kolom menghitung
      dirinya sendiri). Perbaikan (`renderers/jsonb-persist`):
      (a) `addDerivedColumnSQL` memakai `GENERATED ALWAYS AS (…) VIRTUAL` di
      SQLite (PostgreSQL tetap STORED), dan ekspresinya kini dibagi **satu
      sumber** dengan jalur CREATE TABLE (`generatedColumnExpr` di `ddl.go`) —
      kolom ALTER dan kolom CREATE tidak mungkin berbeda;
      (b) kolom polos peninggalan bentuk lama **dideteksi dan dibangun ulang**:
      introspeksi baru `generatedColumns` (SQLite `pragma_table_xinfo.hidden
IN (2,3)`; PostgreSQL `information_schema.columns.is_generated='ALWAYS'`)
      membedakan "kolom generated" dari "kolom biasa yang kebetulan bernama
      sama"; `diffExistingTable` membangun ulang kolom stale — DROP index
      dependen dulu (SQLite menolak DROP COLUMN yang masih dirujuk index),
      DROP COLUMN, ADD COLUMN generated, CREATE index kembali;
      (c) rekonseilasi storage ini berjalan juga di jalur **"checksum sama"**
      (manifest tak berubah) dan di jalur snapshot-diff tanpa duplikasi DDL —
      checksum mem-fingerprint manifest, bukan storage, jadi tanpa ini DB lama
      tidak pernah diperbaiki. Kind perubahan baru: `storage_drift`.
      **Test pengunci:** `TestMigrationRunner_AlteredDerivedColumnEnforcesUnique`
      (jalur ALTER menegakkan unique) ·
      `TestMigrationRunner_StaleDerivedColumnIsRepaired` (DB lama dengan kolom
      polos direncanakan diperbaiki, apply, lalu konvergen).
      **Bukti E2E pada DB kafe lama:** `migrate apply` → `Applied 8
migration(s)` (semua `storage_drift`); `migrate plan` → `No pending
migrations`; `_cashier_id`/`_branch_id` kini `hidden=2` (VIRTUAL) dan
      **terisi**; INSERT shift `open` kedua untuk (cabang, kasir) sama →
      **REJECTED** `UNIQUE constraint failed: cafe_order_shifts._branch_id,
cafe_order_shifts._cashier_id` (aturan bisnis #10 ditegakkan); shift
      kasir lain → diterima; shift `closed` dengan kunci sama → diterima
      (partial index benar). Master todo **15.7** ✅ ditutup bersamaan.
      **Reproduksi (sebelum):** dua shift `status: open` untuk (cabang, kasir) yang sama
      **diterima** (201), padahal `shift/entity.yaml` mendeklarasikan partial
      unique index `(branch_id, cashier_id) WHERE status='open'` (aturan bisnis
      #10) dan index itu memang ada di DB.
      **Akar (bukti langsung dari `sqlite_master`):**
      `_branch_id text GENERATED ALWAYS AS (json_extract(data, '$.branch_id')) STORED, _cashier_id text`
      — `_cashier_id` adalah **kolom biasa** karena ditambahkan lewat ALTER saat
      index parsial dideklarasikan (tabel sudah ada), dan ALTER tidak bisa
      menambah kolom GENERATED di SQLite. Kolom itu **tidak pernah terisi** →
      NULL; di SQLite NULL tidak pernah bentrok di UNIQUE index, jadi index-nya
      tidak menendang apa pun. `_branch_id` terisi karena ia sudah ada saat
      tabel dibuat (GENERATED).
      **Kenapa test DDL lolos:** `TestMigrationRunner_UniqueIndexRejectsDuplicates`
      membuat tabel dari nol (kedua kolom GENERATED), sehingga tidak pernah
      menyentuh jalur ALTER. Ini juga kelas yang sama dengan sub-masalah (iv)
      item 3.9.
      **Sudah tercatat sebagai item master todo 15.7** ("kolom turunan yang
      ditambahkan setelah tabel dibuat di SQLite adalah kolom biasa yang tidak
      pernah terisi … unique index yang baru dibuat bisa lolos dari duplikat");
      item ini menambahkan bukti E2E-nya lewat API.
      **Bukti (sebelum):** `POST /_ui/entity/cafe-order/shift` kedua → **201** (seharusnya
      ditolak); `select _branch_id,_cashier_id,_status from cafe_order_shifts`
      → `_cashier_id` NULL pada kedua baris; `sqlite_master` → `_cashier_id text`
      (tanpa GENERATED).

- [x] **3.10 — Snapshot lama memblokir `migrate apply` (temuan 3.9).**
      ✅ **2026-09-20 — DIPERBAIKI. Dua akar, bukan satu — dan keduanya bukan
      "snapshot lama merekam field turunan" seperti yang dicurigai ledger:**
      (a) **Jalur CLI tidak menormalisasi spec.** `spec.ValidateEntitySpec`
      bukan sekadar validator — ia juga **meng-inject** field milik engine
      (`is_active` dari `soft_deactivate`). Server mendaftarkan entity lewat
      jalur yang memanggilnya (`internal/entity/registry.go`), sedangkan
      `formspec migrate`/`diff` memanggil hanya `RawSpecToEntitySpec` — jadi
      bentuk yang dihitung CLI **tidak punya `is_active`**, sementara snapshot
      yang ditulis server **punya**. Selisih bentuk itulah yang muncul sebagai
      `field_removed is_active` — bukan snapshot yang salah, melainkan dua
      jalur yang melihat entity secara berbeda. Perbaikan:
      `manifest.EntitySpecFromRaw` (helper baru, parse + validate dalam satu
      panggilan) dipakai `cmd/formspec/migrate.go`; komentar di helper
      menjelaskan kontraknya ("setiap jalur yang memberi spec ke storage harus
      melewatinya").
      (b) **Entity framework dihitung sebagai "entity dihapus".** Server
      mendaftarkan `formspec.core.*` (auth, subscription, period) saat runtime,
      jadi DB yang pernah disentuh server punya tabel + snapshot-nya; CLI yang
      hanya memuat spec tree pengguna melaporkan masing-masing sebagai
      `[never] table_removed` — perubahan yang **mustahil dideklarasikan**
      karena tidak pernah ada manifest-nya. Perbaikan:
      `MigrationRunner.IgnoreModules("formspec.core")` (API baru) dipanggil
      `formspec migrate` dan `formspec diff`; `planForgotten` melewati snapshot
      modul yang di-ignore. Test pengunci:
      `TestPlanSpecSet_IgnoresFrameworkModules` (tanpa exclude → ditolak;
      dengan exclude → tidak dilaporkan).
      **Bukti E2E (DB kafe lama, tanpa bootstrap manual):**
      `formspec migrate plan` → `8 change(s) pending`, semuanya
      `[derived] storage_drift` (perbaikan 3.11) — **nol** `field_removed
is_active`, **nol** `table_removed formspec_core_*`;
      `formspec migrate apply` → `Applied 8 migration(s)`;
      `migrate plan` ulang → `No pending migrations`; `formspec diff` →
      `No differences`. Snapshot kini konsisten: 11 snapshot menyebut
      `is_active`, dan CLI menghitung bentuk yang sama.
      **Bukti (sebelum):** `formspec migrate apply --dsn sqlite:.formspec/kafe.db` →
      `Error: apply migrations: 18 destructive change(s) refused` (semuanya
      `field_removed is_active`), sementara plan yang sama pada `data.db`
      bersih. Recovery sementara yang dipakai: hapus 32 baris
      `formspec_schema_snapshot` (bootstrap) → apply 24 migrasi berhasil.
      **Bukti (perintah):** `python3 -c "select sql from sqlite_master where name='idx_uq_cafe_order_orders_number'"`
      pada `.formspec/kafe.db` → `(tenant_id, _number)`; pada DB segar
      (`--dsn sqlite:/tmp/mtest/fresh.db` + `migrate apply`) → `(tenant_id, _branch_id, _number)`.

- [x] **3.6 — #9: `scope_field` natural key sampai ke `ctx.next_key()`** (nomor
      pesanan per cabang).
      _Accept:_ `ORD-` mulai dari 1 di tiap cabang.
      ✅ 2026-09-16 — plan `docs_internal/plan/natural-key-scope.md` · changelog
      `2026-09-16-010`. Ternyata bukan sekadar "teruskan nilai scope": saat
      diuji, cabang kedua **gagal 500**. Nilai scope harus menembus **tiga**
      lapis, dan hanya satu yang sudah benar:
      (a) **Counter** — jalur script (`ctx.next_key`) mengirim scope kosong
      (hardcoded `""` di registry), sedangkan jalur otomatis membacanya dari
      record. `ctx.next_key(field, scope=<nilai>)` kini meneruskan scope, dan
      registry **menolak** mencetak nomor saat rule ber-`scope_field` tapi
      scope-nya kosong — pesannya menyebut field yang harus diisi, bukan sekadar
      menolak. (Sebelumnya: deret global yang terlihat benar sampai dua cabang
      bertabrakan.)
      (b) **Keunikan** — index unik natural key adalah `(tenant_id, _number)`:
      **tanpa cabang**. Jadi deret B2 yang mulai dari 00001 menabrak
      `ORD-2026-00001` milik B1 → `UNIQUE constraint failed`. Kini index-nya
      `(tenant_id, _branch_id, _number)` bila rule-nya ber-scope. Inilah bug yang
      membuat accept tidak mungkin dicapai walau counter-nya sudah benar.
      (c) **DDL** — scope field belum tentu punya kolom turunan (di `order`,
      `branch_id` tidak `index: true`), sehingga index yang menyebut `_branch_id`
      gagal dibuat (`no such column`). Generator DDL kini **membuat kolom turunan
      untuk scope field** sebuah natural key, seperti yang sudah dilakukannya
      untuk field yang dirujuk `indexes:`.
      **Bukti runtime** (dev server, DB segar, empat create anonim):

      | Permintaan | `_number` | cabang |
      | --- | --- | --- |
      | create B1 | `ORD-2026-00001` | B1 |
      | create B2 | `ORD-2026-00001` | B2 |
      | create B1 | `ORD-2026-00002` | B1 |
      | create B2 | `ORD-2026-00002` | B2 |

      — semuanya `201`, dan **nomor yang sama hidup di dua cabang**. Sebelum
      perbaikan: create B2 → `500 UNIQUE constraint failed:
      cafe_order_orders.tenant_id, cafe_order_orders._number`.
      **Bukti unit:** `TestGenerateNaturalKey_ScopedPerBranch` (B1: 1,2,3; B2: 1
      — fixture baru `registry_fixtures/scoped-counter/spec`),
      `TestGenerateNaturalKey_ScopedCounterRefusesEmptyScope` (pesan menyebut
      `branch_id` + cara memperbaikinya), `TestGenerateNaturalKey_UnscopedUnaffected`
      (guard hanya untuk rule ber-scope), dan `TestCtxNextKey_*` di
      `internal/starlark` (argumen `scope=` benar-benar sampai ke handler).
      `go test ./...` hijau · kafe `validate` 0 problem.
      Normatif: `04-persist-backend.md` §2 (`scope_field` wajib diberi nilai
      scope; jalur otomatis vs script).
      **Catatan:** komentar GAP-09 di `order/entity.yaml` kini bisa dihapus/
      diperbarui — nomor pesanan per cabang **boleh** dibuat lewat script sekarang.
      **Sisa (dicatat):** `scope_field` yang menunjuk field `relation` dipetakan
      ke kolom turunan `text` (id referensi), jadi keunikan mengikuti id, bukan
      kode cabang — cukup untuk kafe, tapi perlu diingat bila cabang di-rename.
      Ini **perilaku yang disengaja**, bukan item terbuka.

- [x] **3.7 — #11 + #12: relasi lintas `persist.category` & resolusi tabel target**
      (guard referenceability tidak boleh lolos senyap).
      ✅ 2026-09-16 — plan `docs_internal/plan/relation-guard.md` · changelog
      `2026-09-16-011`. Akarnya satu: **relasi yang tidak bisa diresolusi
      diperlukan sebagai "tidak ada yang perlu diperiksa"**, bukan sebagai
      cacat. Dua tempat:
      (a) **Runtime (#12).** `ValidateRelationTargets` menjawab "target not found
      atau table doesn't exist" dengan `continue` — jadi relasi yang menunjuk ke
      baris yang tidak ada **diterima**, dan nama tabel yang salah (mis. hasil
      `{module}_{plural}` naif untuk relasi lintas module) membuat guard-nya
      **dilewati**, bukan gagal. Kini ketiga kasus dibedakan dan diberi nama:
      target entity tidak teresolusi → error "does not resolve to a registered
      entity"; target entity benar tapi barisnya tidak ada → error "does not
      exist" (referensi menggantung); tabelnya tidak bisa dibaca → error yang
      menyebut nama tabelnya. Relasi opsional yang tidak diisi tetap lolos —
      guard-nya tentang referensi yang **ada**, bukan semua field.
      (b) **Stati k (#11 + akar #12).** Gerbang cross-manifest baru
      `validateRelations` (`cmd/formspec/validate_relations.go`) menolak, dengan
      seluruh spec tree terlihat: `relation.resource` yang **tidak menunjuk
      entity terdaftar** (bentuk dotted maupun `module/entity`, dan field di
      dalam `child` ikut diperiksa) dan relasi yang **melintasi
      `persist.category`** — yang di runtime hanya memblokir sambil menulis satu
      baris log, sehingga list-nya "jalan" padahal relasinya tidak pernah
      resolve.
      **Bukti:** `TestValidateRelations` (4 kasus: lintas module yang resolve →
      diterima, target tak terdaftar → ditolak dengan menyebut nama target,
      lintas kategori → ditolak, entity tanpa kategori tidak dianggap kategori
      lain) dan `TestValidateRelationTargets_RefusesDanglingAndUnresolvable`
      (runtime: resolver gagal → error; baris target tidak ada → error; relasi
      tak diisi → lolos). `go test ./...` hijau dan **kafe `validate` 0 problem**
      — artinya seluruh relasi lintas module kafe (`order` → `cafe-master`) memang
      sudah resolve _dengan nama_, bukan kebetulan cocok.
      **Sisa (dicatat):** blokir cross-category di jalur baca
      (`resolveRelations`) masih berupa log + skip sebagai jaring pengaman — kini
      cacatnya tertangkap lebih awal di validasi, tetapi mengubah jalur baca
      menjadi hard error akan memutus list yang sudah berjalan dan layak
      diputuskan tersendiri → item **4.4.4 ⏸️** di master todo.

---

## Fase 4 — Stok, HPP & pembelian

- [x] **4.1 — #13: valuasi inventory.** Moving-average di Starlark (D3) — perlu 4.2–4.4.
      ✅ **2026-09-20 — SELESAI (Opsi A, keputusan pemilik proyek).** Saat mulai
      dikerjakan ditemukan celah arsitektur: entity `summary` **tidak punya jalur
      tulis yang didukung** (`EntityStore` menolak `CharSummary`; `maintained_by`
      script tidak bisa menulis proyeksinya; engine rebuild hanya merencanakan).
      **Keputusan: Opsi A** — jalur tulis internal khusus pemelihara.
      **Yang dikerjakan:**
      (a) **`EntityStore.UpsertProjection(ctx, workspaceID, match, data)`** —
      upsert by match (AND), hanya untuk summary, atomik (baca-lalu-tulis dalam
      satu transaksi), tidak diekspos ke API.
      (b) **`resource.upsert(entity, match, data)`** — primitif Starlark; rantai
      `builtinUpsert` → `UpsertHandler` → `SetUpsertHandler` → wiring di
      `resource/formspec.go`.
      (c) **Aturan pemanggil:** hanya entity `summary`, dan hanya script yang
      **disebut `maintained_by`** entity itu (`MaintainerRef` = `action.Impl.Ref`
      dibandingkan dengan `MaintainedBy`). Script lain → error.
      (d) **Adopsi kafe:** `stock_level_apply.star` ditulis ulang memakai
      `resource.find` + `resource.upsert` (tanpa SQL, tanpa `ctx.lock`); hook
      `after create` pada `stock-movement` memanggilnya.
      **Dua bug ikut ketemu & diperbaiki** (kelas gagal-senyap):
      — `UpsertProjection` menulis kolom `is_active` yang tidak ada (itu field
      di dalam `data`, bukan kolom tabel) → insert gagal.
      — `FindByFields` mengembalikan `ErrNotFound` alih-alih `nil` saat tidak ada
      baris → `resource.find` melempar error pada pergerakan pertama (sebelum
      baris ada), sehingga pemelihara tidak pernah jalan.
      **Bukti:** `TestUpsertProjection_InsertThenUpdate` (insert lalu update,
      merge parsial, kunci beda → baris beda) · `TestUpsertProjection_RejectsNonSummary`
      · `TestUpsertProjection_RejectsEmptyMatch` · `TestFindByFields_NoMatchReturnsNil`
      · `TestResourceAPI_Upsert_SummaryProjection` + `_RejectsBadArgs` ·
      **`TestSummaryUpsert_MaintainerWritesProjection`** (dua pergerakan 5+3 →
      proyeksi `quantity_on_hand` = 8) · **`TestSummaryUpsert_NonMaintainerRefused`**
      (script bukan pemelihara → ditolak) · kafe `validate` **0 problem** ·
      `go test ./...` hijau. Normatif: `docs/spec/backend/02-core-extended.md`
      §6.1. Plan: `docs_internal/plan/summary-maintainer-write-path.md`.
- [x] **4.2 — S14 + #33: `maintained_by` + `invariants`; hooks/conditions
      benar-benar dipanggil pada `summary`.** Menghapus kondisi "terlihat terpasang
      tapi tidak jalan".
      ✅ **2026-09-18.** Jalur termurah & paling jujur dipilih: **tolak**, bukan
      diam-diam tidak jalan. `ValidateEntitySpec` kini menolak `hooks:` dan
      `conditions:` pada `characteristic: summary` dengan pesan yang menunjuk ke
      `maintained_by` + `invariants` (Core Extended §6.1) — manifest yang
      _terlihat_ terlindungi padahal hook-nya tidak pernah dieksekusi lebih buruk
      daripada manifest yang gagal validasi. **Bukti:** `TestValidateEntitySpec_SummaryRejectsHooks`
      (hook → error; condition → error; master dengan hook → lolos) · spec uji
      `/tmp` → `[FAIL] summary entity declares hooks, … use maintained_by +
invariants instead` · kafe `validate` **0 problem** (69 manifest) ·
      `go test ./...` hijau. Marker GAP-33 di `stock-level/entity.yaml` ditutup;
      `docs/spec/backend/02-core-extended.md` §6.1 diperbarui (validator menolak,
      bukan sekadar "tidak dipanggil").
- [x] **4.3 — #31: API Starlark find-by-field** (menghapus keharusan raw SQL).
      ✅ **2026-09-18.** `resource.find(entity, {field: value, ...})` baru —
      mencari satu record lewat **lapisan entity** (bukan SQL), jadi tenant
      isolation & `row_scope` berlaku dan nama tabel/kolom fisik tidak perlu
      diketahui script. Mengembalikan resource atau `None`. Rantai: builtin
      (`internal/starlark/resource.go`) → `FindHandler` (`executor.go`) →
      `SetFindHandler` (`internal/action/script.go`) → `EntityStore.FindByFields`
      (`renderers/jsonb-persist/crud.go`, multi-field AND, scope-aware via
      `txReadDB`) → wiring di `resource/formspec.go` (dengan `checkCrossModuleUses`).
      **Adopsi kafe (bukti accept):** dua guard keunikan ditulis ulang **tanpa
      SQL** — `guard_menu_item_price_unique.star` dan `guard_shift_open_unique.star`
      kini memakai `resource.find(...)`; deklarasi `uses: {primitives: [db]}`
      di hook-nya dihapus karena tidak lagi menyentuh `ctx.db()`.
      **Bukti:** `TestResourceAPI_Find_ReturnsMatchAndNone` (match → resource,
      miss → None, argumen diteruskan) · kafe `validate` **0 problem** ·
      `go test ./...` hijau. Normatif:
      `docs/spec/platform/02-workspace-app-module.md` §9.3.
- [x] **4.4 — #32: guard keunikan atomik** tanpa reimplementasi UNIQUE yang rapuh.
      ✅ **2026-09-18.** Jawaban kanoniknya adalah **`indexes:` (database)**,
      bukan `ctx.lock` di script: index berlaku untuk **semua** jalur tulis
      (API, script, seed, operator) dan atomik di level DB, sedangkan guard
      hanya pada jalur yang melewatinya dan rentan race. Yang dikerjakan:
      (a) **Bukti constraint benar-benar menegakkan** — `TestMigrationRunner_UniqueIndexRejectsDuplicates`
      (duplikat `(branch,cashier)` open → UNIQUE violation; shift `closed`
      ganda → lolos, membuktikan index **parsial**; cabang lain → lolos) dan
      verifikasi runtime pada DB nyata (`migrate apply` → dua index unik ada,
      termasuk `… WHERE _status = 'open'`).
      (b) **Dokumentasi normatif** — `01-core-basic.md` §3: "Keunikan adalah
      urusan database, bukan script"; guard script hanya **lapis kedua** untuk
      pesan ramah, tidak boleh jadi satu-satunya penegak.
      (c) **Komentar kafe dibersihkan** — `menu-item-price/entity.yaml` tidak
      lagi mengklaim GAP-22/GAP-30 terbuka; guard ditandai "LAPIS KEDUA".
      **Bukti:** `go test ./...` hijau · kafe `validate` **0 problem**.
- [x] **4.5 — #30: `ctx.db()` di dalam transaksi aksi tidak deadlock di SQLite.**
      ✅ **2026-09-18.** Akarnya: `datastore.DBQuerier.Query` selalu memakai
      `q.DB` (pool), padahal aksi sudah memegang satu-satunya koneksi SQLite
      (`SetMaxOpenConns(1)`) → query kedua menunggu selamanya. Kini `Query`
      memakai `db.TxReadDB(ctx, q.DB)` (diekstrak dari `txReadDB` yang sudah
      dipakai jalur baca store) — query berjalan di koneksi transaksi aksi, dan
      sekaligus memberi read-your-own-writes. **Bukti:** `TestDBQuerier_QueryInsideTxScopeNoDeadlock`
      (query di dalam TxScope terbuka → 1 baris, bukan hang) — **diuji bisa
      gagal**: dengan `target := q.DB` dikembalikan, test gagal
      `context deadline exceeded` (5s) · `go test ./...` hijau. `TxScope.Join`
      diekspor untuk pemanggil luar paket.
- [x] **4.6 — S12: satuan & konversi** (gram/kg/pcs) untuk ledakan resep.
      ✅ **2026-09-18.** Deklarasi `unit: {base, convertible}` (1.8) diperluas
      dengan **`factors`** — berapa `base` setara satu satuan itu (`{kg: 1000}`).
      Tanpa faktor, deklarasinya hanya bilang "satuan ini berhubungan" tanpa
      bilang **bagaimana** — persis konvensi-di-script yang ingin dihapus.
      Validator menolak: `convertible` tanpa faktor, faktor untuk `base`,
      faktor di luar `convertible`, faktor ≤ 0. Konversi dihitung engine lewat
      **`ctx.unit.convert(entity, field, value, from, to)`** (melewati `base`:
      `value × factor(from) ÷ factor(to)`); satuan di luar grup **error**, bukan
      `0`. Rantai: `UnitDecl.Convert` (`pkg/spec/entity.go`) → `UnitConvertHandler`
      (`executor.go`) → `SetUnitConvertHandler` (`internal/action/script.go`) →
      wiring di `resource/formspec.go` (resolve entity/field → deklarasi unit).
      **Adopsi kafe:** `ingredient.unit` & `recipe.lines[].unit` menyatakan
      `factors: {kg: 1000}`.
      **Bukti:** `TestUnitDecl_Convert` (kg↔gram, identitas, satuan luar grup →
      error) + `TestValidateEntitySpec_Unit` (8 bentuk ditolak) · kafe `validate`
      **0 problem** · `go test ./...` hijau. Normatif:
      `docs/spec/backend/05-field-types.md` §1.6.
- [x] **4.7 — #34: `HookDecl.uses`** agar akses script terlihat di consent footprint.
      ✅ **2026-09-18.** `HookDecl` mendapat `Uses *UsesDecl` (bentuk sama dengan
      action). Hook uses didaftarkan di permission registry dengan nama sintetis
      `hook:<on>:<action|event>` (`internal/entity/registry.go`), dan honesty scan
      (`cmd/formspec/honesty.go`) kini membandingkan `uses` hook dengan pemakaian
      nyata di script — `ctx.db()` yang tidak dideklarasikan = **error**, persis
      seperti action. **Bukti:** `TestHonestyScan_HookUsesDeclared` (undeclared →
      error; declared → bersih) · kafe `validate` **0 problem** setelah 3 hook
      guard (`menu-item-price` ×2, `shift`) mendeklarasikan `uses: {primitives:
[db]}` — sebelumnya scan melaporkan 2 problem, yang justru membuktikan
      gerbangnya bekerja · `go test ./...` hijau. Normatif:
      `docs/spec/backend/02-core-extended.md` §15.
- [x] **4.8 — #28: verifikasi aritmetika/agregasi `money` di laporan stok** (lihat 1.3).
      ✅ **2026-09-18.** 1.3 menutup semantiknya; item ini memverifikasinya pada
      **bentuk laporan stok kafe yang sebenarnya** — `stock-usage.yaml`
      mengagregasi `total_cost` (money) `sum` dikelompokkan per `ingredient_id`,
      berdampingan dengan `quantity` (decimal). **Bukti:**
      `TestAggregate_StockReportShape` (SUM money per grup: kopi 75000, susu
      30000; grand total 105000; SUM quantity 6 — semua benar, bukan 0) ·
      `go test ./...` hijau. Tidak ada perubahan kode — murni verifikasi, sesuai
      sifat item.

---

## Fase 5 — Kas, shift, void & approval

- [x] **5.1 — Partial unique shift terbuka** (D6) — bergantung 1.6.
      _Accept:_ dua shift `open` untuk (cabang, kasir) **ditolak DB**.
      ✅ **2026-09-20 — sudah tertutup oleh 1.6; diverifikasi ulang.**
      `shift/entity.yaml` mendeklarasikan `indexes: [{fields: [branch_id,
cashier_id], unique: true, where: "status = 'open'"}]`, dan
      `migrate apply` menghasilkan `CREATE UNIQUE INDEX … WHERE _status = 'open'`.
      **Bukti:** `TestMigrationRunner_UniqueIndexRejectsDuplicates` (dua shift
      `open` (B1,C1) → UNIQUE violation; shift `closed` ganda → lolos, membuktikan
      parsial; cabang lain → lolos) · kafe `validate` 0 problem.
- [x] **5.2 — #38/S9: void multi-state-asal lewat approval** (lihat 1.7).
      ✅ **2026-09-20 — sudah tertutup oleh 1.7; diverifikasi ulang.**
      `order-void-approval.yaml` memakai `on.transition.name: void-order`,
      sehingga satu workflow mengawal **seluruh** state asal transisi
      (`paid`/`in_kitchen`/`ready`/`served`). **Bukti:**
      `TestRegistry_ForTransitionByName` (keempat state asal lolos;
      `cancel-order` & entity lain tidak) · `TestValidateWorkflows_ByNameCoversEveryOriginState`
      · `TestValidateWorkflows_RejectsPartialStatePair` (pasangan from/to yang
      hanya mencakup sebagian → ditolak) · kafe `validate` 0 problem.
- [x] **5.3 — #39/S15: `WorkflowStep` punya `title`, `description`, `display_fields`.**
      _Accept:_ ApprovalInbox menampilkan nomor pesanan, total, alasan void.
      ✅ **2026-09-20 — spec-level SELESAI; wiring runtime ApprovalInbox 🟡 sisa.**
      `WorkflowStep` mendapat `Title`, `Description`, `DisplayFields`
      (`pkg/spec/resources.go`), dan `formspec validate` menolak
      `display_fields` yang menunjuk field yang tidak ada di entity workflow
      (`cmd/formspec/validate_workflow.go` — `buildEntityFieldIndex` +
      `workflowDisplayFieldError`). **Adopsi kafe:** `order-void-approval.yaml`
      mendeklarasikan `title`, `description`, dan
      `display_fields: [number, total_amount, void_reason]`.
      **Bukti:** `TestValidateWorkflows_DisplayFieldsMustExist` (field yang ada →
      diterima; `void_reason` yang tidak ada → ditolak dengan menyebut namanya) ·
      kafe `validate` 0 problem · `go test ./...` hijau. Normatif:
      `docs/spec/backend/02-core-extended.md` §2.
      **Sisa (dicatat):** `ApprovalInbox` zero-config mengambil dari langkah
      workflow yang menunggu, tetapi **wiring runtime** yang mengisi `title`/
      `display_fields` ke item inbox belum ada — renderer sudah menampilkan
      `item.title` bila ada, jadi sisanya adalah pekerjaan engine (mengisi item
      dari step). Itu di luar cakupan spec-level item ini → item **5.13.6 ⏸️**
      di master todo.
- [x] **5.4 — #37: shorthand `render: drawer` diterima** (selaraskan loader & schema).
      ✅ **2026-09-20.** Akarnya divergensi loader↔schema: `FormRenderDecl`
      punya `UnmarshalYAML` yang menerima **kedua** bentuk (`render: drawer`
      dan `render: {mode: drawer}`), tetapi schema hanya menerima bentuk objek
      → `render: drawer` ditolak schema (`/spec/render: validation failed`)
      padahal loader menerimanya. Generator schema kini memberi `FormRenderDecl`
      `oneOf: [string, object]` (`internal/genjsonschema/generator.go`, pola yang
      sama dengan `ValidationRule`/`TransitionDecl.from`).
      **Bukti:** spec uji `render: drawer` → **0 problem** (sebelumnya 1) ·
      `render: {mode: drawer}` → 0 problem · `TestFormRenderDecl_AcceptsShorthandAndObject`
      (oneOf punya cabang string **dan** object) · kafe `validate` 0 problem.
- [x] **5.5 — #43: dokumentasikan aturan simetri cancel (7.7.2).**
      ✅ **2026-09-20.** Aturan sudah **ditegakkan** validator
      (`cmd/formspec/validate.go` — `validateIntegrators`), tetapi tidak
      terdokumentasi sehingga penemuannya sulit. Kini
      `docs/spec/backend/02-core-extended.md` §5 menjelaskan **mengapa**
      (efek samping maju butuh jalur pembalik; tanpa itu `cancel` terblokir
      permanen oleh reference guard) dan **bagaimana** (pasangan Integrator
      dengan contoh konkret `on_approved` + `before_cancel`), plus catatan
      bahwa `formspec validate` menolak Integrator tanpa pasangan cancel-nya.
      **Bukti:** `go test ./cmd/formspec/ -run Integrator` hijau · kafe
      `validate` 0 problem.

---

## Fase 6 — Akuntansi & integrasi lintas-app

- [x] **6.1 — #40/S13: keterkaitan transisi ↔ event dinyatakan eksplisit**
      (`emit:` pada transisi). Prasyarat seluruh integrasi stok & jurnal.
      _Accept:_ `order.paid` terverifikasi terpancar saat transisi ke `paid`.
      ✅ **2026-09-20.** `TransitionDecl` mendapat `Emit` (`pkg/spec/entity.go`),
      dan `formspec validate` menolak `emit` yang menunjuk event yang tidak
      dideklarasikan (`ValidateTransitionEmits`). Pemancaran runtime:
      `action.ResolveTransitionEmission` mencari transisi (from→to) dan
      mengembalikan emission-nya; dipanggil di `HandleUpdate` (durable → outbox
      atomik + best-effort delivery) saat state berubah.
      **Adopsi kafe:** `order` — `confirm-payment` → `emit: on_paid`,
      `cancel-order`/`void-order` → `emit: on_cancel`.
      **Bukti:** `TestResolveTransitionEmission` (transisi ber-emit memancarkan;
      multi-asal cocok dari state mana pun; tanpa emit → nil; tak ada transisi →
      nil; tanpa perubahan state → nil) · `TestValidateEntitySpec_TransitionEmits`
      (emit valid → lolos; tanpa emit → lolos; emit tak dikenal → error) · kafe
      `validate` **0 problem** · `go test ./...` hijau. Normatif:
      `docs/spec/backend/01-core-basic.md` §7.
- [x] **6.2 — S6/#41: pemetaan payload pada `IntegratorCall` (`map:`).**
      _Accept:_ order lunas → jurnal seimbang (kas/omzet/pajak/HPP) tanpa script baru di `gl`.
      ✅ **2026-09-20.** `IntegratorCall` mendapat `Map map[string]any`
      (`pkg/spec/resources.go`), dan `internal/integrator/dispatch.go` membangun
      params target dari pemetaan itu (`applyCallMap` + interpolasi rekursif).
      Nilai adalah template `{dotted.path}`; nilai yang **persis satu token**
      mempertahankan tipe aslinya (money tetap objek), token yang tak
      ter-resolve dibiarkan verbatim. Bila `map` absen, payload diteruskan apa
      adanya (perilaku lama).
      **Adopsi kafe:** `order-paid-to-journal.yaml` memetakan pesanan → jurnal
      (kas debit 1-1000, omzet kredit 4-1000, pajak kredit 2-2000) — pengetahuan
      akuntansi kini di manifest, bukan di script `gl`.
      **Bukti:** `TestApplyCallMap` (money tetap objek; string terinterpolasi;
      list of maps rekursif) + `TestApplyCallMap_UnresolvedTokenLeftVerbatim` ·
      kafe `validate` **0 problem** · `go test ./...` hijau. Normatif:
      `docs/spec/backend/02-core-extended.md` §5.
- [ ] **6.3 — #15: cross-app grant ditegakkan + `SyncAgent` tersambung ke router.**
      ⏸️ **DEFERRED 2026-09-20 — di luar cakupan single-server; butuh Control Plane.**
      Ini bukan gap yang bisa ditutup di engine single-server: cross-app grant
      adalah mekanisme **Control Plane** (grant disetujui Data Owner, tercatat,
      revocable, metered — `04-control-plane.md` §5), dan `SyncAgent` adalah
      komponen yang menyinkronkan registry antar-App. Menurut `AGENTS.md`,
      Control Plane + Operator + Marketplace **deferred ke cloud phase**.
      **Yang sudah benar dan tetap berlaku:** spec kafe (`cafe-gl-integrator`)
      **valid** dan menyatakan integrasi yang benar (`consumes: gl:journal-entries` + dua Integrator simetris) — jadi spec-nya siap, jalanannya yang belum
      tersambung. Ini justru nilai test case-nya.
      **Cadangan yang berjalan hari ini** (dicatat di manifest App): tanam
      `kind: Subscription` di dalam module `cafe-order` (satu App) — lebih buruk
      (logika akuntansi menempel, tidak bisa diganti vendor, tanpa batas
      konsen), tetapi berjalan.
      **Alasan tidak dikerjakan sekarang:** menegakkan grant lintas-App tanpa
      Control Plane berarti mengarang model grant yang akan bertabrakan dengan
      desain Control Plane yang sudah ditetapkan. Sesuai aturan ledger, item
      yang butuh keputusan arsitektur tidak ditebak.
- [ ] **6.4 — #42: kepemilikan `publishes` ditentukan** saat dua App meng-mount module sama.
      ⏸️ **DEFERRED 2026-09-20 — butuh keputusan pemilik proyek (desain).**
      Masalahnya nyata dan spesifik ke kafe: module `cafe-order` di-mount oleh
      **dua** App (`kafe-qr` publik + `kafe-pos` privat), dan keduanya
      memproduksi event `on_paid` — karena event milik **module**, bukan App.
      Sementara `publishes` dideklarasikan **per-App** dan grant menyasar
      **App**. Jadi pesanan lunas lewat jalur QR tidak tercakup `publishes`
      milik `kafe-pos`, dan `formspec validate` tetap hijau.
      **Dua usulan yang sudah tercatat** (`11-integrasi-lintas-app.md` #42):
      (1) pemilik antarmuka adalah **MODULE** (event & entity hidup di module),
      sehingga `publishes` berarti "App ini menyajikan antarmuka module X" dan
      grant menyasar module; atau (2) tetap per-App, tetapi nyatakan bagaimana
      dua App yang meng-mount module sama memperlakukan `publishes`, plus
      peringatan validasi bila salah satu tidak mendeklarasikannya.
      **Alasan tidak dikerjakan sekarang:** ini keputusan **desain bahasa spec**
      (siapa pemilik antarmuka) yang menentukan bentuk `publishes`/`consumes`
      dan grant — bukan sesuatu yang boleh ditebak. Ia juga bergantung pada 6.3
      (grant lintas-App), yang deferred ke Control Plane.
- [x] **6.5 — #14: vertical `purchase`** (atau keputusan tertulis untuk tetap model sendiri).
      ✅ **2026-09-20 — keputusan tertulis: TETAP MODEL SENDIRI (opsi b).**
      Kafe sudah memodelkan purchase di module-nya sendiri dengan lengkap:
      `supplier` (master), `purchase-order` (transaction, state machine
      `draft → submitted → received/cancelled`), dan action `receive-goods`
      yang membuat `stock-movement` (in) per baris. Jadi kebutuhan pemilik
      ("supplier & pembelian bahan") **sudah terpenuhi** tanpa vertical.
      **Alasan tidak membuat vertical `purchase` sekarang:**
      (a) Vertical adalah artefak **reusable lintas-aplikasi**; membuatnya
      berarti menetapkan kontrak (field, state machine, integrasi) yang harus
      melayani retail, manufaktur, dan jasa sekaligus — keputusan produk yang
      lebih besar daripada kebutuhan kafe.
      (b) Yang benar-benar kurang dari #14 bukan entity-nya (kafe sudah punya),
      melainkan **integrasi otomatis** purchase → stock & purchase → jurnal.
      Integrasi itu kini **bisa dinyatakan** lewat `kind: Integrator` +
      `call.map` (6.2) dan `emit:` pada transisi (6.1) — jadi jalurnya terbuka
      tanpa vertical.
      (c) Landed cost (biaya kirim masuk HPP) tetap pekerjaan tersendiri; ia
      bergantung pada valuasi inventory (4.1, kini selesai) dan dicatat sebagai
      sisa.
      **Sisa (dicatat) → dipindahkan ke item bernomor (2026-09-22):**
      integrasi purchase → stock & purchase → jurnal belum dinyatakan di spec
      kafe (jalurnya ada: `emit:` + `Integrator` + `map:`) → **10.2 ⏸️**;
      landed cost belum dimodelkan → **10.1 ⏸️**. Keduanya item kafe (adopsi
      spec), bukan gap engine.

---

## Fase 7 — Struk & laporan

- [x] **7.1 — #10: Print `thermal`/`dotmatrix`** diimplementasikan **atau** ditolak
      di `formspec validate` (sekarang validate "bohong").
      _Accept:_ `receipt-thermal.yaml` menghasilkan output thermal 58mm, atau gagal jelas.
      ✅ **2026-09-20 — diimplementasikan (thermal).** `internal/api/print.go`
      kini bercabang pada `output.format`: `thermal` → `renderPrintThermal`
      (ESC/POS: init `ESC @`, align, bold, cut `GS V 0`), `pdf` →
      `renderPrintPDF`, `html` → dirender klien oleh PrintRenderer, dan format
      lain (`dotmatrix`) ditolak **501 NOT_IMPLEMENTED** — bukan diam-diam jadi
      PDF. Sebelumnya handler SELALU membalas PDF tanpa melihat `output.format`,
      jadi `format: thermal` lolos validasi dan menghasilkan PDF bernama .pdf.
      **Bukti:** `TestRenderPrintThermal` (prefix ESC/POS, ada cut command,
      bukan PDF, konten terinterpolasi) · kafe `validate` **0 problem** ·
      `go test ./...` hijau. Marker GAP-10 di `receipt-thermal.yaml` ditutup.
- [x] **7.2 — #29/S16: selaraskan kontrak `ReportColumn` vs `TableColumn`**
      (`widget`, `format`, `aggregate` → enum; dokumentasi berdampingan).
      ✅ **2026-09-20.** `ReportColumn` mendapat `Widget TableCellWidget` (set
      yang sama dengan `TableColumn.widget`), dan `Aggregate`/`Format` menjadi
      **himpunan tertutup** (`ReportAggregate`/`ReportFormat` di
      `pkg/spec/widget.go` — `sum/avg/count/min/max` dan
      `currency/date/datetime/percentage`), sehingga salah ketik tidak lagi
      lolos dan mencetak nilai mentah. Dokumentasi berdampingan ditambahkan di
      `docs/spec/frontend/06-page-kinds.md` (tabel `ReportColumn` vs
      `TableColumn`).
      **Bukti:** `TestReportColumn_ClosedSets` (set tertutup; `median`/`relative`
      ditolak; widget badge diterima) · schema ter-regenerasi memuat
      `$defs/ReportAggregate` + `$defs/ReportFormat` + `ReportColumn.widget →
$ref TableCellWidget` · kafe `validate` **0 problem** · `go test ./...`
      hijau.
- [x] **7.3 — #16: `DashboardWidget.ref` module-qualified.**
      ✅ **2026-09-20.** Lookup widget di `stores/meta.ts` kini memakai
      `byQualified`: entri di-key oleh nama polos **dan** bentuk module-qualified
      (`module/name` dan `module.name`), sehingga `ref: cafe-report/omzet-hari-ini`
      resolve ke widget yang benar walau dua module punya widget bernama sama.
      Nama polos tetap sebagai fallback (backward compatible).
      **Bukti:** `meta.test.ts` (resolve `module/name`, `module.name`, dan nama
      polos) · `tsc` bersih · `vitest` **276 lulus**.
- [x] **7.4 — #17: realtime untuk Timeline** (KDS timeline).
      _Accept:_ tabel/kanban/dashboard/timeline semua menerima update realtime.
      ✅ **2026-09-20.** `TimelineSpec` mendapat `realtime: bool`, dan
      `TimelineRenderer` memakai `useRealtime` — pada event entity yang cocok,
      timeline **reset cursor + refetch dari atas** (karena append-only dengan
      cursor pagination, entri baru harus diambil dari awal). Semantik
      subscription sama dengan Table/Kanban/Dashboard.
      **Catatan adopsi:** kafe memakai **Kanban** untuk KDS (sudah
      `realtime: true`), bukan Timeline — jadi tidak ada Timeline kafe untuk
      diadopsi; ini kemampuan engine yang kini tersedia untuk semua kind.
      **Bukti:** `tsc` bersih · `vitest` **276 lulus** · kafe `validate`
      **0 problem**.

---

## Fase 8 — Dokumen, skill & DX

- [x] **8.1 — #18 + #25: koreksi skill** (`entity-authoring` soal `relation.target`;
      `formspec-kinds` soal `Config.spec.keys` vs `spec.data`). Target: `ai_skills/**`.
      ✅ **2026-09-20 — SELESAI.** #18 sudah tertutup 2026-09-14. #25 kini
      ditutup: `ai_skills/formspec-kinds/SKILL.md` mengajarkan `spec.keys`
      (map key → `{type, default, secret, public}`), bukan `spec.data` yang
      ditolak schema — plus tiga mirror skill di `examples/{crc-management,cafe,arisan}/.agents/skills/`
      dikoreksi sama. Marker GAP-25 di `cafe-settings.yaml` ditutup.
      **Bukti:** `grep -rn "data:" ai_skills/formspec-kinds/SKILL.md` → tidak ada
      lagi contoh `spec.data` · kafe `validate` **0 problem**.
- [x] **8.2 — #19 + #20: kebersihan drift dokumen**
      (`03-kind-renderers.md`, `realtime.md`, `spec.version` vs `formspec-app.yaml`).
      ✅ **2026-09-20 — SELESAI.** Plan `docs_internal/plan/kafe-sisa-gap.md` ·
      changelog `2026-09-20-009`.
      **#19** — `docs/renderers/shadcn-shell/03-kind-renderers.md` (masih bertanggal
      2026-07-16) diperbaiki: §1 `engine/registry.tsx` sudah **dihapus**; baris
      `Table` (`Form.render` kini dihormati → `OverlayHost`, navigasi lewat
      `useSurface().surfacePath` bukan `/_admin`); `Dashboard`/`Widget` (bukan lagi
      placeholder — `metric`/`chart`/`list`/`table`, agregat money-aware, chart tanpa
      library); `Report` (baris totals dirender); §4 ditulis ulang dari katalog
      sebenarnya (**24 form widget + 4 table-cell widget**, gerbang paritas
      `catalog.test.tsx`); §5 (`ConfirmDialog` shadcn, bukan `window.confirm()`).
      `docs/renderers/realtime.md` §5 menyatakan Calendar/ApprovalInbox/
      NotificationCenter "belum diimplementasikan" — ketiganya **ada** dan memakai
      `useRealtime` (di-gate `spec.realtime`): tabel §5 kini memuat **7 kind**,
      bullet keliru di §7 dihapus, §8 menyebut kelima direktori renderer.
      **#20** — klaim ledger **gugur sebagian**: `spec.version` di `Entity.md`/`Module.md`
      benar (`EntitySpec.Version` `pkg/spec/entity.go:124`, `ModuleSpec.Version`
      `pkg/spec/resources.go:14`; spec kafe mengisi `v1`/`1.0.0`), dan
      `formspec-app.yaml` memang config CLI dev/serve. Drift nyatanya `apiVersion`,
      dan keduanya membuat manifest **ditolak** (`ParseVersion` hanya menerima
      `^formspec\.dev/(v\d+)$`): `const APIVersion` = `formspec.dev/v1alpha1` →
      **`formspec.dev/v1`** (+ guard `TestAPIVersionIsStable`), dan
      `apiVersion: formspec/v1` di 3 dokumen (`02-visual-spec-kind.md` ×3,
      `03-renderer-kind.md`, `guides/authoring-a-page-renderer.md`) →
      **`formspec.dev/v1`**.
      **Bukti:** `go test ./pkg/spec/ -run TestAPIVersionIsStable` → PASS ·
      `go build ./...` exit 0 · `grep -rn "formspec/v[0-9]" docs/` → kosong ·
      `grep -rn useRealtime src/kinds/` → 7 renderer (+1 widget dashboard).
- [x] **8.3 — #21: validator menangkap referensi menggantung**
      (`App.spec.modules`, menu `view:`, `impl.ref` ke `.star` tidak ada).
      ✅ **2026-09-20 — SELESAI.** `impl.ref` sudah tertutup 8.7. Sisa dua
      ditutup oleh validator cross-manifest baru `validateDanglingRefs`
      (`cmd/formspec/validate_dangling.go`): `App.spec.modules` wajib menunjuk
      `kind: Module` yang ada, dan `MenuItem.view` (termasuk nested) wajib
      menunjuk Form/Table/Page/Wizard/Report/Kanban/Timeline/Calendar/Dashboard/Listing
      yang terdaftar. Sebelumnya keduanya lolos hijau sambil App mount kosong /
      menu menavigasi ke mana-mana.
      **Bukti:** `TestValidateDanglingRefs` (module tak dikenal → ditolak;
      view tak dikenal → ditolak; nested view → ditolak; yang resolve → lolos) ·
      kafe `validate` **0 problem** · `go test ./...` hijau.
- [x] **8.4 — #24: pesan error port `formspec dev` lebih menuntun.**
      ✅ **2026-09-20 — SELESAI.** Plan `docs_internal/plan/kafe-sisa-gap.md` ·
      changelog `2026-09-20-008`. `internal/devserver/devserver.go` (`EnsurePort`):
      port milik proses asing kini menyebut port + pemilik + PID dan **dua cara
      lanjut** (`kill <pid>`, atau `--addr :<port+1>`); jalur "owner tidak bisa
      diidentifikasi" juga menawarkan `--addr` alih-alih menggantung di pesan
      lama. Nomor port alternatif dihitung dari port yang sibuk.
      **Bukti:** `go test ./internal/devserver/ -run TestEnsurePort -v` → PASS
      (`TestEnsurePort_ForeignOwnerMessageIsActionable` gagal bila pesan lama
      dikembalikan; `TestEnsurePort_FreePortReturnsNil`) · `go build ./...` exit 0.
- [x] **8.5 — S10 lanjutan: enum untuk `ReportParam.type`, `ReportColumn.format/.aggregate`,
      `EventDeliveryDecl.channel`, `PrintOutput.format`, `WorkflowStep.mode`.**
      ✅ **2026-09-20 — SELESAI.** `ReportColumn.format/.aggregate` ditutup di 7.2.
      Sisanya kini himpunan tertutup (`pkg/spec/widget.go`): `ReportParamType`
      (`text/date/datetime/select/relation`), `EventChannel`
      (`audit_log/websocket/queue/reliable_event`), `PrintFormat`
      (`pdf/thermal/dotmatrix/html`), `WorkflowStepMode` (`all/any/sequential`).
      **Bukti:** `TestS10_RemainingClosedSets` (nilai valid diterima; `daterange`/
      `sms`/`docx`/`quorum` ditolak) · spec uji `type: daterange` →
      `schema: /spec/parameters/0/type: validation failed` · schema ter-regenerasi
      memuat keempat `$defs` enum · kafe `validate` **0 problem** · `go test ./...`
      hijau.
- [x] **8.6 — Regenerasi artefak**: `make generate-schema` + `make generate` +
      `make generate-kind-docs`; pastikan `git diff` bersih setelah semua.
      ✅ **2026-09-20 — SELESAI.** Plan `docs_internal/plan/kafe-sisa-gap.md` ·
      changelog `2026-09-20-010`. `make generate-schema` (162 shared types) +
      `make generate-kind-docs` (33 kind docs) dijalankan; **`make generate` masih
      stub** (no-op, `Makefile:102`) — dicatat apa adanya, bukan dianggap bersih.
      Churn regenerasi **tepat** perubahan fitur yang belum pernah di-generate:
      `schemas/formspec.schema.json` (+303/−54 — `$defs` S10: `ReportAggregate`,
      `ReportFormat`, `ReportParamType`, `EventChannel`, `PrintFormat`,
      `WorkflowStepMode`), `schemas/kinds/Timeline.schema.json` + `docs/kind/ui/Timeline.md`
      (`realtime`, 7.4), `docs/kind/ui/Form.md` (`autocomplete`, Fase 17).
      Cacat generator ikut ditutup: deskripsi `Timeline.realtime` terpotong
      (`"…mutation events, so a"`) karena generator memakai **baris pertama**
      komentar Go → komentar di `pkg/spec/frontend.go` ditulis ulang jadi kalimat
      utuh. Audit kelas cacat yang sama di seluruh schema → **0 temuan**.
      **Bukti:** regenerasi dijalankan dua kali + `md5sum -c` → keempat berkas **OK**
      (idempotent) · `go build ./...` exit 0 · `go test ./...` hijau · `make build`
      hijau · kafe `validate --schema ../../schemas` **0 problem** (69 manifest).
- [x] **8.7 — #50: `validate`/`check` memeriksa kompilasi Starlark.** Muat setiap
      script yang dirujuk `impl.ref`/`hooks`/`conditions` dan kompilasi dengan runtime
      yang sama; error bila gagal. Target: `cmd/formspec/validate.go`, `cmd/formspec/check.go`,
      `internal/starlark/`, `internal/manifest/`.
      _Accept:_ `formspec validate` pada spec kafe **sebelum 2.9** → gagal dengan menunjuk
      ketiga script; setelah 2.9 → hijau.
      ✅ 2026-09-14 — **akar masalahnya**: honesty scan sudah melaporkan `parseErr` untuk
      _actions_ (`compareUses`) tetapi **hook hanya dicek `ctx.environment`** — jadi error
      kompilasi script hook tidak pernah dilaporkan. Ditambah `scriptLoadIssue()` untuk
      hook `script_ref`: script gagal kompilasi → error, script tidak ditemukan → error.
      Bukti (spec uji): `hook before (action create): script failed to compile: …: got string
literal, want ','` + `hook before (action update): script not found …`. Spec kafe tetap
      **0 problem**. Test: `TestHonestyScan_HookScriptCompileError`.
- [x] **8.8 — Satu example yang benar-benar memakai jalur DDL custom.**
      Temuan saat 3.4: **tidak ada satu pun example** di repo ini yang masih
      punya manifest `kind: Migration` (kafe menghapus ketiganya di 1.6 karena
      aturan keunikannya kini deklaratif; `find examples -path "*/migrations/*"
-name "*.yaml"` kosong). Akibatnya `ddl`/`ddl_by`/`dml` hanya terbukti lewat
      test unit, dan kind itu tidak pernah melewati jalur nyata: `formspec
migrate plan` → `apply` → DB sungguhan → `formspec validate` pada manifest-nya.
      ✅ 2026-09-16 — **item ini selesai dengan cara yang berbeda, dan alasannya
      lebih kuat daripada itemnya sendiri.** `kind: Migration` **dicabut
      seluruhnya** (opsi B): migrasi struktural otomatis dari diff Entity, DDL di
      luar bahasa pindah ke `persist.raw_ddl` pada Entity, dan perubahan
      destruktif butuh deklarasi (`removed` / `accept_data_loss` + `reason`).
      Yang diminta item ini di atas — "kind yang tidak dipakai example mana pun
      adalah kind yang bisa membusuk tanpa ketahuan" — justru terbukti: kind itu
      membusuk (jalur `migrate data` bahkan mati seluruhnya: `DataMigration`
      tidak pernah terdaftar di kind catalog) sampai akhirnya dicabut. Bukti
      sekarang ada di test, bukan di example: `TestMigrateRefusesUndeclaredRemoval`,
      `TestMigrateRefusesDroppedEntity`, dan test klasifikasi di
      `renderers/jsonb-persist/diff*_test.go`. Rencana:
      `docs_internal/plan/migration-destruktif-otomatis.md`.

## Fase 9 — Verifikasi end-to-end aplikasi kafe

- [x] **9.1 — `formspec validate` hijau** vs `validate-baseline.md` (nol problem di luar baseline).
      ✅ **2026-09-20** — `formspec validate --schema ../../schemas` → **0 problem**
      (69 manifest). Baseline `validate-baseline.md` = 0 problem, jadi cocok.
- [x] **9.2 — `go test ./...` + `make lint` bersih.**
      ✅ **2026-09-20** — `go test ./...` → **39 paket ok**, 0 gagal ·
      `make lint` → **0 issues**.
- [x] **9.3 — `cd renderers/react-shadcn && vitest` bersih.**
      ✅ **2026-09-20** — `vitest run` → **288 test lulus** (19 berkas) ·
      `tsc -p tsconfig.app.json --noEmit` bersih.
- [ ] **9.4 — Walkthrough 3 App** (`formspec dev`), 9 skenario: 1. Pelanggan scan QR → lihat menu bergambar → keranjang → pesan. 2. Bayar QRIS → **lunas** → pesanan otomatis masuk KDS. 3. Bayar di kasir (tunai) → kasir "Lunas" → masuk KDS. 4. Barista: kanban `queued → preparing → ready → served` (per item, `line_status`). 5. Buka shift (kas awal) → transaksi → kas masuk/keluar → tutup shift → selisih. 6. Void: kasir ajukan → supervisor setujui (**semua** state asal) → tercatat. 7. Penjualan → `stock-movement` + `stock-level` (moving average) → `menu-cost` margin. 8. Order lunas → jurnal GL seimbang (kas/pajak/omzet/HPP). 9. 6 report tampil + struk digital (QR) & thermal tercetak.
      🟡 **2026-09-20 — berjalan; hasil per skenario:**

      | # | Skenario | Status | Bukti / penghalang |
      | --- | --- | --- | --- |
      | 1 | Pesan dari QR | ✅ API | rantai anonim penuh: katalog `menu-item` + `menu-item-price` 200 → `table-session` create 201 → `order create` 201 (`ORD-2026-00005`, scope_field per cabang) — 2026-09-20; keranjang & halaman = UI browser (belum) |
      | 2 | Bayar → KDS | ✅ API | payment 201 → `PATCH` order `awaiting_payment → paid` 200; KDS = list `status[in]=paid,…` |
      | 3 | Bayar kasir (tunai) | ✅ API | `method: cash`, `status: settled` → `change` terhitung `{85000 IDR}` (100000−15000) |
      | 4 | Kanban barista | ✅ API | rantai `paid → in_kitchen → ready → served → completed` semua 200 |
      | 5 | Shift & kas | ✅ API | employee 201 → shift buka 201 → kas in/out 201 → tutup shift 200 dengan **`difference = {-12500 IDR}`** (computed `counted_cash - expected_cash`). Aturan bisnis #10 kini **ditegakkan DB** — duplikat shift `open` (cabang, kasir) sama **ditolak** `UNIQUE constraint failed` setelah perbaikan 3.11 (2026-09-20) |
      | 6 | Void via approval | ✅ API (⚠️ temuan) | kasir ajukan void → **202 `approval_required`** (workflow `order-void-approval`, `paid → cancelled`) → non-holder **403** `WORKFLOW_DENIED` → supervisor `{"decision":"approve"}` → **`transition_completed`**, order → `cancelled` + `void_reason` tersimpan. **⚠️ Bug engine ditemukan & diperbaiki (2026-09-21):** interception approval TIDAK ADA di jalur PATCH — `RequiresApproval` hanya dipanggil di jalur custom action, sedangkan transisi `void-order` (tanpa `impl`) hanya bisa dicapai lewat PATCH → workflow-nya **bypass total** (void langsung `cancelled` tanpa approval). Fix: reverse lookup `FindTransitionByStates` (PATCH membawa state TUJUAN, bukan nama transisi; transisi multi-asal tak bisa diidentifikasi dari state saja) + cek di `HandleUpdate`, dengan state asal diambil **sebelum** merge (`merged := current.Data` bukan salinan — merge mengubah current.Data, jadi from==to selamanya jika dibaca sesudahnya) dan `decision` dibuang dari payload record. Test pengunci: `TestFindTransitionByStates` |
      | 7 | Stok/HPP | ✅ API | `ingredient` 201 · dua `stock-movement` in 201 · `stock-level` **1 baris**: `quantity_on_hand 1500`, `moving_avg_cost 60 IDR` (= (1000×50+500×80)÷1500). Akar kegagalan awal (script memakai `float()` pada nilai money) ditutup di **3.12**; sisa `stock_value`/`last_movement_at`/`is_below_min` kini diisi script (2026-09-20): `stock_value 54300` (= 905×60), `is_below_min True` saat qty 905 < min 1000 |
      | 8 | Jurnal GL | ✅ API | **Desain event-driven (keputusan pemilik, 2026-09-21):** order hanya memancarkan event durable `on_paid`; module `gl` (pemilik jurnal, bagan akun, **dan setting pemetaan akun**) mendengarkannya lewat `kind: Subscription` lalu membangun + mem-posting jurnal. Tidak ada integrator, tidak ada panggilan lintas-app — item 6.3 jadi tidak relevan. Bukti E2E: `ORD-2026-00021` → `outbox: completed` → **1 jurnal** `JRN-2026-000055` (`source_id` terisi, idempoten per sumber) · 4 baris di tabel child · **Kas debit 143750 = Omzet 125000 + Pajak 12500 + Service charge 6250** (seimbang) · `status = posted` otomatis · event `journal-posted` terbit. Jurnal tidak seimbang = setting akun GL belum lengkap → error `FORMSPEC.GL.*` (tanggung jawab `gl`), bukan error module pemesanan. **11 bug mesin** ditemukan & diperbaiki di jalurnya (terparah: `fail()` tidak menghentikan script — setiap guard `if bad: fail(...)` di seluruh ekosistem jadi no-op; dan `valuesEqual` panic pada slice → outbox worker mati). Changelog: `docs_internal/changelog/2026-09-21-003-jurnal-gl-dari-event-on-paid.md`. **Ditutup 2026-09-21 (changelog 2026-09-21-004):** `deliver: target` kini benar-benar memanggil action target, dan proyeksi `gl-balance` hidup — POST → 4 saldo benar (Kas 143750, Omzet 125000, Pajak 12500, Service charge 6250), REVERSE → kembali 0, 0 deliver failure. 5 bug lagi di jalur consequence ikut diperbaiki (target action `reverse` yang tidak ada + validate hijau; `payload.fields: [id]` null; closing saldo salah tanda untuk akun kredit-normal; `condition: resource.status` gagal dievaluasi), ditambah validator cross-manifest baru untuk deliver target |
      | 9 | Report + cetak | ✅ API | struk thermal ESC/POS nyata (init `ESC @`, bold, cut `GS V 0`, 348 byte) dirender dari data order live `ORD-2026-00005` (2026-09-20); report & tampilan = UI browser (belum) |

      **Catatan metode:** transisi state machine yang **tanpa `impl`** diterapkan
      lewat `PATCH /_ui/entity/{module}/{entity}/{id}` dengan `{"status": …}`
      (bukan route `/{id}/{action}` — itu hanya untuk action ber-`impl`,
      sesuai kontrak 2.7). `PUT` → 405. Enum `payment.status` =
      `[pending, settled, failed, refunded]` (bukan `success`) — CHECK constraint
      memang menolaknya.
      **Catatan 2026-09-20 (run penuh kedua):** data dev dikurasi ulang —
      user dev `owner` (register 201 + permissions `*`), 3 employee, 2
      dining-table, 2 menu-category + 2 menu-item + 2 harga, shift + 2
      cash-movement, ingredient + 3 stock-movement. Kefailan pertama yang
      berulang: enum spec pakai nilai Indonesia (`position: kasir`,
      `area: outdoor`) dan field `cash-movement.type` (bukan `direction`) —
      CHECK constraint menolak dengan pesan yang menunjuk field-nya.
      `expected_cash` TERNYATA tidak punya mekanisme compute di spec
      (deskripsinya menyebut "compute: kas awal + tunai masuk - kas keluar"
      tapi tidak ada `computed:`/script yang mengisinya) — angka `512500` di
      data lama berasal dari input manual verifikasi sebelumnya; ini dicatat
      sebagai keputusan produk, bukan gap engine (`difference` sendiri
      computed-nya benar).

- [x] **9.5 — Hapus marker `# GAP-nn`** dari `examples/kafe/spec/**` untuk gap yang sudah
      tertutup; update `README.md` status.
      ✅ **2026-09-20 — SELESAI.** 23 file spec dibersihkan: semua komentar
      ber-status "GAP-nn TERTUTUP/ditutup" dihapus atau dilebur menjadi catatan
      teknis biasa tanpa nomor gap (mis. "`time` memakai `timeinput` — jam
      happy hour" tetap, "GAP-01 TERTUTUP (2.14)" hilang). Marker gap yang
      **masih terbuka** (#4, #5, #8, #13, #14, #15, #16, #17, #21, #32, #33,
      #34, #37, #39, #40, #42, #45) sengaja dipertahankan sesuai aturan ledger
      ("`# GAP-nn` marker di `spec/` tetap sampai gap-nya benar-benar
      tertutup"). Verifikasi: `grep -rn TERTUTUP spec/` → **0 match**;
      sisa 62 penyebutan `GAP-` semuanya gap terbuka atau referensi teknis;
      `formspec validate --schema ../../schemas` → **69 manifest, 0 problem**
      (pembersihan komentar tidak menyentuh YAML).
- [x] **9.6 — Update workflow discipline**: `docs_internal/plan/todo.md`,
      `docs_internal/changelog/YYYY-MM-DD-NNN-*.md`, dan plan file terkait.
      ✅ **2026-09-20 — SELESAI.** Master todo 15.7 ✅ ditutup (perbaikan 3.11);
      changelog `2026-09-20-014-kolom-turunan-alter-dan-snapshot-migrate.md`;
      plan `docs_internal/plan/kafe-sisa-gap.md` diperbarui.

---

## Fase 10 — Sisa non-blocker aplikasi kafe (2026-09-22)

Fase ini **bukan** lanjutan gap #1–#53/S1–S16. Ia menampung sisa pekerjaan
**aplikasi kafe** yang sebelumnya hanya hidup sebagai prosa "**Sisa (dicatat)**"
di bawah item `[x]` — tidak bisa di-grep sebagai pekerjaan terbuka, dan menjadi
misinformasi begitu ada entry lain yang menutupnya (`AGENTS.md` §3). Audit
2026-09-22 menemukan 7 sisa seperti itu; semuanya kini punya nomor — lima di
fase ini (10.1–10.5), dua dipindahkan ke master todo karena bukan khas kafe
(`4.2.6` `dml`+`ddl` satu transaksi, `4.4.4` cross-category hard error).

Semuanya adalah **adopsi spec / verifikasi**, bukan gap engine — karena itu ia
**tidak** memblokir apa pun dan sengaja tidak dikerjakan bersama Fase 0–9.

- [x] **10.1 — ✅ 2026-09-22 — Landed cost dimodelkan + baris biaya bebas di BOM.**
      **Selesai.** `purchase-order.shipping_cost` (money) + baris `line_type: cost`
      pada `lines` (`cost_label` + `cost_amount` + `allocated_to` opsional), dan
      pada **resep** (`recipe.lines[].line_type: ingredient|cost`) sebagai
      "custom item biaya yg bisa diisi bebas" — mis. "Gas & listrik", "Kemasan
      takeaway" — yang ikut dijumlahkan `menu-cost.cost_per_portion`.
      `ingredient_id`/`quantity`/`unit_cost` kini `required_when: line_type ==
'ingredient'`, jadi baris biaya tidak lagi dipaksa punya bahan.
      **Alokasi** dilakukan action `receive-goods` (`cafe-stock/purchase_receive.star`):
      proporsional terhadap NILAI baris (qty × unit*cost), bukan jumlah unit,
      dengan **sisa pembulatan dibebankan ke baris bernilai terbesar** supaya
      Σ alokasi = faktur persis.
      **Bukti terukur** (E2E dev server `:8099`, DB kafe): PO 10.000 gram @ 15
      = 150.000 + landed cost 250.000 (200.000 `shipping_cost` + 50.000 baris
      biaya ber-`allocated_to`) → `stock-movement.unit_cost` = **40** (bukan 15),
      dan proyeksi `stock-level` terisi `qty=10000 avg=40 value=400000`.
      Test pengunci: `TestKafe_ReceiveGoodsMaintainsStockLevel`
      (`resource/script_hooks_e2e_test.go`). Changelog `2026-09-22-012`.
      **Sisa (dicatat):** landed cost hanya di `shipping_cost` + baris PO —
      belum ada alokasi berbasis berat/volume per baris (proporsional nilai
      dipilih; lihat komentar script untuk alasannya). Lihat 10.6.
      *(catatan lama, dipertahankan sebagai jejak)\_
      Sisa dari 6.5(c). `purchase-order` kafe sudah lengkap (`supplier`,
      `receive-goods` → `stock-movement` in), tetapi biaya kirim tidak punya
      tempat: HPP hanya menerima harga bahan. Effort: medium (field biaya
      kirim + alokasi per baris + masuk `moving_avg_cost`).
- [x] **10.2 — ✅ 2026-09-22 — Purchase memancarkan event durable; tidak tahu soal jurnal.**
      **Selesai, sesuai keputusan pemilik:** `purchase-order` mendeklarasikan
      `events: [on_po_received, on_po_cancelled]` (`publish: {durable: true}`,
      `deliver: [reliable_event]`) dan transisinya memakai `emit:`, **tanpa**
      Integrator/Subscription di sisi purchase. Saat module `gl` dikerjakan,
      gl cukup menambah `kind: Subscription` di manifest-nya sendiri — persis
      pola `gl/subscriptions/sales-to-journal.yaml`. Tidak ada satu baris pun
      di `cafe-stock` yang perlu diubah untuk itu.
      **Yang juga jadi nyata:** `receive-goods` kini action ber-`impl`
      (`cafe-stock/purchase_receive`), jadi transisi `submitted → received`
      benar-benar membuat `stock-movement` — sebelumnya ia hanya memindahkan
      kolom status.
      **Bukti:** `formspec validate` menegakkan aturan simetri cancel (pasangan
      `on_po_cancelled` wajib); `receive-goods` 200 di dev server dan
      menghasilkan movement dengan landed cost (lihat 10.1).
      **Sisa:** belum ada yang MENG-consume event ini (gl belum punya akun
      utang/persediaan) — itu memang keadaan yang diminta. Lihat 10.7.
      _(catatan lama, dipertahankan sebagai jejak)_
      Sisa dari 6.5(b). `receive-goods` **sudah** membuat `stock-movement`
      (bagian stock jalan), tetapi purchase → **jurnal** belum ada Integrator/
      Subscription-nya, jadi utang dagang tidak pernah tercatat. Jalurnya
      tersedia (`emit:` pada transisi + `kind: Subscription` + `map:`, persis
      pola `gl/subscriptions/sales-to-journal.yaml`). Effort: small–medium
      (satu transisi `emit:` + satu subscription + pemetaan akun utang).
- [x] **10.3 — ✅ 2026-09-22 — 6 role + 7 akun ter-seed, hak akses benar, bisa dites.**
      **Selesai.** `examples/kafe/spec/modules/formspec.core/seeds/roles.yaml`
      mendeklarasikan role **kasir, barista, dapur, pelayan, supervisor, manajer**
      dengan grants yang membedakan hak secara nyata (kasir: jualan+shift tanpa
      approval; supervisor: + `cancel` order; manajer: master/stok/pembelian/
      laporan **tanpa** `delete`; barista/dapur: kanban + order saja).
      **Bukti — diuji dengan memanggil endpoint nyata per role** (dev server,
      `app` scope dikirim saat login):

      | role | app | order | ingredient | purchase-order | user (core) |
      | --- | --- | --- | --- | --- | --- |
      | kasir | kafe-pos | 200 | 404 | 404 | 404 |
      | supervisor | kafe-pos | 200 | 404 | 404 | 404 |
      | manajer | kafe-pos | 200 | 200 | 200 | 404 |
      | barista | kafe-kds | 200 | 404 | 404 | 404 |
      | dapur | kafe-kds | 200 | 200 | 404 | 404 |
      | pelayan | kafe-pos | 200 | 404 | 404 | 404 |

      Akun `kasir.dua` (dua cabang) → login membalas **409 CONTEXT_REQUIRED +
      2 choices**, jalur multi-cabang terbukti hidup. Password akun seed
      `kafe123` (bcrypt di DB, bukan plaintext).
      **Dua bug mesin ditemukan & diperbaiki saat mengerjakan ini** (changelog
      `2026-09-22-012`/`-013`): (1) `CreateUser` menulis `"active": u.Active`
      yang zero-value Go = **false**, menimpa default entity → setiap akun baru
      tercipta nonaktif dan login menolaknya dengan "invalid username or
      password"; (2) grant yang gagal materialisasi **dilewati tanpa suara** —
      `approval-inbox:` bukan navigation kind yang didukung, sehingga role
      `supervisor` DAN `manajer` kehilangan SELURUH grant dan semua request-nya
      404 tanpa error apa pun. Resolver kini menerima `SetLogger` dan
      melaporkannya.
      _(catatan lama, dipertahankan sebagai jejak)_
      Dua bagian: (a) role kafe belum punya **grant tersimpan** (jadi permission
      per-role belum bisa diuji dari spec); (b) pemetaan
      `employee.assignments` → `formspec.core.user.assignments` (data/seed)
      belum ditulis, sehingga kasir dev harus diberi assignment manual lewat DB.
      Effort: small–medium (seed `kind: Seed` + role grants).

- [x] **10.4 — ✅ 2026-09-22, diperbarui 2026-09-23 — Menu + gambar ter-seed & terverifikasi tampil.**
      **Selesai.** Seed master kafe memuat **9 menu** (Nasi Goreng Spesial, Sate
      Ayam Madura, Gado-Gado, Nasi Uduk Komplit, Es Teh Manis, Es Jeruk, Kopi
      Tubruk, **Kopi Susu**, **Roti Bakar**), 4 kategori, 11 harga per cabang (2
      cabang dengan harga BERBEDA untuk menu yang sama — membuktikan D1), 4 meja,
      6 karyawan; plus bahan, resep (dengan baris biaya), supplier, dan bagan
      akun GL.
      **Gambar:** **9 foto** dari **Wikimedia Commons** (CC BY / CC BY-SA,
      atribusi lengkap di
      `examples/kafe/spec/modules/cafe-master/assets/ATTRIBUTION.md`), di-commit
      ke repo (seed tidak boleh bergantung jaringan) dan **diunggah lewat storage
      service** oleh seed itu sendiri lewat marker `$asset` (lihat 10.14).
      **Bukti (diperbarui 2026-09-23):** 9/9 foto — `GET .../{id}/photo` →
      **200, image/jpeg, byte-for-byte identik dengan file di `assets/menu/`**
      (sha256 dibandingkan satu per satu, lihat 10.14). Sebelumnya
      `es-jeruk.jpg` adalah **gambar kopi** (File:Kopi O.jpg), dan dua menu
      (Kopi Susu, Roti Bakar) hanya ada di DB dev tanpa foto — keduanya
      diperbaiki/ditambahkan di perubahan yang sama.
      **Sisa:** verifikasi **di browser** (kurva UI, bukan endpoint) — lihat 10.9. Endpoint+byte di atas membuktikan datanya benar,
      bukan bahwa katalognya terlihat benar di layar.
      _(catatan lama, dipertahankan sebagai jejak)_
      Sisa dari 2.5 dan 2.14, dan bagian UI dari 9.4. Menu bergambar, katalog QR,
      keranjang, KDS, kartu meja QR, dan struk (2 dari 9 skenario 9.4) seluruhnya
      masih **API-level** — tidak ada sesi browser yang pernah menjalankannya.
      Termasuk mengunggah foto sungguhan lalu melihatnya di katalog (butuh sesi
      terautentikasi; dev server tanpa seed user) dan pratinjau numpad di
      perangkat sentuh. Effort: medium (butuh seeder user + skrip browser).
      Ditemukan sebagai kelas masalah yang sama di dua tempat berbeda.
- [ ] **10.5 — ⏸️ Sisa prosa lain yang masih terbuka (3 item).**
      Semuanya "dicatat" tanpa nomor di item `[x]` dan **diperiksa ulang
      2026-09-22** — kandidat yang ternyata sudah tertutup atau bukan pekerjaan
      (mis. `HookDecl.uses`/1.7 sudah ditutup 4.7/7.4.7; pemetaan `scope_field`
      ke kolom `text` adalah perilaku yang disengaja) dibuang dari daftar ini:
      (a) **2.2** — alur scan QR masih dua langkah (token → sesi → ID sesi)
      karena `find` tidak bisa di-scope; satu paket dengan **2.15 ⏸️**.
      (b) **3.2** — pada SQLite kolom turunan `money` memakai cast `REAL`, jadi
      presisi eksak tetap milik payload JSON; DB lama perlu recreate agar
      kolomnya ikut berubah tipe.
      (c) **2.7** — halaman kontrak REST di dokumen masih ditulis tangan,
      belum digenerate dari `pkg/spec` seperti bagian per-entity yang dilayani
      `formspec describe`; Report/Print/dashboard masih di luar cakupan halaman.
      Effort: small masing-masing; (a) menunggu keputusan bentuk **2.15**.

- [x] **10.6 — ✅ 2026-09-22 — Strategi alokasi bisa dipilih; `weight` jadi default.**
      **Selesai.** `ingredient.weight_per_unit` (gram per satuan dasar) baru,
      dan `allocation_strategy` pada Config module `cafe-stock`
      (`weight` | `value` | `quantity`). Default: `weight` bila berat semua bahan
      diketahui, selain itu `value`.
      **Terukur pada PO yang sama** (beras 10.000 unit / 150.000 ; kopi 2.000
      unit / 240.000 ; biaya kirim 300.000):

      | strategi | beras | kopi |
      | --- | --- | --- |
      | `weight` (89,3% vs 10,7% berat) | **42** | **136** |
      | `value` (38,5% vs 61,5% nilai) | **27** | **212** |

      **Menolak, bukan menebak:** `weight` yang diminta eksplisit tetapi ada
      bahan tanpa berat → gagal `FORMSPEC.PURCHASE.WEIGHT_MISSING` yang menyebut
      bahan mana, **bukan** diam-diam beralih ke `value`.
      **Bug ikut ketemu:** `unit_cost` hasil alokasi tersimpan dengan 15 desimal
      (`41.785714285714285`) — bukan nilai uang yang sah untuk IDR (skala 0).
      `money_like` kini membulatkan half-up ke skala mata uang.
      **Test pengunci:** `TestKafe_LandedCostAllocationStrategy` (weight/value)
      dan `TestKafe_WeightStrategyRefusesUnknownWeight` — terbukti gagal saat
      cabang `weight` dinonaktifkan (beras 27, want 42).
      _(catatan lama, dipertahankan sebagai jejak)_ Action
      `receive-goods` membagi biaya kirim proporsional terhadap NILAI baris
      (qty × unit_cost), bukan berat/volume. Untuk kafe (bahan makanan, nilai
      sebanding berat) ini cukup dan terdokumentasi di script, tetapi PO dengan
      bahan sangat murah & sangat berat (mis. 50 kg beras vs 100 g vanili) akan
      membebani beras kurang dari porsi fisiknya. Butuh field berat per bahan
      atau pilihan strategi alokasi (`by_value` | `by_weight` | `by_quantity`).
      Effort: medium. **Teramati:** alokasi saat ini hanya punya satu rumus;
      grep `by_weight` di spec → 0.

- [x] **10.7 — ✅ 2026-09-22 — Event pembelian dikonsumsi; jurnal persediaan/utang jalan.**
      **Selesai.** `gl/subscriptions/purchase-to-journal.yaml` mendengarkan
      `on_po_received`/`on_po_cancelled` dan `gl/scripts/journalize_purchase.star`
      membangun jurnalnya: **Persediaan Bahan (1-2000) debit = Utang Dagang
      (2-3000) kredit**. Baris biaya (`line_type: cost`) sengaja DILEWATI saat
      menghitung nilai barang — biaya kirim sudah masuk `unit_cost` pergerakan
      stok lewat alokasi landed cost, jadi menjumlahkannya lagi di sini akan
      menghitung dua kali.
      **Terukur** (dev server, PO 200 × 2.500): jurnal `JRN-2026-000062`
      `status=posted`, `source_id` terisi, **debit 500.000 = kredit 500.000**,
      akun tepat (1-2000 / 2-3000).
      **Dua kegagalan nyata ditemukan saat mengerjakan ini** — keduanya SENYAP: 1. `emit:` pada transisi TANPA `emits:` pada action. State machine terbaca
      benar dan `formspec validate` hijau, tetapi **tidak ada event yang
      pernah dikirim** — penerimaan barang tidak menghasilkan akuntansi apa
      pun. Diperbaiki dengan menambahkan `emits:` pada `receive-goods` dan
      `cancel-po`. 2. Script subscription memanggil fungsi dari file `.star` **lain**.
      Starlark mengompilasi tiap file sebagai unit tersendiri, jadi ini gagal
      di RUNTIME (`undefined: journalize_sale`) pada event pertama — dan
      `formspec validate` **tidak bisa melihatnya**. Diperbaiki dengan membuat
      script itu mandiri.
      **Test pengunci:** `TestKafe_PurchaseReceivedCreatesJournal`
      (`resource/purchase_journal_e2e_test.go`) — dibuktikan **gagal** pada kedua
      mode di atas (menghapus `emits:` → timeout; mengembalikan panggilan
      lintas-file → timeout padahal `validate` hijau).
      _(catatan lama, dipertahankan sebagai jejak)_
      `purchase-order` memancarkan `on_po_received`/`on_po_cancelled` durable
      (10.2), tetapi module `gl` belum punya akun persediaan/utang dagang dan
      belum mendengarkannya, jadi pembelian belum masuk jurnal. Ini **keadaan
      yang diminta pemilik** ("nanti kalau modul GL dikerjakan, bisa listen
      event tersebut") — dicatat sebagai pekerjaan, bukan sebagai bug.
      Effort: medium (akun 1-2000/2-1000 + Subscription + pemetaan).
      **Teramati:** `grep "on_po_received" examples/kafe/spec` → hanya muncul di
      manifest pemancar, nol di sisi consumer.
- [x] **10.8 — ✅ 2026-09-22 — Katalog publik diverifikasi; tiga bug nyata ditemukan & diperbaiki.**
      **Selesai.** Verifikasi dijalankan pada jalur yang benar-benar dipakai
      browser (SPA route + endpoint yang sama, anonim), dan menemukan: 1. **Foto menu 403 untuk tamu.** `menu-item.photo` tidak menyatakan
      `visibility`, default-nya `private` → unduhan menuntut permission
      `view` yang tidak dimiliki tamu. Halaman katalog publik bisa membaca
      daftar menu (200) tetapi **setiap fotonya 403** — jadi menu bergambar
      tidak pernah bergambar di permukaan pelanggan. Diperbaiki
      (`visibility: public`). 2. **Storage hilang setelah hot-reload.** `ReloadSpec` membangun
      `RouterBuilder` baru tanpa me-wire ulang object store → **setiap**
      upload/download membalas `STORAGE_UNAVAILABLE: storage not configured`
      setelah reload pertama. Terukur di dev server: foto 200 → sentuh file
      spec (memicu watcher) → foto **500**. Penyebab dan gejalanya tidak
      berhubungan, itulah yang membuatnya mahal. 3. **Widget `recent-journals`** dirujuk `gl-dashboard` tetapi manifest-nya
      tidak pernah ada (diperbaiki lebih awal hari ini).
      **Bukti akhir:** `GET /app/menu/x` → **200 text/html**;
      `GET menu-item` anonim → **200, 7 menu** dengan `photo` terisi;
      `GET menu-category` anonim → **200, 4 kategori**;
      `GET menu-item/{id}/photo` anonim → **200 `image/jpeg` 83.189 byte**
      (magic `FF D8 FF E2`) — **juga sesudah hot-reload**.
      **Test pengunci:** `TestKafe_PublicMenuPhotoIsReadableAnonymously` dan
      `TestReloadSpecKeepsFileStorage` (`resource/storage_reload_e2e_test.go`) —
      keduanya gagal saat fix-nya dinonaktifkan (403 dan 500).
      **Sisa → 10.9 (sebagian tertutup 10.10):** kurva UI (ukuran sel, tata
      letak) belum dinilai dengan mata — yang dibuktikan adalah jalur data &
      otorisasinya. Verifikasi browser 10.10 menambah bukti bahwa
      halaman-halamannya **ada dan hidup**, tetapi belum menilai tampilannya.
      _(catatan lama, dipertahankan sebagai jejak)_ Foto
      terbukti terunduh & tersaji sebagai JPEG (`GET .../photo` → 200,
      `image/jpeg`), dan `menu-catalog.yaml` merender `widget: image` untuk
      kolom `photo`. Yang belum: membuka halamannya di browser dan memastikan
      katalognya benar-benar tampil bergambar (kurva UI, ukuran sel, perilaku
      kosong). Berbeda dari 10.4 yang menutup sisi DATA. Effort: small.
      **Teramati:** seluruh verifikasi 10.4 lewat HTTP endpoint, bukan DOM.

- [x] **10.10 — ✅ 2026-09-22 — Menu App difilter per-role; App publik tidak
      lagi membocorkan permukaan admin.**
      **Selesai.** Verifikasi 10.9 di browser menemukan 7 bug yang tidak terlihat
      lewat HTTP endpoint (semuanya item 10.1–10.8 diuji via endpoint):
      (1) sesi di-scope dengan segmen URL (`pos`) bukan nama App (`kafe-pos`) →
      **0 permission** → bundle kosong (terukur: 20 entity vs 0);
      (2) redirect pasca-login menjatuhkan prefix `/app/pos` → pengguna
      mendarat di App lain (`kafe-qr`, publik);
      (3) catch-all **diam-diam** mengalihkan ke root surface, jadi link mati
      tampak hidup (klik "Pelanggan" → merender daftar Order);
      (4) root App ber-`root_url` selalu 404 (route `index` tak pernah cocok);
      (5) menu tidak difilter terhadap isi bundle;
      (6) dashboard dikirim walau seluruh widget-nya terfilter → empat
      placeholder "Widget definition not found";
      (7) `entities: null` melempar `e.entities is not iterable` dan mematikan
      panel lewat ErrorBoundary.
      **Dua paparan (exposure) di App publik** — `AppContext.allows` selalu
      meloloskan `formspec.core` sementara `internal/api/meta.go` mengganti
      permission checker dengan selalu-true untuk App publik, sehingga `allows`
      jadi satu-satunya gerbang:
      (a) `GET /_meta/ui?app=kafe-qr` anonim mengembalikan entity
      `formspec.core` (user/role/api-key/session) + halaman `/access-management`,
      dan browser anonim merender tabel dengan kolom `Password Hash`;
      (b) bundle publik mengirim SELURUH module (13 entity, termasuk
      `cafe-master.members` berisi nomor HP pelanggan, `employees`, `shifts`,
      `cash-movements`) karena `public_entities` diabaikan di jalur bundle.
      Endpoint data tetap 401/menegakkan allowlist, jadi **tidak ada baris yang
      bocor** — yang bocor adalah permukaan admin dan **skema** data privat.
      **Bukti akhir:** bundle anonim `kafe-qr` = **4 entity** (persis
      allowlist; 13 → 4), dari framework hanya `/_auth/*`; audit browser
      6 role × semua entri menu = **0 link mati, 0 placeholder**.
      **Test pengunci** (semuanya terbukti gagal saat fix dinonaktifkan):
      `TestBuildBundle_MenuDropsUnreachableItems`,
      `TestBuildBundle_MenuDropsRoutesTheBundleRemoved`,
      `TestBuildBundle_DashboardFollowsItsWidgets`,
      `TestBuildBundle_PublicAppHidesFrameworkAdmin`,
      `meta.appname.test.ts`, `meta.test.ts`. Changelog `2026-09-22-015`.

- [⏸️] **10.11 — Guardrail: entri menu tanpa grant tidak terdeteksi saat
  build.** Filter menu (10.10) memperbaiki gejalanya di runtime, tetapi
  **tidak ada** yang memperingatkan saat manifest ditulis bahwa sebuah entri
  menu menunjuk entity yang tidak di-grant role mana pun. Terukur: sebelum
  10.10, `Pembayaran`/`Pelanggan`/`Loyalitas`/`Sesi Meja` adalah link mati
  untuk SEMUA role non-manajer, dan `formspec validate` hijau. Yang terbuka:
  pemeriksaan validate-time (atau `formspec check`) yang membandingkan entri
  menu App dengan grant yang bisa dihasilkan seed role — hanya mungkin
  bila role ikut terbaca, jadi butuh kesepakatan bentuknya. Effort: medium.
  **Teramati:** `grep` entri menu kafe vs `roles.yaml` → 4 entri tanpa
  grant untuk kasir/pelayan/supervisor sebelum 10.10.

- [x] **10.12 — ✅ 2026-09-23 — `check` kini memeriksa NAMA
      lintas-file; pemeriksanya tertutup di level engine (7.8.16).**
      **Selesai.** Item ini membuka _keberadaan_ pemeriksa, bukan tambalan
      manifest: ketiga rujukan di kafe sudah diperbaiki manual pada 10.10, tetapi
      **tidak ada** yang bisa menangkapnya sebelum runtime. Pemeriksanya sekarang
      ada: `checkReferences` (`cmd/formspec/check.go`) memeriksa `spec.entity`
      pada sepuluh kind + rujukan form/table/component/widget di Page (blocks &
      tabs) + rujukan widget di Dashboard (`widgets` dan `defaults`).
      **Bukti bahwa ia menggigit:** pada contoh yang dikirim, arisan 4→4,
      Clinic-UI-Showcase 4→4, **Midtrans-Payment-Gateway 0→2** (dua bug
      nyata: Page merujuk Form/Table yang tidak pernah ada). Kafe sendiri 0.
      **Sisa (engine, bukan kafe):** 7.8.17 ⏸️ — tidak ada kind UI
      untuk mengedit `kind: Config` atau menampilkan log webhook, yang persis
      yang dibutuhkan contoh Midtrans. Changelog `2026-09-23-004`.

- [⏸️] **10.13 — App publik: `public_entities` diuji hanya lewat satu
  jalur.** 10.10 memperbaiki jalur **bundle**; jalur penegakan permintaan
  (`internal/api/router.go` `isPublicAction`) sudah benar sejak awal. Yang
  belum dibuktikan: keduanya **sepakat** pada seluruh kombinasi
  entity×action×scope (bundle bilang "tidak ada" padahal endpoint
  melayani, atau sebaliknya). Effort: small–medium.
  **Teramati:** 13 entity di bundle vs 4 yang dilayani endpoint — kini
  keduanya 4, tetapi tidak ada test yang mengunci kesamaan itu.

- [⏸️] **10.18 — Kontrol auth hidup HANYA di chrome, jadi App ber-`auth: none`
  tidak punya jalan masuk/keluar sama sekali — dan invariannya tidak dijaga.**
  Ini bukan sekadar "tombolnya hilang", tetapi **lubang di sistem kind**: menu
  adalah dekorasi tier App (§5 Chrome Composition), sementara Page/Form tidak
  bisa menyatakan login/logout.
  Bukti rantai (semuanya diukur/dibaca, bukan disimpulkan):
  1. `AuthArea` dirender **hanya** di dalam header chrome, di ketiga shell
     (`NoNavShell.tsx:104`, `SideNavShell.tsx:147`, `TopNavShell.tsx:188`).
     Tidak ada satu pun `UserMenu`/`LogoutButton` yang dipasang di luar chrome.
  2. `AuthArea` → `if (mode !== "links" && mode !== "button") return null`
     — `auth: none` berarti **nol** kontrol, bukan "kontrol default".
  3. Kontrol auth di dalam header itu sendiri: `{chrome?.brand !== "hide" && (…)}`
     — jadi `chrome: {brand: hide, auth: links}` **juga** menghapus semua
     kontrol auth. Dua jalur, satu jebakan yang sama.
  4. `auth_action` (satu-satunya cara auth dinyatakan di manifest non-chrome)
     adalah closed set `login, register, change_password, forgot_password,
reset_password` — **`logout` tidak ada**. Page kind juga tidak punya blok
     atau CTA untuk aksi auth (`SectionCTA` cuma `{label, href, variant}`).
     Jadi tidak ada cara sah menyatakan logout di Page/Form; hanya custom
     `asset` (`chrome_auth` / komponen) yang bisa.
  5. `useAutoLogout` **tidak dipasang** pada permukaan publik
     (`!isPublic && …` di `App.tsx:383`), jadi sesi di App publik juga tidak
     pernah kedaluwarsa sendiri. Tidak ada jalan pulang sama sekali.
  6. Validator **meloloskan** kombinasi tanpa jalan keluar ini: App
     `access: private` + `app_renderer: no-nav` + `chrome: {auth: none}`
     → `formspec validate` **0 problem** (diuji pada salinan spec kafe dengan
     `kafe-kds` diubah ke `auth: none`). Tidak ada invarian "App privat harus
     punya jalan masuk".
     Terukur di browser: sesi aktif (`app: kafe-qr`) di `/kafe/cafe-master/
menu-categories` → kontrol yang ada hanya `kafe-qr`, `New`, `Previous`,
     `Next`; **tidak ada Sign out**, dan halaman tampak seperti kunjungan anonim.
     Effort: **medium** — bukan "munculkan tombol", karena keputusan bentuknya
     belum ada: (a) apakah `chrome.auth: none` tetap bermakna pada App **privat**
     (jika tidak → tolak saat validasi), (b) apakah Page/Form perlu kemampuan
     menyatakan aksi auth (`logout`) supaya "hak akses menu" tidak lagi
     mandatory-by-omission, dan (c) apa perilaku aman bila `brand: hide`.
     **Teramati:** `grep AuthArea shell/*.tsx` → hanya 3 shell + definisinya;
     `grep logout pkg/spec/frontend.go` → 0; validasi `auth: none` privat → OK.

- [x] **10.19 — ✅ 2026-09-24 — Tombol "New" digerbangi permission `create`.**
      **Selesai** (satu paket dengan 10.23). Tombol "New" kini
      `hasCreate && canDoEntityAction(me, entity, "create")`, dan tombol "Edit"
      di halaman detail ikut digerbangi `update` (sebelumnya hanya `hasSave`,
      padahal targetnya rute yang digerbangi). Changelog `2026-09-24-001`, plan
      `docs_internal/plan/bundle-authorized-actions.md`.
      **Terukur:** manajer (POS) → "New" ada + modal terbuka; kasir (POS, hanya
      `list`+`view`) → "New" **tidak ada**, aksi baris `["View"]` saja.

- [⏸️] **10.20 — App publik ber-`root_url: /` mendaratkan pengunjung di daftar
  entity pertama, bukan di halaman yang bermakna.** `DefaultRedirect` memakai
  urutan: (1) halaman ber-route `/` App itu, (2) entri menu pertama App itu,
  (3) **entity pertama non-summary**. `kafe-qr` tidak punya (1) — katalognya
  hidup di `/menu/:session_id`, yang butuh token sesi meja — dan (2) kosong
  karena `app_renderer: no-nav` mengosongkan menu. Jadi setiap kunjungan ke
  `/kafe` (termasuk pengunjung yang baru login di App publik) jatuh ke (3):
  daftar **Menu Category** yang tidak bisa diapa-apakan pelanggan — bukan
  "halaman depan kafe". Laporan pengguna: _"mengapa kasir hanya menampilkan Menu
  Category saja tanpa bisa melakukan apapun?"_. Ini juga yang membuat App publik
  tampak seperti App admin yang rusak, padahal katalog pelanggannya sehat di
  `/kafe/menu/{session_id}`.
  Effort: medium — butuh keputusan bentuknya (halaman depan `listing`/
  `section` di App publik, atau fallback yang sadar `public_entities` sehingga
  tidak pernah menawarkan _derived list_ yang tidak boleh dipakai anonim).
  **Teramati:** bundle `?app=kafe-qr` → `menu: []`, tidak ada page ber-route
  `/`, entity pertama non-summary = `cafe-master/menu-category`; `GET /kafe`
  → 302 ke `/kafe/cafe-master/menu-categories`.

- [⏸️] **10.21 — Login menerima `app` yang tidak ada (atau App yang tidak
  memuat pemakainya) dan tetap mengembalikan 200 dengan sesi 0-permission.**
  `POST /_ui/auth/login {app: "app-ngawur"}` → **200 + token**, bukan error;
  `User.App` disetel apa adanya, resolver melewati semua role (app tidak
  cocok), dan `/_meta/ui?app=app-ngawur` dijawab 200 dengan bundle kosong —
  bukan "unknown app". Terukur: sesi `app-ngawur` = 0 permission, 0 entity,
  0 menu. Kasus nyata (bukan hanya typo): pemakainya adalah **typo **atau App
  yang salah** — mis. pengunjung App publik `kafe-qr` memakai kredensial staf
  (`kasir`), yang role-nya dideklarasikan `app: kafe-pos`. App publik memang
  *dirancang* tanpa role staf, jadi jangan langsung menghukum; yang hilang
  hanya **sinyal yang jujur**: pengguna mendarat di halaman kosong tanpa satu
  pun pesan yang menyebut sebabnya.
  Effort: small — validasi `req.App` terhadap daftar App ter-resolve di
  `HandleLogin` (tolak `unknown app` dengan 400), lalu beri pesan eksplisit
  di shell saat bundle sebuah App **privat** berisi 0 entity karena 0
  permission. Perlu keputusan: apakah App publik tetap boleh menerima login
  tanpa role (agar alur pelanggan masa depan jalan) — kalau ya, hanya pesannya
  yang diperbaiki, bukan statusnya.
  **Teramati:** `POST /_ui/auth/login` app=`app-ngawur` → **200**;
  `/_meta/me` → `permissions: []`; `GET /_meta/ui?app=app-ngawur` → **200**
  (`entities: 0`, `menu: 0`). Bandingkan preseden `_admin`: `GET
/_meta/ui?admin=true` tanpa `_admin.access` → **403\*\* (bergerbang jujur).

- [⏸️] **10.22 — App privat berisi 0 entity tidak bergerbang, jadi pengguna
  mendarat di halaman kosong, bukan penolakan.** Preseden sudah ada dan benar:
  `_admin` menjawab **403** `missing permission: _admin.access` untuk kasir.
  Tetapi App privat lewat `/_meta/ui?app=…` **tidak** punya gerbang setara —
  cukup `access: private` + token sah, dan bundle dijawab **200** walau
  pemanggilnya 0 permission dan isinya 0 entity. Terukur: `barista` (role
  `app: kafe-kds`) login ke `app=kafe-pos` → **permission 0**, bundle
  **200** dengan **0 entity**, dan SPA merender _"No entities found. Load a
  manifest to get started."_ — pesan yang menuduh manifestnya rusak, padahal
  manifestnya sehat; yang kurang hanya hak akses.
  Effort: small — tolak bundle App privat yang kosong karena permission
  dengan pesan eksplisit (mis. 403 + nama App/role), atau pesan shell yang
  jujur menyebut "App ini tidak memuat role Anda".
  **Teramati:** sesi `barista`→`kafe-pos`: `/_meta/me` permissions `[]`;
  `GET /_meta/ui?app=kafe-pos` → **200** (`entities: 0`, `menu: 0`); browser
  menampilkan "No entities found".

- [x] **10.23 — ✅ 2026-09-24 — Bundle kini mengirim `authorized_actions`; route
      turunan berhenti menebak.**
      **Selesai** (satu paket dengan 10.19). `EntitySchema` membawa himpunan
      aksi yang boleh dilakukan **pemanggil ini**, dihitung server dengan
      checker yang sama yang memutuskan entity ikut bundle
      (`internal/ui/meta.go:authorizedActions`). `buildRoutes` mendaftarkan
      route sesuai himpunan itu; route yang tidak diizinkan **tetap
      didaftarkan** dengan elemen not-found — bukan dihilangkan — karena
      menghilangkannya membuat `/new` ditangkap `:id` (id literal `"new"`) dan
      merender daftar yang **tampak berhasil**.
      Kosakata aksi juga diselaraskan (`view→find`, `edit→update`); sebelumnya
      klien menghitung `.edit` yang tidak pernah ada, sehingga `manajer`
      kehilangan tombol Edit yang justru boleh ia pakai.
      Changelog `2026-09-24-001`, plan `docs_internal/plan/bundle-authorized-actions.md`.
      **Terukur sesudah:** sesi `kafe-qr` (grant `[list, find]`) → daftar tanpa
      "New", `/new` → **Page not found**, `/{id}/edit` → **Page not found**,
      `/{id}` → **detail terisi**. Tanpa regresi: manajer (POS) "New" ada +
      modal terbuka, baris `["View","Edit"]`; kasir (POS) tanpa "New", baris
      `["View"]`. Test: `router.gating.test.tsx` + `permissions.test.ts`
      **dibuktikan gagal** saat gate dinonaktifkan.

      _(bukti sebelum perbaikan, dipertahankan sebagai jejak)_
      `shell/router.tsx` mendaftarkan **empat** route per entity yang ikut
      bundle (list, `new`, `:id`, `:id/edit`) tanpa melihat grant, dan
      `getLifecycle(entity)` hanya membaca `characteristic`+`lifecycle` — jadi
      affordance-nya dibuat lebih dulu, dan pemeriksaan `canDoEntityAction` baru
      muncul **di dalam** komponen. Akibatnya permukaan publik `kafe-qr`
      **menawarkan** lajur tulis yang tidak pernah diberikan. Anonim:
      `POST`/`PATCH`/`DELETE` `menu-category` → **401**, jadi tidak ada data
      bocor — tetapi klien menyajikan halaman yang selalu gagal.
      **Catatan metode:** status HTTP **bukan** bukti di sini — `/kafe/<apa pun>`
      selalu 200 dari SPA catch-all. Yang membuktikan adalah DOM yang dirender.

- [x] **10.24 — ✅ 2026-09-24 — Grant `find` tidak pernah cocok di bundle.**
      Ditemukan & ditutup di jalur 10.23. `publicEntityChecker` membandingkan
      aksi grant (`find`) dengan aksi permission (`view`), sehingga grant
      `[find]` **tidak pernah cocok** dan entity-nya hilang dari bundle padahal
      rutenya dilayani. Terukur sebelum perbaikan:
      `cafe-master.dining-table` (grant `[find]`) **tidak ikut bundle** padahal
      `GET /_ui/entity/cafe-master/dining-table/{id}` anonim → **200**;
      `cafe-order.table-session` (grant `[create, find]`) juga hilang karena
      visibilitas hanya dihitung dari `list`/`view`. Kini ada
      `grantActionForPermission` (`view→find`), dan kedua entity ikut dengan
      `dining-table → ['find']`, `table-session → ['find', 'create']`.
      Changelog `2026-09-24-001`.

- [x] **10.25 — ✅ 2026-09-24 — Kolom `money` di Table turunan tampil sebagai
      JSON.**
      **Diminta:** kolom **Jumlah** di
      `/kafe/app/pos/cafe-order/cash-movements` — "format currency masih
      salah".
      **Terukur sebelum perbaikan:** sel berisi
      `{"amount":"50000","currency":"IDR"}` (DOM), bukan `Rp50.000`.
      Bundle App `kafe-pos` membawa `tables: []`, jadi `TableRenderer` memakai
      `deriveTable(entity)`; `engine/derive.ts` `tableFormat()` hanya memetakan
      `datetime`/`date`/`percent`, sehingga field `money` **tidak pernah**
      mendapat `format` dan `renderCellValue` jatuh ke `JSON.stringify`.
      `cellHintsForField()` di `lib/renderCell.tsx` sudah memetakan
      `money → currency` — satu kosakata, dua tabel, satu bolong.
      **Selesai.** `tableFormat()` memetakan `money → currency`; heuristik
      `decimal + rules[min] → currency` **dibuang** (menebak mata uang dari
      batas bawah yang juga dipakai field non-uang — `gl-balance.
opening_balance`, `setting.tax_percent`, `visit.total` — dilarang
      `05-field-types.md` §2). `MoneyInput` juga memakai mata uang **nilai**
      sebagai fallback pratinjau, supaya `{amount, currency: USD}` tidak
      dicetak ber-simbol `Rp`.
      **Terukur sesudah:** Jumlah → `Rp50.000` / `Rp100.000` / `Rp25.000` /
      `Rp12.500`, **tanpa** `{"amount"` di DOM; detail page AMOUNT
      `Rp50.000`; `payments` (`Rp0`), `shifts` (`Rp500.000`), `menu-item-prices`
      (`Rp45.000`) ikut benar.
      Test: 3 case `derive.test.ts` + 1 case `money-time-input.test.tsx`
      (**dibuktikan gagal** sebelum patch). Suite frontend **324 lulus** (dari 320) · `tsc` bersih · `go build ./...` ok.
      Plan `docs_internal/plan/money-table-format-derivation.md`, changelog
      `2026-09-24-002`.

- [x] **10.26 — ✅ 2026-09-24 — `TableColumn.align`/`width` diabaikan renderer.**
      **Terukur sebelum perbaikan:** `grep -rn "col\.align\|col\.width"
renderers/react-shadcn/src` → **0 hasil**; `getComputedStyle` pada tabel
      `stock-level-table` menunjukkan `text-align: right` yang dideklarasikan
      tidak pernah sampai ke DOM. Manifest yang menyatakan niat lalu diabaikan
      tanpa peringatan — kelas yang sama dengan 10.25.
      **Selesai.** Helper bersama `src/lib/tableColumn.ts`
      (`columnAlignClass`, `columnJustifyClass`, `columnWidthStyle`) dipakai
      **kedua** renderer; `align` diterapkan ke `<th>` **dan** `<td>` (satu saja
      tidak cukup — `<td>` sibling `<th>`, `text-align` tidak diwarisi), plus
      `justify-*` untuk header sortable yang labelnya di dalam flex row.
      `width` diterapkan ke `<th>` (kotak yang dipakai browser menentukan lebar
      kolom) + `minWidth`. `align` kini **enum tertutup** di skema
      (`pkg/spec/frontend.go`), jadi `align: rigth` ditolak validasi alih-alih
      diam-diam diabaikan. Kontrak normatif ditulis di
      `docs/spec/frontend/06-page-kinds.md` §3.1.1 + gotcha di
      `docs/kind/ui/Table.md`.
      **Terukur sesudah (browser, sesi `manajer` app-scoped `kafe-pos`):**
      `stock-levels` → `Saldo`/`Biaya Rata-rata`/`Nilai Persediaan`
      `thAlign=right` **dan** `tdAlign=right`, kolom lain tetap `left`;
      `orders` → `Total` `text-right` + header `justify-end`.
      Test: 10 case baru di `src/lib/tableColumn.test.tsx` (4 gagal sebelum
      patch: 2 sumber-paritas + 2 DOM). Suite frontend **334 lulus** (dari 324)
      · `tsc` bersih · `go build ./...` ok · kafe `validate` **85 manifest, 0
      problem**. Plan `docs_internal/plan/table-column-align-width.md`,
      changelog `2026-09-24-003`.

- [⏸️] **10.27 — `TableColumn.link` diterima skema tetapi tidak berefek.**
  Ditemukan saat menutup 10.26 (satu-satunya atribut `TableColumn` yang
  tersisa tidak dikonsumsi). **Teramati:** `grep -rn "col\.link"
renderers/react-shadcn/src` → **0 hasil**; `grep -rn "link:"
examples/ --include=*.yaml` → **0 hasil** (contohnya hanya di
  `docs/spec/frontend/06-page-kinds.md` §3, `docs/kind/ui/Table.md`, dan
  `reff_docs/`). Yang belum ditetapkan: bagaimana `:param` route Page tujuan
  (`/orders/:id`) diisi dari record baris — konvensi yang sama belum
  dinyatakan untuk `Calendar` (§5) maupun `Kanban`, jadi mengimplementasikan
  sekarang berarti menebak. Sudah ditandai **Open** di spec §3 (tabel
  paritas `TableColumn` vs `ReportColumn`) supaya pembaca spec tahu `link`
  belum mengikat.
  Effort: medium (butuh keputusan kontrak dulu, bukan sekadar kode).

- [x] **10.28 — ✅ 2026-09-24 — Kolom relasi menampilkan UUID; angka tanpa
      pemisah ribuan.**
      **Diminta:** "mengapa cabang dan bahan masih uuid?" & "mengapa saldo
      tidak ada pemisah ribuan?" di
      `/kafe/app/pos/cafe-stock/stock-levels`.
      **Terukur sebelum perbaikan:** sel `branch_id`/`ingredient_id` berisi
      `01a0bf3d-9ee2-7019-9051-7517d6194be5`, dan `quantity_on_hand` → `20000`.
      **Akar #1:** API mengirim **dua-duanya** — skalar FK + objek relasi pada
      alias (`branch_id` + `branch: {name}`) — tetapi hanya `derive.ts`
      (kolom **turunan**) dan `DetailPage` yang membaca alias itu;
      `TableRenderer`/`ListingRenderer` hanya `String(value)`, jadi tabel
      **tertulis** yang menyebut FK langsung mencetak UUID. `ListingRenderer`
      lebih buruk lagi: ia membaca `row["branch.name"]` (kunci bersarang) →
      `undefined` bahkan untuk dot-path.
      **Akar #2:** `decimal`/`integer` tidak punya cabang format sama sekali di
      `renderCellValue` (`currency`/`date`/`relative`/`percent` saja), sehingga
      jatuh ke `String(value)`. `formatter.number()` ada tapi hanya dipakai
      Dashboard/`NumberInput` — kolom tabel tidak punya cara memintanya.
      **Selesai.** `src/lib/relation.ts` (resolver bersama, menerima kedua
      ejaan) + `resolveColumnCell()` di `renderCell.tsx` dipakai **kedua**
      renderer; `format: number` ditambahkan ke kosakata sel, dengan **skala
      field menang** atas `settings.decimal_scale` (`decimal, scale: 3` tidak
      boleh dicetak dengan skala global 2 — itu membulatkan digit tersimpan).
      Format angka sengaja **opt-in**: `decimal`/`integer` sama seringnya
      pengenal (`line_number`) maupun kuantitas.
      **Terukur sesudah (browser, sesi `manajer`):** Cabang `Kafe Senayan`,
      Bahan `Beras Putih`, Saldo `20.000`, **tanpa UUID di DOM**;
      `menu-item-prices` → Cabang `Kafe Senayan`. Kontrak normatif di
      `docs/spec/frontend/06-page-kinds.md` §3.1.2 + gotcha
      `docs/kind/ui/Table.md`.
      Test: `src/lib/relation.test.tsx` (10 case, termasuk **DOM** dari payload
      API nyata — dibuktikan gagal tanpa perbaikan) + 2 case `number(scale)`.
      Suite frontend **346 lulus** (dari 334) · `tsc` bersih ·
      `go build ./...` ok · kafe `validate` 85 manifest 0 problem.
      Plan `docs_internal/plan/relation-display-and-number-format.md`,
      changelog `2026-09-24-004`.

- [⏸️] **10.29 — Sortir kolom relasi mengurutkan UUID, bukan nama.**
  Ditemukan saat menutup 10.28. `sortable: true` pada kolom relasi
  **menjanjikan** urutan yang tidak diberikan. **Teramati (API langsung,
  sesi `manajer`):** `?sort=branch_id` → **200** tapi mengurutkan UUID;
  `?sort=branch.name` → **422 `sort: unknown field "branch.name"`**;
  `?sort=branch` → **422 `sort: unknown field "branch"`**. Akarnya
  `internal/api/handler.go` (`checkField`) hanya menerima nama field entity
  — jadi nama relasi tidak bisa jadi kunci sortir server-side, sementara
  kolom dot-path (`derive.ts`, clinic `visit-table`) sudah memakai ejaan
  itu. Konsekuensinya sama untuk `filters` bertipe dot-path
  (`?branch.name=…` → 422) padahal `06-page-kinds.md` §3.3
  mendokumentasikan `field` filter "boleh dot-path relation".
  Effort: medium–large (butuh jalur sortir lewat relasi di PersistBackend,
  atau tombol sortir dimatikan untuk kolom relasi — keputusan kontrak).

- [x] **10.30 — ✅ 2026-09-24 — `TableColumn.format` belum himpunan tertutup.**
      Ditemukan saat menutup 10.28. `ReportColumn.format` adalah enum sejak S16
      (`currency`/`date`/`datetime`/`percentage`), tetapi `TableColumn.format`
      masih `type: string` di skema (`schemas/formspec.schema.json` →
      `description: "currency | date | relative | ..."`) — jadi salah ketik
      (`format: currncy`) **lolos validasi** dan mencetak nilai mentah, bukan
      ditolak. Sudah ditandai **Open** di spec §3.1.2.
      **Selesai.** `TableCellFormat` (closed set) + `ValidateTableCellFormat` di
      `pkg/spec/widget.go`, dipanggil `ValidateTableColumns` yang sudah ada.
      Himpunannya **bukan** salinan `ReportFormat`: `relative`/`number` hanya ada
      di sel, `datetime` hanya di laporan — jadi `format: datetime` pada kolom
      tabel ditolak **dengan petunjuk** "is a report format, not a table cell
      format", bukan diterima lalu mencetak nilai mentah.
      **Terukur:** `format: currncy` → `unknown cell format "currncy" (allowed:
currency, number, date, relative, percent)`; `format: datetime` → pesan +
      petunjuk. Diff schema = `TableCellFormat` + `$ref` saja.
      **Drift ikut ditutup:** spec §3.1.2 (yang saya tulis sesi sebelumnya)
      mencantumkan `datetime` sebagai format kolom tabel padahal `renderCellValue`
      tidak mengimplementasikannya — baris itu dihapus.
      Test: 3 case baru `widget_test.go` + 2 case sumbu `IsTableCellFormat` vs
      `IsReportFormat`. Changelog `2026-09-24-005`.

- [x] **10.31 — ✅ 2026-09-24 — Sidebar nav tidak bisa di-scroll; item bawah tak
      terjangkau.**
      Dilaporkan pengguna: menu sidebar `kafe-pos` lebih tinggi dari viewport
      (isi 1383px di viewport 560px) tetapi **tidak ada** vertical scroll bar,
      jadi item di bawahnya tidak bisa diakses — terukur, bukan disimpulkan.
      **Selesai.** `ScrollArea` sudah dipasang (`flex-1 py-2`) dan Viewport sudah
      `overflow: scroll`, jadi akarnya bukan overflow yang lupa diset:
      `min-height` flex item tetap `auto`, jadi `flex-1` tidak meng-clamp — Root
      tumbuh mengikuti isi (1354px), `aside` (`overflow: visible`) meluber, dan
      Scrollbar base-ui tidak di-mount karena `hasOverflowY` false
      (`shouldRender` → null). Diperbaiki di primitif
      (`src/components/ui/scroll-area.tsx` → `min-h-0`), bukan di dua call-site,
      supaya kelas cacatnya tertutup untuk semua pemakaian `ScrollArea`.
      **Terukur (browser, sebelum → sesudah):** root 1354 → **504**; viewport
      1338/1338 (tak scrollable) → 488/1338 (**scrollable**); scrollbar tak ada →
      **10×504** di kanan dalam; `aside.scrollHeight` 1383 → **560**; item
      terbawah bottom 1398 → **548** setelah scroll; wheel di atas sidebar →
      `scrollTop` **850** (= maxScroll).
      Test `src/components/ui/scroll-area.test.tsx` (4 case, **kedua** varian
      sidebar: desktop + overlay mobile) gagal 2 bila `min-h-0` dilepas. Spec
      §2 (`docs/spec/frontend/05-app-kinds.md`) + doc renderer diperbarui.
      Changelog `2026-09-24-006`. Master todo **5.20.1**.

- [x] **10.9 — ✅ 2026-09-23 — Katalog publik dinilai dengan mata; dua
      cacat visual ditemukan & diperbaiki.**
      **Selesai.** Sesi meja nyata dibuat lewat alur pelanggan
      (`POST /_ui/entity/cafe-order/table-session` → 201), lalu halaman publik
      `/kafe/menu/{session_id}` dibuka dan **diukur**, bukan sekadar dilihat.
      Dua cacat yang tidak terlihat dari endpoint mana pun: 1. **Kartu bisa diklik tetapi kursornya `default`.** Seluruh kartu adalah
      `<button>` (klik pada **gambarnya** menambah keranjang — diukur:
      `Kopi Tubruk` → total Rp18.000), tetapi kursor di atasnya `default`
      sementara chip kategori di layar yang sama — `<button>` juga —
      menunjukkan `pointer`. Sebabnya **Tailwind v4 menghapus aturan base v3
      `button { cursor: pointer }`**. Diperbaiki di `@layer base`
      (`src/index.css`), dengan pengecualian `:disabled` dan
      `[aria-disabled=true]` supaya `disabled:cursor-not-allowed` tetap
      bermakna. 2. **Foto dipaksa rasio 2.22:1.** Kotak tinggi tetap `h-28 w-full` pada
      lebar kolom 251px memaksa bingkai **251×112** apa pun rasio aslinya;
      dua foto portrait (960×1280) kehilangan **~66%** tinggi. Diganti
      `aspect-square`, yang juga menyamakan bingkai sel tanpa foto sehingga
      baris grid rata.
      **Bukti terukur sesudah perbaikan:** sembilan kartu → **semua bingkai
      251×251 (rasio 1.00)**, termasuk foto portrait dan dua sel tanpa foto;
      pada layar 390px bingkai 147×147 tanpa overflow horizontal; kursor
      `pointer` pada kartu, `default` pada tombol `disabled`/`aria-disabled`.
      Changelog `2026-09-23-001`.
      **Sisa (bukan cacat kode):** tinggi kartu beda 16px antar-baris (360 vs 344) karena jumlah baris deskripsi berbeda (2 vs 1) — itu isi data.

- [x] **10.14 — ✅ 2026-09-23 — Aset seed diunggah lewat storage service (`$asset`); `seed-kafe-assets` dihapus.**
      **Diminta:** "seharusnya ketika seed, selain mengisi data ke db, juga
      meng-copy seed picture ke storage, copy-nya harus melalui service";
      sekaligus hapus `make seed-kafe-assets` dan perbaiki gambar (es jeruk
      salah, roti bakar & kopi susu belum ada).
      **Selesai.** Marker baru `$asset` pada record seed:
      `photo: { $asset: "menu/sate-ayam.jpg" }`. Seed meng-INSERT dulu (id
      tercipta), lalu **mengunggah file lewat storage service** (datastore
      registry: filesystem di dev, garage/minio/s3 di prod) dan menulis **key
      kanonik** `{ws}/{module}/{entity}/{id}/{field}/{uuid}-{nama}` — bentuk yang
      sama persis dengan jalur unggah HTTP, jadi rute unduh dan
      `storage.visibility: public` bekerja tanpa cabang baru. `allowed_types` +
      `max_size_mb` ditegakkan dengan matcher yang sama dengan handler unggah.
      Aset dipindah ke **module-relative** `spec/modules/cafe-master/assets/menu/`
      (9 foto), sehingga ikut terbawa bila module di-vendor.
      **Menjawab "di prod bagaimana?":** sebelumnya tidak ada jawabannya — `cp`
      Makefile hanya bisa menulis filesystem lokal, jadi "seed bergambar" hanya
      hidup di dev. Sekarang jalur tulisnya adalah service yang sama dengan yang
      dipakai server, jadi prod (garage/minio/s3) ikut berfungsi. Sekaligus
      `formspec seed` memakai `resolveDSN` (DSN relatif di-anchor ke project
      root, seperti `dev`/`backup`/`repl`) — tanpa itu seed dari CWD lain menulis
      database yang berbeda dari yang dibaca server.
      **Idempotensi tidak lagi "skip".** Record yang sudah ada tetapi field-nya
      berbeda kini **di-update** dan dilaporkan (`0 inserted, 25 updated, 49
skipped` pada DB dev kafe) — tanpa itu memperbaiki nilai di seed tidak
      pernah sampai ke database yang sudah ada. Dua pengecualian: field
      **write-only** (`masked: true`) dan record yang `doc_status`-nya bukan
      `draft`.
      **Bug kredensial ikut tertangkap:** reconcile pertama kali menulis
      `password: 'kafe123'` **plaintext** ke record user (field `masked` di-hash
      hook menjadi `password_hash`, sedangkan `UpdateFields` tidak menjalankan
      hook). Diperbaiki: seluruh field `masked` dilewati, dan `user` selalu
      `skip` (akun bukan data seed). Terverifikasi: 0 user dengan key `password`.
      **Bukti terukur sesudah perbaikan:**

                      | Uji | Hasil |
                      | --- | --- |
                      | `make seed-kafe` pada DB dev lama | `0 inserted, 25 updated, 49 skipped`; rerun → `0 inserted, 0 updated, 74 skipped` |
                      | 9/9 foto via `GET .../{id}/photo` | **200, image/jpeg**, sha256 **byte-for-byte sama** dengan file di `assets/menu/` |
                      | DB **baru** (`formspec seed` ke `/tmp`) | `74 inserted`, 9 key kanonik `kafe/cafe-master/menu-item/<id>/photo/<uuid>-<nama>.jpg` |
                      | `rm -rf` storage lalu seed ulang | **9 objek dipulihkan** (= menggantikan `seed-kafe-assets`), rerun `0 updated` |
                      | `es-jeruk.jpg` diganti | terkirim ke record yang sudah ada (`update ... code=MNM-002 ([photo])`) — perbandingan **byte**, bukan sekadar "objek ada" |
                      | `go test ./...` | hijau; 6 test baru (`TestSeedAssetUploadsThroughStorage`, `TestSeedReconcilesExistingRecord`, `TestSeedReconcileIsIdempotent`, `TestSeedRestoresMissingObjects`, `TestSeedReconcileSkipsMaskedFields`, `TestSeedAssetRejectedOnNonFileField`) |
                      | `formspec validate --spec examples/kafe/spec --schema schemas` | **85 manifest, 0 problem** |
                      Plan `docs_internal/plan/seed-assets-and-reconcile.md` · changelog
                      `2026-09-23-002`.

- [x] **10.15 — ✅ 2026-09-24 — `docs/kind/` belum punya halaman `Seed`.**
      `kind: Seed` sudah terdaftar (`pkg/spec/seed.go`) tetapi tidak ada di
      `kindGroups` (`internal/genkinddocs/markdown.go`), jadi
      `make generate-kind-docs` melewatinya. Akibatnya kontrak
      `$asset`/`$ref`/reconcile hanya terdokumentasi di godoc `pkg/spec/seed.go`
      dan `docs/cli-tools/02-formspec-cli.md` §6, bukan di tempat pembaca kind
      mencari.
      **Selesai.** Entri grup ditambahkan ke **`data`** — `docs/kind/README.md`
      sudah menghitung `data/` = **11** sejak awal, dan Seed memang deklarasi data
      domain (record yang ditulis lewat `EntityStore`), bukan struktur workspace.
      Halaman `docs/kind/data/Seed.md` ditulis lengkap (Kapan Memakai / Contoh /
      Gotchas; **0 TODO** tersisa).
      **Terukur:** total halaman kind **34** (dari 33), `data/` **11**,
      `make generate-kind-docs` idempoten (narasi tidak tersentuh saat regenerate).
      **Stale ikut diperbaiki:** tabel taksonomi `docs/spec/platform/03-kind-system.md`
      §1 masih menulis "33 kind" dengan Curation=2 **tanpa** `Seed`/`Workspace`,
      padahal kenyataannya 34 halaman dan Curation=3 — tabelnya, §4 (plane), dan
      rincian per grup diselaraskan dengan `KindMapping()`. Changelog
      `2026-09-24-005`.
- [x] **10.16 — ✅ 2026-09-24 — `formspec backup` hanya menyertakan storage FILESYSTEM, bukan service object-store.**
      `backup create` menambahkan `storage/` ke tar dengan membaca hardcoded
      `{StateDirFromDSN(dsn)}/storage` — bukan lewat storage service. Begitu sebuah
      `kind: Datastore` menyajikan `storage` dengan driver garage/minio/s3 (jalur
      prod), seluruh objek `file` **tidak masuk backup** dan tidak ada peringatan.
      **Selesai — dua cacat, bukan satu.**
      (1) _Sisi baca_ diganti `ResolveStorage` dari datastore registry yang sama
      dengan server (`backupStorageService`), dan kunci objek **di-enumerasi dari
      record** yang ikut ter-backup — kontrak `Storage` hanya Upload/Download/Stat
      (tak ada list portable), dan ini cukup karena field `file` menyimpan kunci
      kanoniknya.
      (2) \*Sisi restore **hilang sama sekali\***: entri `storage/*` disaring oleh cek
      ekstensi `.jsonl`, jadi restore menulis record yang field `file`-nya menunjuk
      objek yang tak pernah ditulis. Sekarang objek di-upload **verbatim** (kunci
      identik dengan yang dirujuk record — remap akan memutusnya), dengan tally
      `storage_objects` di manifest + laporan `--dry-run`.
      Komentar file-header yang masih menulis "not yet included (4.8.1)" ikut
      diperbaiki.
      **Terukur:** `TestRestoreUploadsStorageObjects` **gagal sebelum patch**
      (`expected 1 object restored, got 0`) dan lulus sesudah; byte objek
      diverifikasi identik dan kuncinya sama dengan referensi record.
      Changelog `2026-09-24-005`.
- [x] **10.17 — ✅ 2026-09-24 — `formspec dev` masih memakai state dir CWD-relatif untuk DSN non-sqlite.**
      `StateDirFromDSN` hanya memahami DSN SQLite; untuk `postgres://…` ia
      mengembalikan `.formspec` relatif **CWD**, sehingga lokasi fallback
      filesystem storage (dan `dev-jwt-secret`) bergantung pada direktori
      pemanggilan.
      **Selesai.** `StateDirFor(dsn, specPath)` meng-anchor ke project root, dan
      membiarkan path absolut apa adanya (DSN SQLite sudah di-anchor `resolveDSN`,
      jadi anchoring kedua akan menggandakan root). Dipakai di
      `resource/formspec.go` (3 tempat), `cmd/formspec/dev.go`, dan
      `cmd/formspec/seed.go` (logika salinan diganti helper bersama).
      **Terukur (A/B dua binary, CWD `/tmp/ab/run-here`, spec `/tmp/ab/proj/spec`):**
      binary lama mencetak `.formspec/dev-jwt-secret` yang mendarat di
      `/tmp/ab/run-here/.formspec`; binary baru mencetak
      `/tmp/ab/proj/.formspec/dev-jwt-secret` (project root). Test
      `resource/statedir_test.go` (2 case). Changelog `2026-09-24-005`.
- [x] **10.18 — ✅ 2026-09-24 — Empat banner "Expression error: unexpected
      token: '" di form promo (create).**
      `/kafe/app/pos/cafe-master/promos?action=create&form=promo-form&mode=drawer`
      menampilkan empat banner; keenam field kondisional (`percent`, `value`,
      `buy_qty`, `get_qty`, `menu_item_id`, `menu_category_id`) gagal
      dievaluasi.
      **Akar:** `FormSpecExpr` adalah subset **Starlark** (`08-formspec-expr.md`
      §2), jadi kutip tunggal sah — server (`internal/starlark`) memang
      menerimanya, `TestEvaluateGuard_SumLineBuiltin` lulus dengan
      `sum_line('debit')` — tetapi lexer klien hanya punya `case '"'`, sehingga
      `'` menjadi token `ILLEGAL`. Enam `visible_when` kafe memakai
      `fields.type == 'percentage'` (kolom 16/22 = kutip pembuka, 27/31/32 =
      penutup — cocok persis dengan empat pesan).
      **Perbaikan di interpreter, bukan manifest:** memperbaiki `promo-form.yaml`
      saja akan membuat halaman jalan sambil meninggalkan 15 situs lain rusak
      (kafe 11, Clinic-UI-Showcase 5) dan tetap melanggar kontrak. `lexer.ts`
      `readString(quote)` menerima kedua gaya kutip, menutup hanya pada kutip
      pembukanya, dan menolak string tak tertutup.
      **Gap kedua yang ikut ketemu:** `formspec check -f examples/kafe/spec`
      melaporkan `0 error(s)` untuk keenam ekspresi itu — `validateExprGrammar`
      hanya memeriksa `ctx.`, kata kunci, dan delimiter seimbang. Gate kini
      memindai token (string opaque; karakter/operator asing & string tak
      tertutup ditolak), sehingga janji §4 "lolos apply ⇒ bisa dievaluasi"
      benar-benar berlaku.
      **Terukur:** `vitest` 365 lulus (+15), `tsc -b` bersih, `go test ./...`
      hijau; browser: tanpa banner, `type = Percentage` → `percent`, `type =
    Fixed` → `value`, `applies_to = Menu item` → `menu_item_id`. Changelog
      `2026-09-24-007`; sisa paritas grammar → master todo 5.11.7 ⏸️ & 5.11.8 ⏸️.
- [x] **10.19 — ✅ 2026-09-24 — Caption field form promo tampil sebagai nama
      mentah, bukan label.**
      Temuan lanjutan dari 10.18 setelah banner hilang: caption drawer promo
      menampilkan `min_purchase`, `menu_item_id`, `start_date`,
      `max_uses_per_member` — padahal entity `promo` sudah mendeklarasikan
      `title` untuk semuanya (`Minimum Belanja`, `Menu Spesifik`, `Mulai
    Berlaku`, `Batas per Member`). `Jam Mulai`/`Jam Selesai` tampak benar
      hanya karena `label:` ditulis eksplisit di YAML.
      **Akar:** `deriveForm()` (entity tanpa `kind: Form`) membaca `field.title`
      lewat `fieldLabel()`, tetapi `resolveForm()` mengembalikan manifest
      **authored** apa adanya sehingga renderer jatuh ke `field.label ??
    field.name`. Jadi menulis Form — di kafe demi urutan/section/
      `visible_when`, bukan label — menurunkan caption. Terukur dua arah dengan
      probe pada entity yang sama: `DERIVED: min_purchase=Minimum Belanja` vs
      `AUTHORED: min_purchase=(no label)`.
      **Perbaikan di fungsi resolusi, bukan 8 renderer:** `entityFieldLabel` +
      `withEntityFieldLabels`/`withEntityColumnLabels` (`engine/derive.ts`,
      presedensi `label` → `title` → nama di-humanise; `label` eksplisit selalu
      menang; mengembalikan salinan, tidak memutasi entry bundle yang dibagi).
      Ikut diselaraskan: Form, Table, Wizard (3 file), Listing (kolom+filter),
      Report, dan DetailPage — yang sebelumnya `field.name.replace(/_/g," ")`
      dan **mengabaikan `title` sepenuhnya**.
      **Terukur:** drawer promo → `Kode Promo`, `Nama Promo`, `Cabang`,
      `Prioritas`, `Persentase (%)`, `Minimum Belanja`, `Menu Spesifik`, `Batas
    per Member`, `Batas Total`; header tabel promo & detail cabang (`Kode
    Cabang`, `Pajak (%)`, `Header Struk`) ikut benar; regex nama mentah di DOM
      → false. `vitest` 377 lulus (+12), `tsc -b` bersih, `go test ./...` hijau.
      Changelog `2026-09-24-008`. Berdampak ke **seluruh** aplikasi kafe: 111
      dari 161 field di 21 form authored tidak menulis `label:`, jadi semua
      form kini menampilkan title entity. Sisa → master todo 5.14.6 ⏸️.

- [x] **10.32 — ✅ 2026-09-24 — Klik FOTO MENU membuka tab peramban baru, bukan
      dialog.**
      Dilaporkan pengguna: "di FOTO MENU di klik, tampilkan di jendela lain,
      ubah menjadi tampilkan di popup dialog" — pada
      `/kafe/app/pos/cafe-master/menu-items/{id}`.
      **Selesai.** `<img>` sudah benar (2.5/#4), tetapi pembungkusnya anchor
      `target="_blank"`, jadi klik membuka JPEG telanjang di **tab baru**:
      App, sidebar, dan record yang sedang dibuka hilang — di permukaan kasir,
      tab nyasar mudah terlupakan. Tiga situs bercacat sama (`DetailPage` field
      file; `FileInput` pratinjau readonly; `FileInput` thumbnail mode edit) →
      satu komponen bersama `components/ui/image-lightbox.tsx`. Tautan unduh
      berkas non-gambar (PDF/CSV) tetap bertab.
      **Terukur (browser, viewport 923×560):** `openTabs` 2 → **1**, URL record
      tidak berubah, sesudah Close dialog 0, foto 655×439 di panel 831×489
      tercentang dengan rasio asli. Ukuran panelnya jadi temuan sendiri:
      default dialog (`sm:max-w-sm` = 384px) hanya memberi **423px** — lebih
      sempit dari foto 960px — dan `w-auto` **lebih buruk**, sebab elemen
      `fixed` pada `left: 50%` shrink-to-fit ke `viewport − 50%` (**462px** di
      viewport 923px). Yang dipakai: `w-full sm:max-w-[92vw]`.
      Test `src/components/ui/image-lightbox.test.tsx` (6) mengunci DOM dialog
      **dan** ketiga situs render, dibuktikan **gagal** (1 failed / 5 passed)
      saat `DetailPage` dikembalikan ke anchor `target="_blank"`; vitest **383
      lulus** (+6), `tsc -b` bersih. Changelog `2026-09-24-009`. Sisa → master todo
      **5.21.2 ⏸️** (sel tabel ber-`widget: image` + kartu katalog
      `PickerPanel`, keduanya punya konflik hit target: navigasi baris dan
      tambah-ke-keranjang).

- [x] **10.33 — ✅ 2026-09-24 — "Hari Berlaku" diisi sebagai JSON mentah.**
      Dilaporkan pengguna: "field Hari Berlaku, ubah widget menjadi
      SelectMultiTag, jadi seperti tag input, tapi dari selection, bukan dari
      ketikan. buat type widget baru. item yg sudah diselect tidak bisa
      diselect lagi" — pada
      `/kafe/app/pos/cafe-master/promos?action=create&form=promo-form&mode=drawer`.
      **Selesai, di sisi engine — spec kafe tidak didegradasi.** `days_of_week`
      (`type: json`) dirender editor JSON; `widget: tags` yang sudah ada bukan
      jawabannya sebab `tags` menerima **ketikan bebas**, jadi himpunan
      `1=Senin..7=Minggu` tidak bisa ditegakkan (`9` tersimpan). Ditambah
      **atribut `Field.options`** (`{value,label}`; `enum_values` hanya nilai,
      jadi `[1,2]` tidak pernah terbaca "Senin, Selasa") + widget baru
      **`select-multi-tag`** (katalog form 24 → 25).
      **Yang ditegakkan widget** (bukan konvensi): opsi terpilih **tidak
      ditawarkan lagi**; chip urut **deklarasi** (tampilan saja — array tersimpan
      mempertahankan urutan isian, jadi buka-lalu-simpan tidak menulis ulang
      record); nilai di luar deklarasi tetap **tampil & bisa dihapus**, tidak
      dibuang senyap saat save; nilai non-daftar → **error terlihat**, bukan
      diganti `[]`; bentuk nilai dipertahankan (`json` array, `string`
      comma-separated) dan tipe skalar mengikuti deklarasi.
      **Terukur (browser `:8099`, `manajer`/`kafe123`):** pilih `Senin` → daftar
      berikutnya `[Selasa…Minggu]` (Senin **hilang**); chip `Senin, Selasa,
      Jumat` walau urutan klik berbeda; halaman detail `preCount: 0` (chip
      berlabel, bukan `[1,2]` mentah); simpan dari UI → API membaca
      `days_of_week: [1, 5, 2]` dengan `types: [int,int,int]` — **angka**, bukan
      `"1"`. Duplikat ditolak engine: `options[1] duplicates the value "1" from
      options[0] — a duplicate choice can never be selected twice` (probe spec,
      `1` ≡ `"1"`). Test `select-multi-tag.test.tsx` (20) **dibuktikan gagal**
      saat filter "sudah dipilih" dilepas; `go test ./...` 39 paket hijau,
      vitest **403 lulus** (+20), `tsc -b` bersih, kafe `validate` **85 manifest,
      0 problem**. Plan `docs_internal/plan/select-multi-tag-widget.md`, changelog
      `2026-09-24-010`. Sisa → master todo **5.10.17 ⏸️** (Wizard step tidak
      memakai kosakata widget sama sekali — kelas yang sama, ditemukan saat
      menelusuri pembaca `widget:`) + **5.10.18 ⏸️** (filter `select` belum
      membaca `options`).

        **Lanjutan 2026-09-25 — ✅ cardinality dipindah ke Entity.** Laporan
        pengguna lanjutan: "`options` tidak jelas single-select atau multi-select;
        `promo-form.yaml` memakai `widget: select-multi-tag` — seharusnya penentuan
        single/multi ada di level Entity, level Form hanya mengikutinya." Benar:
        atribut 10.33 hanya menyatakan **nilai mana yang sah**, bukan **berapa
        banyak** — sehingga Form yang menentukannya, dan spec kafe **wajib**
        menulis `widget:` (jalur authored hanya memetakan `money`/`time`).
        Ditutup dengan **`Field.multiple`** (`days_of_week` kini `multiple: true`)
        dan `options` yang sah juga di field skalar (single-select ber-caption):
        kafe mendapat field baru **`promo.channel`** (`type: string`,
        `multiple: false`, options `pos/qris/online`). `widget: select-multi-tag`
        **dihapus** dari `promo-form.yaml` — Form kini benar-benar mengikuti Entity,
        dan `formspec check` menolak kalau tidak.
        **Terukur (browser `:8099`, App `kafe-pos`, form promo tanpa `widget:`):**
        chip hari urut deklarasi (`Senin, Jumat` walau klik `Jumat` lebih dulu);
        DB `days_of_week=[5, 1]` bertipe **int** (urutan isian — tampilan tidak
        menulis ulang data); `channel='qris'` skalar (bukan caption `QRIS`);
        halaman detail menampilkan chip hari + caption `QRIS`, bukan `qris`.
        Filter `select` kini juga ber-caption (menutup 5.10.18). Test: vitest
        **454**, `tsc -b` bersih, `go test ./...` 39 paket hijau, kafe `validate`
        85 manifest 0 problem, `check` 0/0. Plan
        `docs_internal/plan/options-cardinality-entity.md`, changelog
        `2026-09-25-007`. Sisa → master todo **5.10.20 ⏸️** (penegakan server),
        **5.10.21 ⏸️** (paritas lintas-shell), **5.10.22 ⏸️** (`generate` belum
        emit union `options`), **5.10.23 ⏸️** (caption `enum`), plus **5.10.17 ⏸️**
        (Wizard) yang tetap terbuka.
        Catatan verifikasi: `go test ./resource/` ternyata **flaky pra-eksisting**
        (`TestKafe_OnPaidCreatesBalancedJournal`: `journal status = "draft", want
        posted`; HEAD bersih 2/8 run, working tree 1/8 — setara, jadi bukan regresi
        perubahan ini) → master todo **5.10.24 ⏸️**. Atribusi lama ke item kafe
        10.7 (`gl/config/gl.yaml`) **salah**: `git diff HEAD -- examples/kafe/spec/

  modules/gl/` kosong.

---

## Indeks Gap → Fase

Tidak ada gap yang boleh hilang. Tabel ini adalah jaring pengaman.

| Gap | Fase         | Gap | Fase     | Gap | Fase     | Gap | Fase   |
| --- | ------------ | --- | -------- | --- | -------- | --- | ------ |
| #1  | 1.4 ✅, 2.14 | #13 | 4.1      | #25 | 8.1      | #37 | 5.4    |
| #2  | 2.4 ✅       | #14 | 6.5      | #26 | 2.3 ✅   | #38 | 5.2    |
| #3  | 2.6          | #15 | 6.3      | #27 | 3.3 ✅   | #39 | 5.3    |
| #4  | 2.5          | #16 | 7.3      | #28 | 1.3, 4.8 | #40 | 6.1    |
| #4b | 2.5          | #17 | 7.4      | #29 | 7.2      | #41 | 6.2    |
| #5  | 1.5 ✅       | #18 | 8.1      | #30 | 4.5      | #42 | 6.4    |
| #6  | 1.2 ✅       | #19 | 8.2      | #31 | 4.3      | #43 | 5.5    |
| #7  | ✅ 0.1       | #20 | 8.2      | #32 | 4.4      | #44 | 2.1 ✅ |
| #8  | 1.8 ✅, 3.5  | #21 | 8.3 🟡   | #33 | 4.2      | #45 | 2.2 ✅ |
| #9  | 1.1, 3.6     | #22 | 3.1 ✅   | #34 | 4.7      | #46 | 2.3 ✅ |
| #10 | 7.1          | #23 | 1.6, 3.2 | #35 | 3.4      | #47 | 2.7    |
| #11 | 3.7          | #24 | 8.4      | #36 | 3.4      | #48 | 2.8    |
| #12 | 3.7          |     |          |     |          |     |        |
| #49 | 2.9 ✅       | #50 | 8.7 ✅   | #51 | 2.10 ✅  |     |        |
| #52 | 2.12 ✅      | #53 | 2.13 ✅  | #28 | 1.3 ✅   |     |        |

| S / D | Fase           | S / D | Fase        |
| ----- | -------------- | ----- | ----------- |
| S1    | 1.5 ✅         | S9    | 1.7 ✅, 5.2 |
| S2    | 1.1 ✅         | S10   | 1.4 ✅, 8.5 |
| S3    | 1.2 ✅         | S11   | 1.8 ✅      |
| S4    | 2.6            | S12   | 1.8 ✅, 4.6 |
| S5    | 1.8 ✅, 3.5    | S13   | 6.1         |
| S6    | 6.2            | S14   | 1.8 ✅, 4.2 |
| S7    | 1.3 ✅         | S15   | 5.3         |
| S8    | 1.6 ✅, 3.1    | S16   | 7.2         |
| D1–D7 | 0.3 ✅, 1.9 ✅ |       |             |

---

## Catatan

- **Estimasi effort belum ditetapkan** — daftar akan menyusut setelah Fase 0
  (sebagian gap sudah tertutup; #7 terverifikasi `CLOSED`).
- **Jangan edit** `reff_docs/` (arsip read-only).
- **Jangan tambahkan konten historis ke `docs/`** — changelog ke `docs_internal/changelog/`.
- Ikuti Workflow Discipline `AGENTS.md`: plan → changelog → todo.
