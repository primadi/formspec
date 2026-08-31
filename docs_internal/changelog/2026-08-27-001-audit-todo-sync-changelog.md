# 2026-08-27-001 — Audit & Sinkronisasi todo.md dengan Realita Kode

## Apa yang diubah

Audit menyeluruh item ⬜ di `docs/plan/todo.md` terhadap changelog + verifikasi kode
(grep + baca implementasi). Baseline: `go test ./...` 864 pass, vitest 155 pass, 0 gagal.

**Fase 5 — dinyatakan COMPLETE (5.1–5.16).** Item yang tadinya masih ⬜ ternyata sudah
terimplementasi sesuai `docs/plan/fase5-completion.md` (WS-A s.d. WS-J) tapi todo belum
di-update. Yang ditandai ✅ kini:

- 5.4.2/5.4.3 inline & batch editing (TableRenderer)
- 5.5.3 drag_guard + 5.5.5 zero-config Kanban (KanbanRenderer eval drag_guard)
- 5.6.1–5.6.6 Calendar kind end-to-end (5.6.7 tetap ⏸️ per spec)
- 5.7.3 customizable dashboard + 5.7.4 widget catalog visibility (prefs store)
- 5.11.1–5.11.3 + 5.11.5 FormSpecExpr (check.go grammar validation, runtime error banner)
- 5.13.1a source.filter, 5.13.2 Print PDF server-side, 5.13.3 ApprovalInbox,
  5.13.4 NotificationCenter (renderers di `renderers/react-shadcn/src/kinds/`)
- 5.16.1–5.16.3 renderer registry & resolution (`internal/manifest/renderer.go`)

**Fase 7 — hampir lengkap.** Ditandai ✅ (changelog 2026-08-25-001 s/d 2026-08-26-001,
diverifikasi di kode): 7.1 Service runtime, 7.2 Config runtime, 7.3 Subscription
(+streaming+dynamic+channels), 7.4 Workflow (+audit+escalation), 7.5 state machine unify
(`internal/starlark/guard.go`), 7.6 Webhook, 7.7 Integrator (+saga), 7.8 Hook engine,
7.9.6 katalog rule L1–L3, 7.10 denormalisasi finansial, 7.11 period closing
(`internal/period/`), 7.12 rate limiter (`internal/api/resource_ratelimit.go`),
7.13 async job tracker (`internal/job/tracker.go`), 7.14 sandbox limits
(`internal/starlark/limits.go`), 7.16 money type (`pkg/spec/money.go`), 7.17.2 storage
spec enforcement.

**Yang TETAP terbuka (terverifikasi memang belum ada):**

- 7.9.1–7.9.4 (L4–L6 validation) — kontrak deklarasi belum dispesifikasikan di `pkg/spec`
- 7.9.5 — envelope `ErrorDetailItem` ada tapi `Level` hardcoded, `Field` belum diisi
- 7.15.1 — field `spec.runtime` belum ada di `ModuleSpec` (spawn sidecar sudah jalan)
- 7.17.3 — server-side transform belum dikerjakan
- 7.18 KindDefinition runtime, 7.19 Mockup runtime — hanya enum/allowlist, tidak ada eksekusi

Header status + progress lines Fase 5/7 di-update; `Last Updated` → 2026-08-27.

## Kenapa diubah

Workflow Discipline §6: hasil audit harus tercatat dan gap yang bisa diselesaikan
(sinkronisasi status) diselesaikan sebelum mengerjakan pekerjaan baru, supaya prioritas
iterasi berikutnya akurat.

## File terdampak

- `docs/plan/todo.md` (status saja — tidak ada perubahan kode)

## Referensi

- Plan: `docs/plan/todo.md` (dokumen itu sendiri) · `docs/plan/fase5-completion.md`
- Changelog sumber: 2026-08-24-027 s/d -032, 2026-08-25-001 s/d -021, 2026-08-26-001
