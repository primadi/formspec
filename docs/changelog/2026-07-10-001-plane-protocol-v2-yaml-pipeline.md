# 2026-07-10-001 — Plane Protocol v0.2.0: YAML Two-Stage Pipeline

**Apa:** Update Plane Protocol spec ke v0.2.0 dengan YAML Registration Pipeline dua-tahap
(registrasi → deploy), ETag-based conditional pull, hash-based deployment optimization,
dan dev mode simplification. Update Core Basic spec §6 untuk mencerminkan pipeline baru.
Restruktur todo.md dengan Fase 0 (Plane Protocol & YAML Pipeline) sebagai foundation.

**Kenapa:** Arsitektur dua-plane (Control + Resource) mengharuskan YAML tidak langsung di-load
dari filesystem. Spec sebelumnya sudah mendeskripsikan model konseptual yang benar, tapi
 tidak memiliki deskripsi konkret tentang pipeline registrasi → deploy, mekanisme pull,
dan optimasi hash. Update ini menutup gap tersebut sebelum implementasi code dimulai.

**Keputusan teknis:**
- **ETag-based conditional pull**, bukan persistent stream — Control Plane harus stateless
  dan horizontal-scalable. Ribuan koneksi persistent tidak scalable; ETag comparison O(1).
- **Hash-based deployment optimization** — sha256 di snapshot dibandingkan dengan local
  deployment manifest; hash match = skip (zero network transfer untuk artifact).
- **Dev mode: pull 10 detik** (bukan 3 detik), ditambah `POST /v1/poll` trigger lokal
  untuk akselerasi. Tidak ada persistent stream di dev maupun prod.
- **`forma apply`** sebagai satu-satunya cara registrasi YAML — Resource Plane tidak boleh
  membaca filesystem langsung.

**File yang terkena dampak:**

| File | Tipe | Perubahan |
|---|---|---|
| `docs/spec/06-plane-protocol.md` | EDIT | v0.1.0 → v0.2.0; tambah §0 YAML Pipeline; update §3.1 ETag pull; update §3.2 hash optimization; update §3.3 convergence; update §4 evidence types; update §7 conformance; tambah changelog |
| `docs/spec/02-core-basic.md` | EDIT | Update §6 dengan §6.0 Two-Stage YAML Pipeline, dev mode, architectural rule |
| `docs/plan/todo.md` | EDIT | Tambah Fase 0 (Plane Protocol & YAML Pipeline) dengan 6 sub-fase (spec, control API, resource deploy, code mod, dev workflow, testing); update Fase 5; update last updated date |

**Referensi:** `docs/spec/06-plane-protocol.md`, `docs/spec/02-core-basic.md`, `docs/plan/todo.md`, `docs/plan/document-model-code-alignment.md`
