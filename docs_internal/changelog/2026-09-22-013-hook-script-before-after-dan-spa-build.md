# 2026-09-22-013 — Hook script (`before`/`after`) + SPA build: dua gap yang lebih luas dari catatannya

**Tanggal:** 2026-09-22 · **Trigger:** "lanjutkan perbaikan untuk bug yg masih
ditemukan" · **Item:** master todo `7.8.9⏸️` dan `5.15.3⏸️` → keduanya **✅**

## A. `resource.save`/`create` — gap hook lebih luas dari dugaan (7.8.9)

Item 7.8.9 ditulis untuk **memeriksa** apakah `resource.save`/`update` punya gap
hook yang sama seperti `resource.create` (7.8.8). Jawabannya ya — dan dua
temuan lagi yang tidak diduga.

### (a) `after` hook hilang di `resource.save` — seperti dugaan

`resource.save` menulis lewat `EntityStore` langsung (INSERT bila handle belum
punya ID, UPDATE bila punya). `EntityStore` tidak menjalankan hook — lapisan API
yang menjalankan. Jadi entitas yang memelihara proyeksi dari `after update`
diam sama seperti pada kasus `after create` di 7.8.8.

### (b) Hook `before` tidak berjalan di SEMUA jalur script — tidak diduga, dan lebih serius

Ini temuan yang paling penting, dan bukan bagian dari item aslinya.

`before` adalah **guard**, bukan efek samping. Entity yang mendeklarasikan
`before create` untuk menolak nilai di luar rentang — pola yang dipakai kafe
(`guard_menu_item_price_unique.star`, `guard_shift_open_unique.star`) —
**ditegakkan di setiap create HTTP dan dilewati di setiap create script**.
Baris buruknya masuk; tidak ada error, tidak ada log, dan tidak ada bedanya di
manifest untuk menandai bahwa jalur itu tidak dijaga.

**Diukur, bukan disimpulkan.** Spec uji dengan entity ber-guard
(`quantity > 0`) dan action yang menulisnya lewat script:

| | widget tersimpan setelah 1 tulis sah + 1 tulis `quantity: -3` |
| --- | --- |
| sebelum fix | **2** — guard dilewati, baris buruk tersimpan |
| sesudah fix | **1** — guard menolak, write dibatalkan |

### (c) `resource.save` tidak memeriksa `uses.resources` — celah konsen

`resource.create`/`call`/`load`/`find` semuanya memanggil
`checkCrossModuleUses` (todo 2.6.4). `resource.save` **tidak** — jadi sebuah
script bisa menulis entity module lain hanya dengan menyebut namanya, dan
`uses.resources` berubah dari batas menjadi deskripsi niat. Kini diperiksa.

### Yang dikerjakan

- `SaveHandler`/`SetSaveHandler` mendapat `fromModule` + `callerResources`.
- Engine (`internal/starlark`) mendapat `ActionUses` — per-execution, pola yang
  sama dengan `MaintainerRef` — supaya hook mewarisi konsen action-nya.
- Helper bersama `runBeforeWriteHooks` (mengembalikan error → write dibatalkan)
  dan `runAfterWriteHooks` (best-effort, seperti jalur HTTP), dipakai di ketiga
  titik tulis script: `resource.save` (insert + update) dan `resource.create`.
- `resource.set()` di dalam `before` hook tetap terlihat pemanggil: hasil hook
  disalin kembali ke map yang ditulis.

**Bukti:** `resource/script_before_hook_e2e_test.go` —
`TestScriptWriteEnforcesBeforeHook` dan `TestScriptWriteEnforcesBeforeHookOnSave`.
Keduanya **gagal** saat pemanggilan fase dinonaktifkan
(`guard bypassed on the script path: 2 widgets exist … want 1`), hijau
sesudahnya · `go test ./...` hijau.

**Sisa jujur:** fase `after` tetap best-effort (tidak rollback) — limitasi model
fase yang sudah tercatat di 2.1.1, bukan dari perubahan ini. Fase `before` pada
jalur hook kini membatalkan write, sama seperti HTTP.

## B. SPA tidak bisa di-build (5.15.3)

Tiga cacat di working tree (bukan dari sesi ini):

1. Dua `import` tergabung dalam satu baris di `DetailPage.tsx` dan
   `TableRenderer.tsx` (`} from "@/engine/permissions"import {`) → `TS1005`.
2. **Komentar menelan baris kode** di `DetailPage.tsx`:
   `// Find transition to check for confirm message    const transition = …` —
   deklarasinya jadi bagian komentar, sehingga `transition` tidak pernah ada dan
   tiga pemakaian di bawahnya `TS2552`. Ini terlihat wajar saat dibaca sepintas;
   hanya kompilasi yang menangkapnya.
3. Impor `can as checkPermission` yang tidak dipakai lagi (digantikan
   `canDoEntityAction`) di kedua berkas → `TS6133`.

**Verifikasi:** `tsc -p tsconfig.app.json --noEmit` bersih · `vitest` **295
lulus** · `make web-build` sukses · **`make build` penuh sukses** (4 binary —
sebelumnya gagal di `build-spa`).

Dampak lanjutan: `make seed-kafe` (dibuat di `2026-09-22-012`) tidak lagi perlu
dipisah dari build SPA, karena build-nya sudah bekerja.

## Verifikasi kafe setelah kedua perbaikan

`validate` → 83 manifest, 0 problem · `check` → 0 error/0 warning · boot dev
server → **0 warning** · matriks 6 role tetap benar (kasir 200/404/404/404,
manajer 200/200/200/404, dapur 200/200/404/404, dst.) · landed cost tetap
`stock-level avg=40` · `go test ./...` hijau.
