# TODO — Menutup Gap agar Aplikasi Kafe Berjalan

**Tujuan:** menutup **seluruh** gap yang didokumentasikan di folder ini sehingga
`examples/kafe` (3 App: `kafe-qr` publik, `kafe-pos` privat, `kafe-kds` no-nav;
6 module; 24 entity) dapat dijalankan **end-to-end dengan benar**.

**Cakupan:** #1–#48 (ledger temuan), S1–S16 (kelengkapan bahasa spec,
`13-kelengkapan-spec-untuk-kafe.md`), D1–D7 (semantik yang belum ditetapkan).

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

- [ ] **0.1 — Verifikasi ulang #1–#48** terhadap binary & kode terkini.
      Setiap gap: jalankan perintah yang bisa gagal (curl ke `/_ui/`, `formspec migrate plan`,
      `formspec validate`, baca DDL hasil generate). Catat: status + bukti + file:line.
      _Accept:_ 48 entri punya bukti; yang `CLOSED`/`RETIRED` diberi alasan.
      **Progres 2026-09-14:** #7 ✅`CLOSED`; #22, #27, #44, #46, #10, #17, #21, S10 🔴`OPEN`
      (bukti: `14-temuan-fase-0.md`); **#52, #53 🔴 baru ditemukan**; sisanya belum diuji ulang.
- [ ] **0.2 — Verifikasi ulang S1–S16** terhadap `schemas/formspec.schema.json` &
      `pkg/spec/*.go` terkini (bukan cache `v0.0.8`).
      _Accept:_ tiap S-item ditandai `OPEN`/`PARTIAL`/`CLOSED` + kutipan schema.
      **Progres 2026-09-14:** S4 🔴OPEN (tak ada widget/field `qrcode`); S9 🔴OPEN
      (`WorkflowTransitionRef.From string` tunggal, `pkg/spec/resources.go:632`);
      S10 🔴OPEN (`widget` = string bebas di schema); S13 🔴OPEN (`TransitionDecl`
      tanpa field `emit`, `pkg/spec/entity.go:708`); S16 🔴OPEN (`ReportColumn` tanpa `widget`).
      Sisa S-item belum diuji satu per satu.
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
- [ ] **0.5 — Klasifikasi ulang ledger** ke SPEC / ENGINE / DOC / PERILAKU sesuai
      hasil Fase 0; hapus entri yang gugur dari `README.md`.
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
      **Sisa (bukan bagian 1.6):** GAP-36 tetap — constraint menolak baris baru,
      tidak bisa merapikan duplikat yang sudah ada (DML ditolak di `kind: Migration`).
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
      menuntut kind/halaman yang men-resolve sesi dari token, bukan dari ID.

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
      apakah transform benar-benar digenerate saat upload.
- [ ] **2.6 — #3/S4: QR code.** 🟡 **widget siap; adopsi kafe & jalur cetak belum**
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
      money lewat widget yang sama.
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
      halaman, dan kontrak `api/v1` tetap terpisah (§8.2/§8.4).
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
      / `resource` (jalur non-dev); pengaruh ke `tenant_id` diverifikasi lewat
      flag pada catatan Fase 0, bukan lewat penulisan record di sesi ini.
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

---

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
      perlu recreate/`kind: Migration` agar kolomnya ikut berubah tipe.
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
      Postgres belum diverifikasi.
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
- [ ] **3.5 — #8/S5: scope cabang ditegakkan engine** (setelah 1.1 & 1.8): entity
      ter-scope difilter otomatis; `TenantDecl` diganti deskriptor dimensi.
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
      dibaca permukaan tamu (`find` by id).

- [ ] **3.8 — Konteks sesi: (principal, role, cabang).**
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
      diputuskan tersendiri.

---

## Fase 4 — Stok, HPP & pembelian

- [ ] **4.1 — #13: valuasi inventory.** Moving-average di Starlark (D3) — perlu 4.2–4.4.
- [ ] **4.2 — S14 + #33: `maintained_by` + `invariants`; hooks/conditions
      benar-benar dipanggil pada `summary`.** Menghapus kondisi "terlihat terpasang
      tapi tidak jalan".
- [ ] **4.3 — #31: API Starlark find-by-field** (menghapus keharusan raw SQL).
- [ ] **4.4 — #32: guard keunikan atomik** tanpa reimplementasi UNIQUE yang rapuh.
- [ ] **4.5 — #30: `ctx.db()` di dalam transaksi aksi tidak deadlock di SQLite.**
- [ ] **4.6 — S12: satuan & konversi** (gram/kg/pcs) untuk ledakan resep.
- [ ] **4.7 — #34: `HookDecl.uses`** agar akses script terlihat di consent footprint.
- [ ] **4.8 — #28: verifikasi aritmetika/agregasi `money` di laporan stok** (lihat 1.3).

---

## Fase 5 — Kas, shift, void & approval

- [ ] **5.1 — Partial unique shift terbuka** (D6) — bergantung 1.6.
      _Accept:_ dua shift `open` untuk (cabang, kasir) **ditolak DB**.
- [ ] **5.2 — #38/S9: void multi-state-asal lewat approval** (lihat 1.7).
- [ ] **5.3 — #39/S15: `WorkflowStep` punya `title`, `description`, `display_fields`.**
      _Accept:_ ApprovalInbox menampilkan nomor pesanan, total, alasan void.
- [ ] **5.4 — #37: shorthand `render: drawer` diterima** (selaraskan loader & schema).
- [ ] **5.5 — #43: dokumentasikan aturan simetri cancel (7.7.2).**

---

## Fase 6 — Akuntansi & integrasi lintas-app

- [ ] **6.1 — #40/S13: keterkaitan transisi ↔ event dinyatakan eksplisit**
      (`emit:` pada transisi). Prasyarat seluruh integrasi stok & jurnal.
      _Accept:_ `order.paid` terverifikasi terpancar saat transisi ke `paid`.
- [ ] **6.2 — S6/#41: pemetaan payload pada `IntegratorCall` (`map:`).**
      _Accept:_ order lunas → jurnal seimbang (kas/omzet/pajak/HPP) tanpa script baru di `gl`.
- [ ] **6.3 — #15: cross-app grant ditegakkan + `SyncAgent` tersambung ke router.**
- [ ] **6.4 — #42: kepemilikan `publishes` ditentukan** saat dua App meng-mount module sama.
- [ ] **6.5 — #14: vertical `purchase`** (atau keputusan tertulis untuk tetap model sendiri).

---

## Fase 7 — Struk & laporan

- [ ] **7.1 — #10: Print `thermal`/`dotmatrix`** diimplementasikan **atau** ditolak
      di `formspec validate` (sekarang validate "bohong").
      _Accept:_ `receipt-thermal.yaml` menghasilkan output thermal 58mm, atau gagal jelas.
- [ ] **7.2 — #29/S16: selaraskan kontrak `ReportColumn` vs `TableColumn`**
      (`widget`, `format`, `aggregate` → enum; dokumentasi berdampingan).
- [ ] **7.3 — #16: `DashboardWidget.ref` module-qualified.**
- [ ] **7.4 — #17: realtime untuk Timeline** (KDS timeline).
      _Accept:_ tabel/kanban/dashboard/timeline semua menerima update realtime.

---

## Fase 8 — Dokumen, skill & DX

- [ ] **8.1 — #18 + #25: koreksi skill** (`entity-authoring` soal `relation.target`;
      `formspec-kinds` soal `Config.spec.keys` vs `spec.data`). Target: `ai_skills/**`.
      🟡 2026-09-14 — **#18 tertutup**: baris `relation` di `ai_skills/entity-authoring/SKILL.md`
      kini mengajarkan `relation: {type: belongs_to, resource: "<module>.<entity>"}` dan
      menegaskan key-nya `resource`, bukan `target`. Sisa: #25 (`Config.spec.keys` vs `spec.data`).
- [ ] **8.2 — #19 + #20: kebersihan drift dokumen**
      (`03-kind-renderers.md`, `realtime.md`, `spec.version` vs `formspec-app.yaml`).
- [ ] **8.3 — #21: validator menangkap referensi menggantung**
      (`App.spec.modules`, menu `view:`, `impl.ref` ke `.star` tidak ada).
      🟡 2026-09-14 — **`impl.ref` sudah tertutup** (bersama 8.7: script hilang → error).
      Sisa: `App.spec.modules` menunjuk module yang tidak ada, dan menu `view:` menunjuk
      Form/Table yang tidak ada.
- [ ] **8.4 — #24: pesan error port `formspec dev` lebih menuntun.**
- [ ] **8.5 — S10 lanjutan: enum untuk `ReportParam.type`, `ReportColumn.format/.aggregate`,
      `EventDeliveryDecl.channel`, `PrintOutput.format`, `WorkflowStep.mode`.**
- [ ] **8.6 — Regenerasi artefak**: `make generate-schema` + `make generate` +
      `make generate-kind-docs`; pastikan `git diff` bersih setelah semua.
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
- [ ] **8.8 — Satu example yang benar-benar memakai `kind: Migration`.**
      Temuan saat 3.4: **tidak ada satu pun example** di repo ini yang masih
      punya manifest `kind: Migration` (kafe menghapus ketiganya di 1.6 karena
      aturan keunikannya kini deklaratif; `find examples -path "*/migrations/*"
  -name "*.yaml"` kosong). Akibatnya `ddl`/`ddl_by`/`dml` hanya terbukti lewat
      test unit, dan kind itu tidak pernah melewati jalur nyata: `formspec
  migrate plan` → `apply` → DB sungguhan → `formspec validate` pada manifest-nya.
      Yang diminta: satu example (boleh di `examples/reference-app` atau module
      baru) dengan satu migration yang **butuh** jalur ini — mis. index atas
      ekspresi JSONB (memaksa `ddl_by`) atau perbaikan duplikat sebelum unique
      index (memaksa `dml` + `reason`) — lalu dijalankan di walkthrough 9.4.
      _Accept:_ ada manifest `kind: Migration` di `examples/**` yang dijalankan
      `migrate apply` di CI/walkthrough, sehingga kind ini tidak bisa membusuk
      tanpa ketahuan.

## Fase 9 — Verifikasi end-to-end aplikasi kafe

- [ ] **9.1 — `formspec validate` hijau** vs `validate-baseline.md` (nol problem di luar baseline).
- [ ] **9.2 — `go test ./...` + `make lint` bersih.**
- [ ] **9.3 — `cd renderers/react-shadcn && vitest` bersih.**
- [ ] **9.4 — Walkthrough 3 App** (`formspec dev`), 9 skenario: 1. Pelanggan scan QR → lihat menu bergambar → keranjang → pesan. 2. Bayar QRIS → **lunas** → pesanan otomatis masuk KDS. 3. Bayar di kasir (tunai) → kasir "Lunas" → masuk KDS. 4. Barista: kanban `queued → preparing → ready → served` (per item, `line_status`). 5. Buka shift (kas awal) → transaksi → kas masuk/keluar → tutup shift → selisih. 6. Void: kasir ajukan → supervisor setujui (**semua** state asal) → tercatat. 7. Penjualan → `stock-movement` + `stock-level` (moving average) → `menu-cost` margin. 8. Order lunas → jurnal GL seimbang (kas/pajak/omzet/HPP). 9. 6 report tampil + struk digital (QR) & thermal tercetak.
- [ ] **9.5 — Hapus marker `# GAP-nn`** dari `examples/kafe/spec/**` untuk gap yang sudah
      tertutup; update `README.md` status.
- [ ] **9.6 — Update workflow discipline**: `docs_internal/plan/todo.md`,
      `docs_internal/changelog/YYYY-MM-DD-NNN-*.md`, dan plan file terkait.

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
- **Jangan edit** `docs_old/` dan `reff_docs/` (arsip read-only).
- **Jangan tambahkan konten historis ke `docs/`** — changelog ke `docs_internal/changelog/`.
- Ikuti Workflow Discipline `AGENTS.md`: plan → changelog → todo.
