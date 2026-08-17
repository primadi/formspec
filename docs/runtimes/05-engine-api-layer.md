# Engine API Layer

**Updated:** 2026-07-16 · Status: Draft

> Draft: audit dari kode `internal/api/` yang benar-benar berjalan hari ini
> (bukan desain aspirasional). §5 mencatat status implementasi/gap yang
> masih terbuka — bagian itu boleh berubah tanpa mengubah kontrak.

## 1. Peran

`internal/api` adalah **lapisan HTTP tipis di atas entity engine** — ia tidak
mengimplementasikan bisnis logic sendiri, hanya:

- **Routing & serialisasi**: menerjemahkan `Document`/`Entity` spec (`Expose`,
  `Actions`) menjadi route chi + payload JSON, dan sebaliknya (request →
  parameter panggilan ke `internal/db`/`internal/action`).
- **Penegakan auth/permission per-request**: memverifikasi token dan
  memblokir request yang tidak punya permission — tapi _sumber kebenaran_
  permission (apa yang dipunyai user, apa yang dibutuhkan action) datang dari
  `internal/auth` dan spec, bukan didefinisikan di paket ini.
- **Endpoint meta**: menyajikan bundle manifest visual (`internal/ui`) ke
  Shell manapun — lihat §3.
- **Transport realtime**: hub websocket (`internal/api/wshub.go`) yang
  meneruskan event dari `internal/events.Hub` ke koneksi klien — lihat §4.

Batasnya dengan **entity engine core**: CRUD sesungguhnya (query, DDL,
optimistic concurrency, natural key, idempotency) ada di `internal/db`;
eksekusi custom action (dispatch by `impl.type`) ada di `internal/action`;
registry entity/manifest ada di `internal/entity`. `internal/api` cuma
memanggil API publik paket-paket itu dari handler HTTP —
`internal/api/handler.go`'s `HandlerFactory` menerima `EntityStoreProvider`
(adapter tipis ke `entity.Registry.GetEntityStore`) dan `*action.Dispatcher`,
bukan mengimplementasikan penyimpanan sendiri (`internal/api/handler.go:21-37`).

Batasnya dengan **frontend renderer**: `internal/api` tidak tahu apa pun
soal Shell/Page/component. Ia menyajikan _data_ (entity schema, manifest
visual sudah difilter permission, bundle CRUD) lewat kontrak
[Spec Resolution API](../spec/frontend/04-spec-resolution-api.md); bagaimana
data itu dirender sepenuhnya urusan Shell (`docs/renderers/shadcn-shell/`).
Satu-satunya kode yang tahu bentuk visual di paket ini adalah static-file
serving SPA (`spaHandler`/`spaHandlerFS`, `internal/api/router.go:277-398`) —
itu pun cuma file server dengan fallback `index.html`, bukan rendering.

Penegakan permission yang **mengikat** selalu terjadi di paket ini
(`RequirePermission`, §2) atau di `internal/action` saat action dieksekusi —
bukan berasumsi Shell sudah menyaring. Ini konsisten dengan prinsip
"authorization tidak pernah di lapisan UI" (Spec Resolution API §4).

## 2. Router & Middleware

### 2.1 Struktur Route

Router dibangun `chi` (`internal/api/router.go`), workspace-scoped seluruhnya:

```
/{workspace}/api/v1/_meta/apps
/{workspace}/api/v1/_meta/ui
/{workspace}/api/v1/_meta/me
/{workspace}/api/v1/_meta/entities/{module}/{name}
/{workspace}/api/v1/_ws                              (websocket upgrade)
/{workspace}/api/v1/{module}/{plural}                (CRUD list/create — auto-generated)
/{workspace}/api/v1/{module}/{plural}/{id}            (CRUD find/update/delete)
/{workspace}/api/v1/{module}/{plural}/{id}/{action}   (custom action, POST-only)
/{workspace}/_admin, /{workspace}/_admin/*            (static SPA, kalau webDir/webFS di-set)
/{workspace}/app, /{workspace}/app/*                  (static SPA)
/health                                               (di luar prefix workspace)
```

Route CRUD dan custom action digenerate dari spec, bukan didaftarkan manual:
`GenerateRoutes` membaca `Expose: [{type: rest, actions: [...]}]` tiap entity
dan menghasilkan `RouteDescriptor` untuk `list/find/create/update/delete`
(`internal/api/generator.go:10-51,54-107`); `delete` di-skip secara default
kecuali disebut eksplisit di `actions:` (`generator.go:85-88`).
`GenerateCustomActionRoutes` menghasilkan `POST .../{id}/{action}` untuk
setiap `Action` yang punya `impl` dan bukan action standar
(`generator.go:115-174`). Entity tanpa `Expose` sama sekali tidak
menghasilkan route apa pun — deny-by-default (`generator.go:22-24,127-129`).
Standard action yang ditandai `disabled: true` juga tidak pernah
menghasilkan route, di surface manapun (`generator.go:33-40,70-72`).

`RouterBuilder.registerRoute` (`router.go:188-265`) memetakan tiap
`RouteDescriptor` ke handler chi: `Handler: "auto"` → salah satu dari
`HandlerFactory.HandleList/HandleFind/HandleCreate/HandleUpdate/HandleDelete`;
`Handler: "custom"` → `HandleCustomAction` setelah melihat kembali definisi
`Action` di `EntitySpec` (`router.go:213-241`). Kalau `RequiredPermission`
tidak kosong, sub-router dibungkus `RequirePermission(...)` lewat pola
`r.With(...)` chi (`router.go:246-250`) — route dengan permission kosong
(mis. `public`) tidak pernah melewati middleware itu sama sekali.

### 2.2 Middleware Chain

Urutan global (`BuildHTTP`, `router.go:110-118`):

```
Recovery → Logging → CORS → RequestID → Workspace → Auth
  → (per-route) RequirePermission → Handler
```

- **RecoveryMiddleware** (`middleware.go:181-191`) — recover dari panic,
  balas 500 `INTERNAL_ERROR`.
- **LoggingMiddleware** (`middleware.go:194-200`) — log method/path/request-id
  ke stdout; tidak ada structured logger, cuma `fmt.Printf`.
- **CORSMiddleware** (`middleware.go:166-178`) — header CORS permisif
  (`Access-Control-Allow-Origin: *`), untuk development; tidak ada
  konfigurasi allow-list origin.
- **RequestIDMiddleware** (`middleware.go:156-163`) — ID acak per request,
  disuntik ke context dan header `X-Request-ID`.
- **WorkspaceMiddleware** (`middleware.go:34-54`) — mengekstrak segmen
  pertama path (`/{workspace}/...`) sebagai workspace ID; fallback ke
  `"demo"` kalau kosong. Nilai ini disimpan dua kali di context: sebagai
  "URL workspace" (`WithURLWorkspace`, dipakai untuk pengecekan silang di
  `AuthMiddleware`) dan sebagai workspace aktif (`WithWorkspace`,
  ditimpa lagi oleh identity di langkah berikutnya kalau identity punya
  workspace sendiri).
- **AuthMiddleware** (`middleware.go:66-115`) — memvalidasi token
  `Authorization: Bearer ...` lewat `authValidator` global (`JWTValidator`
  prod, `DevValidator` dev — `SetAuthValidator`, dipanggil sekali saat
  startup). Token kosong → identitas nil (anonymous, hanya lolos di route
  `public`). Token ada tapi invalid → 401. **Cross-tenant/cross-workspace →
  404, bukan 403** (`middleware.go:89-97`): kalau `identity.WorkspaceID`
  hasil validasi token tidak sama dengan workspace di URL, response-nya
  `NOT_FOUND` — workspace lain "tidak ada" bagi caller itu, mencegah
  workspace-ID enumeration lewat status code.
- **RequirePermission(required)** (`middleware.go:128-153`) — per-route,
  bukan global. `required == "" || "public"` → lolos tanpa auth apa pun.
  Identity nil → 401. Identity ada tapi tidak punya permission → 403.
  Pengecekan sesungguhnya didelegasikan ke `auth.Identity.HasPermission`
  (`internal/auth/auth.go:38-62`): match exact, wildcard akhiran `.*`
  (`"billing.invoices.*"` mencakup `"billing.invoices.list"` tapi tidak
  mencakup dua level lebih dalam), dan super-wildcard `"*"`.

**Yang secara sengaja tidak ada di sini**: `internal/permission` (registry
"uses" declaration, footprint modul lintas-modul) **tidak** dipanggil dari
jalur request `internal/api` manapun — package itu dipakai saat load
manifest (`internal/entity/registry.go`) dan `resource/formspec.go` untuk
validasi statis, bukan sebagai penjaga per-request. Penegakan permission
per-request murni `RequirePermission` + `auth.Identity.HasPermission` di
atas. `UsesEnforcement` (`middleware.go:255-268`) ada sebagai kerangka
middleware tapi **selalu meloloskan request** — karena enforcement "uses"
yang nyata hidup di runtime script (`resource/formspec.go`'s
`checkCrossModuleUses`, todo 2.6.4), bukan di middleware HTTP ini; middleware
tidak terdaftar di `BuildHTTP`.

## 3. Endpoint Meta

Empat endpoint read-only di bawah `/{ws}/api/v1/_meta/` (`internal/api/meta.go`),
diregistrasi **tanpa** `RequirePermission` wrapper (`router.go:126-131`) —
karena masing-masing melakukan filtering permission sendiri di dalam handler,
bukan gate biner di depannya:

- **`GET /_meta/apps`** (`meta.go:52-71`) — daftar `kind: App` yang resolved
  di workspace ini (`name`, `root_url`); Shell mencocokkan
  `window.location.pathname` untuk memilih App aktif.
- **`GET /_meta/ui?app={name}`** (`meta.go:117-166`) — bundle penuh:
  `ui.Registry.BuildBundle` (`internal/ui/meta.go:116-212`) merakit
  `EntitySchema` tiap entity plus seluruh manifest visual (`Page`, `Form`,
  `Table`, `Dashboard`, `Widget`, `Report`, `Wizard`, `Kanban`, `Timeline`,
  `Print`, `Theme`, `Menu`) yang sudah difilter: entity ikut hanya kalau
  caller punya permission `{module}.{plural}.list` atau `.view`
  (`internal/ui/meta.go:128-132`); manifest turunan entity (Form/Table/dst)
  ikut mengikuti visibilitas entity induknya (`entityVisible`,
  `meta.go:137-143`); Page dengan `permissions:` eksplisit butuh salah satu
  (any-of, `allowedPage`, `meta.go:216-226`); kind navigasi-saja (Dashboard,
  Theme, Wizard) selalu ikut, elemen di dalamnya digerbangi client-side.
  ETag dihitung dari SHA-256 body data (`meta.go:150-159`), mendukung
  conditional GET (`If-None-Match` → 304). `?admin=true` menyajikan varian
  unscoped-App (semua module, tanpa filter per-entity), digerbangi satu
  permission biner `_admin.access` (`meta.go:102-116,125-131`).
- **`GET /_meta/me`** (`meta.go:171-197`) — identitas caller: `user_id`,
  `workspace`, `roles`, `permissions` — sumber gating client-side. Caller
  anonim mendapat `user_id: "anonymous"`, roles/permissions kosong (bukan
  nil, supaya JSON-nya `[]` bukan `null`).
- **`GET /_meta/entities/{module}/{name}`** (`meta.go:201-231`) — satu
  entity schema penuh (untuk lazy-load form berat). Aturan visibilitas sama
  dengan bundle: kalau caller tidak punya `list`/`view`, balasnya **404**
  (`meta.go:220-224`), bukan 403 — pola yang sama dengan cross-workspace di
  §2 (menyamarkan keberadaan resource dari caller tak berhak), dan memang
  eksplisit disebut di kontrak (Spec Resolution API §2).

### 3.1 Terhadap Syarat Backend-Agnostic (Spec Resolution API §3)

Kontrak melarang bentuk data yang dikirim ke Shell membocorkan detail
PersistBackend (nama kolom fisik, path JSONB). Audit kode hari ini
**menemukan endpoint ini sudah memenuhi syarat itu** untuk fitur yang
benar-benar berjalan:

- `EntitySchema.Fields` adalah `[]spec.Field` apa adanya (`internal/ui/meta.go:23`,
  `pkg/spec/entity.go:120-138`) — field manifest (`name`, `type`,
  `enum_values`, `relation`, dst), sama sekali tidak ada nama kolom fisik
  atau jejak strategi skema.
- Payload CRUD (`list`/`find`) berasal dari `db.EntityRecord.MarshalJSON`
  (`internal/db/crud.go:976-985`), yang **secara sengaja** meratakan
  `Data map[string]any` (hasil dekode JSONB) sejajar dengan kolom framework
  (`id`, `version`, dst) ke satu objek datar berkunci nama field — bukan
  `{"data": {...}, "id": ...}` bernuansa storage. Klien tidak pernah melihat
  nama kolom generated (`ext_*`, `data->>'field'`) lewat jalur ini.

Audit backend-agnostic atas `/_meta` dengan demikian sudah dilakukan dan
hasilnya **bersih** untuk permukaan yang berjalan hari ini (entity schema +
CRUD standar). Itu tidak berarti tidak ada risiko sama sekali ke depan:
field extension (`ext_{namespace}`,
lihat [`jsonb-persist/02-schema-strategies.md`](../renderers/jsonb-persist/02-schema-strategies.md)
§2) belum punya jalur baca/tulis runtime apa pun (`ExtensionStore` baru
sebatas DDL saat install) — begitu jalur itu dibangun, ia wajib melalui pola
flatten-by-field-name yang sama, bukan mengekspos `columnName` mentah.

## 4. WebSocket & Event Delivery

### 4.1 Model Koneksi

`WSHub` (`internal/api/wshub.go:31-39`) adalah connection manager
**workspace-scoped**: peta `map[workspaceID]map[connID]*wsConn`, dilindungi
`sync.RWMutex`. Satu-satunya target broadcast yang didukung adalah
`{scope: workspace}` — tidak ada indeks per-user atau per-permission
(komentar eksplisit di `wshub.go:22-30`). Tiap koneksi (`wsConn`,
`wshub.go:15-20`) punya channel `send` buffered (kapasitas 32,
`wshub.go:114`) dan writer goroutine sendiri (`writePump`,
`wshub.go:133-151`) — socket yang lambat/macet di-drop pesannya
(`select ... default:`, `Broadcast`, `wshub.go:76-84`), tidak memblokir
hub atau koneksi lain.

`HandleWS` (`wshub.go:101-131`) meng-upgrade request terautentikasi ke
websocket (`github.com/coder/websocket`), scope-nya diambil dari
`workspaceFromContext` — jalur context yang sama yang sudah dilalui
`AuthMiddleware` (identity workspace, atau `"demo"` dev fallback), **tanpa**
gate identity terpisah di titik ini. Protokolnya **push-only untuk v1**:
tidak ada application-level message dari klien; `readPump`
(`wshub.go:153-159`) cuma membaca (dan membuang) frame masuk supaya
ping/close control frame terlayani dan disconnect klien terdeteksi.

### 4.2 `events.Hub.Broadcast`

`internal/events` (`internal/events/hub.go`) mendefinisikan kontrak `Hub`
minimal: `Broadcast(workspaceID string, msg EventMessage)`, tanpa parameter
permission/penerima apa pun. `WSHub.Broadcast` (`wshub.go:67-85`)
mengimplementasikannya persis sesuai kontrak itu: kunci baca peta koneksi
workspace tersebut, salin daftar target, lalu kirim `msg` ke channel `send`
setiap koneksi — **tanpa memeriksa apakah penerima punya permission `view`
atas `msg.Resource`**. `EventMessage` (`hub.go:11-16`) memang membawa field
`Resource` ("module/entity"), tapi `Broadcast` tidak pernah membacanya untuk
memutuskan siapa yang boleh menerima — cuma dipakai downstream oleh klien
(yang "dipercaya mengabaikan pesan yang tidak relevan", sesuai frasa yang
sama dipakai kontrak).

### 4.3 Status Terhadap Kontrak Realtime (Spec Resolution API §5)

Kontrak mensyaratkan filter sisi-server _per pesan_: caller menerima event
hanya kalau punya permission `view` atas entity itu, dievaluasi ulang tiap
pesan (bukan sekali saat koneksi dibuka). **Audit kode saat ini
mengonfirmasi gap ini masih ada**: `WSHub` cuma mengenal `workspaceID`
sebagai unit isolasi (satu
level tenant), sama sekali tidak tahu permission per-koneksi atau
per-pesan. Setiap koneksi websocket yang berhasil ter-autentikasi ke satu
workspace menerima **semua** event yang di-broadcast ke workspace itu,
terlepas permission `view`-nya atas entity yang disebut di `msg.Resource`.
Ini gap keamanan nyata (bukan detail kosmetik) yang harus ditutup sebelum
Spec Resolution API naik ke status Final — lihat §5 di bawah.

## 5. Status Implementasi Hari Ini (Gap)

1. **Realtime broadcast tidak difilter per permission penerima** (§4.3) —
   `WSHub.Broadcast` (`internal/api/wshub.go:67-85`) mengirim tiap
   `EventMessage` ke seluruh koneksi satu workspace tanpa mengecek
   permission `view` caller atas `msg.Resource`. Ini melanggar kontrak
   realtime di [`../spec/frontend/04-spec-resolution-api.md`](../spec/frontend/04-spec-resolution-api.md) §5
   secara langsung — payload event bisa bocor ke koneksi yang tidak berhak
   melihat entity tersebut. Perbaikan butuh `WSHub` (atau lapisan di
   depannya) tahu permission caller per koneksi (dari `Identity` yang
   sudah tersedia saat `HandleWS` meng-upgrade), dan mengevaluasinya ulang
   tiap pesan keluar — bukan sekali di awal koneksi (permission caller bisa
   berubah selama koneksi hidup).
2. **Penegakan `uses` kini hidup di runtime script, bukan middleware** (§2.2) —
   `internal/api/middleware.go:255-268` tetap kerangka kosong yang tidak
   terdaftar di `BuildHTTP` (dead code). Enforcement nyata ada di
   `resource/formspec.go`'s `newDispatcher`: caller `uses.resources` di-thread
   melalui rantai script (`internal/action/script.go` →
   `internal/starlark/executor.go`) dan `checkCrossModuleUses` memblokir
   cross-module `resource.call()`/`fetch()`/`create()` yang tidak
   dideklarasikan dengan `USES_VIOLATION` (todo 2.6.4, 2026-08-17).
   **Gap tersisa**: `ctx.db`/`ctx.secrets`/`ctx.*` enforcement menunggu 2.9.1
   (`CtxAPI.SetDatastoreResolver`); module auto-suspend + incident audit belum
   ada.
3. **CORS permisif tanpa konfigurasi** (§2.2) — `CORSMiddleware`
   (`middleware.go:166-178`) selalu mengizinkan `Access-Control-Allow-Origin: *`,
   tidak ada allow-list per-workspace/environment; cocok untuk dev, belum
   ada jalur produksi yang membatasi origin.
4. **Endpoint meta bersih dari kebocoran backend untuk fitur yang berjalan
   hari ini** (§3.1) — audit ini menutup pertanyaan lama soal syarat
   backend-agnostic `/_meta`, dengan syarat: extension field
   (`ext_{namespace}`) belum punya jalur baca/tulis runtime sama sekali,
   jadi klaim "bersih" ini belum teruji untuk permukaan yang belum
   diimplementasikan itu (lihat `jsonb-persist/02-schema-strategies.md` §2).
5. **State machine enforcement dan `emits:` events** tidak diaudit ulang di
   sini secara detail — sudah dicatat sebagai gap di
   [`02-formspec-resource.md`](02-formspec-resource.md) §7 poin 4 dan 5
   (`internal/api.HandleCustomAction` tidak memanggil
   `entity.StateMachineEngine`; `ExecuteResult.Events` tidak pernah
   diisi/dikonsumsi). Disebut ulang di sini karena keduanya terjadi persis
   di titik yang dilewati handler paket ini
   (`internal/api/handler.go` `HandleCreate`/`HandleUpdate`/`HandleCustomAction`),
   bukan gap baru.

## 6. References

| Dokumen                                                   | Isi                                                                    |
| --------------------------------------------------------- | ---------------------------------------------------------------------- |
| `docs/spec/frontend/04-spec-resolution-api.md`            | Kontrak normatif Meta API + Realtime yang diaudit di §3–§4 dokumen ini |
| `docs/spec/backend/01-core-basic.md` §5, §8               | `required_permission`, response envelope/kode error                    |
| `docs/runtimes/02-formspec-resource.md`                   | Engine yang di-serve paket ini (entity engine, dispatcher, registry)   |
| `docs/renderers/jsonb-persist/02-schema-strategies.md` §2 | Extension column (`ext_*`), status implementasi baca/tulis             |
