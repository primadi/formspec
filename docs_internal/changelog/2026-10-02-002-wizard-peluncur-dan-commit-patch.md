# 2026-10-02-002 — Wizard: peluncur dari tombol transisi + commit `PATCH`

Plan: `docs_internal/plan/wizard-commit-patch-dan-peluncur.md` ·
Todo: **5.25.4 (C) + 5.25.5 (B)** · Prasyarat: registrasi `registered_views`
(`2026-10-02-001`).

## Apa

Dua arah yang membuat `kind: Wizard` benar-benar berfungsi saat ia mengikat
dirinya ke sebuah **transisi** (`entity` + `action` = transisi `via`):

**B — commit.** `WizardRenderer` memilih call commit dengan aturan yang sama
yang sudah dipakai `DetailPage` (kafe 10.48, `has_route` dari bundle):

- `has_route` → `POST /{module}/{entity}/{id}/{action}`
- selain itu, bila `action` cocok transisi → `PATCH /{module}/{entity}/{id}`
  dengan `{state_field: to, …field terkumpul}`

Id record dibaca dari `?id=`. Sekaligus diperbaiki: path POST tidak lagi
mengirim `spec.action` **mentah** (dulu `POST …/_ui/entity/close-shift` —
bentuk yang mustahil, tanpa module/entity/id); payload hanya memuat field
entity; dan `on_complete` default mendarat di **daftar entity wizard di surface
saat ini** (dulu `adminPath()` — salah untuk App surface; kasir kafe-pos tidak
punya `_admin.access`).

**C — peluncur.** `DetailPage.handleTransition` memeriksa
`findWizardForTransition` (wizard dengan `spec.entity` menunjuk entity ini
**dan** `spec.action === via`) sebelum dialog input/konfirmasi, lalu menavigasi
ke `surfacePath("wizard", name)?id=<record-id>`. Wizard sendirilah yang
mengumpulkan input, jadi membuka dialog generik lebih dulu akan bertanya dua
kali.

## Kenapa

Sebelum ini `close-shift-wizard` (kafe) tidak bisa dipakai sama sekali: tidak
ada jalur UI menuju wizard (tombol transisi menjalankan `PATCH status` langsung
dan **melompatinya**), dan andai dibuka lewat URL, submit-nya POST ke path yang
tidak ada tanpa id record. Akibatnya `counted_cash`/`note`/`supervisor_id`
tidak pernah terkumpul.

## File

- `src/engine/wizardCommit.ts` (baru) — `usesActionRoute`,
  `resolveWizardCommit`, `findWizardForTransition` (murni).
- `src/kinds/wizard/WizardRenderer.tsx` — pakai helper; `?id=`; landing
  surface-aware.
- `src/kinds/page/DetailPage.tsx` — deteksi wizard → navigasi.

## Bukti

**Terukur di browser (kafe-pos, login `kasir`, shift seed diubah ke tanggal
hari ini agar lolos backdate policy 3 hari):**

1. `GET /kafe/app/pos/cafe-order/shifts/<id>` → tombol
   **"Tutup shift: hitung fisik, tampilkan selisih, konfirmasi (Wizard)"**.
2. Klik → `/kafe/app/pos/wizard/close-shift-wizard?id=01a0bf96-…` (Step 1 of 3)
   — **C**.
3. Isi langkah 2 → Complete → **`PATCH /kafe/_ui/entity/cafe-order/shift/
01a0bf96-… -> 200`**, toast "Wizard completed successfully", dan halaman
   mendarat di `/kafe/app/pos/cafe-order/shifts` (surface-aware, bukan
   `/_admin`) — **B**.
4. Record: `status=closed`, `counted_cash={amount:"150000",currency:"IDR"}`
   (sebelumnya **selalu `None`**), `note="kurang 10000"`, `version` 1→3.

**Otomatis:** `npx tsc -b` bersih; `npx vitest run` 47 file / 608 test lulus
(`src/engine/wizardCommit.test.ts` +11). `formspec check -f examples/kafe/spec`
0 error / 0 warning.

## Sisa

- **5.25.6 ⏸️** — seed role usang: `close-shift` tidak ada di
  `authorized_actions` kasir meski `seeds/roles.yaml` mendeklarasikannya; DB
  grant adalah otoritas, jadi `make seed-kafe` + restart wajib. Belum ada
  sinyal drift seed↔DB. Ditemukan saat verifikasi ini.
- **5.25.2 / 5.25.3 ⏸️** — tak tersentuh (picker non-routable; lint
  pre-existing).
- `expected_cash`/`difference` tetap `null` di record: keduanya `computed` di
  Form/Wizard, bukan dihitung server pada `PATCH`. Pre-existing, di luar B+C.
