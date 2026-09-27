# 10.48: klien tidak lagi menebak jalur transisi (`has_route`)

**Ledger:** kafe **10.48 ✅** · sisa → **10.48a ⏸️**
**Terkait:** 10.50 (klik 404), plan `docs_internal/plan/via-sebagai-action-penuh.md`

## Masalah

Bentuk sementara dari 10.50: klien **menembak `POST /{entity}/{id}/{action}`
dulu**, menerima **404**, lalu jatuh ke `PATCH`. Bekerja — tetapi:

- satu **round-trip terbuang** per transisi tanpa `impl`,
- **404 tercetak di konsol** untuk jalur yang sepenuhnya normal (terlihat seperti
  error padahal bukan),
- klien menyimpulkan **properti statis manifest** dari kode status HTTP.

## Fix

**`ActionSummary.HasRoute`** (`internal/ui/meta.go`), diturunkan
`!Disabled && Impl != nil` — **kondisi yang identik** dengan generator route
(`internal/api/generator.go`, yang melewati `Disabled || Impl == nil`), supaya
bundle tidak pernah menjanjikan route yang tidak dilayani server.

`DetailPage` memilih jalur dari bundle:

```
declared?.has_route ?? (declared !== undefined)
```

Menebak hanya dipakai bila field **absen** (server lama) — dinyatakan eksplisit
di kode, bukan kebetulan.

## Jebakan yang menangkap diri sendiri

Versi pertama memakai `json:"has_route,omitempty"`. Untuk `bool`, `omitempty`
membuang **`false`**, sehingga dua keadaan menyatu menjadi _absen_:

| Keadaan nyata                               | Wire  |
| ------------------------------------------- | ----- |
| action **tanpa** route (`has_route: false`) | absen |
| server lama yang **tidak punya** field ini  | absen |

Klien lalu kembali menebak **persis** untuk action yang flag-nya ada untuk
menyelesaikan itu. **Terukur:** setelah versi pertama, klik masih `POST` → **404
masih muncul**; konsol browser memperlihatkan 404 yang sama.

Setelah `omitempty` dibuang, `has_route=false` **terkirim**, dan jalur menebak
tidak lagi terpakai.

**Pelajaran:** `omitempty` pada `bool` menjadikan "false" dan "tidak ada" tidak
bisa dibedakan — fatal untuk field yang justru ada untuk membedakan keduanya.
(Sama untuk pointer `*bool` bila nilai-nilai penting; di sini solusinya adalah
mengirim eksplisit.)

## Bukti

**Bundle (terukur):** seluruh 7 action meja melaporkan `has_route=False`.

**Browser (terukur, verifikasi akhir):**
klik "Tandai meja dipesan" → toast **"State updated"**, `Status Meja` →
**`Reserved`**, `Version` 7 → 8, dan **`errorResponses: []`** (nol respons
4xx/5xx pada `/_ui/entity/*`). Sebelumnya daftar itu berisi
`404 …/dining-table/{id}/reserve`.

**Guard:**

- Go: `TestBuildEntitySchema_ActionHasRoute` — tiga kasus (tanpa `impl`,
  ber-`impl`, `Disabled` menang atas `impl`).
- Klien: `__transitionRoute.test.ts` (5 test) — mem-pin **aturan pemilihan
  jalur**, termasuk fallback server lama.

`go test ./...` hijau · vitest **534 lulus** / 40 file · `tsc -b` bersih ·
`gofmt` bersih · kafe `validate` 85 manifest 0 problem.

## File

`internal/ui/meta.go` · `internal/ui/authorized_actions_test.go` ·
`renderers/react-shadcn/src/kinds/page/DetailPage.tsx` (helper `errorStatus`
dihapus — tidak lagi dipakai) · `renderers/react-shadcn/src/types/manifest.ts` ·
`renderers/react-shadcn/src/kinds/page/__transitionRoute.test.ts` (baru)

## Sisa

- **10.48a ⏸️** — jalur fallback "coba POST lalu fallback PATCH" **sengaja
  dipertahankan** untuk bundle tanpa `has_route` (server lama), jadi kode
  menebak itu masih hidup — hanya tidak terpakai pada bundle saat ini. Bila
  kompatibilitas server lama tidak lagi diperlukan, hapus jalur itu.
- Tidak berubah: 10.46 ⏸️, 10.47 ⏸️, 10.42 ⏸️, 10.40b ⏸️, 10.41 ⏸️ · L3/L4/L5/L7
  dari plan `via`-sebagai-action-penuh.
