# Plan — Konteks sesi: (principal, role, cabang)

**Status:** desain disetujui 2026-09-16 (pemilik proyek), belum diimplementasikan.
**Menggantikan** rencana "daftar nilai + `op: in`" yang tercatat sebagai sisa 3.5.

## Model

Sesi selalu punya boundary yang **spesifik**: siapa, sebagai **role apa**, di
**cabang mana**. Bukan "user punya daftar role + daftar cabang", melainkan **satu
daftar pasangan (role, cabang)**:

```
user: admin
  assignments:
    - { role: sales, dimension: branch, value: cafe-master.branch/B1 }
    - { role: admin, dimension: branch, value: cafe-master.branch/B2 }
```

Login memilih **satu** baris di antara daftar itu. Kalau hanya ada satu, dipilih
otomatis.

## Keputusan yang sudah diambil

| Pertanyaan                     | Jawaban                                                                                            |
| ------------------------------ | -------------------------------------------------------------------------------------------------- |
| Permission bila >1 role        | **Hanya role yang dipilih** — bukan union. Boundary spesifik, audit bersih                         |
| Multi-cabang                   | **Beberapa baris assignment** (role × cabang), bukan satu baris berisi daftar                      |
| Pilihan terakhir (untuk OAuth) | **Di klien, per-device** (`localStorage`); server tidak menyimpan preferensi                       |
| Registrasi                     | `employee`/carrier domain tetap ada untuk data kepegawaian; **assignment** adalah sumber otorisasi |

## Yang berubah dari model sekarang

| Sekarang                                                                         | Sesudah                                                                                              |
| -------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| `formspec.core.user.roles: [a, b]` (daftar nama role)                            | daftar **assignment** `{role, dimension, value}`                                                     |
| Permission = union grant semua role                                              | grant **role terpilih** saja                                                                         |
| `Session{JTI, UserID, WorkspaceID, App, Expiry}`                                 | + `Role`, `ScopeDimension`, `ScopeValue`                                                             |
| Klaim token `roles: [...]`                                                       | `role: <satu>` + `attrs` berisi nilai dimensi terpilih                                               |
| Cabang diselesaikan lewat `assignments` entity (lookup per request, memo 30 dtk) | cabang **dibawa sesi** — tidak perlu lookup; `assignments` jadi _daftar pilihan_, bukan sumber nilai |
| `read_all` untuk pemilik                                                         | tetap — akun tanpa assignment (pemilik/service) tak punya boundary                                   |

Catatan penting: yang terakhir itu **menyederhanakan** 1.8. `row_scope: from
session` tetap fail closed, tetapi atributnya kini datang dari token/sesi, jadi
resolusi `assignments` tidak lagi di jalur kritis setiap request.

## Alur

1. **Login** (`POST /_ui/auth/login`) — body bertambah `assignment` (opsional):
   - 0 assignment → sesi tanpa role/scope (pemilik lewat `read_all`, service account).
   - 1 assignment → dipakai otomatis.
   - \>1 tanpa `assignment` → **belum menerbitkan token**; balas daftar pilihan
     (mis. `409 CONTEXT_REQUIRED` + `choices: [{id, role, dimension, value}]`),
     klien menampilkan pemilih, lalu kirim ulang.
2. **OAuth** — tidak ada langkah memilih, jadi pakai pilihan terakhir dari
   `localStorage`; kalau tidak ada (device baru), perlakukan seperti ">1 tanpa
   pilihan" → arahkan ke pemilih setelah callback.
3. **Pindah konteks** — `POST /_ui/auth/switch` `{assignment}`: menerbitkan token
   pair baru, memperbarui record sesi, dan klien menyimpan pilihan terbaru.
   Sesi lama di-revoke (atau di-update) supaya tidak ada dua konteks hidup.
4. **Assignment dicabut / role dihapus** → token berikutnya gagal validasi
   konteks: fail closed dengan pesan "pilih ulang", bukan diam-diam memakai yang
   lama.

## Yang perlu disentuh

- `internal/auth/user.go` — `User.Roles` → assignment list; `formspec.core.user`
  schema (child `assignments` = `{role, dimension, value}`, cocok dengan `picker` S1).
- `internal/auth/resolver.go` + `materialize.go` — materialisasi dari role terpilih.
- `internal/auth/service.go` — `Login`/`OAuthLogin`/`issuePair` menerima konteks;
  `switch` baru.
- `internal/auth/session.go` — record sesi membawa role + scope.
- `internal/auth/token.go`/`jwt.go` — klaim `role` (tunggal) + `attrs` terisi dari
  pilihan; validator memastikan keduanya konsisten.
- `internal/api/auth_handler.go` — body login, respons `CONTEXT_REQUIRED`, endpoint switch.
- `internal/api/scope.go` — atribut sesi dari klaim (jalur `assignments` turun jadi fallback).
- SPA: layar pemilih setelah login, pengalih di header, `localStorage`.
- Kafe: `employee` + assignment per (role, cabang).

## Tahapan

| #   | Isi                                                        | Ukuran |
| --- | ---------------------------------------------------------- | ------ |
| 1   | Schema assignment + materialisasi dari role terpilih       | medium |
| 2   | Login memilih konteks + klaim sesi + `row_scope` dari sesi | medium |
| 3   | OAuth pakai pilihan terakhir (klien) + endpoint switch     | small  |
| 4   | UI pemilih & pengalih                                      | medium |
| 5   | Adopsi kafe + dokumentasi normatif                         | medium |

## Bukti yang harus ada saat selesai

- Kasir dengan dua assignment (sales@A, admin@B): login tanpa memilih → daftar
  pilihan; memilih sales@A → hanya data cabang A **dan** hanya permission role
  sales; memilih admin@B → kebalikannya.
- Pindah konteks tanpa logout → token baru, akses berubah, audit mencatat.
- OAuth di device yang sudah pernah memilih → langsung memakai pilihan itu.
- Akun tanpa assignment (pemilik) → tetap bisa lintas cabang lewat `read_all`.
- Assignment dicabut → 401/409 + diminta memilih, bukan lanjut dengan konteks lama.
