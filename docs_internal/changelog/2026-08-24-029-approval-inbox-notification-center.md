# 2026-08-24-029 — ApprovalInbox + NotificationCenter Kinds (5.13.3, 5.13.4)

## Apa yang diubah

Implementasi `kind: ApprovalInbox` (06-page-kinds.md §11) dan `kind: NotificationCenter`
(§12) end-to-end — keduanya zero-config pages.

**Backend:**

- `pkg/spec/spec.go` — `KindApprovalInbox`, `KindNotificationCenter` + `IsValidKind`.
- `internal/manifest/loader.go` — `KnownKinds`.
- `internal/ui/registry.go` — maps + register cases + `ResolveViewRoute`.
- `internal/ui/meta.go` — `Bundle.ApprovalInboxes`/`NotificationCenters` + build
  (zero-config — selalu ship, tidak entity-gated).
- `internal/ui/validate.go` — validasi struktural (filter field non-empty).
- `internal/ui/ui_test.go` — `TestCalendarApprovalNotificationKinds`.

**Frontend:**

- `types/manifest.ts` — `ApprovalInboxSpec`, `NotificationCenterSpec` + bundle fields.
- `shell/router.tsx` — routes `/approval-inbox/{name}` + `/notification-center/{name}`.
- `kinds/approval-inbox/ApprovalInboxRenderer.tsx` — pending approvals list, approve/reject
  inline, badge count, realtime; load dari entity approval konvensional bila ada, empty-state
  jujur bila tidak.
- `kinds/notification-center/NotificationCenterRenderer.tsx` — notification list, unread
  badge, mark-read, realtime; load dari entity notification konvensional bila ada.

## File terdampak

- `pkg/spec/spec.go`, `internal/manifest/loader.go`, `internal/ui/{registry,meta,validate}.go`
- `renderers/react-shadcn/src/kinds/approval-inbox/ApprovalInboxRenderer.tsx`
- `renderers/react-shadcn/src/kinds/notification-center/NotificationCenterRenderer.tsx`
- `renderers/react-shadcn/src/types/manifest.ts`, `shell/router.tsx`

## Referensi

- Plan: `docs/plan/fase5-completion.md` (WS-I)
- Todo: `docs/plan/todo.md` §5.13.3, §5.13.4
