// ─── Page Renderer ───
//
// Renders a kind: Page manifest — compositions of blocks or tabs.
//
// Two mutually exclusive variants (Frontend §3):
//   - `blocks`: grid layout of form/table/widget/html blocks
//   - `tabs`: tabbed interface, each tab has form/table/component
//
// Design doc §5.5 Page kind (F3)

import { lazy, Suspense, useEffect, useMemo, useState } from "react"
import { useParams, useSearchParams } from "react-router-dom"
import type {
  Entry,
  PageSpec,
  PageBlock,
  PageTab,
  PageBinds,
  AssetNeeds,
} from "@/types/manifest"
import { useMetaStore } from "@/stores/meta"
import { useSessionStore } from "@/stores/session"
import { can as checkPermission } from "@/engine/permissions"
import { resolveEntityRef } from "@/engine/entityRef"
import { apiGet } from "@/lib/api"
import { interpolate } from "@/lib/interpolate"
import { Skeleton } from "@/components/ui/skeleton"
import { EmptyState } from "@/components/ui/empty-state"
import { cn } from "@/lib/utils"
import { SectionBlockRenderer } from "@/components/sections/SectionBlocks"
import { AssetRenderer } from "@/shell/AssetRenderer"

/**
 * Resolve a `:param`-style placeholder (the only kind Page blocks author,
 * e.g. `form.id: ":id"`, `table.param: { patient_id: ":id" }`) against the
 * current route's params. Anything not starting with `:` is a literal value,
 * passed through unchanged.
 */
function resolveRouteParam(
  raw: string | undefined,
  routeParams: Readonly<Record<string, string | undefined>>,
): string | undefined {
  if (!raw) return raw
  if (!raw.startsWith(":")) return raw
  return routeParams[raw.slice(1)]
}

function resolveRouteParams(
  raw: Record<string, unknown> | undefined,
  routeParams: Readonly<Record<string, string | undefined>>,
): Record<string, string> {
  if (!raw) return {}
  const out: Record<string, string> = {}
  for (const [key, value] of Object.entries(raw)) {
    if (typeof value !== "string") continue
    const resolved = resolveRouteParam(value, routeParams)
    if (resolved != null) out[key] = resolved
  }
  return out
}

// Lazy load kind renderers to avoid circular deps
const TableRenderer = lazy(() => import("@/kinds/table/TableRenderer"))
const FormRenderer = lazy(() => import("@/kinds/form/FormRenderer"))

interface PageRendererProps {
  entry: Entry<PageSpec>
}

export default function PageRenderer({ entry }: PageRendererProps) {
  const me = useSessionStore((s) => s.me)

  // Permission check
  const permitted = useMemo(() => {
    if (!entry.spec.permissions?.length) return true
    if (!me) return false
    return entry.spec.permissions.some((p) =>
      checkPermission(p, me.permissions),
    )
  }, [entry.spec.permissions, me])

  if (!permitted) {
    return (
      <div className="flex items-center justify-center p-12">
        <div className="text-center">
          <h2 className="text-xl font-semibold">403</h2>
          <p className="text-sm text-muted-foreground mt-1">
            You don't have permission to view this page.
          </p>
        </div>
      </div>
    )
  }

  // Custom page — full-code page owned by an asset (06-page-kinds.md §13).
  // No blocks/tabs; the asset renders 100% of the markup and declares its
  // backend footprint via `binds`.
  if (entry.spec.mode === "custom") {
    return <CustomPage entry={entry} />
  }

  // Tabs variant
  if (entry.spec.tabs?.length) {
    return <PageTabs entry={entry} />
  }

  // Default: blocks variant
  return <PageBlocks entry={entry} />
}

// ── Custom Page Variant (mode: custom) ──

// Convert a custom page's `binds` footprint into the AssetNeeds shape the
// formspec client enforces client-side (07-component-kinds.md §4). Each bound
// entity becomes a `module.entity.*` action grant so the asset may touch any
// action on it; explicit actions and subscribe channels pass through.
function bindsToNeeds(binds?: PageBinds): AssetNeeds | undefined {
  if (!binds) return undefined
  const actions = [...(binds.actions ?? [])]
  for (const e of binds.entities ?? []) {
    actions.push(`${e}.*`)
  }
  return { actions, subscribe: binds.subscribe }
}

function CustomPage({ entry }: { entry: Entry<PageSpec> }) {
  const asset = entry.spec.asset
  const needs = useMemo(
    () => bindsToNeeds(entry.spec.binds),
    [entry.spec.binds],
  )

  if (!asset) {
    return (
      <div className="rounded-md border border-destructive/50 p-4 text-sm text-destructive">
        Custom page requires an `asset` (module-relative asset path).
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">
          {entry.spec.title}
        </h1>
        {entry.spec.description && (
          <p className="text-sm text-muted-foreground">
            {entry.spec.description}
          </p>
        )}
      </div>
      <AssetRenderer asset={asset} needs={needs} />
    </div>
  )
}

// ── Blocks Variant ──

function PageBlocks({ entry }: { entry: Entry<PageSpec> }) {
  const blocks = entry.spec.blocks ?? []
  const columns = entry.spec.layout?.columns ?? 1
  const routeParams = useParams()
  const { workspace = "default" } = useParams<{ workspace: string }>()
  const rootUrl = useMetaStore((s) => s.bundle?.app.root_url ?? "/")
  const getEntity = useMetaStore((s) => s.getEntity)
  const getForm = useMetaStore((s) => s.getForm)
  const getClient = useSessionStore((s) => s.getClient)

  // Title interpolation (e.g. "Pasien — {patient.name}") needs whichever
  // record its tokens reference — fetch the first form block that resolves
  // to a concrete record id, keyed by that block's own entity name so
  // "{patient.name}" and "{name}" both work.
  const titleNeedsData = /\{[\w.]+\}/.test(entry.spec.title ?? "")
  const [titleCtx, setTitleCtx] = useState<Record<string, unknown> | null>(null)

  useEffect(() => {
    if (!titleNeedsData) return
    let cancelled = false
    const load = async () => {
      for (const block of blocks) {
        if (!block.form) continue
        const formEntry = block.form.ref ? getForm(block.form.ref) : undefined
        const entityRef = formEntry?.spec.entity
        if (!entityRef) continue
        const entity = getEntity(
          ...resolveEntityRef(entityRef, formEntry?.module ?? entry.module),
        )
        const id = resolveRouteParam(block.form.id, routeParams)
        if (!entity || !id) continue
        try {
          const client = getClient()
          const record = await apiGet<Record<string, unknown>>(
            client,
            `${entity.module}/${entity.name}/${id}`,
          )
          if (!cancelled) setTitleCtx({ ...record, [entity.name]: record })
        } catch {
          // leave titleCtx null — interpolate() falls back to the literal token
        }
        return
      }
    }
    load()
    return () => {
      cancelled = true
    }
  }, [
    titleNeedsData,
    blocks,
    routeParams,
    getForm,
    getEntity,
    getClient,
    entry.module,
  ])

  const title = titleCtx
    ? interpolate(entry.spec.title, titleCtx)
    : entry.spec.title

  // Full-custom page (06-page-kinds.md §1) — a single `component:` block
  // (no blocks/tabs) renders full-bleed: no grid wrapper, no border.
  if (blocks.length === 1 && blocks[0].component) {
    return (
      <div className="space-y-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{title}</h1>
          {entry.spec.description && (
            <p className="text-sm text-muted-foreground">
              {entry.spec.description}
            </p>
          )}
        </div>
        <PageBlockRenderer
          block={blocks[0]}
          module={entry.module}
          routeParams={routeParams}
          workspace={workspace}
          rootUrl={rootUrl}
          bare
        />
      </div>
    )
  }

  // Master-detail split layout (06-page-kinds.md §1.1) — a master list block
  // on the left drives a detail block on the right via `binds`.
  if (entry.spec.layout?.mode === "split") {
    return <PageSplit entry={entry} title={title} />
  }

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{title}</h1>
        {entry.spec.description && (
          <p className="text-sm text-muted-foreground">
            {entry.spec.description}
          </p>
        )}
      </div>

      <div
        className="grid gap-4"
        style={{ gridTemplateColumns: `repeat(${columns}, 1fr)` }}
      >
        {blocks.map((block, idx) => (
          <PageBlockRenderer
            key={idx}
            block={block}
            module={entry.module}
            routeParams={routeParams}
            workspace={workspace}
            rootUrl={rootUrl}
          />
        ))}
      </div>
    </div>
  )
}

// ── Split Variant (layout.mode: split) ──

// Master-detail: a master Table block's row selection drives a detail block
// (Form or Table) via `binds: { source, param }` — no route navigation
// (06-page-kinds.md §1.1). The detail refetches on selection change; without a
// selection it shows an empty-state.
function PageSplit({
  entry,
  title,
}: {
  entry: Entry<PageSpec>
  title: string
}) {
  const blocks = entry.spec.blocks ?? []
  const routeParams = useParams()
  const { workspace = "default" } = useParams<{ workspace: string }>()
  const rootUrl = useMetaStore((s) => s.bundle?.app.root_url ?? "/")

  // The detail block is the one carrying `binds`; its `source` names the
  // master Table block's `ref`.
  const detailBlock = blocks.find(
    (b) => b.form?.binds || b.table?.binds || b.component?.binds,
  )
  const binds = detailBlock?.form?.binds ?? detailBlock?.table?.binds
  const masterBlock = blocks.find((b) => b.table?.ref === binds?.source)

  const [selectedRecord, setSelectedRecord] = useState<Record<
    string,
    unknown
  > | null>(null)

  // Detail id = the selected master record's `binds.param` field (usually id).
  const detailId =
    selectedRecord && binds?.param
      ? String(selectedRecord[binds.param] ?? "")
      : undefined

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{title}</h1>
        {entry.spec.description && (
          <p className="text-sm text-muted-foreground">
            {entry.spec.description}
          </p>
        )}
      </div>

      <div className="grid gap-4 lg:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]">
        {/* Master — narrow left */}
        <div className="rounded-md border p-4">
          {masterBlock ? (
            <Suspense fallback={<Skeleton className="h-48" />}>
              <PageBlockRenderer
                block={masterBlock}
                module={entry.module}
                routeParams={routeParams}
                workspace={workspace}
                rootUrl={rootUrl}
                bare
                onSelect={setSelectedRecord}
              />
            </Suspense>
          ) : (
            <p className="text-sm text-muted-foreground">
              Master block not found (binds.source: {binds?.source})
            </p>
          )}
        </div>

        {/* Detail — wide right */}
        <div className="rounded-md border p-4">
          {detailBlock ? (
            detailId ? (
              <Suspense fallback={<Skeleton className="h-48" />}>
                <PageBlockRenderer
                  block={detailBlock}
                  module={entry.module}
                  routeParams={routeParams}
                  workspace={workspace}
                  rootUrl={rootUrl}
                  bare
                  overrideId={detailId}
                  overrideFilters={
                    selectedRecord && binds?.param
                      ? { [binds.param]: detailId }
                      : undefined
                  }
                />
              </Suspense>
            ) : (
              <EmptyState
                title="Pilih baris di panel kiri"
                description="Detail akan muncul setelah Anda memilih satu baris pada daftar master."
              />
            )
          ) : (
            <p className="text-sm text-muted-foreground">
              Detail block not found (no block declares `binds`).
            </p>
          )}
        </div>
      </div>
    </div>
  )
}

function PageBlockRenderer({
  block,
  module,
  routeParams,
  workspace,
  rootUrl,
  bare,
  onSelect,
  overrideId,
  overrideFilters,
}: {
  block: PageBlock
  module: string
  routeParams: Readonly<Record<string, string | undefined>>
  workspace: string
  rootUrl: string
  /** Skip the outer border wrapper — used by the split layout whose panels
   *  already provide the border, and by full-custom pages. */
  bare?: boolean
  /** Master-detail: fired with the clicked row's record (Table blocks). */
  onSelect?: (record: Record<string, unknown>) => void
  /** Master-detail: record id injected into a Form block, taking precedence
   *  over the `:id` route param (06-page-kinds.md §1.1). */
  overrideId?: string
  /** Master-detail: extra fixed filters merged into a Table block's fetch. */
  overrideFilters?: Record<string, string>
}) {
  const getEntity = useMetaStore((s) => s.getEntity)
  const getForm = useMetaStore((s) => s.getForm)
  const getTable = useMetaStore((s) => s.getTable)

  const wrap = (inner: React.ReactNode) =>
    bare ? inner : <div className="rounded-md border p-4">{inner}</div>

  // Form block — entity resolved from the referenced Form manifest's spec.entity
  if (block.form) {
    const formEntry = block.form.ref ? getForm(block.form.ref) : undefined
    const entityRef = formEntry?.spec.entity
    const entity = entityRef
      ? getEntity(...resolveEntityRef(entityRef, formEntry?.module ?? module))
      : undefined

    if (entity) {
      return wrap(
        <Suspense fallback={<Skeleton className="h-32" />}>
          <FormRenderer
            entity={entity}
            mode={(block.form?.mode as "create" | "edit" | "view") ?? "view"}
            id={overrideId ?? resolveRouteParam(block.form?.id, routeParams)}
            formRef={block.form?.ref}
          />
        </Suspense>,
      )
    }
    return (
      <div className="rounded-md border p-4 text-sm text-muted-foreground">
        Form: {block.form.ref}
      </div>
    )
  }

  // Table block — entity resolved from the referenced Table manifest's spec.entity
  if (block.table) {
    const tableEntry = block.table.ref ? getTable(block.table.ref) : undefined
    const entityRef = tableEntry?.spec.entity
    const entity = entityRef
      ? getEntity(...resolveEntityRef(entityRef, tableEntry?.module ?? module))
      : undefined
    // eslint-disable-next-line react-hooks/rules-of-hooks -- block.table is stable per element (keyed list), so this hook order is consistent across renders
    const fixedFilters = useMemo(
      () => ({
        ...resolveRouteParams(block.table?.param, routeParams),
        ...overrideFilters,
      }),
      [block.table?.param, routeParams, overrideFilters],
    )

    if (entity) {
      return wrap(
        <Suspense fallback={<Skeleton className="h-48" />}>
          <TableRenderer
            entity={entity}
            hideTitle
            fixedFilters={fixedFilters}
            onSelect={onSelect}
          />
        </Suspense>,
      )
    }
    return (
      <div className="rounded-md border p-4 text-sm text-muted-foreground">
        Table: {block.table.ref}
      </div>
    )
  }

  // Widget block
  if (block.widget) {
    return (
      <div className="rounded-md border p-4">
        <h3 className="text-sm font-medium mb-2">{block.widget.ref}</h3>
        <p className="text-xs text-muted-foreground">Widget renderer</p>
      </div>
    )
  }

  // Component block (custom asset component, todo 5.9.1)
  if (block.component) {
    if (block.component.asset) {
      return (
        <AssetRenderer
          asset={block.component.asset}
          props={block.component.props}
          needs={block.component.needs}
        />
      )
    }
    return (
      <div className="rounded-md border p-4 text-sm text-muted-foreground">
        Component: {block.component.ref || "unknown"}
      </div>
    )
  }

  // HTML block
  if (block.html) {
    return (
      <div
        className="rounded-md border p-4 prose prose-sm max-w-none"
        dangerouslySetInnerHTML={{ __html: block.html }}
      />
    )
  }

  // Section block — declarative presentation (06-page-kinds.md §1). Rendered
  // full-bleed (no border wrapper) since sections are full-width regions.
  if (block.section) {
    return (
      <SectionBlockRenderer
        block={block.section}
        workspace={workspace}
        rootUrl={rootUrl}
      />
    )
  }

  return null
}

// ── Tabs Variant ──

function PageTabs({ entry }: { entry: Entry<PageSpec> }) {
  const [searchParams, setSearchParams] = useSearchParams()
  const activeTab = searchParams.get("tab") ?? entry.spec.tabs?.[0]?.label ?? ""

  const tabs = entry.spec.tabs ?? []

  if (tabs.length === 0) {
    return <PageBlocks entry={entry} />
  }

  const activeIndex = tabs.findIndex((t) => t.label === activeTab)

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">
          {entry.spec.title}
        </h1>
        {entry.spec.description && (
          <p className="text-sm text-muted-foreground">
            {entry.spec.description}
          </p>
        )}
      </div>

      {/* Tab bar */}
      <div className="flex border-b">
        {tabs.map((tab) => (
          <button
            key={tab.label}
            onClick={() => {
              const next = new URLSearchParams(searchParams)
              next.set("tab", tab.label)
              setSearchParams(next)
            }}
            className={cn(
              "px-4 py-2 text-sm font-medium transition-colors",
              activeTab === tab.label
                ? "border-b-2 border-primary text-primary"
                : "text-muted-foreground hover:text-foreground",
            )}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Active tab content. `key={activeTab}` forces a full remount on tab
          switch — tabs can point at different records of the same entity
          (Configuration Page pattern), and without it React would reuse the
          previous tab's FormRenderer instance, briefly showing its
          already-loaded record instead of a loading state. */}
      {activeIndex >= 0 && (
        <div className="pt-2">
          <Suspense fallback={<Skeleton className="h-48" />}>
            <TabContent
              key={activeTab}
              tab={tabs[activeIndex]}
              module={entry.module}
            />
          </Suspense>
        </div>
      )}
    </div>
  )
}

function TabContent({ tab, module }: { tab: PageTab; module: string }) {
  const getEntity = useMetaStore((s) => s.getEntity)
  const getForm = useMetaStore((s) => s.getForm)
  const getTable = useMetaStore((s) => s.getTable)

  if (tab.form) {
    // Resolve entity from the referenced Form manifest's spec.entity
    const formEntry = tab.form.ref ? getForm(tab.form.ref) : undefined
    const entityRef = formEntry?.spec.entity
    const entity = entityRef
      ? getEntity(...resolveEntityRef(entityRef, formEntry?.module ?? module))
      : undefined

    if (entity) {
      return (
        <Suspense fallback={<Skeleton className="h-32" />}>
          <FormRenderer
            entity={entity}
            mode={(tab.form?.mode as "create" | "edit" | "view") ?? "view"}
            id={tab.form?.id}
            formRef={tab.form?.ref}
          />
        </Suspense>
      )
    }
    return (
      <div className="rounded-md border p-4 text-sm text-muted-foreground">
        Form: {tab.form.ref}
      </div>
    )
  }

  if (tab.table) {
    // Resolve entity from the referenced Table manifest's spec.entity
    const tableEntry = tab.table.ref ? getTable(tab.table.ref) : undefined
    const entityRef = tableEntry?.spec.entity
    const entity = entityRef
      ? getEntity(...resolveEntityRef(entityRef, tableEntry?.module ?? module))
      : undefined

    if (entity) {
      return (
        <Suspense fallback={<Skeleton className="h-48" />}>
          <TableRenderer entity={entity} hideTitle />
        </Suspense>
      )
    }
    return (
      <div className="rounded-md border p-4 text-sm text-muted-foreground">
        Table: {tab.table.ref}
      </div>
    )
  }

  if (tab.component) {
    return (
      <div className="rounded-md border border-dashed p-8 text-center text-sm text-muted-foreground">
        Component: {tab.component.ref || tab.component.asset || "unknown"}
      </div>
    )
  }

  return null
}
