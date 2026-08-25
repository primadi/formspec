// ─── Dynamic Route Builder ───
//
// Builds React Router route configuration from the Meta bundle:
//   - Each `kind: Page` → route from `spec.route`
//   - Derived CRUD routes per entity (list, detail, create, edit)
//   - Wizard/Kanban/Timeline routes
//
// Design doc §5.2 step 3.

import { lazy, Suspense } from "react"
import type { RouteObject } from "react-router-dom"
import { Skeleton } from "@/components/ui/skeleton"
import type { MetaBundle } from "@/types/manifest"

// ─── Lazy-loaded kind renderers ──

const TableRenderer = lazy(() => import("@/kinds/table/TableRenderer"))
const FormRenderer = lazy(() => import("@/kinds/form/FormRenderer"))
const DetailPage = lazy(() => import("@/kinds/page/DetailPage"))
const PageRenderer = lazy(() => import("@/kinds/page/PageRenderer"))
const DashboardRenderer = lazy(
  () => import("@/kinds/dashboard/DashboardRenderer"),
)
const WidgetRenderer = lazy(() => import("@/kinds/widget/WidgetRenderer"))
const WizardRenderer = lazy(() => import("@/kinds/wizard/WizardRenderer"))
const KanbanRenderer = lazy(() => import("@/kinds/kanban/KanbanRenderer"))
const TimelineRenderer = lazy(() => import("@/kinds/timeline/TimelineRenderer"))
const ReportRenderer = lazy(() => import("@/kinds/report/ReportRenderer"))
const PrintRenderer = lazy(() => import("@/kinds/print/PrintRenderer"))
const ListingRenderer = lazy(() => import("@/kinds/listing/ListingRenderer"))
const CalendarRenderer = lazy(() => import("@/kinds/calendar/CalendarRenderer"))
const ApprovalInboxRenderer = lazy(
  () => import("@/kinds/approval-inbox/ApprovalInboxRenderer"),
)
const NotificationCenterRenderer = lazy(
  () => import("@/kinds/notification-center/NotificationCenterRenderer"),
)

function Loading() {
  return (
    <div className="flex items-center justify-center p-8">
      <div className="space-y-4 w-full max-w-lg">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-4 w-96" />
        <Skeleton className="h-32 w-full" />
      </div>
    </div>
  )
}

// ── Route Builder ──

export interface RouteBuilderOptions {
  bundle: MetaBundle
  basePath: string // e.g. "/acme" or "/acme/_admin"
}

/**
 * Build a flat array of RouteObject from the Meta bundle.
 */
export function buildRoutes(options: RouteBuilderOptions): RouteObject[] {
  const { bundle, basePath } = options
  const routes: RouteObject[] = []

  // 1. Page routes
  for (const page of bundle.pages ?? []) {
    const route = page.spec.route.startsWith("/")
      ? page.spec.route
      : `/${page.spec.route}`

    routes.push({
      path: `${basePath}${route}`,
      Component: () => (
        <Suspense fallback={<Loading />}>
          <PageRenderer entry={page} />
        </Suspense>
      ),
    })
  }

  // 2. Derived CRUD routes per entity
  for (const entity of bundle.entities ?? []) {
    const base = `${basePath}/${entity.module}/${entity.plural}`

    // List route
    routes.push({
      path: base,
      Component: () => (
        <Suspense fallback={<Loading />}>
          <TableRenderer entity={entity} />
        </Suspense>
      ),
    })

    // Create route (derived form)
    routes.push({
      path: `${base}/new`,
      Component: () => (
        <Suspense fallback={<Loading />}>
          <FormRenderer entity={entity} mode="create" />
        </Suspense>
      ),
    })

    // Detail route
    routes.push({
      path: `${base}/:id`,
      Component: () => (
        <Suspense fallback={<Loading />}>
          <DetailPage entity={entity} />
        </Suspense>
      ),
    })

    // Edit route
    routes.push({
      path: `${base}/:id/edit`,
      Component: () => (
        <Suspense fallback={<Loading />}>
          <FormRenderer entity={entity} mode="edit" />
        </Suspense>
      ),
    })
  }

  // 3. Dashboard routes
  for (const dashboard of bundle.dashboards ?? []) {
    routes.push({
      path: `${basePath}/dashboard/${dashboard.name}`,
      Component: () => (
        <Suspense fallback={<Loading />}>
          <DashboardRenderer entry={dashboard} />
        </Suspense>
      ),
    })
  }

  // 4. Widget routes
  for (const widget of bundle.widgets ?? []) {
    routes.push({
      path: `${basePath}/widget/${widget.name}`,
      Component: () => (
        <Suspense fallback={<Loading />}>
          <WidgetRenderer entry={widget} />
        </Suspense>
      ),
    })
  }

  // 5. Wizard routes (kind: Wizard)
  for (const wizard of bundle.wizards ?? []) {
    routes.push({
      path: `${basePath}/wizard/${wizard.name}`,
      Component: () => (
        <Suspense fallback={<Loading />}>
          <WizardRenderer entry={wizard} />
        </Suspense>
      ),
    })
  }

  // 6. Kanban routes
  for (const kanban of bundle.kanbans ?? []) {
    routes.push({
      path: `${basePath}/kanban/${kanban.name}`,
      Component: () => (
        <Suspense fallback={<Loading />}>
          <KanbanRenderer entry={kanban} />
        </Suspense>
      ),
    })
  }

  // 7. Timeline routes
  for (const timeline of bundle.timelines ?? []) {
    routes.push({
      path: `${basePath}/timeline/${timeline.name}`,
      Component: () => (
        <Suspense fallback={<Loading />}>
          <TimelineRenderer entry={timeline} />
        </Suspense>
      ),
    })
  }

  // 8. Report routes
  for (const report of bundle.reports ?? []) {
    routes.push({
      path: `${basePath}/report/${report.name}`,
      Component: () => (
        <Suspense fallback={<Loading />}>
          <ReportRenderer entry={report} />
        </Suspense>
      ),
    })
  }

  // 9. Print routes — with and without `:id` (browse the template with no
  // data, or print a specific record — PrintRenderer already handles both
  // via `useParams().id` being present or not).
  for (const print of bundle.prints ?? []) {
    const printElement = (
      <Suspense fallback={<Loading />}>
        <PrintRenderer entry={print} />
      </Suspense>
    )
    routes.push({
      path: `${basePath}/print/${print.name}`,
      Component: () => printElement,
    })
    routes.push({
      path: `${basePath}/print/${print.name}/:id`,
      Component: () => printElement,
    })
  }

  // 10. Listing routes — public catalog (06-page-kinds.md §10). A listing is
  // a read-only list (no row/bulk actions, no create). Clicking a row
  // navigates to the entity's public detail route.
  for (const listing of bundle.listings ?? []) {
    routes.push({
      path: `${basePath}/listing/${listing.name}`,
      Component: () => (
        <Suspense fallback={<Loading />}>
          <ListingRenderer entry={listing} />
        </Suspense>
      ),
    })
  }

  // 11. Calendar routes (06-page-kinds.md §5)
  for (const calendar of bundle.calendars ?? []) {
    routes.push({
      path: `${basePath}/calendar/${calendar.name}`,
      Component: () => (
        <Suspense fallback={<Loading />}>
          <CalendarRenderer entry={calendar} />
        </Suspense>
      ),
    })
  }

  // 12. ApprovalInbox routes (06-page-kinds.md §11) — zero-config
  for (const inbox of bundle.approval_inboxes ?? []) {
    routes.push({
      path: `${basePath}/approval-inbox/${inbox.name}`,
      Component: () => (
        <Suspense fallback={<Loading />}>
          <ApprovalInboxRenderer entry={inbox} />
        </Suspense>
      ),
    })
  }

  // 13. NotificationCenter routes (06-page-kinds.md §12) — zero-config
  for (const center of bundle.notification_centers ?? []) {
    routes.push({
      path: `${basePath}/notification-center/${center.name}`,
      Component: () => (
        <Suspense fallback={<Loading />}>
          <NotificationCenterRenderer entry={center} />
        </Suspense>
      ),
    })
  }

  return routes
}

// ── Stub kind renderer exports ──
// These are placeholder files that the lazy imports reference.
// They'll be replaced with real implementations in Fase 4.F3.

// We need to ensure the import paths resolve. Let's create stub modules.
