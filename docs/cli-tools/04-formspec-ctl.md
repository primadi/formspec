# formspec-ctl — Emergency CLI Reference

**Version:** 1.0
**Status:** Draft
**License:** Creative Commons CC0 (dokumen) — kode-nya sendiri FSL (open source, bagian dari `formspec-ctl`)

> `formspec-ctl` **bukan binary terpisah**. Ini adalah entrypoint darurat Cloud Owner/Platform Operator, berupa **kode konvensional di dalam binary `formspec-ctl`** — sengaja dibangun sesederhana mungkin dan **tidak bergantung pada platform yang sedang ia perbaiki**. Kalau OPA policy engine, database artifact, atau komponen lain sedang rusak, `formspec-ctl` tetap harus bisa dipakai. Dokumen ini normatif — implementasi kode mengikuti dokumen ini.

---

## 1. Kenapa Bukan Binary Terpisah, dan Kenapa Itu Penting

Filosofi desainnya: alat darurat yang bergantung pada sistem yang sedang diperbaiki adalah kontradiksi. Kalau verb emergency ini adalah binary/proses terpisah yang butuh koneksi ke database, policy engine, atau API layer normal server `serve` untuk berfungsi, maka saat komponen itu yang rusak — verb emergency ikut tidak bisa dipakai, persis saat paling dibutuhkan.

Karena itu:
- **Compiled ke dalam binary `formspec-ctl` yang sama** yang menjalankan `serve` — bukan dependency eksternal, bukan network call ke proses lain untuk operasi paling kritikal (`freeze`, `revoke sessions`)
- **Jalur kode konvensional/imperatif** — bukan lewat OPA/Rego evaluation yang mungkin sedang jadi sumber masalah
- Tetap **wajib** menyertakan alasan (`--reason`), ditandatangani aktor, dan tercatat di transparency log — darurat bukan alasan untuk melewati audit

Dipanggil sebagai `formspec-ctl <verb>` di samping `formspec-ctl serve` — keduanya subcommand dari binary yang persis sama (`cmd/formspec-ctl/main.go`), analog dengan bagaimana `busybox` atau `git` punya banyak "personality" dari satu binary.

Prinsip desain lengkap ("bedrock exception" — kenapa ini pengecualian arsitektural dibanding konsol resmi lain seperti `formspec/console`/`formspec/studio`/`formspec/ops`): [`docs/spec/platform/04-control-plane.md`](../spec/platform/04-control-plane.md) §8.

---

## 2. Verb Reference

### 2.1 Emergency Controls

```bash
formspec-ctl freeze --reason "suspected key compromise"
formspec-ctl revoke sessions --all --environment production
formspec-ctl key rotate --environment production
```

| Verb | Fungsi |
|---|---|
| `freeze --reason <text>` | Bekukan seluruh operasi write di environment (deploy baru ditolak, workspace existing tetap serve traffic baca) |
| `revoke sessions --all --environment <env>` | Cabut semua sesi aktif di environment tertentu — dipakai saat dugaan kompromi kredensial |
| `key rotate --environment <env>` | Rotasi platform signing key untuk environment tertentu |

Setiap aksi **wajib** `--reason`, ditandatangani aktor yang menjalankan, dicatat ke transparency log — kontrak lengkap: [`docs/spec/platform/04-control-plane.md`](../spec/platform/04-control-plane.md) §6, §8.

### 2.2 Policy Testing

```bash
formspec-ctl policy test
```

Menjalankan table-driven test terhadap `kind: Policy` yang sudah dikompilasi ke Rego — dipakai memverifikasi perubahan policy sebelum diterapkan. Kontrak lengkap `kind: Policy` (vocabulary terstruktur + escape hatch Rego, policy floor yang tidak bisa dikonfigurasi lepas): [`docs/spec/platform/04-control-plane.md`](../spec/platform/04-control-plane.md) §5.

### 2.3 Transparency Log Verification

```bash
formspec-ctl log verify --checkpoint <file>
```

Membuktikan tidak ada perubahan sejarah (history rewrite) sejak checkpoint tertentu — verifikasi Merkle tree independen dari proses region control yang normal berjalan. Kontrak lengkap transparency log (struktur Merkle tree, inclusion proof, publikasi checkpoint): [`docs/spec/platform/04-control-plane.md`](../spec/platform/04-control-plane.md) §6.

---

## 3. Perbandingan dengan Emergency Command di `formspec` (Resource Plane Side)

Dua CLI yang berbeda untuk dua sisi yang berbeda — **jangan disatukan**:

| | `formspec freeze` / `formspec rollback` / `formspec lock workspace` | `formspec-ctl freeze` / `revoke sessions` / `key rotate` |
|---|---|---|
| **Dijalankan oleh** | App Admin yang diotorisasi (Workspace Owner side) | Cloud Owner / Platform Operator |
| **Sisi** | Resource Plane (workspace/app spesifik) | Control Plane (environment/region) |
| **Scope** | Satu workspace/app | Seluruh environment |
| **Binary** | `formspec` (CLI developer/app owner) | `formspec-ctl` (mode CLI, bukan binary lain) |

Lihat [`02-formspec-cli.md`](02-formspec-cli.md) §11 untuk sisi Resource Plane.

---

## 4. Model Otorisasi

`formspec-ctl` tidak "melewati" sistem otorisasi — ia jalur eksekusi yang berbeda, bukan jalur tanpa otorisasi:

- Dijalankan dengan kredensial Cloud Owner (private key HSM/KMS, atau delegated admin key)
- Perintah paling destruktif (`key rotate`, `revoke sessions --all`) idealnya butuh **konfirmasi eksplisit** (`--confirm` atau prompt interaktif) sebelum eksekusi
- Semua audit trail identik dengan operasi normal — hanya jalur eksekusinya yang bypass dependency non-esensial (OPA, artifact DB), bukan bypass logging/signing

---

## 5. Status Implementasi Hari Ini

**Kerangka dispatcher ada, verb emergency-nya belum.** `cmd/formspec-ctl/main.go` sudah membedakan `serve` (fungsional — lihat [`docs/runtimes/01-formspec-ctl.md`](../runtimes/01-formspec-ctl.md)) dari verb emergency (`freeze`, `revoke`, `key`, `policy`, `log`), tapi kelima verb itu langsung mencetak:

```
formspec-ctl freeze: not implemented yet — see docs/cli-tools/04-formspec-ctl.md §5
```

dan exit dengan status 1 — **bukan** silent no-op, dan bukan pura-pura sukses. Ini konsisten dengan pola stub yang sudah dipakai di tempat lain di codebase (mis. `internal/action/sidecar.go`'s `SidecarExecutor`).

Implementasi sungguhan kelima verb ini bergantung pada fondasi yang sendiri belum ada (lihat [`docs/runtimes/01-formspec-ctl.md`](../runtimes/01-formspec-ctl.md) §7): tidak ada OPA/policy engine, tidak ada persistent artifact store, tidak ada transparency log implementasi nyata — sehingga **tidak bisa dibangun sebelum** ketiga fondasi itu ada.

### 5.1 Urutan Pembangunan yang Disarankan

1. Bangun dulu fondasi yang jadi prasyarat: persistent artifact store, transparency log dasar (lihat [`docs/runtimes/01-formspec-ctl.md`](../runtimes/01-formspec-ctl.md) §7.1) — `formspec-ctl log verify` tidak ada gunanya tanpa log yang nyata.
2. `formspec-ctl freeze`/`revoke sessions` bisa dibangun lebih awal dari `key rotate` — perilakunya lebih sederhana (set flag status di environment record, tidak butuh HSM/KMS integration). Ganti stub `case "freeze", "revoke", ...` di `main.go` dengan implementasi nyata satu per satu, bukan sekaligus.
3. `policy test` menunggu `kind: Policy` + integrasi OPA (belum ada sama sekali di kode).
4. Sepanjang jalan, jaga prinsip §1 — jangan sampai implementasi verb emergency diam-diam butuh service lain hidup untuk berfungsi (mis. jangan taruh di belakang HTTP API internal yang bisa down bareng komponen yang diperbaiki). Implementasikan sebagai kode langsung dalam proses `formspec-ctl`, bukan sebagai HTTP client ke `serve` yang sedang berjalan.

---

## 6. Referensi

| Dokumen | Isi |
|---|---|
| [`docs/runtimes/01-formspec-ctl.md`](../runtimes/01-formspec-ctl.md) | Binary yang meng-host `formspec-ctl`, termasuk gap fondasi §7 |
| [`02-formspec-cli.md`](02-formspec-cli.md) §11 | Emergency command sisi Resource Plane (`formspec freeze`, dst) |
| [`docs/spec/platform/04-control-plane.md`](../spec/platform/04-control-plane.md) | Kontrak: Policy, transparency log, REPL governance, emergency controls, bedrock exception |
