# Plan alur sesi meja kafe + temuan blocker (10.37–10.39, 10.35a)

**Tipe:** plan + pencatatan temuan. **Tidak ada perubahan kode** pada perubahan
ini — plan ini menandai apa yang **harus** dibangun sebelum alur sesi meja bisa
dinyatakan.

**Pemicu:** pemilik menetapkan alur pesan-via-meja (scan QR statis → pilih+bayar
→ occupied → dapur selesai → kasir clear meja → available), dengan PIN 4 digit
untuk tamu berbeda yang scan saat meja occupied.

## Yang diputuskan (masukan pemilik)

- **QR statis per meja** + **PIN dinamis** (bukan pemilih meja).
- **Meja occupied = ADA SESI TERBUKA.** Pembayaran tidak mengubah status meja;
  "available" ⇔ tidak ada `table-session` ber-status `open`. Ini juga menghindari
  state duplikat yang bisa drift, dan penting karena v1 **tidak punya** timer
  (`Schedule`/`Cron` tidak ada di `KnownKinds`; `durable` sudah dicabut) —
  jadi status meja **tidak mungkin** di-update otomatis saat bayar.
- Nomor 3 & 5 alur tidak otomatis: yang bisa adalah order `paid` → `served`
  (event/transition di `order`), dan clear meja manual oleh kasir.

## Blocker yang ditemukan (terukur)

| Item       | Temuan                                                                                                                                               | Bukti                                                                                                   |
| ---------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------- |
| **10.35a** | `close-session` **tanpa `impl:`** → **tidak ada route**; tidak ada role yang memegang `close-session`/`abandon`; `manajer` pun 403 di `PATCH status` | `POST .../close-session` → 404; `PATCH` → 403; `/_meta/me` manajer → 0 permission; grep seed → 0        |
| **10.37**  | `FormSubmit.Redirect` **field mati** di renderer                                                                                                     | grep `redirect` di `src/` → hanya redirect auth; `manifest.ts:900` ada, 0 pembaca                       |
| **10.38**  | **Tidak ada** random/nonce/PIN; **tidak ada** hashing di jalur tulis                                                                                 | `grep rand` di `internal/starlark pkg/spec` → 0; `masked:` menyamarkan respons, bukan menghash simpanan |
| **10.39**  | `kind: Service` **tidak bisa publik**                                                                                                                | `/_ui/service` = session-authenticated (`internal/api/router.go:558`); `ServiceSpec` tanpa `public`     |

**Koreksi penting (10.34b):** find-by-`natural_key` **bukan** blocker — sudah
tersedia dan terverifikasi (`GET .../menu-item/KPI-001` anonim → **200**;
`EntityStore.GetByID` fallback `WHERE _<field> = ?`). Jadi membuka sesi dari QR
cukup menandai `dining-table.qr_token` sebagai `natural_key`. Usulan
"hidden index marker" yang sempat muncul di 10.34 **tidak diperlukan**.

## Yang sudah siap dipakai (tanpa perubahan framework)

- `table-session` sudah membawa `dining_table_id` top-level + `guest_token` →
  `public_entities[].scope` sudah bekerja (`list order` anonim tanpa token →
  **403**, terukur).
- Menambah pesanan pada sesi yang sama **sudah bisa**: `order-form-qr` sudah
  mengisi `table_session_id`/`guest_token`/`branch_id` dari konteks sesi.
- Unique parsial (`where:`) sudah dipakai `shift` → pola siap untuk
  "satu sesi terbuka per meja" (10.34c).
- `rate_limit` sudah ada di `pkg/spec` (0 manifest kafe memakainya → 10.36).

## Urutan kerja

1. **10.34c** unique parsial satu-sesi-terbuka-per-meja (small)
2. **10.36** `rate_limit` pada `table-session` + `order` (small)
3. **10.35a** `impl` + grant `close-session`/`abandon` — **prioritas**: tanpa ini
   meja yang terisi tidak bisa dibersihkan siapa pun (small)
4. **10.37** `submit.redirect` + `{id}` hidup (small) **atau** halaman custom (medium)
5. **10.38** module native Go: nonce + PIN + hash + verify (medium)
6. **10.39** Service publik untuk `table-status` + `verify-pin` (medium)
7. **10.20** landing page (small, setelah halaman masuk ada)

1–3 bisa dikerjakan sekarang tanpa menyentuh `pkg/spec`. 5–6 menyentuh
`pkg/spec` → wajib `make generate-schema` + `make generate-kind-docs`.

## Catatan keamanan

PIN 4 digit = 10.000 kemungkinan. Tanpa **10.36** (rate limit) + kunci setelah N
kegagalan, PIN memberi rasa aman palsu. PIN juga **tidak boleh** plaintext dan
**tidak boleh** muncul di respons API mana pun.

**Plan lengkap:** `docs_internal/plan/kafe-qr-table-session-flow.md`
**Ledger:** kafe `10.34`, `10.34c`, `10.35`, `10.35a`, `10.36`, `10.37`,
`10.38`, `10.39`, `10.20` (semua `⏸️` kecuali yang disebut).
