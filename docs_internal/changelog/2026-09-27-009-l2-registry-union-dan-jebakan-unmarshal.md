# L2: registry action = `actions` ∪ `via` (+ jebakan unmarshal ditutup sebagai kelas)

**Plan:** `docs_internal/plan/via-sebagai-action-penuh.md` (L2)
**Ledger:** kafe 10.47 ⏸️, 10.48 ⏸️ (tidak berubah) · temuan baru: guard kelas
`UnmarshalYAML`

## L2 — satu lookup, dua sumber

`Registry.GetActionSpec` (`internal/entity/registry.go`) kini menelusuri
**gabungan** `actions:` dan `state_machine.transitions[].via`.

Kenapa: `via` **adalah** action — ia menamai apa yang dipanggil, dan jalur yang
menerapkan transisinya (`PATCH`) sudah menerimanya. Membaca `actions:` saja
menjadikan `via` kelas dua: tidak bisa di-dispatch, tidak bisa dapat route, dan
harus diduplikasi sebagai entri action supaya bekerja sama sekali.

Aturan yang dipakai:

- **Action yang dideklarasikan menang.** Transisi hanya **menambah** nama.
- **Action sintetis tidak membawa `impl`** — jadi tidak pernah diam-diam
  mendapat route; ia hanya menjadi _resolvable_.
- **Gate tidak disalin.** `require_permission` transisi sengaja **tidak**
  ditaruh di `RequiredPermission` action sintetis: keduanya dibaca lapisan
  berbeda (generator route menurunkan `{module}.{plural}.{name}`; jalur PATCH
  menegakkan gate transisi), dan menyalinnya akan membuat
  `spec.ValidateTransitionPermissions` melihat permission yang sama dideklarasikan
  dua kali — bentuk yang justru ditolaknya sebagai dua sumber yang bisa
  menyimpang.
- **Nama kosong tidak resolve.** Transisi tanpa `via` (keputusan C) tidak boleh
  cocok dengan nama action `""`.

**Belum termasuk (langkah lanjutan, bukan celah senyap):** transisi belum
didaftarkan sebagai permission yang bisa di-grant (L5), dan generator route belum
membaca gabungan ini (L3).

## Temuan dari test: guard `t.Action != ""`

Test yang saya tulis **langsung menangkap bug** pada versi pertama L2: tanpa guard
`t.Action != ""`, transisi **tanpa `via`** cocok dengan nama kosong dan
mengembalikan action hantu bernama `""` — lengkap dengan objek `Action` kosong.
Ini penting karena keputusan C membuat transisi tanpa `via` **sah**, jadi jalur
ini pasti terlewati.

**Dibuktikan:** guard dihapus → test gagal (`empty name must not resolve, got
&{Name: ...}`); dipulihkan → hijau.

## Jebakan `UnmarshalYAML` ditutup sebagai KELAS

Jebakan yang sudah tiga kali menggigit (`emit`, lalu `require_permission`, lalu
`description`): field baru pada `TransitionDecl` **hilang diam-diam saat load**
karena `UnmarshalYAML` memakai struct lokal yang harus menyebut tiap field —
sementara `formspec validate` membaca **file** dan tetap hijau. Kombinasi
terburuk: manifest benar, validasi benar, engine kosong.

Pendekatan "ingat menambah di dua tempat" jelas tidak bekerja — tiga kali gagal.
Sekarang unmarshaler **menyematkan `TransitionDecl` dengan `,inline`**, sehingga
field baru **ikut otomatis**. (Alias `action:` tetap ditangani terpisah karena
`via` sudah terikat oleh struct yang disematkan.)

**Guard kelas:** `TestTransitionDecl_UnmarshalCarriesEveryField` memuat nilai
untuk **setiap** field YAML. Dibuktikan gagal saat penyematan diganti gaya lama
yang membuang `description` (`description dropped: ""`). Tiga test per-field lama
(`KeepsEmit`, `KeepsRequirePermission`, `KeepsDescription`) tetap dipertahankan
sebagai jaring tambahan, plus dua test baru untuk alias `action:` (tetap bekerja;
`via` menang bila keduanya ada).

## File

`internal/entity/registry.go` · `internal/entity/action_union_test.go` (baru) ·
`pkg/spec/entity.go` · `pkg/spec/transition_permission_test.go`

## Bukti

`go test ./...` **hijau** · `gofmt` bersih · `go build ./...` · vitest **529
lulus** / 39 file · `tsc -b` bersih · kafe `validate` 85 manifest 0 problem ·
`make generate-schema` + `generate-kind-docs` dijalankan.

## Sisa

- **L3 ⏸️** — generator route membaca gabungan ini (transisi ber-`impl` dapat
  route dari `via`).
- **L4 ⏸️** — validator anti-duplikat (`actions[].name == via` ditolak) + migrasi
  **85** deklarasi duplikat (sebagian membawa `impl`, jadi L3 harus dulu).
- **L5 ⏸️** — footprint/grant editor membaca gabungan → grant editor menawarkan
  `via`; **10.47 hilang sendiri**.
- **L7 ⏸️** — docs (`docs/kind/data/Entity.md`, `docs/spec/backend/01-core-basic.md`
  §1.6, `02-core-extended.md` §2) + rebuild `buildTransitionIndex`.
- Tidak berubah: 10.46 ⏸️, 10.48 ⏸️, 10.42 ⏸️, 10.40b ⏸️, 10.41 ⏸️.
