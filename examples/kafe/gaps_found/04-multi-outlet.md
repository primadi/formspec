# Gap #8 & #9 — Multi-Outlet (Multi-Cabang)

Kebutuhan: **beberapa outlet**, stok & harga per outlet, nomor pesanan per
outlet, dan pemilik melihat semua outlet dalam satu dashboard.

FormSpec punya jawabannya — tapi jawabannya adalah **"model sendiri sebagai
field biasa"**, bukan sesuatu yang framework pahami. Untuk kafe, itu berarti
sebagian besar pengamanan outlet harus ditulis tangan.

---

## Gap #8 — Hanya `tenant_id` yang di-inject; `TenantDecl` tidak terpakai ✅ Pasti

### Bukti

`docs/architecture/07-vertical-modules.md` §5 (Branch model) — dinyatakan
eksplisit dan jujur:

> Branch adalah business concept, **not a framework one**. Evidence:
> `internal/db/ddl.go` auto-injects **only `tenant_id`** (plus
> version/timestamps/soft-delete) into every table — **no
> `branch_id`/`company_id`/`org_unit_id` anywhere**. `pkg/spec/entity.go`'s
> field-type enum has **no tree/hierarchical type**; relations are only
> `belongs_to | has_many | has_one`.

Dan yang paling penting:

> `pkg/spec/entity.go` already has a `TenantDecl{Isolated bool}` struct sitting
> on `EntitySpec.Tenant` that looks exactly like what a framework-level branch
> flag would need — but it **has zero consumers anywhere in `internal/db` or
> `internal/entity`** (confirmed by grep). It's an **aspirational field the
> engine never wired up**.

Masuk daftar gap resmi §8:

| Gap | Bukti | Status |
| --- | --- | --- |
| `EntitySpec.Tenant *TenantDecl` unwired | Zero consumers in `internal/db`/`internal/entity` | Same class of "spec got ahead of the engine" gap |

Dan jalan keluar yang dipilih (juga dinyatakan eksplisit):

> So `branch_id` ships as an **ordinary relation field**, standardized purely as
> a **naming convention** (always that name, always `belongs_to company.branch`).

### Dampak ke aplikasi kafe: **BLOCKER** untuk multi-cabang

Yang **tidak** ada:

1. **Tidak ada penanda scope deklaratif.** Setiap entity yang butuh scoping
   cabang harus mendeklarasikan field relasi `branch_id` manual — dan ingat
   menambahkannya ke **setiap** entity yang relevan (13 entity di aplikasi ini).
2. **Tidak ada row-level filtering otomatis.** Tidak ada padanan "RLS per
   outlet". Kalau kasir outlet A membuka daftar pesanan, ia melihat pesanan
   **semua outlet** kecuali tabel itu diberi `fixed_filters` — dan
   `fixed_filters` adalah **UI-level**, bukan otorisasi server. Jadi bukan
   pengamanan.
3. **Tidak ada isolasi stok antar outlet yang dijamin engine.** Stok per outlet
   harus dimodelkan sendiri (mis. `stock-level` dengan dimensi outlet).
4. **Tidak ada izin per-outlet.** Permission = `module.entity.action` — tidak
   ada dimensi lokasi. "Kasir hanya boleh outlet yang ditugaskan" tidak bisa
   dinyatakan.
5. **Satu workspace = satu tenant.** Ini justru *menghalangi* pendekatan
   "satu outlet = satu workspace": pemilik ingin **membandingkan** outlet dalam
   satu dashboard, dan data antar-workspace tidak bisa di-query bersama.

### Praktik yang disarankan oleh FormSpec sendiri

Pakai vertical `company` dengan entity `branch` sebagai master data:
`code`, `name`, `parent_id` (opsional untuk hierarki), `is_active`. Lalu
field `branch_id` (relasi `belongs_to` ke master cabang) di entity yang perlu
scoping. Aplikasi kafe ini melakukan persis itu — entitynya bernama
`cafe-master.branch` dengan field `branch_id`, memakai **nama konvensi**
supaya siap menerima mekanisme framework nanti (lihat `architecture.md` D10).

**Konsekuensi yang harus diterima:**

- Scoping adalah **tanggung jawab aplikasi**, bukan framework.
- Butuh script/`Service` untuk memaksa filter outlet di setiap query yang
  sensitif.
- `Wizard`/`Report`/`Dashboard` perlu `fixed_filters` outlet yang
  di-inject per pengguna — dan itu belum ada mekanisme "current outlet" di
  engine.

### Apa Itu `TenantDecl` — dan kenapa ini penting

`TenantDecl` adalah satu-satunya tempat di spec FormSpec yang **sudah
disiapkan** untuk pertanyaan "apakah entity ini ter-scope?".

**Bentuknya** (`pkg/spec/entity.go`) — sebuah field pada `EntitySpec`:

```go
Tenant *TenantDecl `yaml:"tenant,omitempty" json:"tenant,omitempty"`
// TenantDecl{ Isolated bool }
```

**Arti yang dimaksudkan** — dari Foundation Document D23:

> Setiap Entity **tenant-isolated by default**, tanpa kecuali dan tanpa
derivasi implisit. Global/reference adalah pengecualian eksplisit:
> `tenant.isolated: false` hanya valid jika `characteristics: [reference]`
> (divalidasi).

Jadi di YAML seharusnya terlihat seperti `tenant: { isolated: false }` — untuk
data acuan bersama (provinsi, tarif pajak, chart of accounts) yang **tidak**
milik satu tenant. Default semua entity: terisolasi.

**Statusnya: dorman.** Dokumen arsitektur FormSpec menyatakannya terbuka:

> `pkg/spec/entity.go` already has a `TenantDecl{Isolated bool}` struct sitting
> on `EntitySpec.Tenant` ... but it has **zero consumers** anywhere in
> `internal/db` or `internal/entity` (confirmed by grep). It's an
> **aspirational field the engine never wired up**.

Artinya: menulis `tenant: { isolated: false }` hari ini **tidak melakukan
apa-apa** — engine mengabaikannya, dan validasi "hanya untuk `reference`" belum
ada.

**Kenapa ini pintu masuk yang tepat untuk cabang.** Dokumen arsitektur FormSpec
sendiri menunjuk ke sana:

> a future framework-level mechanism has **one consistent field to adopt** —
> most naturally by finally wiring up the dormant `TenantDecl`, rather than
> inventing a second, parallel mechanism.

**Tapi ada nuansa penting:** `TenantDecl.Isolated` hanyalah **flag boolean**
("terisolasi atau tidak"), bukan **deskriptor dimensi** (field mana yang
membawa scope). Untuk mendukung cabang, ia perlu **diperluas** dulu — mis.
menjadi `{ isolated, dimension, field }` — baru disambungkan. "Bangunkan
`TenantDecl`" berarti **perluas + sambungkan**, bukan sekadar sambungkan.

### Rekomendasi Bentuk Ideal

**Cabang = atribut bisnis dengan penanda `scope` deklaratif. Bukan auto-inject
seperti `tenant_id`.**

Alasan intinya: **`tenant_id` adalah batas isolasi; `branch_id` adalah
kebijakan visibilitas.** Dua hal itu tidak sekelas.

| | `tenant_id` | `branch_id` |
| --- | --- | --- |
| Sifat | Batas **isolasi** (storage boundary) | Kebijakan **visibilitas** (policy) |
| Nilainya untuk siapa | **Sama untuk semua** — setiap baris milik satu tenant | **Berbeda per pengguna** — kasir 1 cabang, pemilik semua |
| Universal? | Ya, **setiap** baris pasti punya | **Tidak** — banyak entity tidak punya |
| Kalau salah | Kebocoran antar-tenant (fatal) | Salah filter (mengganggu, bisa diperbaiki) |
| Perlakuan wajar | **Default-on** (fail-safe) | **Opt-in eksplisit** |

Boundary keamanan wajar default-on; aturan visibilitas yang nilainya berbeda
per role wajar eksplisit — karena "benar" itu sendiri berbeda per pemanggil.

**Kenapa bukan auto-inject — cabang bukan universal:**

| Kasus | Kenapa satu `branch_id` tidak bisa |
| --- | --- |
| Entity `branch` itu sendiri | Self-reference — `branch_id` = dirinya sendiri? |
| Data acuan global: provinsi, tarif pajak, COA, mata uang | Tidak milik cabang mana pun |
| Entity platform: `formspec.core.user`, role, notifikasi | Bukan domain cabang |
| Transfer antar cabang | Butuh **dua**: `from_branch_id` + `to_branch_id` |
| Member/supplier/promo lintas cabang | Satu FK tidak bisa bilang "berlaku di 3 cabang" |
| Karyawan bertugas di 2 cabang | Many-to-many, bukan FK tunggal |
| Ringkasan konsolidasi perusahaan | Sengaja **melampaui** cabang |

Kolomnya jadi *nullable*, dan begitu nullable, filter otomatis jadi ambigu:
`NULL` itu "terlihat semua orang" atau "tidak terlihat siapa pun"? Keduanya
pilihan buruk — dan yang kedua membuat laporan konsolidasi hilang **diam-diam**.
Ini pola gagal senyap yang sama dengan temuan lain di `gaps_found/`. `tenant_id`
tidak pernah ambigu seperti ini: tidak pernah many-to-many, tidak pernah
hierarkis.

### Tiga Tingkat Perbaikan (jangan langsung dua)

| Tingkat | Usulan | Kenapa |
| --- | --- | --- |
| **1** | Perluas + sambungkan `TenantDecl` menjadi deklarasi scope; tambah validasi "hanya untuk `reference`" | Murah, langsung berguna (`scope_field` natural key jadi menemukannya generik), tidak mengunci desain |
| **2** | Penugasan pengguna↔cabang + enforcement server-side (row filtering otomatis) | **Inilah yang benar-benar memberi isolasi.** Tanpa "pengguna X di cabang mana", filter tidak punya nilai sumber |
| **3** | Permission berdimensi lokasi + hierarki (`parent_id`) | "Manajer region melihat semua cabang di bawahnya" tidak bisa diungkapkan `branch_id` datar |

**Bentuk deklarasi yang diusulkan** (memperluas `tenant`):

```yaml
spec:
  version: v1
  scope: { dimension: branch, field: branch_id, required: true }
```

**Prasyarat sebenarnya bukan kolomnya.** FormSpec sendiri mengakui:

> building real framework-level branch scoping now would be invasive engine
> surgery for a guarantee nothing currently needs — there is **no
> permission/auth enforcement at all yet to hook it into**.

Jadi urutan yang disarankan: **(1) dulu** — murah dan tidak mengunci apa pun;
**(2) sesudahnya** — begitu ada model izin untuk dilekati. Untuk kafe,
"guarantee" isolasi cabang **memang dibutuhkan**, jadi ini kandidat kuat untuk
dipromosikan dari aspirational ke wired.

---

## Gap #9 — `scope_field` tidak sampai ke `ctx.next_key()` ✅ Pasti

### Bukti

`docs/architecture/07-vertical-modules.md` §6 — `natural_key_rule.scope_field`
menangani penomoran **per-scope** (mis. nomor pesanan per outlet):

> `pkg/spec/entity.go`'s `NaturalKeyRuleDecl` gained `ScopeField string`
> (`scope_field` in YAML) — names a field on the same entity (e.g. `branch_id`)
> whose value becomes the counter's scope.

Tapi ada limitasi eksplisit:

> **Known limitation, not fixed here:** `internal/entity/registry.go`'s
> `GenerateNaturalKey` (the backing for an explicit `ctx.next_key(field)` call
> from a script) has **no resource data in scope** to resolve `scope_field`
> from — it only ever sees `tenantID, module, name, fieldName`. ...
> **`ctx.next_key()` always uses the tenant-wide scope regardless of
> `scope_field` today**; the automatic on-create path (what document numbering
> actually needs) is what's fixed.

### Dampak ke aplikasi kafe: **HIGH**

- ✅ **Jalur otomatis aman**: kalau `order.number` diisi otomatis saat create,
  penomoran per outlet **berfungsi**.
- ❌ **Jalur script rusak**: kalau nomor pesanan dibuat di dalam action/script
  (`ctx.next_key("number")` — pola yang dipakai `verticals/billing` di
  `order_checkout.star`), maka **scope diabaikan** dan counter menjadi
  tenant-wide. Dua outlet akan **berbagi satu deret nomor**.

Untuk kafe, nomor pesanan per outlet itu penting (kasir dan pelanggan
menyebutkan nomor; nomor tidak boleh bertabrakan antar cabang). Jadi jalur
script tidak boleh dipakai untuk nomor pesanan sampai ini diperbaiki.

### Usulan

- Threading resource data ke `GenerateNaturalKey` adalah pekerjaan yang sudah
  dipetakan sendiri oleh FormSpec ("*would mean threading resource data through
  `internal/action/dispatcher.go`, `internal/starlark/executor.go`, and
  `resource/formspec.go`'s `NextKeyHandler` chain*"). Kandidat baik untuk
  dikerjakan karena kecil dan berdampak jelas.
- Sementara: **pakai jalur otomatis** (jangan `ctx.next_key` dari script) untuk
  penomoran per-cabang, dan jangan lupa set `scope_field: branch_id`.

---

## Ringkasan Praktis untuk Kafe Multi-Outlet

| Kebutuhan | Bisa hari ini? | Cara |
| --- | --- | --- |
| Entitas cabang | ✅ | vertical `company` → `branch`, atau entity `branch` sendiri (konvensi nama) |
| Field cabang di transaksi | ✅ | relasi `belongs_to` manual di tiap entity (13 entity di kafe) |
| Nomor pesanan per cabang | ✅ (jalur otomatis) | `natural_key_rule.scope_field: branch_id` |
| Nomor pesanan per cabang via script | ❌ | counter jadi global — Gap #9 |
| Stok per cabang | ✅ | `stock-level` dengan dimensi `branch_id` |
| Kasir hanya lihat cabangnya | ⚠️ UI-only | `fixed_filters` (bukan otorisasi) |
| Pemilik lihat semua cabang | ✅ | dashboard lintas entity/module |
| Isolasi data per cabang dijamin engine | ❌ | Gap #8 — tidak ada row-level scope |
| Penanda scope deklaratif (bentuk ideal) | ❌ | Gap #8 — `TenantDecl` dorman; lihat "Rekomendasi Bentuk Ideal" |
