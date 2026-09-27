# Plan: Vite dev proxy sadar multi-workspace (`--dev-ui`)

**Pemicu:** pemilik melaporkan `http://localhost:5174/kafe` error.
**Terkait:** todo `2.11.7` (dev-ui proxy multi-workspace), `docs_internal/plan/named-workspaces.md`.

## Gejala (terukur)

Buka `http://localhost:5174/kafe` (port forward dari Vite `:5173`) → halaman
berhenti di "Loading...", lalu hanya teks kecil:

```
Unexpected token '<', "<!doctype "... is not valid JSON
```

Terukur lewat probe di browser (page `efbaa1f5`, viewport ter-forward):

| Permintaan                    | Status | Content-Type       | Isi                |
| ----------------------------- | ------ | ------------------ | ------------------ |
| `GET /kafe/_ui/_meta/apps`    | 200    | `text/html`        | `<!doctype html>…` |
| `GET /default/_ui/_meta/apps` | 200    | `application/json` | envelope meta      |

Jadi SPA menerima **index.html** padahal meminta JSON → `fetchMetaApps` gagal
parse → `RootSurface` menampilkan `err.message` mentah.

## Akar masalah

Ada **dua** proxy, dan item 2.11.7 hanya memperbaiki satu:

1. **Go `viteSPAProxy`** (`cmd/formspec/dev.go:635` `isWorkspaceAPIPath`) —
   sudah benar: ia meneruskan `/{ws}/_ui|api/` untuk **semua** slug workspace
   (diperbaiki 2.11.7, changelog `2026-09-07-004`).
2. **Vite `server.proxy`** (`renderers/react-shadcn/vite.config.ts:36,44`) —
   masih **literal** `"/default/api/v1"` + `"/default/_ui/"`. Slug apa pun
   selain `default` gagal cocok → Vite menjawab dengan SPA (`index.html`).

Alur `formspec dev --dev-ui` menjalankan **Vite langsung** (bukan lewat Go:
`:8080`), jadi proxy Go tidak menolong; hanya `server.proxy` yang melihat
permintaan. `docs/guides/how-to-run.md` §2 memang mendokumentasikan Vite
sebagai cara jalan dev.

Kelas bug: kontrak "prefix API per-workspace" ditegakkan di **dua** implementasi
proxy, dan hanya satu yang diuji. Sama bentuknya dengan 5.11.7/5.14.6 (aturan
diduplikasi antar-implementasi) — tetapi lebih murah ditutup karena kita bisa
menulis guard yang memakai **config nyata**.

## Fix

Ganti dua key literal dengan **key RegExp** (fitur Vite: key yang diawali `^`
diperlakukan sebagai `RegExp`), mengikuti predikat Go:

```
^/[a-z0-9-]+/_ui/     → http://localhost:8080  (ws: true)
^/[a-z0-9-]+/api/v1   → http://localhost:8080  (ws: true)
```

`[a-z0-9-]+` = charset slug workspace (`^[a-z0-9]+(-[a-z0-9]+)*$`,
`pkg/spec/workspace.go`). Ancor `^` membuat `_ui`/`api` hanya cocok sebagai
**segmen pertama** setelah slug — sama seperti `isWorkspaceAPIPath` (segmen
kedua dari 3). Karena `^` wajib untuk regresi ini, gunakan `\\^` escape hanya
di literal JSON, bukan di source (repo pakai backslash escape di template).

`/health` tidak ikut (Vite menjawab SPA) — paritas dengan `viteSPAProxy` yang
meneruskan `/health` ke backend. Ini **gap kecil dan sengaja dibiarkan**:
SPA tidak pernah memanggil `/health`, dan menambahkannya berarti key proxy
yang tidak bisa tumpang-tindih dengan slug (karena `health` reserved). Bila
kelak CLI flow membutuhkannya, tambah key `"/health"` sendiri.

### Bukti regex bekerja (dua lapis, dijalankan sebelum menulis kode)

Stub backend (`node:http`, port 18099) + Vite dengan key regex, probe:

| Path                      | Hasil   | Alasan                             |
| ------------------------- | ------- | ---------------------------------- |
| `/kafe/_ui/_meta/apps`    | PROXIED | slug + `_ui`                       |
| `/kafe/_ui/_ws`           | PROXIED | WebSocket path                     |
| `/kafe/api/v1/orders`     | PROXIED | slug + `api/v1`                    |
| `/cafe/_ui/entity/x/y`    | PROXIED | slug lain                          |
| `/kafe/_admin/menu/_ui/x` | SPA     | `_ui` bukan segmen pertama (ancor) |
| `/kafe/app/_ui/x`         | SPA     | `_ui` bukan segmen pertama (ancor) |
| `/kafe/_admin`            | SPA     | SPA route                          |

## Guard (test yang gagal tanpa fix)

File: `renderers/react-shadcn/src/lib/api/devProxyConfig.test.ts`.

Menjalankan **Vite dev server nyata** (`createServer` dari `vite`) di atas
`vite.config.ts` asli, dengan `server.proxy[*].target` diarahkan ke stub backend
yang dinyalakan test itu sendiri pada port ephemeral, lalu memukul path probe
dan meng-assert `PROXIED` vs `SPA`. Ini menguji middleware Vite yang
sesungguhnya (termasuk urutan proxy vs `indexHtmlMiddleware`), bukan tiruan
semantik.

Wajib untuk `ws: true`: WebSocket upgrade di `/kafe/_ui/_ws` harus diteruskan.
`ws` tidak terpasang sebagai dependency, jadi handshake diverifikasi manual di
dev server (bukan di test) — lihat Verifikasi.

**Kalibrasi:** dengan key `/default/...` lama, `/kafe/_ui/_meta/apps` →
`text/html` → test GAGAL. Dibuktikan dengan menyuntikkan regresi ke salinan
config.

## Perubahan file

| File                                                        | Perubahan                                                                   | Effort |
| ----------------------------------------------------------- | --------------------------------------------------------------------------- | ------ |
| `renderers/react-shadcn/vite.config.ts`                     | key proxy → regex; komentar menjelaskan paritas dengan `isWorkspaceAPIPath` | small  |
| `renderers/react-shadcn/src/lib/api/devProxyConfig.test.ts` | **Baru** — guard Vite-real                                                  | small  |
| `docs/guides/how-to-run.md`                                 | snippet `vite.config.ts` diverifikasi ulang + tabel URL multi-workspace     | small  |
| `docs_internal/changelog/2026-09-26-017-*.md`               | catatan perubahan                                                           | small  |
| `docs_internal/plan/todo.md`                                | koreksi klaim 2.11.7 (hanya sisi Go) + item 2.11.7a                         | small  |
| `docs_internal/plan/named-workspaces.md`                    | tambah baris tabel §vite.config                                             | small  |

## Verifikasi

```bash
cd renderers/react-shadcn && npx tsc -b && npx vitest run src/lib/api/devProxyConfig.test.ts
```

Lalu dev server nyata (port forward: Vite :5173 → host 5174):

- [ ] `http://localhost:5174/kafe` render katalog QR (bukan "Loading...").
- [ ] `http://localhost:5174/kafe/app/pos` (private) → layar login app.
- [ ] `http://localhost:5174/default/_admin` → admin panel.
- [ ] Realtime (WS) hidup: `^/[a-z0-9-]+/_ui/` + `ws: true` → handshake
      `/kafe/_ui/_ws` berhasil (tombol/indikator realtime, atau jaringan
      browser menunjukkan `101 Switching Protocols`).

## Bukan bagian plan ini

- `/health` lewat Vite (lihat di atas — sengaja dibiarkan).
- Menggabungkan dua proxy (Go & Vite) menjadi satu sumber kebenaran: pilihan
  arsitektur lebih besar, di luar perbaikan ini.
