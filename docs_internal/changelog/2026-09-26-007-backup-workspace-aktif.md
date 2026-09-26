# `backup`/`restore` membaca workspace aktif, bukan `"demo"` hardcoded (todo 4.8.7)

## Apa yang diubah

`formspec backup create` dan `formspec restore` kini memakai **aturan workspace #48
yang sama dengan `formspec dev`** alih-alih literal `"demo"` di tujuh tempat
(`cmd/formspec/backup.go`). Keduanya menerima `--workspace <slug>`, dan
`manifest.json` mencatat workspace sumbernya.

**Bug-nya diukur, bukan disimpulkan.** Item 4.8.7 menyebut
`27 table(s), 0 record(s)`; direproduksi ulang persis:

```
# sebelum (perilaku lama, dipaksa via --workspace demo)
Backup written to /tmp/kafe-demo.tar (27 table(s), 0 record(s), 0 storage object(s))
# sesudah (aturan #48: kafe satu-satunya workspace yang dideklarasikan)
Backup written to /tmp/kafe-before.tar (27 table(s), 160 record(s), 0 storage object(s))
```

Perbedaan ini tidak menimbulkan error apa pun di kedua sisi — arsip kosong yang
"berhasil" tidak bisa dibedakan dari aplikasi yang memang kosong. Karena itu
`manifest.json` sekarang memuat `workspace`, `backup inspect` menampilkannya
(atau menyatakan arsip lama tidak mencatatnya), dan `restore --dry-run` memakai
tenant yang sama dengan `backup create`.

**Refactor yang menghindari bug ini terulang:** aturan #48 diekstrak dari
`resolveActiveWorkspace` (yang hanya menerima `DevConfig`) menjadi
`activeWorkspaceFor(specPath, current, explicit)` di
`cmd/formspec/workspace_active.go`, sehingga perintah CLI non-`dev` bisa memakai
aturan yang **sama** — bukan menyalinnya, dan bukan meng-hardcode `"demo"`.
`resolveActiveWorkspace` kini menjadi pembungkus tipis; perilaku `formspec dev`
tidak berubah.

`activeWorkspaceFor` punya satu cabang yang tidak dibutuhkan jalur dev: ketika
pemanggil **tidak tahu** workspace mana yang ia baca (kasus `backup`/`restore`)
dan spec mendeklarasikan tepat satu, slug itu diadopsi. Dengan beberapa
dideklarasikan, ia **tidak menebak** — tetap memakai default dan memperingatkan,
karena memilih salah satu secara acak berarti menulis ke tenant yang salah tanpa
jejak.

## Kenapa

Menutup item 4.8.7. Ini kelas "gagal senyap" yang paling mahal dari kelompok
3.7b: `docs/cli-tools/02-formspec-cli.md` sudah memuat peringatan tentangnya,
tetapi peringatan itu tidak menolong siapa pun yang tidak membacanya, dan
arsipnya tetap dihasilkan.

## File terdampak

- `cmd/formspec/backup.go` — `activeWorkspaceFor` dipakai di kedua perintah,
  flag `--workspace` (create + restore), `BackupManifest.Workspace`,
  `workspace` diteruskan ke `collectStorageKeys`/`restoreFromWithStorage`,
  output `Resource mapping` + `--dry-run`/`inspect` menampilkan tenant
- `cmd/formspec/workspace_active.go` — `activeWorkspaceFor` diekstrak
- `cmd/formspec/backup_workspace_test.go` — **baru**
- `cmd/formspec/backup_test.go`, `backup_mapresource_test.go` — call site disesuaikan
- `docs/cli-tools/02-formspec-cli.md` — "batasan yang diketahui" menjadi perilaku
  yang dijelaskan

## Bukti

- **Terukur pada DB kafe nyata** (salinan `examples/kafe/.formspec/kafe.db`):
  `0 record(s)` → **`160 record(s)`**; `manifest.json` memuat `"workspace": "kafe"`;
  `backup inspect` mencetak `Workspace: kafe`; `restore --dry-run` mencetak
  `[formspec] workspace: kafe` dan 2/6/10/4/9/11 record untuk enam entity pertama.
- `TestBackupReadsNamedWorkspace` — seed ke `staging`, bukan `demo`; memverifikasi
  `staging=2` **dan** `demo=0` lebih dulu (supaya test tidak lulus karena alasan
  yang salah), lalu mem-pin kedua cabang aturan: tanpa flag → `staging`; eksplisit
  `demo` → tetap `demo`.
- `go build ./...` bersih; `go test ./cmd/formspec/ ./resource/` hijau; `gofmt -l` bersih.

## Rujukan

Todo **4.8.7** (tertutup) · aturan #48 di `cmd/formspec/workspace_active.go`
(kafe ledger 2.8 / gap #48) · kelompok 3.7b.
