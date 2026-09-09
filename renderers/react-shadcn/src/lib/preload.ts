// ─── Renderer Preloader ───
//
// Eagerly imports ALL kind renderers so their chunks download in the
// background during boot. This matters beyond perceived latency: a
// not-yet-loaded renderer makes React suspend into its Skeleton fallback,
// and a page transition started at that moment captures/animates the
// skeleton instead of the real page (the intermittent "empty wireframe"
// on first navigation). Once cached, the fallback never appears again —
// which is why the glitch is hard to reproduce.
//
// Calling `preloadCommonRenderers()` early in the app lifecycle triggers
// Vite to fetch these modules *before* the user clicks a menu item.
// React.lazy in router.tsx reuses the already-loading/loaded module.

// Deferred to idle so the kick-off does not compete with the critical
// boot fetches (/_meta/me, /_meta/ui).
export function preloadCommonRenderers(): void {
  const kick = () => {
    // These dynamic imports are deliberately not awaited — we just want to
    // start the HTTP request for the chunk.
    import("@/kinds/table/TableRenderer")
    import("@/kinds/form/FormRenderer")
    import("@/kinds/page/DetailPage")
    import("@/kinds/page/PageRenderer")
    import("@/kinds/dashboard/DashboardRenderer")
    import("@/kinds/widget/WidgetRenderer")
    import("@/kinds/wizard/WizardRenderer")
    import("@/kinds/kanban/KanbanRenderer")
    import("@/kinds/timeline/TimelineRenderer")
    import("@/kinds/report/ReportRenderer")
    import("@/kinds/print/PrintRenderer")
    import("@/kinds/listing/ListingRenderer")
    import("@/kinds/calendar/CalendarRenderer")
    import("@/kinds/approval-inbox/ApprovalInboxRenderer")
    import("@/kinds/notification-center/NotificationCenterRenderer")
  }
  if (typeof requestIdleCallback === "function") {
    requestIdleCallback(kick, { timeout: 2000 })
  } else {
    setTimeout(kick, 0)
  }
}
