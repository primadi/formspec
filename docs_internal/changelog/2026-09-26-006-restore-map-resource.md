# `formspec restore --map-resource` (todo 3.7.7)

## Apa yang diubah

`formspec restore` kini menerima `--map-resource <src>=<dst>` (boleh diulang),
yang mengarahkan record dari resource di dalam arsip ke **resource lain** di spec
tujuan — mis. memuat sample produksi ke entity dev:

```
formspec restore --from prod.tar --map-resource beta/customer=staging/lead --dry-run
Resource mapping:
  beta/customer -> staging/lead
Dry-run compatibility report:
  beta/customer -> staging/lead: 2 restore, 0 skip, 0 remap, 0 fail
```

Sebelumnya flag ini **diterima parser tetapi tidak melakukan apa pun** — `--conflict
remap` (yang mengganti natural key saat bentrok) sudah jalan, tetapi pasangan
pemetaan resource-nya belum. Keduanya sekarang dibedakan eksplisit di
`docs/cli-tools/02-formspec-cli.md`, karena contoh di dokumen itu menulis
`--map-resource` dengan komentar "remap saat konflik ID" — deskripsi yang keliru
dan menyesatkan pembaca tentang apa yang sebenarnya terjadi.

**Tiga keputusan perilaku:**

1. **Target wajib ada di spec.** Kalau tidak, perintah berhenti (`exit 2`)
   **sebelum** menyentuh database. Tanpa cek ini, target yang tidak resolve
   membuat record dilewati dan laporan berbunyi `0 failed` — terbaca sukses
   padahal tidak ada yang ditulis.
2. **Kedua sisi menerima `module/entity` atau `module_entity`.** Bentuk
   underscore adalah spelling yang dicetak `formspec backup inspect`; bentuk slash
   adalah yang dipakai spec. Memaksa satu spelling berarti operator harus
   menerjemahkan sendiri, dan salah menerjemahkan tidak terlihat.
3. **Laporan dikunci ke entri arsip, bukan ke target.** `RestoreEntityReport`
   dapat field `MappedTo`; `Module`/`Entity` tetap menunjuk berkas arsipnya.
   Alasannya konkret: satu entri arsip bisa dipetakan ke satu target, tetapi
   laporan yang menampilkan nama target saja membuat "berkas mana yang menghasilkan
   angka ini" tidak bisa dijawab, dan dua entri yang menuju target sama jadi tak
   terbedakan.

Konsumen nyata: pemetaan berlaku sebelum store **dan** spec entity di-resolve,
sehingga validasi record memakai aturan (natural key, required, backdate) entitas
**tujuan** — bukan aturan sumber, yang akan menolak data yang sah di tujuan.

## Kenapa

Menutup item 3.7.7 (kelompok 3.7b, audit `docs_internal/plan/audit-open-items-prosa.md`).
Item itu mencatat "flag diterima parser tapi belum melakukan apa pun… CLI tampak
mendukung, perilakunya tidak" — kelas yang persis sama dengan 3.7.6 (ternyata
sudah jalan) dan 4.8.7 (masih bug). Verifikasi per-flag memang perlu: dua dari
lima item di kelompok itu ternyata sudah selesai, satu masih bug.

## File terdampak

- `cmd/formspec/backup.go` — `parseResourceMap`, `normalizeResourceRef`,
  `parseResourceRef` (baru), `RestoreEntityReport.MappedTo`, mapping di loop
  restore, blok "Resource mapping" di output, `restoreFromWithStorage` +1 parameter
- `cmd/formspec/backup_mapresource_test.go` — **baru** (2 test)
- `cmd/formspec/backup_test.go` — 2 call site disesuaikan
- `docs/cli-tools/02-formspec-cli.md` — tabel beda `--map-resource` vs `--conflict remap`

## Bukti

- `TestRestoreMapResource` end to end (2 entity, backup → restore dengan
  pemetaan): record mendarat di **target** (`lead` = 2), sumber tetap **kosong**
  (`customer` = 0 — pemetaan mengalihkan, bukan menggandakan), dan laporan
  mencatat `MappedTo`. **Dibuktikan gagal** saat pemetaan dinetralkan — tiga
  assertion berbunyi (`MappedTo = ""`, `lead has 0 records`, `customer has 2 records`).
- `TestParseResourceMap` — dua spelling + empat bentuk yang harus ditolak
  (`alpha/customer` tanpa `=`, `=alpha/lead`, `alpha/customer=`, `customer`).
- `go build ./...` bersih; `go test ./cmd/formspec/` hijau; `gofmt -l` bersih.

## Rujukan

Todo **3.7.7** (tertutup) · kelompok 3.7b · `docs/spec/backend/04-persist-backend.md` §3.
