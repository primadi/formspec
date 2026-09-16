# Kontrak REST Surface `/_ui/`

**Status:** Draft · **Kontrak:** [`../spec/backend/01-core-basic.md`](../spec/backend/01-core-basic.md) §8.1

Surface `/_ui/` adalah **kontrak publik** untuk setiap klien FormSpec — SPA
bawaan pun memakainya. Halaman ini menjelaskannya sekali, supaya klien baru tidak
menemukannya dengan trial-and-error (gap #47).

Untuk kontrak satu entity, **jangan menyalin dari sini** — cetak dari sumber yang
sama dengan yang mendaftarkan route:

```bash
formspec describe entity order --spec examples/kafe/spec
```

Perintah itu memakai generator route yang sama dengan server
(`api.UIRoutesForEntity`), jadi ia tidak bisa menyebut endpoint yang tidak ada
atau melewatkan yang ada.

## 1. Bentuk path

```
/{workspace}/_ui/entity/{module}/{entity}[/{id}][/{field}|/{action}]
```

- `{entity}` adalah `metadata.name` (singular) — **bukan** `plural`. Plural hanya
  dipakai di nama permission dan URL `api/v1`.
- Semua endpoint entity berada di bawah prefix workspace. Workspace dapat berupa
  slug (`kafe`) atau UUID.

| Method       | Path                                                       | Aksi                        | Permission                   |
| ------------ | ---------------------------------------------------------- | --------------------------- | ---------------------------- |
| `GET`        | `…/{module}/{entity}`                                      | `list`                      | `{module}.{plural}.list`     |
| `GET`        | `…/{module}/{entity}/{id}`                                 | `find`                      | `{module}.{plural}.view`     |
| `POST`       | `…/{module}/{entity}`                                      | `create`                    | `{module}.{plural}.create`   |
| `PATCH`      | `…/{module}/{entity}/{id}`                                 | `update`                    | `{module}.{plural}.update`   |
| `DELETE`     | `…/{module}/{entity}/{id}`                                 | `delete`                    | `{module}.{plural}.delete`   |
| `POST`       | `…/{module}/{entity}/{id}/submit` (juga `cancel`, `amend`) | lifecycle                   | `{module}.{plural}.{action}` |
| `POST`       | `…/{module}/{entity}/{id}/{action}`                        | custom action ber-`impl`    | `{module}.{plural}.{action}` |
| `POST`/`GET` | `…/{module}/{entity}/{id}/{field}`                         | upload / unduh field `file` | `update` / `view`            |

Aturan yang sering mengejutkan, dan **bukan** detail implementasi:

- **Route hanya ada kalau spec-nya memang menghasilkan aksi itu.** Aksi
  `disabled: true` tidak punya route (dan tidak punya permission terdaftar);
  entity `lifecycle-free` tidak punya `submit`/`cancel`/`amend`; entity
  `characteristic: summary` hanya `list` + `find`.
- **Transisi state machine tanpa `impl` tidak punya endpoint sendiri.** Ia
  diterapkan lewat `update`; guard transisinya yang memvalidasi perpindahan.
  Hanya aksi dengan `impl` yang punya route `{id}/{action}`.
- Route `{id}/{field}` hanya melayani field bertipe `file`/`attachment`; segmen
  lain di posisi itu menjawab `404`, bukan `403` ([#52]).

## 2. Request body

**Body bersifat flat — envelope ditolak.**

```jsonc
// create / update / action
{ "number": "ORD-2026-00001", "channel": "qr_table" }   // ✅
{ "data": { "number": "..." } }                          // ❌ 400 unknown field: "data"
```

Nilai mengikuti tipe field ([`../spec/backend/05-field-types.md`](../spec/backend/05-field-types.md)):
`money` dikirim sebagai `{"amount": "25000", "currency": "IDR"}` (server
menormalisasi angka/string telanjang, §2), `decimal` sebagai string desimal,
`relation` sebagai id target, `child` sebagai array objek baris.

`update` mengirim **hanya field yang berubah**, dan wajib membawa `version` untuk
optimistic concurrency — mismatch → `409 CONFLICT`.

## 3. Response envelope

```jsonc
// list
{ "data": [ … ],
  "meta": { "page": 1, "per_page": 20, "total": 137, "total_pages": 7 },
  "links": { "first": "…", "last": "…" } }

// satu record (create / find / update)
{ "data": { … }, "meta": { "request_id": "…", "timestamp": "…" } }

// error
{ "error": { "code": "VALIDATION_ERROR", "message": "…", "details": { … } },
  "meta": { "timestamp": "…" } }
```

Kode error: `VALIDATION_ERROR` (422), `UNAUTHORIZED` (401), `FORBIDDEN` (403),
`NOT_FOUND` (404), `CONFLICT` (409), `STATE_TRANSITION_ERROR` (422),
`INTERNAL_ERROR` (500). Daftar lengkap + kode kanonik `FORMSPEC.*`:
[`../spec/backend/error-glossary.yaml`](../spec/backend/error-glossary.yaml).

## 4. Query untuk `list`

```
?page=1&per_page=20&sort=-transaction_date&direction=asc&fields=number,status&search=kopi
?filter[status][eq]=paid&filter[total_amount][gte]=50000
```

- `per_page` default **20**, maksimum **100** (di atas maksimum di-_clamp_, bukan
  ditolak).
- Operator filter: `eq neq gt gte lt lte between in nin like ilike null notnull`.
- `sort` atas field JSONB di-_cast_ ke tipe field, jadi urutannya numerik/temporal
  — bukan leksikografis.
- **Parameter `row_scope` bukan filter.** Kalau sebuah entity mendeklarasikan
  scope `from: route`, parameter itu adalah konteks request: nilainya dipakai
  engine untuk membatasi baris, dan nilainya tidak bisa dilebarkan klien
  ([`../spec/backend/01-core-basic.md`](../spec/backend/01-core-basic.md) §1.7).

## 5. Surface publik

App dengan `access: public` membuka sebagian endpoint untuk anonim, tetapi hanya
pasangan entity+aksi yang didaftarkan di `public_entities`, dan sebuah grant boleh
membatasi **baris**-nya lewat `scope`
([`../spec/frontend/05-app-kinds.md`](../spec/frontend/05-app-kinds.md) §1.1).
Ringkasnya: grant berlaku untuk **anonim**; pemanggil yang sudah terautentikasi
tetap wajib memegang permission entity itu.

## 6. Di luar cakupan halaman ini

Surface eksternal (`/{workspace}/api/v1/…`) punya kontrak terpisah dan
deny-by-default lewat `spec.expose` (§8.2/§8.4). Print, Report, dan dashboard
memakai endpoint-nya sendiri di surface yang sama; kontrak HTTP per entity
dicetak oleh `formspec describe`, sedangkan kontrak kind ada di
[`../kind/`](../kind/).
