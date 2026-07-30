// ─── Page Renderer ───
//
// Renders a kind: Page manifest — compositions of blocks or tabs.
//
// Two mutually exclusive variants (Frontend §3):
//   - `blocks`: grid layout of form/table/widget/html blocks
//   - `tabs`: tabbed interface, each tab has form/table/component
//
// Design doc §5.5 Page kind (F3)

import { lazy, Suspense, useMemo } from "react"
import { useSearchParams } from "react-router-dom"
import type { Entry, PageSpec, PageBlock, PageTab } from "@/types/manifest"
import { useMetaStore } from "@/stores/meta"
import { useSessionStore } from "@/stores/session"
import { can as checkPermission } from "@/engine/permissions"
import { resolveEntityRef } from "@/engine/entityRef"
import { Skeleton } from "@/components/ui/skeleton"
import { cn } from "@/lib/utils"

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
    return entry.spec.permissions.some((p) => checkPermission(p, me.permissions))
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

  // Tabs variant
  if (entry.spec.tabs?.length) {
    return <PageTabs entry={entry} />
  }

  // Default: blocks variant
  return <PageBlocks entry={entry} />
}

// ── Blocks Variant ──

function PageBlocks({ entry }: { entry: Entry<PageSpec> }) {
  const blocks = entry.spec.blocks ?? []
  const columns = entry.spec.layout?.columns ?? 1

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{entry.spec.title}</h1>
        {entry.spec.description && (
          <p className="text-sm text-muted-foreground">{entry.spec.description}</p>
        )}
      </div>

      <div
        className="grid gap-4"
        style={{ gridTemplateColumns: `repeat(${columns}, 1fr)` }}
      >
        {blocks.map((block, idx) => (
          <PageBlockRenderer key={idx} block={block} module={entry.module} />
        ))}
      </div>
    </div>
  )
}

function PageBlockRenderer({ block, module }: { block: PageBlock; module: string }) {
  const getEntity = useMetaStore((s) => s.getEntity)
  const getForm = useMetaStore((s) => s.getForm)
  const getTable = useMetaStore((s) => s.getTable)

  // Form block — entity resolved from the referenced Form manifest's spec.entity
  if (block.form) {
    const formEntry = block.form.ref ? getForm(block.form.ref) : undefined
    const entityRef = formEntry?.spec.entity
    const entity = entityRef
      ? getEntity(...resolveEntityRef(entityRef, formEntry?.module ?? module))
      : undefined

    if (entity) {
      return (
        <div className="rounded-md border p-4">
          <Suspense fallback={<Skeleton className="h-32" />}>
            <FormRenderer
              entity={entity}
              mode={(block.form?.mode as "create" | "edit" | "view") ?? "view"}
              id={block.form?.id}
              formRef={block.form?.ref}
            />
          </Suspense>
        </div>
      )
    }
    return <div className="rounded-md border p-4 text-sm text-muted-foreground">Form: {block.form.ref}</div>
  }

  // Table block — entity resolved from the referenced Table manifest's spec.entity
  if (block.table) {
    const tableEntry = block.table.ref ? getTable(block.table.ref) : undefined
    const entityRef = tableEntry?.spec.entity
    const entity = entityRef
      ? getEntity(...resolveEntityRef(entityRef, tableEntry?.module ?? module))
      : undefined

    if (entity) {
      return (
        <div className="rounded-md border p-4">
          <Suspense fallback={<Skeleton className="h-48" />}>
            <TableRenderer entity={entity} hideTitle />
          </Suspense>
        </div>
      )
    }
    return <div className="rounded-md border p-4 text-sm text-muted-foreground">Table: {block.table.ref}</div>
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

  // Component block (custom component placeholder)
  if (block.component) {
    return (
      <div className="rounded-md border border-dashed p-4">
        <p className="text-sm text-muted-foreground text-center">
          Custom component: {block.component.asset || block.component.ref || "unknown"}
        </p>
        <p className="text-xs text-muted-foreground text-center mt-1">
          Component blocks supported in Fase 4.F6
        </p>
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
        <h1 className="text-2xl font-bold tracking-tight">{entry.spec.title}</h1>
        {entry.spec.description && (
          <p className="text-sm text-muted-foreground">{entry.spec.description}</p>
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
            <TabContent key={activeTab} tab={tabs[activeIndex]} module={entry.module} />
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
    return <div className="rounded-md border p-4 text-sm text-muted-foreground">Form: {tab.form.ref}</div>
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
    return <div className="rounded-md border p-4 text-sm text-muted-foreground">Table: {tab.table.ref}</div>
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

