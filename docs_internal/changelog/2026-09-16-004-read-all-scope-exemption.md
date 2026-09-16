# 2026-09-16-004 — `read_all`: pengecualian `row_scope` dinyatakan sebagai permission

Keputusan desain yang menghambat `examples/kafe/gaps_found/TODO.md` **3.5**
(menyalakan penyaringan per cabang). Pemilik proyek memilih **permission
eksplisit**, bukan bypass `*` implisit dan bukan nilai wildcard di atribut. Plan:
`docs_internal/plan/read-all-scope-exemption.md`.

**Masalahnya.** `row_scope` fail closed: principal tanpa nilai dimensi (tidak ada
baris `employee` yang `username`-nya cocok) mendapat 403 di **setiap** pembacaan.
Itu benar untuk kasir, tapi salah untuk pemilik workspace, auditor lintas cabang,
dan identitas dev (`*`) — yang justru tidak punya dimensi karena tugasnya memang
melihat semuanya. Tanpa pengecualian, menyalakan 3.5 mematikan aplikasi bagi
mereka, termasuk walkthrough `formspec dev`.

**Yang dikerjakan.** `{module}.{plural}.read_all` — permission **kebijakan**
(yang tidak punya route sendiri, karena ia tidak menjalankan operasi melainkan
mengecualikan) dengan aturan:

- pemegangnya **dilewati** dari `row_scope` entity itu: tidak ada filter dipasang,
  dan atribut yang tak bisa diselesaikan bukan lagi error (bagi mereka,
  "tidak punya cabang" adalah keadaan normal);
- **didaftarkan** bersama permission standar sehingga bisa diberikan dan terlihat
  di audit — "siapa boleh membaca lintas cabang" dijawab oleh daftar grant, bukan
  penafsiran wildcard;
- `*` juga memenuhi (pemeriksaannya memakai `HasPermission` yang sama di
  mana-mana), sehingga identitas dev tetap hidup sementara kasir yang hanya
  memegang `.list` **tetap ter-scope**;
- cakupannya **per entity**: `read_all` pada `order` tidak memberi akses lintas
  cabang pada `shift`.

Signature `applyRowScope` bertambah `module, entity` supaya pemeriksaan bisa
dilakukan **sebelum** scope diselesaikan — bagi pemegang `read_all`, atribut yang
tak bisa diresolusi memang keadaan normal, bukan kegagalan.

**Bukti.** `TestApplyRowScope_ReadAllPermissionExempts` (pemegang `read_all`
dengan atribut yang tak bisa diresolusi → tidak 403 dan tidak difilter; kasir
tanpa `read_all` pada entity yang sama → tetap fail closed),
`TestReadAllPermission` (bentuk nama + fallback plural), `go test ./...` hijau,
kafe `validate` 0 problem. Normatif: `docs/spec/backend/01-core-basic.md` §1.7
(pengecualian) dan §8.6 (kelas permission kebijakan).

**Efeknya:** penghambat 3.5 hilang — spec kafe bisa memasang `row_scope` dan
cukup memberi role pemilik `read_all` per entity; 3.5 sendiri tetap item Fase 3.
