# 2026-09-16-006 — Workspace aktif: satu yang dideklarasikan dipakai, sisanya diperingatkan

Item `examples/kafe/gaps_found/TODO.md` **2.8** (gap **#48**). Plan:
`docs_internal/plan/active-workspace-resolution.md`.

**Masalah.** `kind: Workspace` adalah seed declaration — mendaftarkan slug, bukan
memilihnya. Workspace aktif datang dari `--workspace-id`. Salah membacanya tidak
memunculkan error: mendeklarasikan `kafe`, menjalankan dev tanpa flag, semua
tulisan tersimpan dengan `tenant_id: "default"`, dan `GET /kafe/...` menjawab
`200` dengan nol baris — permukaan yang terlihat sehat tapi berbeda dari yang
manifest-nya janjikan.

**Aturan.** Ketepatan lebih penting di sini daripada kenyamanan, jadi ketiga
keadaan diperlakukan berbeda: (a) **tepat satu** workspace dideklarasikan dan
flag tidak diberikan → dipakai, dan diumumkan; (b) **lebih dari satu** → tetap
`default` plus peringatan berisi daftarnya (memilih diam-diam mengejutkan);
(c) `--workspace-id` yang **tidak dideklarasikan** → diperingatkan (salah ketik
mengirim semua tulisan ke tenant yang tak pernah dideklarasikan). Pembeda
"diberikan pengguna" vs "nilai default" ditambahkan sebagai
`DevConfig.WorkspaceIDExplicit`, karena tanpa itu kasus (a) tidak bisa
dibedakan dari pengguna yang memang menulis `--workspace-id default`.

**Bukti.** Runtime: dev pada spec kafe mencetak
`workspace: kafe (the only one declared under spec/workspaces; override with
--workspace-id)` alih-alih diam-diam `default`. Unit: `TestResolveActiveWorkspace`
(4 kasus: satu diadopsi, flag eksplisit menang, tree tanpa workspace tetap
`default`, dua workspace tetap `default` + peringatan). `go test ./...` hijau ·
kafe `validate` 0 problem.

**Dokumen.** `docs/spec/platform/02-workspace-app-module.md` §1 kini menyatakan
eksplisit bahwa manifest Workspace **mendaftarkan, bukan memilih**, plus perilaku
dev di ketiga keadaan — kalimat lama mudah dibaca sebaliknya, dan itulah akar
kebingungan gap #48.

**Sisa.** Peringatan yang sama belum dipasang di `formspec serve`/`resource`.
