# 3.8 — Konteks sesi: (principal, role, cabang)

**Tanggal:** 2026-09-20 · **TODO:** `examples/kafe/gaps_found/TODO.md` 3.8 · **Plan:** `docs_internal/plan/session-context-role-branch.md`

## Model yang diimplementasikan

Sesi selalu **spesifik**: siapa, sebagai **role apa**, di **cabang mana** — satu
baris `(role, dimension, value)`, bukan "daftar role × daftar cabang" yang bisa
dikombinasikan bebas.

```yaml
# formspec.core.user.spec.assignments
- { role: sales, dimension: branch_id, value: <id cabang A> }
- { role: admin, dimension: branch_id, value: <id cabang B> }
```

`dimension` adalah **nama field** yang dibandingkan `row_scope.field`
(`branch_id`) — bukan nama dimensi dekoratif (`scope.dimension: branch`), karena
`row_scope: {from: session}` membaca atribut dengan kunci itu.

## Apa yang dikerjakan

| Lapis | Perubahan |
| --- | --- |
| Data | `formspec.core.user.assignments` (json list, 3 bagian required); `formspec.core.session` + `role`/`scope_dimension`/`scope_value`; `auth.Assignment{Role, Dimension, Value}` + `User.Context` (transient) |
| Login | `LoginWithContext(..., assignmentID)`; aturan: **0** assignment → perilaku lama (union role, tanpa boundary), **1** → otomatis, **>1** tanpa pilihan → **409 `CONTEXT_REQUIRED` + choices**, id tak dikenal/dicabut → 409 juga (fail closed) |
| Token | klaim baru `role` (tunggal) + `attrs` (dimension→value); `JWTValidator` memakai `role` dan **mengabaikan** `roles` bila keduanya ada, sehingga daftar role basi tidak bisa melebarkan sesi |
| Permission | materialisasi memakai **role terpilih saja**: `issuePair` mengganti `User.Roles` dengan `[Context.Role]` selama resolusi (satu tempat yang memutuskan "role mana yang berlaku", bukan jalur resolver kedua) |
| Refresh | sesi ber-konteks divalidasi ulang (`contextStillValid`: assignment masih ada **dan** role masih ada) → gagal = **409 minta pilih ulang**, bukan lanjut dengan boundary lama |
| Switch | `POST /_ui/auth/switch` (body `refresh_token` + `assignment`): sesi lama **di-revoke**, pair baru diterbitkan — tidak pernah ada dua konteks hidup |
| API | `POST /_ui/auth/login` menerima `assignment`; respons 409 berisi `choices[{id, role, dimension, value}]` (id = `<role>@<value>`) |
| Admin | `EntityUserStore.SetAssignments` (mencabut assignment tidak perlu berburu sesi terbuka — refresh berikutnya yang menutupnya) |

**OAuth**: tidak punya langkah memilih, jadi aturan otomatis berlaku di
`issuePair` (0 → tanpa boundary, 1 → dipakai, >1 → minta pilih). Pilihan terakhir
per-device (`localStorage`) adalah pekerjaan klien dan belum dikerjakan —
lihat "Sisa".

## Bukti

**Test** (`internal/auth/context_test.go`, 11 kasus):

| Test | Isi |
| --- | --- |
| `TestAssignment_Identity` | id `role@value` (value boleh berisi `@`), attrs, aturan lengkap/tidak lengkap |
| `TestResolveAssignment` | 0/1/banyak/id eksplisit/id dicabut/baris tidak lengkap |
| `TestService_LoginRequiresContextChoice` | dua konteks → `ContextRequiredError` + 2 choices |
| `TestService_LoginWithChosenContextScopesRoleAndAttrs` | klaim `roles=[sales]` (bukan union), `attrs.branch_id=A`; konteks lain → `[admin]`/B |
| `TestService_RefreshFailsClosedWhenAssignmentRevoked` | assignment dicabut → refresh = `ContextRequiredError` |
| `TestService_SwitchContext` | switch → role/attrs berubah, refresh token lama ditolak, konteks tak dikenal → gagal |
| `TestService_LoginWithoutAssignmentsKeepsLegacyBehavior` | akun tanpa assignment tetap union + tanpa attrs (aditif) |

**E2E** (dev server kafe `:8099`, user `kasir2` dengan `sales@cabangA` +
`admin@cabangB`, 2 cabang nyata):

| Permintaan | Hasil |
| --- | --- |
| `POST /_ui/auth/login` tanpa `assignment` | **409** `CONTEXT_REQUIRED` + choices `sales@…`, `admin@…` |
| login `assignment: sales@<A>` | **200**; klaim: `role=sales`, `roles=["sales"]` (bukan `["sales","admin"]`), `attrs={branch_id: A}` |
| `GET /_ui/entity/cafe-order/order` dengan token itu | `total: 2`, **hanya** baris cabang A |
| login `assignment: admin@<B>` → list yang sama | `total: 0` (tidak ada baris cabang A yang bocor) |

Suite: `go test ./...` hijau · `make lint` **0 issues** · kafe `validate` **0 problem**.

## Temuan E2E (bukan bagian 3.8, dicatat)

Membuat order di cabang **kedua** pada DB kafe yang sudah ada gagal:

```
constraint failed: UNIQUE constraint failed: cafe_order_orders.tenant_id, cafe_order_orders._number
```

Akarnya bukan kode hari ini: index di DB lama masih berbentuk
`(tenant_id, _number)`, sedangkan 3.6 mengubahnya menjadi
`(tenant_id, _branch_id, _number)` untuk counter per cabang. **Schema sync tidak
merekonsiliasi index yang definisinya berubah** pada tabel yang sudah ada (ia
hanya membuat index yang belum ada). Pada DB segar gejalanya tidak muncul —
karena itu 3.6 lolos. Dicatat sebagai item **3.9** di ledger.

## Sisa (jujur)

- **Tahap 4 plan (UI) belum**: layar pemilih konteks + pengalih di header +
  penyimpanan pilihan terakhir di `localStorage`. Sampai itu ada, klien harus
  mengirim `assignment` sendiri; respons 409 sudah membawa `choices` yang
  dibutuhkan layar itu.
- **Adopsi kafe (tahap 5) sebagian**: role kafe belum punya grant tersimpan,
  jadi bukti E2E memakai permission langsung pada user; pemetaan
  `employee.assignments` → `user.assignments` untuk data nyata belum ditulis
  (itu pekerjaan data/seed, bukan bahasa spec).
- OAuth memakai aturan otomatis; pilihan terakhir per-device menyusul bersama UI.
