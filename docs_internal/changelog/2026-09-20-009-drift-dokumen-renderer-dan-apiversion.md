# 8.2 — Kebersihan drift dokumen (#19 + #20)

**Tanggal:** 2026-09-20 · **TODO:** `examples/kafe/gaps_found/TODO.md` 8.2 · **Plan:** `docs_internal/plan/kafe-sisa-gap.md`

## #19 — Dokumen renderer diselaraskan dengan kode

Dua dokumen diklaim kedaluwarsa; setelah diperiksa per klaim terhadap kode
renderer, **keduanya memang salah**, dan yang salah bukan hal kecil:

- `docs/renderers/shadcn-shell/03-kind-renderers.md` masih bertanggal
  **2026-07-16**. Yang diperbaiki: §1 (`engine/registry.tsx` sudah **dihapus**,
  bukan "ada tapi tidak dipanggil"); baris `Table` (`Form.render` kini
  **dihormati** — `modal`/`drawer` → `OverlayHost` — dan navigasi memakai
  `useSurface().surfacePath`, bukan prefiks `/_admin` hardcode);
  `Dashboard`/`Widget` (bukan lagi placeholder: `metric`/`chart`/`list`/`table`
  dirender, metric menghitung agregat money-aware, chart tanpa library
  charting); `Report` (baris totals **dirender** — bug lama sudah hilang);
  §4 ditulis ulang dari katalog sebenarnya (**24 form widget + 4 table-cell
  widget**, sumber `src/widgets/catalog.ts` ↔ `pkg/spec/widget.go`, gerbang
  paritas `catalog.test.tsx`, alias tipe field, `UnknownWidget`); §5
  (konfirmasi kini pakai `ConfirmDialog` shadcn, bukan `window.confirm()`).
- `docs/renderers/realtime.md` §5 menyatakan Calendar / ApprovalInbox /
  NotificationCenter **"belum diimplementasikan"** padahal ketiga renderer itu
  ada dan ketiganya memakai `useRealtime` (di-gate `spec.realtime`, refetch
  silent — dibaca langsung di `CalendarRenderer.tsx:203`,
  `ApprovalInboxRenderer.tsx:88`, `NotificationCenterRenderer.tsx:87`). Tabel §5
  kini memuat **tujuh** kind yang berjalan, §7 kehilangan bullet yang keliru,
  dan §8 menyebut kelima direktori renderer yang benar.

## #20 — `apiVersion`/`spec.version`: satu drift nyata, satu klaim gugur

**Klaim ledger gugur sebagian.** `spec.version` di `docs/kind/data/Entity.md`
dan `docs/kind/curation/Module.md` ternyata **benar**: `EntitySpec.Version`
(`pkg/spec/entity.go:124`) dan `ModuleSpec.Version` (`pkg/spec/resources.go:14`)
memang ada dan wajib, dan spec kafe mengisinya (`version: v1` /
`version: 1.0.0`). Begitu pula `formspec-app.yaml` — itu config CLI dev/serve
(bukan kind manifest), persis seperti yang didokumentasikan
(`docs/spec/platform/08-project-layout.md` §1.1). Tidak ada yang diubah di sana.

**Drift yang benar-benar ada** justru di tempat lain, dan keduanya membuat
manifest yang dihasilkan **ditolak**: `formspec.dev/v1alpha1` ditolak eksplisit
oleh `internal/schemaregistry.ParseVersion` (`apiVersionRegexp` =
`^formspec\.dev/(v\d+)$`).

| Lokasi | Sebelum | Sesudah |
| --- | --- | --- |
| `pkg/spec/spec.go` (`const APIVersion`) | `formspec.dev/v1alpha1` | `formspec.dev/v1` |
| `docs/spec/frontend/02-visual-spec-kind.md` (3×) | `apiVersion: formspec/v1` | `apiVersion: formspec.dev/v1` |
| `docs/spec/frontend/03-renderer-kind.md` | `apiVersion: formspec/v1` | `apiVersion: formspec.dev/v1` |
| `docs/guides/authoring-a-page-renderer.md` | `apiVersion: formspec/v1` | `apiVersion: formspec.dev/v1` |

Grup `formspec/v1` (tanpa `.dev`) bukan sekadar beda ejaan: ia gagal
`ParseVersion`, jadi contoh YAML di dokumen itu tidak akan lolos
`formspec validate`. (`docs/architecture/06-k8s-operator.md` tetap
`formspec.dev/v1alpha1` — itu **CRD** Kubernetes di grup yang sama, bukan
manifest FormSpec.)

## Bukti

| Perintah | Hasil |
| --- | --- |
| `go test ./pkg/spec/ -run TestAPIVersionIsStable -v` | **PASS** (test baru `pkg/spec/apiversion_test.go` menolak nilai pra-stabil) |
| `go build ./...` | exit 0 |
| `grep -rn "formspec/v[0-9]" docs/` | tidak ada lagi (sebelumnya 5 temuan di 3 berkas) |
| `grep -rn "useRealtime" src/kinds/` | 7 renderer (+1 di dashboard untuk widget) — dasar tabel §5 `realtime.md` |
