# Arsitektur — Clinic UI Showcase

## Pendekatan: Spec-First

Seluruh perilaku aplikasi dideklarasikan sebagai manifest YAML
(`apiVersion: formspec.dev/v1`). Engine FormSpec me-derive:

- **REST API** untuk setiap entity
- **UI** (Table, Form, Page, dst.) yang di-render dari Meta API
  `/_ui/_meta/ui` oleh renderer `renderers/react-shadcn/`
- **State machine** dari deklarasi `state_machine`
- **Aksi custom** dari deklarasi `actions` (script Starlark via
  `impl: { type: script_ref }`)
- **Event** async + deliver channel (`audit_log`, `websocket`)
- **Hooks** `before/create` (Core Extended §8) dari deklarasi `hooks`

Tidak ada kode implementasi yang ditulis manual untuk CRUD biasa — cukup
deklarasi entity, sisanya otomatis.

## Dua App, Dua Module

**2 module** — sengaja dipisah supaya sidebar module-grouping, cross-module
relation, dan cross-module widget ikut teruji:

| Module     | Context            | Isi                                                                                        |
| ---------- | ------------------ | ------------------------------------------------------------------------------------------ |
| `clinic`   | Operasional klinik | Pasien, dokter, poliklinik, kunjungan, pembayaran, rekam medis, pengaturan, summary harian |
| `pharmacy` | Farmasi            | Obat, resep (antrian peracikan), penjualan bebas OTC                                       |

**2 App** — bukti konkret bahwa 1 Module bisa di-mount lebih dari 1 App
dalam 1 workspace (many-to-many App ↔ Module):

| App               | `root_url`           | Modules              | Populasi user                                                                   |
| ----------------- | -------------------- | -------------------- | ------------------------------------------------------------------------------- |
| `klinik-internal` | `/klinik`            | `clinic`, `pharmacy` | Staf klinik — adopsi utuh default menu suggestion kedua module (`type: module`) |
| `klinik-portal`   | `/app/klinik-portal` | `clinic`             | Calon pasien publik — menu ramping, hanya pendaftaran pasien (wizard)           |

`pharmacy` menyatakan `depends: [formspec/core, clinic]` — relasi resep ke
`clinic.visit` dan `clinic.patient` ditulis dot notation lintas module.

## Karakteristik Entity

Karakteristik menentukan perilaku yang di-derive engine. Showcase ini
**semuanya memakai** (berbeda dari contoh lain yang hanya pakai 2–3):

| Karakteristik | Dipakai di                                                       | Alasan                                                                  |
| ------------- | ---------------------------------------------------------------- | ----------------------------------------------------------------------- |
| `master`      | `polyclinic`, `doctor`, `patient`, `medicine`                    | Data stabil yang dirujuk entity lain                                    |
| `transaction` | `visit`, `payment`, `medical-record`, `prescription`, `otc-sale` | Data transaksional + state machine                                      |
| `reference`   | `setting`                                                        | Configuration Page pattern — hanya Update, tanpa New/Delete             |
| `summary`     | `daily-visit-summary`                                            | Proyeksi baca-saja; router hanya generate list+find, tanpa menu derived |

> Catatan: projection engine (recompute otomatis) belum ada, jadi widget
> dashboard membaca entity live `clinic/visit`.

## Pola Lifecycle (§1.7 spec frontend)

| Pola                               | Entity                                                   | Mekanisme                                                     |
| ---------------------------------- | -------------------------------------------------------- | ------------------------------------------------------------- |
| Plain CRUD (tanpa draft/Submit)    | `patient`, `doctor`, `polyclinic`, `medicine`, `setting` | `actions: - name: submit, disabled: true`                     |
| 2-step + auto-save (default)       | `visit`                                                  | submit aktif (tidak ditulis) — update dibatasi kondisi status |
| 1-step create-submit (quick entry) | `payment`                                                | reserved action `create-submit` + hint `ui:`                  |
| Reference (Update-only)            | `setting`                                                | `characteristic: reference`                                   |
| Append-only (tanpa edit/hapus)     | `medical-record`                                         | `update`+`delete` disabled                                    |

## Keputusan Desain Penting

### 1. Natural key dengan sequence + reset

Kode bisnis di-generate engine lewat `natural_key_rule`, bukan diisi manual:

- `visit.queue_number`: `Q20260906-001` — prefix dari `Config clinic.queue_prefix`,
  `reset: daily`
- `payment.number`: `PAY-202609-00001` — `reset: monthly`
- `prescription.number`: `RX-20260906-001`, `otc-sale.number`:
  `OTC-20260906-001` — `reset: daily`

### 2. Child table jsonb

`visit.treatments`, `prescription.items`, `otc-sale.items` memakai
`type: child` dengan `storage: jsonb` + `sequence_field: line_number`.

### 3. Relasi lintas module memakai dot notation

```yaml
- name: visit_id
  type: relation
  relation: { type: belongs_to, resource: clinic.visit }
```

### 4. Event realtime

`visit.completed` / `completed-overdue`, `prescription.created`,
`otc-sale.completed` di-deliver ke channel `audit_log` + `websocket`
(scope workspace) — renderer refetch saat event/reconnect.

### 5. Hooks `before/create` (Core Extended §8)

- `prescription` — `derive-patient-name.star`: cross-module
  `resource.fetch("clinic.patient", ...)` untuk auto-fill `patient_name`
  (dideklarasikan lewat `uses.resources`).
- `otc-sale` — `stock-guard.star`: `fail()` membatalkan create bila stok
  obat tidak mencukupi, sebelum baris pernah dibuat.

### 6. Backdate policy + path khusus diaudit

`visit` memakai `backdate_policy` (`max_days_back: 3`) dengan
`override_permission: visits.resolve-stale` — staf ber-perm khusus bisa
menyelesaikan kunjungan stale lewat aksi `resolve-stale` yang diaudit, tanpa
memperlebar policy global. Field computed `is_stale`/`stale_label` menandai
kartu di board.

## TypeScript Sidecar (Polyglot Demo)

`formspec-app.yaml` mengaktifkan runtime Node.js:

- `runtime: node`, `app-dir: app`, `app-entrypoint: src/app.ts` (dev →
  `npx tsx --watch`)
- `listen: unix_socket` — ctx listener + app endpoint via unix socket
- Aksi `otc-sale.sell` bisa di-handle oleh `app/src/handlers/otc_sell.ts`
  sebagai demo sidecar pattern (Go core ↔ Node app), selain versi Starlark
  `sell.star`
- Theme tambahan dimuat dari `../../ui-theme/*` tanpa menyalin ke
  `spec/modules/`

Tanpa `formspec-app.yaml`, `formspec dev` berjalan single-process
(Starlark saja).

## Menu & Navigasi

- Menu tidak lagi kind tersendiri — **menu suggestion** dideklarasikan di
  `spec.menu` tiap module (`clinic/module.yaml`, `pharmacy/module.yaml`),
  item `view:` me-resolve route dari resource yang dirujuk (tidak ada lagi
  `route:` string yang bisa drift)
- App mengadopsi via `type: module` atau menulis menu kustom
  (`klinik-portal`)
- Item menu tanpa authored View memakai **route escape hatch** ke
  derived entity-list route (mis. `/clinic/payments`)
- Menu entries derived untuk entity yang tidak disebut di menu
  (`daily-visit-summary` tidak dapat menu — characteristic summary)
