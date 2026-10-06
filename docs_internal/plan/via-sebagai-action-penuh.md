# Plan: `via` menjadi action penuh (`description`, transisi tanpa `via`)

**Sumber:** keputusan pemilik 2026-09-27 (lanjutan koreksi "gate hidup di
transisi", changelog `2026-09-27-007`).
**Ledger:** kafe 10.47 ⏸️, 10.44 ✅, 10.46 ⏸️, 10.45 ⏸️ + item baru (10.48, 10.49).

## Keputusan pemilik

1. **`via` menjadi action penuh** — satu nama yang menghasilkan route,
   permission, gate, dan deskripsi; tidak perlu lagi dideklarasikan ulang di
   `actions:`.
2. **`description` di transition**, dan itu **menjadi description dari `via`**.
3. **`via` dan `action` masuk registry yang sama** (satu sumber).
4. **Transisi boleh TANPA `via`** → **keputusan C: tanpa tombol**, murni
   dipicu `PATCH` set-state / script / subscription.

## Kenapa berlapis — 13 field hilang

`Action` punya 18 field, `TransitionDecl` 6. Menghapus duplikat `actions[].name`
tanpa menyerap fieldnya **merusak diam-diam**:

| Field hilang                                                          | Akibat                                                                   |
| --------------------------------------------------------------------- | ------------------------------------------------------------------------ |
| `impl`                                                                | **route hilang** — `gl/journal-entry.post` tak bisa di-post, jurnal mati |
| `emits`                                                               | event tak terbit — `on_paid` tak sampai outbox                           |
| `audit`, `idempotent`, `idempotency_key`, `conditions`, `ui`          | audit & tombol hilang senyap                                             |
| `uses`, `params`, `expose`, `call`, `track`, `callback`, `rate_limit` | consent footprint & rate limit hilang                                    |

**Urutan tidak bisa dibalik:** serap field dulu (L1), baru hapus duplikat (L4).

## Yang SUDAH berlaku (terverifikasi)

- **Transisi tanpa `via` sudah valid** — lolos `ValidateTransitionEmits` +
  `ValidateTransitionPermissions`; `PATCH` & script mencocokkan **(from,to)**
  (`FindTransitionByStates`, `validateStateTransition`). Yang kurang **hanya UI**.
- **`via` tidak wajib punya action** — membuang seluruh `actions:` tetap
  `85 manifest(s) validated, 0 problem(s)`.
- Duplikat `via`↔`action`: **85** (reff_docs 9 terpisah); via-only **2**.

## Steps

**L1 — `TransitionDecl` menyerap field action** _(prasyarat semua)_ ✅ 2026-09-27
`description` (keputusan 2) lalu `impl`, `audit`, `emits`, `idempotent`,
`idempotency_key`, `conditions`, `ui`, `uses`, `params`, `expose`, `rate_limit`.
⚠️ **Wajib ditulis di DUA tempat**: struct + struct lokal `UnmarshalYAML`
(`pkg/spec/entity.go:1766`). Kalau tidak, field **hilang diam-diam saat load** —
`formspec validate` hijau (baca file), runtime kosong. Test pengunci per field.

✅ **Selesai 2026-09-27.** Jebakan dua-tempat sudah **ditutup di akarnya**:
`UnmarshalYAML` kini memakai `type plain TransitionDecl` + `,inline`, jadi
menambah field cukup di SATU tempat. `uses`/`params`/`expose`/`rate_limit`
ditambahkan setelah pengukuran menunjukkan migrasi L4 akan **lossy** tanpa
mereka:

| field pada deklarasi ganda       | jumlah | nasib setelah migrasi                     |
| -------------------------------- | ------ | ----------------------------------------- |
| `required_permission`            | 76     | pindah ke `require_permission` (gate)     |
| `uses`                           | 11     | **hilang** tanpa field baru → ditambahkan |
| `params`                         | 2      | **hilang** tanpa field baru → ditambahkan |
| bersih (aman dihapus apa adanya) | 9      | —                                         |

Pengunci: `TestActionSources_TransitionCarriesUsesAndParams`
(`pkg/spec/action_sources_test.go`), terkalibrasi gagal saat penyalinan field
dihapus. Skema JSON ikut diregenerasi (`make generate-schema`).

**L2 — satu registry, dua sumber** _(depends on L1)_
`GetActionSpec` (`internal/entity/registry.go:669`) menelusuri `Actions` ∪
transisi ber-`via`. Precedence: `Actions` menang; transisi menambah
`{From,To,Guard}`. `ReservedActionNames` tetap mengikat.

**L3 — route & collision** _(depends on L2)_ ✅ 2026-09-27, **dikoreksi 2026-09-28**
`GenerateCustomActionRoutes` (`internal/api/generator.go:322`) +
`UICustomActionRoutesForEntity` (`:416`) membaca registry gabungan → transisi
ber-`impl` dapat route. `IsReservedAction(via)` → error jelas. Nama route
ber-`via` lama tetap → **additif, URL tidak berubah**.

⚠️ **Koreksi (2026-09-28).** Klaim di atas benar untuk
`UICustomActionRoutesForEntity`, tetapi **salah untuk
`GenerateCustomActionRoutes`**: sampai 2026-09-28 fungsi itu masih memindai
`es.Actions` langsung. Bukti L3 hanya mengukur surface `/_ui/entity/` (404 → 403),
sehingga celah di `/api/v1/…` tidak terlihat — dan `formspec generate`, yang
mencerminkan surface REST, tidak punya method maupun tipe params untuk transisi
`via`+`impl`. Diperbaiki ke `ActionSources()` (changelog `2026-09-28-005`, todo
5.24.3). Pelajaran yang bisa dipakai ulang: "generator sudah membaca union"
harus diperiksa **per fungsi**, karena satu surface bisa benar sementara
surface lain masih memindai `es.Actions`.

⚠️ **Descriptor bukan route.** Generator menghasilkan `RouteDescriptor` dengan
benar sejak awal; yang belum ikut adalah **registrasi handler**
(`registerRouteWithPattern`, `internal/api/router.go:918`) dan **generator
`prepare`** (`generatePrepareRoutes`, `:262`) — keduanya masih memindai
`EntitySpec.Actions` langsung. Karena `ActionSources()` **mensintesis** `via`
tanpa menuliskannya kembali ke `es.Actions`, penelusuran itu mengembalikan nil →
registrasi dilewati → permintaan jatuh ke handler file generik.

Terukur di server hidup (`POST /kafe/_ui/entity/gl/journal-entry/1/{action}`):

| action                              | sebelum                                      | sesudah                                              |
| ----------------------------------- | -------------------------------------------- | ---------------------------------------------------- |
| `post` (hanya transisi)             | **404** `no such file field or action: post` | **403** `missing permission: gl.journal-entrys.post` |
| `reverse` (hanya transisi)          | **404**                                      | **403** `…journal-entrys.reverse`                    |
| `submit` (kontrol, deklarasi ganda) | 401 anonim / 403 auth                        | tidak berubah                                        |

Pesan 404 menyebut **field FILE** — itu petunjuknya: route milik handler file.
`403` pada `post` setara `submit` membuktikan gate transisi ikut berlaku.

Tiga situs diperbaiki ke `b.registry.GetActionSpec(...)` (union): dua cabang
`prepare` (`/api/v1` dan `/_ui`) dan cabang `custom` di
`registerRouteWithPattern`. Sekarang `grep "EntitySpec.Actions" internal/api/router.go`
= kosong.

Guard: `internal/api/transition_route_registration_test.go` — menguji **status
code**, bukan sekadar "bukan 200"; kalibrasi terbukti 404 saat satu cabang
dikembalikan ke penelusuran langsung.

**L4 — validator anti-duplikat + transisi tanpa `via`** _(depends on L1/L2)_ 🚧
Tolak `actions[].name == transitions[].via`. `via` kosong sah.

**Rencana terpisah:** `docs_internal/plan/l4-validator-anti-duplikat.md`
(temuan pengukuran mengubah desain — baca itu sebelum mengerjakan).

✅ **Validator mendarat 2026-09-27** (`ValidateActionTransitionDuplication`,
`pkg/spec/entity.go`), dengan **dua pengecualian yang wajib ada**:
`ReservedActionNames` (nama lifecycle punya route generik; entri `actions:`
adalah satu-satunya cara mempersempit permission-nya) dan entri yang membawa
`required_permission`/`impl` eksplisit. Test pengunci:
`pkg/spec/duplication_test.go`, terkalibrasi gagal saat penolakan dinetralkan.

⏸️ **Migrasi 83 deklarasi belum dikerjakan — dan TIDAK boleh dikerjakan buta.**
Pengukuran 2026-09-27: **41 duplikat aman dihapus, 36 mengubah otorisasi.**
Rinciannya (Kategori A/B/C) ada di plan L4. Yang butuh keputusan pemilik lebih
dulu: mana bentuk permission yang benar, `{module}.{action}` (dipakai 36 entitas
hari ini) atau `{module}.{plural}.{action}` (fallback + route lifecycle generik
untuk `cancel`).

⚠️ **Urutan yang benar: L5 → L4-migrasi.** Memigrasi kafe lebih dulu akan
menghapus entri `actions:` yang saat itu masih menjadi sumber `buildEntitySchema`
→ tombol transisi hilang. L5 mendarat 2026-09-27, jadi jalannya kini terbuka.

**L5 — footprint, materialize, UI** _(depends on L2)_ ✅ 2026-09-27
`entityFootprint` (`internal/auth/materialize.go:174`) + `authorizedActions`
(`internal/ui/meta.go:1263`) membaca registry gabungan → grant editor
menawarkan `via`, UI menampilkan tombolnya. **10.47 hilang sendiri** (nama grant
= nama gate, lahir dari satu nama).

✅ **Selesai, dan ternyata lebih luas dari yang direncanakan.** Dua pembaca
tambahan yang tidak disebut plan ikut diperbaiki karena bukti bundle
menunjukkannya:

- `buildEntitySchema` (`internal/ui/meta.go:1217`) juga masih membaca
  `es.Actions` — sehingga `schema.actions` **kosong** untuk entitas yang
  deklarasi action-nya sudah dihapus. Ini **prasyarat keras migrasi L4**: tanpa
  perbaikan ini, menghapus entri `actions:` menghilangkan tombol transisi
  (regresi 10.49).
- `entityFootprint` (`internal/auth/materialize.go:174`) — terukur:
  `authorized_actions` untuk kasir kembali memuat `release, reserve` setelah
  diperbaiki, dan **hilang** saat blok `actions:` kafe dihapus tanpa perbaikan
  ini.

🔴 **Bug dedup ditemukan oleh bukti bundle, bukan oleh test:**
`ActionSources()` mensintesis satu action **per transisi**, sehingga
`dining-table` melaporkan `['occupy','occupy','mark-table-served','occupy',…]`
— tiga transisi memakai `via: occupy`. Itu duplikat nama di
`schema.actions` → React key yang sama dua kali di daftar tombol transisi.
Diperbaiki: satu action per nama, transisi **pertama** yang menang (konsisten
dengan `GetActionSpec`). Pengunci: `TestActionSources_SharedViaYieldsOneAction`

- `_FirstTransitionWinsForSharedVia`, terkalibrasi (gagal dengan
  `` `occupy` appears 3 times ``).

**Bukti end-to-end (server hidup, browser):** blok `actions:` kafe
`dining-table` **dihapus seluruhnya** —
`POST /kafe/_ui/_meta/ui?app=kafe-pos` mengembalikan
`actions: [occupy, mark-table-served, release, reserve, unreserve, mark-not-available, mark-available]`
(tanpa duplikat, 7 nama), `authorized_actions: [list, find, update, release, reserve]`,
tombol **"Tandai meja dipesan"** tampil di `/kafe/app/pos/cafe-master/dining-tables/{id}`,
dan PATCH `table_status: reserved` → **200**, status menjadi `reserved`.

**L6 — UI/TS + keputusan C** _(depends on L1)_
`TransitionDecl` (TS, `manifest.ts:592`) belum punya `description`/
`require_permission`. `engine/lifecycle.ts:80`:
`label: action?.ui?.button_label ?? transition.description ?? via`.
**C:** `getAvailableTransitions` **memfilter `via` kosong secara eksplisit**
(jangan andalkan `canDoEntityAction("") === false`), sekaligus menutup React
key collision `key={t.action}`.

**L7 — migrasi + docs + ledger** _(depends on L1–L6)_
`docs/kind/data/Entity.md`, `docs/spec/backend/01-core-basic.md` §1.6,
`02-core-extended.md` §2; 85 deklarasi (mulai kafe); rebuild
`cmd/formspec/validate_workflow.go` `buildTransitionIndex`; tutup 10.47; item
baru **10.48** (transisi tanpa `via` tidak bisa di-gate `kind: Workflow`).

**L8 — validator UI membaca union** _(2026-10-02)_ ✅
`actionExists` (`internal/ui/validate.go:426`) menelusuri `es.Actions` langsung.
Akibatnya setiap `via` yang dipakai sebagai tombol di `kind: Form`/`Table`
diperingatkan `action "…" not on entity …` saat boot, walaupun aksi itu ada —
dan justru **karena** manifest tidak lagi menulisnya di `actions:` (yang sejak
L4 ditolak). Terukur di `examples/kafe`: 8 warning dari `order-form-pos`
(`start-preparing`, `mark-ready`, `mark-served`, `complete-order`) dan
`order-table-pos` (keempatnya) — sementara `confirm-payment`/`void-order` yang
masih punya entri `actions:` bersih, yang persis membuktikan sumbernya.

Ini pembaca union kelima yang terlewat (L3 `registerRouteWithPattern` +
`generatePrepareRoutes`; L5 `buildEntitySchema` + `authorizedActions`; 5.24.3
`GenerateCustomActionRoutes`). **Pelajaran:** pola "satu registry, dua sumber"
gagal dengan cara yang sama setiap kali — pembaca baru ditulis terhadap
`es.Actions` karena itu yang tampak seperti daftar action. Kandidat audit
lanjutan yang disebut di L4 (`internal/auth/materialize.go:242` dan
`pkg/spec/entity.go:1597`) **ditutup di L9** (2026-10-02, changelog `-008`);
celah kelas yang sama (`rate_limit` transisi via-only) **ditutup di L9 juga**
(2026-10-02, changelog `-009`).

Perbaikan: `ActionSources()`; pesan validator menyebut union secara eksplisit.
Pengunci: `internal/ui/validate_transition_action_test.go` (Form + Table dari
satu fixture, terkalibrasi gagal saat regresi disuntikkan). Changelog
`2026-10-02-007`; kafe 10.59 ✅ — sisa audit dua pembaca lain (materialize
`uses`, `ValidateHooks`) dibuka sebagai kafe **10.60 ⏸️**.

**L9 — dua pembaca action terakhir membaca union** _(2026-10-02)_ ✅
Menutup kafe 10.60. Dua situs yang tersisa dari L8, keduanya sebelumnya hanya
"kandidat audit" (belum jadi bug terukur):

1. **`ValidateHooks` (`pkg/spec/entity.go`).** `ValidateEntitySpec` memanggil
   `ValidateHooks(d.Hooks, d.Actions)`, jadi `hooks.on: before|after|on_error`
   yang menyebut `via` transisi ditolak `hook action %q does not match any
declared action` — menolak persis bentuk yang L4 wajibkan.
   → `ValidateHooks(d.Hooks, d.ActionSources())`.
2. **`entityFootprint` (`internal/auth/materialize.go`).** Peta `disabled`
   membaca `es.Actions` langsung. Sinerginya: `ActionSources()` dihitung **satu
   kali** dan dipakai baik untuk `disabled` maupun loop action kustom (yang
   sejak L5 sudah union). Ini migrasi **netral-perilaku** — action sintetis
   tidak pernah `disabled` — jadi tidak ada test yang bisa membuktikannya gagal;
   yang dijaga adalah pembacaan union yang **load-bearing** di fungsi itu
   (kemasukan action kustom), lihat guard di bawah.

**Yang sengaja TIDAK disentuh (dan alasannya).** `resolveAction`
(`internal/api/handler.go`) tetap membaca `es.Actions`; ia dipakai `create`/
`update` dan — menurut plan ini §Further Considerations #1 — justru
**melindungi** dari tabrakan `via: update/delete` yang belum diputuskan.
Satu celah kelas yang sama **ditemukan saat audit ini** dan dibuka sebagai
kafe **10.60a**: `checkRateLimit` memakai `resolveAction`, sehingga `rate_limit`
yang dideklarasikan pada transisi via-only tidak ditegakkan — terikat pada
keputusan §Further Considerations #1, jadi tidak diputuskan sendiri.

✅ **10.60a DITUTUP 2026-10-02** (changelog `2026-10-02-009`) — dan ternyata
**tidak** butuh keputusan itu. `HandleCustomAction` sudah **memegang** spec
hasil union; yang salah adalah pemeriksaan rate limit me-resolve **ulang** lewat
`resolveAction` alih-alih memakai spec yang ada. Perbaikan sempit tanpa
menyentuh `resolveAction`: `checkRateLimitAction` (menerima action hasil
resolve; `checkRateLimit` lama jadi pembungkus agar pemanggil lama netral) +
`rateLimitForAction`. **Terukur lewat handler nyata:** entity dengan
`rate_limit {max:1, per:1s}` hanya pada `via: confirm` → panggilan ke-2 **200**
sebelum, **429** sesudah; guard terkalibrasi gagal saat wiring di-revert.
Pelajaran bentuk: versi pertama test memanggil helper langsung dan tetap hijau
saat wiring di-revert — **guard yang tidak menjaga**; test harus melewati
handler sungguhan.

Pengunci: `pkg/spec/entity_hooks_union_test.go` (ValidateEntitySpec) dan
`internal/auth/materialize_union_test.go` (footprint + Materialize), keduanya
dengan kalibrasi (nama tak dikenal tetap ditolak/tidak termaterialisasi).
Changelog `2026-10-02-008`.

**D2 — bug terpisah, prioritas tinggi** _(parallel)_
`authorizedActions` **tidak memasukkan action kustom** → `canDoEntityAction`
mengabaikan permission → **tombol transisi kustom tak pernah tampil**. Server
mengizinkan (terukur `200`), UI tidak menawarkan. Kasir kehilangan tombol
"clear meja". Perbaikan: `authorizedActions` memakai daftar action dari
`buildEntitySchema` (satu sumber), bukan daftar CRUD hardcoded. → **kafe 10.49 ⏸️**.

## Files

- `pkg/spec/entity.go` — `Action` (1613), `TransitionDecl` (1727),
  `UnmarshalYAML` (1766), `ValidateTransitionPermissions` (2017)
- `internal/entity/registry.go` — `GetActionSpec` (669), `LoadEntities` (170)
- `internal/api/generator.go` — (322/353/358), (416/429/433), (84/114)
- `internal/api/handler.go` — `resolveAction` (389), PATCH gate (1047),
  `HandleCustomAction` (2005, state check 2087)
- `internal/action/events.go` — `ResolveTransitionEmission` (76)
- `internal/entity/state_machine.go` — `findTransition` (122),
  `FindTransitionByStates` (146)
- `internal/auth/materialize.go` — `entityFootprint` (174)
- `internal/ui/meta.go` — `buildEntitySchema` (1192), `authorizedActions` (1263)
- `renderers/jsonb-persist/crud.go` — `validateStateTransition` (3079)
- `renderers/react-shadcn/src/engine/lifecycle.ts` (80),
  `kinds/page/DetailPage.tsx` (95–108, 133)

## Verification

1. `go build ./...` · `gofmt` · `go test ./...` hijau.
2. **Test pengunci**: setiap field `TransitionDecl` baru ada di struct lokal
   `UnmarshalYAML` — dibuktikan gagal bila dihapus.
3. `gl/journal-entry` `via: post` **tanpa** entri `actions` → route
   `POST /_ui/entity/gl/journal-entry/{id}/post` tetap ada, jurnal ter-post,
   `emits` terbit.
4. Duplikat `actions[].name == via` → **validate menolak**; repo hijau setelah migrasi.
5. Transisi tanpa `via` → valid, **tanpa tombol** di UI (difilter eksplisit),
   tetap tercapai lewat `PATCH` status; nol React key warning.
6. `formspec validate` kafe 0 problem · `make generate-schema` + `generate-kind-docs`.
7. UI: kasir melihat tombol "Kosongkan meja" → `200`; pelayan tidak melihatnya.

## Decisions

- **Additif**: URL route lama tidak berubah; manifest lama valid sampai migrasi.
- **`description` transition menang** atas `action.description` untuk label;
  `ui.button_label` tetap prioritas tertinggi.
- **C diterima**: transisi tanpa `via` = tanpa tombol; tidak bisa di-gate
  `kind: Workflow` (workflow memilih transisi lewat `via`) — didokumentasikan.
- **Tidak termasuk**: menggabungkan `kind: Workflow` ke registry ini; menghapus
  `actions:` sepenuhnya (action non-transisi seperti `create-submit` tetap ada).

## Further Considerations

1. **`via: update` / `delete`** (nama reserved, menabrak `resolveAction(es,"update")`
   → transisi itu akan **mematikan PATCH biasa**): (a) tolak dengan pesan jelas
   _(rekomendasi)_ / (b) petakan ke flow standar / (c) izinkan tanpa gate.
2. **D2 dikerjakan sekarang atau bersama L5?** Rekomendasi **sekarang** — bug
   nyata (tombol clear meja tak pernah muncul), independen dari plan induk.
