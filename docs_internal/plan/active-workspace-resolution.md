# Plan — Workspace aktif: default + peringatan (kafe TODO 2.8 / gap #48)

Sumber: `examples/kafe/gaps_found/TODO.md` 2.8; `12-hasil-verifikasi-runtime.md`
gap #48.

## Masalah

`kind: Workspace` adalah **seed declaration**: ia mendaftarkan slug, bukan
memilihnya. Workspace aktif datang dari `--workspace-id` (atau config file),
default `default`. Salah membacanya **tidak menimbulkan error apa pun**:

| Pengamatan                      | Hasil                                        |
| ------------------------------- | -------------------------------------------- |
| `spec/workspaces/kafe.yaml` ada | validasi lulus                               |
| banner `formspec dev`           | `open http://localhost:18100/default/_admin` |
| `POST /default/...`             | tersimpan `tenant_id: "default"`             |
| `GET /kafe/...`                 | **200 OK tapi kosong** — terlihat sehat      |

## Aturan yang dipilih

| Keadaan                                  | Perilaku                                        | Alasan                                                         |
| ---------------------------------------- | ----------------------------------------------- | -------------------------------------------------------------- |
| flag tidak ada, **tepat satu** workspace | **dipakai**, diumumkan                          | inilah kasus yang manifest-nya terbaca seperti pilihan         |
| flag tidak ada, **beberapa** workspace   | tetap `default` + peringatan berisi daftar      | memilih salah satu diam-diam justru mengejutkan                |
| flag ada                                 | dipakai; bila tidak dideklarasikan → peringatan | salah ketik mengirim tulisan ke tenant yang tak dideklarasikan |
| tanpa workspace di tree                  | tanpa perubahan                                 | tidak ada yang perlu dikatakan                                 |

Pembeda "diberikan pengguna" vs "nilai default" perlu dinyatakan eksplisit
(`DevConfig.WorkspaceIDExplicit`), karena aturan "tepat satu → pakai" harus bisa
dibedakan dari pengguna yang memang menulis `--workspace-id default`.

## File

- `cmd/formspec/workspace_active.go` (baru) — `declaredWorkspaces()` (scan
  `kind: Workspace` lewat loader yang sama dengan validate) + `resolveActiveWorkspace()`.
- `cmd/formspec/dev.go` — `WorkspaceIDExplicit` + pemanggilan di awal `runDev`.
- `cmd/formspec/workspace_active_test.go` — 4 kasus.
- `docs/spec/platform/02-workspace-app-module.md` §1 — "mendaftarkan, bukan
  memilih", plus perilaku dev-nya.

## Bukti

Runtime: `formspec dev` pada spec kafe mencetak
`workspace: kafe (the only one declared…)` alih-alih diam-diam `default`.
Unit: 4 kasus `TestResolveActiveWorkspace`. `go test ./...` hijau · kafe
`validate` 0 problem.

## Sisa

Belum dipasang di `formspec serve`/`resource` (jalur non-dev).

## Estimasi: **small**
