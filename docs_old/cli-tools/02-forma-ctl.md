# forma-ctl — Emergency CLI Reference

**Version:** 1.0
**Status:** Draft
**License:** Creative Commons CC0 (dokumen) — kode-nya sendiri FSL (open source, bagian dari `forma-ctl`)
**Governed by:** `docs/spec/01-overview.md` §11.C, `docs/spec/04-control-plane.md` §11–12, `docs/spec/11-reference.md` (D43)

> `forma-ctl` **bukan binary terpisah**. Ini adalah entrypoint darurat Cloud Owner/Platform Operator, berupa **kode konvensional di dalam binary `forma-ctl`** (D43, "bedrock exception") — sengaja dibangun sesederhana mungkin dan **tidak bergantung pada platform yang sedang ia perbaiki**. Kalau OPA policy engine, database artifact, atau komponen lain sedang rusak, `forma-ctl` tetap harus bisa dipakai.

---

## 1. Kenapa Bukan Binary Terpisah, dan Kenapa Itu Penting

Filosofi desainnya: alat darurat yang bergantung pada sistem yang sedang diperbaiki adalah kontradiksi. Kalau verb emergency ini adalah binary/proses terpisah yang butuh koneksi ke database, policy engine, atau API layer normal server `serve` untuk berfungsi, maka saat komponen itu yang rusak — verb emergency ikut tidak bisa dipakai, persis saat paling dibutuhkan.

Karena itu:
- **Compiled ke dalam binary `forma-ctl` yang sama** yang menjalankan `serve` — bukan dependency eksternal, bukan network call ke proses lain untuk operasi paling kritikal (`freeze`, `revoke sessions`)
- **Jalur kode konvensional/imperatif** — bukan lewat OPA/Rego evaluation yang mungkin sedang jadi sumber masalah
- Tetap **wajib** menyertakan alasan (`--reason`), ditandatangani aktor, dan tercatat di transparency log — darurat bukan alasan untuk melewati audit

Dipanggil sebagai `forma-ctl <verb>` di samping `forma-ctl serve` — keduanya subcommand dari binary yang persis sama (`cmd/forma-ctl/main.go`), analog dengan bagaimana `busybox` atau `git` punya banyak "personality" dari satu binary.

---

## 2. Verb Reference

### 2.1 Emergency Controls

```bash
forma-ctl freeze --reason "suspected key compromise"
forma-ctl revoke sessions --all --environment production
forma-ctl key rotate --environment production
```

| Verb | Fungsi |
|---|---|
| `freeze --reason <text>` | Bekukan seluruh operasi write di environment (deploy baru ditolak, workspace existing tetap serve traffic baca) |
| `revoke sessions --all --environment <env>` | Cabut semua sesi aktif di environment tertentu — dipakai saat dugaan kompromi kredensial |
| `key rotate --environment <env>` | Rotasi platform signing key untuk environment tertentu |

Setiap aksi **wajib** `--reason`, ditandatangani aktor yang menjalankan, dicatat ke transparency log (`docs/spec/04-control-plane.md` §11).

### 2.2 Policy Testing

```bash
forma-ctl policy test
```

Menjalankan table-driven test terhadap policy yang sudah dikompilasi ke Rego — dipakai memverifikasi perubahan `kind: Policy` sebelum diterapkan (`docs/spec/04-control-plane.md` §2, embedded OPA evaluation dari structured key + `rego:` escape hatch).

### 2.3 Transparency Log Verification

```bash
forma-ctl log verify --checkpoint <file>
```

Membuktikan tidak ada perubahan sejarah (history rewrite) sejak checkpoint tertentu — verifikasi Merkle tree independen dari proses region control yang normal berjalan.

---

## 3. Perbandingan dengan Emergency Command di `forma` (Resource Plane Side)

Dua CLI yang berbeda untuk dua sisi yang berbeda — **jangan disatukan**:

| | `forma freeze` / `forma rollback` / `forma lock workspace` | `forma-ctl freeze` / `revoke sessions` / `key rotate` |
|---|---|---|
| **Dijalankan oleh** | App Admin yang diotorisasi (Workspace Owner side) | Cloud Owner / Platform Operator |
| **Sisi** | Resource Plane (workspace/app spesifik) | Control Plane (environment/region) |
| **Scope** | Satu workspace/app | Seluruh environment |
| **Binary** | `forma` (CLI developer/app owner) | `forma-ctl` (mode CLI, bukan binary lain) |

Lihat `01-forma-cli.md` §11 untuk sisi Resource Plane.

---

## 4. Model Otorisasi

`forma-ctl` tidak "melewati" sistem otorisasi — ia jalur eksekusi yang berbeda, bukan jalur tanpa otorisasi:

- Dijalankan dengan kredensial Cloud Owner (private key HSM/KMS, atau delegated admin key)
- Perintah paling destruktif (`key rotate`, `revoke sessions --all`) idealnya butuh **konfirmasi eksplisit** (`--confirm` atau prompt interaktif) sebelum eksekusi
- Semua audit trail identik dengan operasi normal — hanya jalur eksekusinya yang bypass dependency non-esensial (OPA, artifact DB), bukan bypass logging/signing

---

## 5. Status Implementasi Hari Ini

**Kerangka dispatcher ada, verb emergency-nya belum.** `cmd/forma-ctl/main.go` sudah membedakan `serve` (fungsional — lihat `docs/runtimes/01-forma-ctl.md`) dari verb emergency (`freeze`, `revoke`, `key`, `policy`, `log`), tapi kelima verb itu langsung mencetak:

```
forma-ctl freeze: not implemented yet — see docs/cli-tools/02-forma-ctl.md §5
```

dan exit dengan status 1 — **bukan** silent no-op, dan bukan pura-pura sukses. Ini konsisten dengan pola stub yang sudah dipakai di tempat lain di codebase (mis. `internal/action/sidecar.go`'s `SidecarExecutor`).

`docs/plan/todo.md` menandai implementasi sungguhan verb ini sebagai item terpisah:

- Fase 5.7: `forma-ctl` emergency CLI — freeze, revoke sessions, key rotate — ⏳
- Fase 5.2: `kind: Policy` — OPA/Rego integration, `forma-ctl policy test` — ⏳

Keduanya bergantung pada fitur yang sendiri belum ada (`docs/runtimes/01-forma-ctl.md` §7: tidak ada OPA/policy engine, tidak ada persistent store, tidak ada transparency log implementasi nyata) — jadi implementasi sungguhan verb ini **tidak bisa dibangun sebelum** ketiga fondasi itu ada.

### 5.1 Urutan Pembangunan yang Disarankan

1. Bangun dulu fondasi yang jadi prasyarat: persistent artifact store, transparency log dasar (lihat `docs/runtimes/01-forma-ctl.md` §7.1) — `forma-ctl log verify` tidak ada gunanya tanpa log yang nyata.
2. `forma-ctl freeze`/`revoke sessions` bisa dibangun lebih awal dari `key rotate` — perilakunya lebih sederhana (set flag status di environment record, tidak butuh HSM/KMS integration). Ganti stub `case "freeze", "revoke", ...` di `main.go` dengan implementasi nyata satu per satu, bukan sekaligus.
3. `policy test` menunggu `kind: Policy` + OPA integration (belum ada sama sekali di kode).
4. Sepanjang jalan, jaga prinsip §1 — jangan sampai implementasi verb emergency diam-diam butuh service lain hidup untuk berfungsi (mis. jangan taruh di belakang HTTP API internal yang bisa down bareng komponen yang diperbaiki). Implementasikan sebagai kode langsung dalam proses `forma-ctl`, bukan sebagai HTTP client ke `serve` yang sedang berjalan.

---

## 6. References

| Dokumen | Isi |
|---|---|
| `docs/spec/04-control-plane.md` §11–12 | Emergency controls, conformance |
| `docs/spec/11-reference.md` D43 | Bedrock exception, dogfooding consoles |
| `docs/runtimes/01-forma-ctl.md` | Binary yang meng-host `forma-ctl` |
| `docs/cli-tools/01-forma-cli.md` §11 | Emergency command sisi Resource Plane (`forma freeze`, dst) |
