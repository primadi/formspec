# Spec Resolution API

**Version:** 0.1.0 · **Status:** Draft

> Draft: isi di bawah kontrak yang berlaku. §7 mencatat status implementasi
> hari ini terhadap kontrak ini — bagian itu boleh berubah tanpa mengubah
> kontrak.

## 1. Peran
Seam runtime antara engine dan Shell manapun: kontrak internal yang dipanggil
interpreter Shell untuk mendapat representasi siap-render dari App/Page/Entity.
Rendering adalah interpretasi runtime — Shell di-deploy sekali dan membaca spec
saat runtime; tidak ada build artifact per-app.

## 2. Endpoint
Shell boot dari satu round-trip `GET .../_meta/ui`, ditambah endpoint pelengkap
untuk identitas dan schema entity granular:

```
GET /{ws}/api/v1/_meta/apps
  → daftar App yang resolved di workspace ini (name, root_url) — Shell
    mencocokkan window.location.pathname terhadap tiap root_url untuk
    menentukan App mana yang aktif.

GET /{ws}/api/v1/_meta/ui?app={name}
  → Bundle: seluruh manifest visual App itu dalam satu payload — entity
    schema, Page, Form, Table, Dashboard, Widget, Report, Wizard, Kanban,
    Calendar, ApprovalInbox, NotificationCenter, Timeline, Print, Theme,
    dan Menu (sudah resolved) — permission-filtered
    per caller (§4), scoped ke App tersebut (manifest dari Module yang tidak
    di-depends_on App tidak ikut). `?app=` boleh dilewatkan kalau workspace
    cuma punya satu App. ETag di response body men-dukung conditional GET
    (304 kalau bundle tidak berubah) — mekanisme caching, bukan mekanisme
    kompatibilitas versi (lihat §6).
  → `?admin=true`: varian unscoped-App (seluruh Module, dipakai `_admin`
    surface), digerbangi permission tunggal `_admin.access` alih-alih
    filtering per-entity.

GET /{ws}/api/v1/_meta/me
  → identitas caller: user_id, workspace, roles, effective permissions —
    sumber gating client-side (§4).

GET /{ws}/api/v1/_meta/entities/{module}/{name}
  → satu entity schema penuh, untuk lazy-load form berat yang tidak perlu
    ikut bundle awal. Tunduk aturan visibilitas yang sama dengan §4 (404
    kalau caller tidak punya permission list/view entity itu — bukan 403,
    supaya keberadaan entity tidak bocor ke caller yang tidak berhak).
```

Bentuk `EntitySchema` yang dikirim tiap endpoint: field (tipe, validasi,
relasi), `state_machine` (kalau ada), daftar `actions` (tiap action membawa
`permission`-nya sendiri untuk gating client-side, §4), `lifecycle`
(`plain_crud` | `two_step_autosave`), dan `label_field` (natural key → `name`
→ `title` → `number` → `id`, urutan fallback).

## 3. Wajib Backend-Agnostic
API menyerahkan bentuk data (field/type/validation/permission) — bukan query
result mentah. Dilarang membocorkan detail PersistBackend (nama kolom fisik,
path JSONB). Ini syarat agar Shell tidak perlu tahu backend apa di baliknya.

## 4. Permission Filtering
Filtering terjadi pada tiga granularitas, semuanya di sisi resolusi (bukan di
renderer):
- **Entity** — schema sebuah entity (dan seluruh manifest yang mereferensikannya:
  Form, Table, Kanban, Timeline, Print, Report) hanya ikut terkirim kalau
  caller punya permission `list` atau `view` entity itu. Kalau tidak, entity
  itu (dan manifest turunannya) tidak ada sama sekali di payload — bukan
  dikirim lalu disembunyikan.
- **Page** — kalau Page mendeklarasikan `permissions` eksplisit, Page itu ikut
  cuma kalau caller punya salah satu (any-of). Page tanpa `permissions` selalu
  ikut.
- **Action** — tiap `Action` membawa `permission`-nya sendiri di payload
  (bukan disaring hilang) — ini disengaja: Shell perlu tahu permission apa
  yang dibutuhkan untuk merender kontrol yang tepat (tombol aktif/nonaktif),
  bukan cuma tahu boleh-tidaknya. Pengecekan sebenarnya tetap terjadi di
  `/_meta/me` (katalog permission caller) yang dicocokkan Shell client-side,
  dan — yang mengikat — di resource saat action itu benar-benar dipanggil.

Kind navigasi-saja (Dashboard, Theme, Wizard) selalu ikut di bundle; elemen
di dalamnya (widget individual, langkah wizard) digerbangi client-side
terhadap `/_meta/me`, bukan disaring di endpoint ini.

**Kenapa bukan otorisasi berbasis halaman.** Model "bisa lihat halaman →
implisit bisa simpan entity-nya" ditolak sebagai mekanisme enforcement: asal
UI (apakah request benar-benar datang dari halaman yang berwenang) tidak bisa
diverifikasi server — masalah *confused deputy* klasik — dan client unmanaged
(Flutter, API mentah) tidak pernah melewati "halaman" sama sekali. Karena itu
enforcement **selalu** di resource (`required_permission`, lihat
`spec/backend/01-core-basic.md` §5), tidak pernah di lapisan UI. Endpoint ini
hanya menyediakan *derivasi* footprint kapabilitas per Page (dari komposisinya:
Form → action, Table → list, component → deklarasi `needs:` eksplisit) supaya
Shell bisa merender kontrol yang tepat — bukan sumber otorisasi itu sendiri.

**Administrasi berbasis tugas: grant-per-halaman termaterialisasi jadi
permission.** Penolakan otorisasi berbasis halaman di atas soal *enforcement*,
bukan soal UX granting. Ketika Workspace Owner/admin memberi user "akses ke
halaman X" lewat UI admin (pengalaman granting yang task-based / berorientasi
halaman), framework **wajib** mematerialisasikan grant itu menjadi permission
resource konkret, biasa, auditable, dan revocable — string `required_permission`
yang sama yang dipakai di mana-mana ([`../backend/01-core-basic.md`](../backend/01-core-basic.md)
§5) — **tidak pernah** sebagai flag opaque "boleh lihat halaman ini". Dengan
begitu UX admin tetap sederhana (grant per halaman, footprint kapabilitas Page
di atas jadi bahan derivasinya) sementara enforcement di baliknya tetap seragam
dan auditable (string permission, argumen *confused deputy* di atas tetap
berlaku).

## 5. Realtime
Realtime adalah **kapabilitas inti** Spec Resolution API — standar websocket
untuk browser shell yang didefinisikan di sini bagian dari kontrak, bukan
ekstensi opsional; implementasi renderer boleh mendarat bertahap (§7) tapi
kontraknya inti.

Subskripsi deklaratif terhadap perubahan entity, terpisah dari `/_meta/ui`:
- **Konvensi channel:** `entity:{module}.{name}` dengan event `created |
  updated | deleted`, payload = field event, selalu tenant/workspace-scoped.
- **Filter sisi-server:** caller menerima sebuah event hanya kalau ia punya
  permission `view` entity itu — dievaluasi **per pesan**, bukan sekali saat
  koneksi dibuka (permission caller bisa berubah selama koneksi hidup).
- **Pemakaian deklaratif:** `realtime: true` pada Table/Dashboard/Kanban =
  auto-subscribe + patch baris di tempat.
- **Pemakaian programatik:** `forma.subscribe("billing.order", cb)` di
  component custom.
- Realtime **non-durable by definisi** — client yang reconnect refetch lewat
  `/_meta/ui`/`/_meta/entities/...`, tidak ada replay.

## 6. Versi & Kompatibilitas
Versi API ini ada di path (`/api/v1/...`) — Shell resmi dibangun terhadap satu
versi mayor, breaking change menaikkan segmen versi. ETag pada `/_meta/ui`
(§2) adalah mekanisme caching (conditional GET, 304 kalau bundle tak berubah)
— bukan mekanisme kompatibilitas, jangan dicampur artinya dengan versi API.

## 7. Status Implementasi Hari Ini (Gap)
- Filter per-pesan pada Realtime (§5) **belum diimplementasikan** sesuai
  kontrak: hub websocket saat ini broadcast ke seluruh koneksi ter-registrasi
  di satu workspace tanpa memfilter per `resource`/permission penerima —
  client dipercaya mengabaikan pesan yang tidak relevan. Ini gap yang harus
  ditutup sebelum dokumen ini naik ke Final, bukan detail kosmetik: tanpa
  filter sisi-server, event bisa membocorkan payload ke caller yang tidak
  punya permission `view` atas entity itu.
- Endpoint hari ini (§2) sudah sesuai bentuk kontrak ini (bundle per-App, bukan
  per-page `view-spec` seperti draft awal dokumen ini sebelum direvisi).

## 8. Referensi
| Dokumen | Isi |
|---|---|
| [`01-visual-hierarchy.md`](01-visual-hierarchy.md) | Shell, App/Page/Component renderer |
| [`02-visual-spec-kind.md`](02-visual-spec-kind.md) | VisualSpecKind, slot system |
| [`spec/backend/01-core-basic.md`](../backend/01-core-basic.md) §5 | `required_permission` di resource — enforcement sebenarnya |
