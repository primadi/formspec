// ─── Grants Editor Widget ───
//
// Checkbox-tree editor for a Role's `grants` field (page → tab → action).
// Builds the tree from the meta bundle's pages (block pages list actions
// directly; tabbed pages group actions per tab). Writes the grants JSON in
// the same shape the backend Materializer consumes:
//
//   [{ "page": "...", "actions": [{"name": "create"}],
//      "tabs": [{ "tab": "...", "actions": [{"name": "list"}] }] }]

import { useMemo, useState } from "react"
import { Checkbox } from "@/components/ui/checkbox"
import { useMetaStore } from "@/stores/meta"
import type { MetaBundle, PageSpec, Entry, BlockRef } from "@/types/manifest"

interface GrantsEditorProps {
  value?: unknown
  onChange?: (value: unknown) => void
  readonly?: boolean
  error?: string
}

interface GrantAction {
  name: string
}

interface GrantTab {
  tab: string
  actions: GrantAction[]
}

interface Grant {
  page: string
  actions?: GrantAction[]
  tabs?: GrantTab[]
}

interface PageModel {
  page: string
  label: string
  // Block page: flat actions.
  actions: string[]
  // Tabbed page: tab label → actions.
  tabs: { label: string; actions: string[] }[]
}

// Derive the actions a page exposes (simplified footprint — mirrors the
// backend Materializer: form mode + custom form actions, table list + custom
// table actions).
function blockActions(
  block: { form?: BlockRef; table?: BlockRef },
  bundle: MetaBundle,
): string[] {
  const out: string[] = []
  if (block.form?.ref) {
    const form = bundle.forms.find((f) => f.name === block.form?.ref)
    if (form) {
      out.push(block.form?.mode ?? form.spec.mode ?? "create")
      form.spec.actions?.forEach((a) => out.push(a.action))
    }
  }
  if (block.table?.ref) {
    const table = bundle.tables.find((t) => t.name === block.table?.ref)
    if (table) {
      out.push("list")
      table.spec.row_actions?.forEach((a) => out.push(a.action))
      table.spec.bulk_actions?.forEach((a) => out.push(a.action))
    }
  }
  return [...new Set(out)]
}

function buildPageModels(bundle: MetaBundle): PageModel[] {
  return (bundle.pages ?? []).map((page: Entry<PageSpec>) => {
    if (page.spec.tabs?.length) {
      return {
        page: page.name,
        label: page.spec.title || page.name,
        actions: [],
        tabs: page.spec.tabs.map((tab) => ({
          label: tab.label,
          actions: blockActions(tab, bundle),
        })),
      }
    }
    return {
      page: page.name,
      label: page.spec.title || page.name,
      actions: (page.spec.blocks ?? []).flatMap((b) => blockActions(b, bundle)),
      tabs: [],
    }
  })
}

// Key format: "page" | "page::action" | "page::tab::action"
function grantKey(page: string, tab?: string, action?: string): string {
  if (tab && action) return `${page}::${tab}::${action}`
  if (action) return `${page}::${action}`
  return page
}

function grantsToKeys(grants: Grant[] | undefined): Set<string> {
  const keys = new Set<string>()
  for (const g of grants ?? []) {
    if (g.tabs?.length) {
      for (const t of g.tabs) {
        for (const a of t.actions) keys.add(grantKey(g.page, t.tab, a.name))
      }
    } else {
      for (const a of g.actions ?? [])
        keys.add(grantKey(g.page, undefined, a.name))
    }
  }
  return keys
}

function keysToGrants(models: PageModel[], keys: Set<string>): Grant[] {
  const grants: Grant[] = []
  for (const m of models) {
    if (m.tabs.length) {
      const tabs: GrantTab[] = []
      for (const t of m.tabs) {
        const actions = t.actions
          .filter((a) => keys.has(grantKey(m.page, t.label, a)))
          .map((a) => ({ name: a }))
        if (actions.length) tabs.push({ tab: t.label, actions })
      }
      if (tabs.length) grants.push({ page: m.page, tabs })
    } else {
      const actions = m.actions
        .filter((a) => keys.has(grantKey(m.page, undefined, a)))
        .map((a) => ({ name: a }))
      if (actions.length) grants.push({ page: m.page, actions })
    }
  }
  return grants
}

export function GrantsEditor({
  value,
  onChange,
  readonly = false,
  error,
}: GrantsEditorProps) {
  const bundle = useMetaStore((s) => s.bundle)
  const models = useMemo(
    () => (bundle ? buildPageModels(bundle) : []),
    [bundle],
  )
  const [keys, setKeys] = useState<Set<string>>(() =>
    grantsToKeys(value as Grant[] | undefined),
  )

  const toggle = (key: string, checked: boolean) => {
    const next = new Set(keys)
    if (checked) next.add(key)
    else next.delete(key)
    setKeys(next)
    onChange?.(keysToGrants(models, next))
  }

  if (readonly) {
    const grants = keysToGrants(models, keys)
    return (
      <pre className="py-1 text-xs font-mono whitespace-pre-wrap break-words text-muted-foreground">
        {grants.length ? JSON.stringify(grants, null, 2) : "-"}
      </pre>
    )
  }

  if (!models.length) {
    return (
      <p className="text-xs text-muted-foreground">
        Tidak ada page yang tersedia untuk grants.
      </p>
    )
  }

  return (
    <div className="space-y-3 rounded-lg border p-3">
      {error && <p className="text-sm text-destructive">{error}</p>}
      {models.map((m) => (
        <div key={m.page} className="space-y-1.5">
          <div className="flex items-center gap-2">
            <Checkbox
              checked={m.actions.every((a) =>
                keys.has(grantKey(m.page, undefined, a)),
              )}
              onCheckedChange={(c: boolean | "indeterminate") => {
                const next = new Set(keys)
                for (const a of m.actions) {
                  if (c) next.add(grantKey(m.page, undefined, a))
                  else next.delete(grantKey(m.page, undefined, a))
                }
                setKeys(next)
                onChange?.(keysToGrants(models, next))
              }}
              disabled={readonly}
            />
            <span className="text-sm font-medium">{m.label}</span>
            <span className="text-xs text-muted-foreground">({m.page})</span>
          </div>

          {m.tabs.length > 0 ? (
            <div className="ml-6 space-y-1.5 border-l pl-3">
              {m.tabs.map((t) => (
                <div key={t.label} className="space-y-1">
                  <div className="flex items-center gap-2">
                    <Checkbox
                      checked={t.actions.every((a) =>
                        keys.has(grantKey(m.page, t.label, a)),
                      )}
                      onCheckedChange={(c: boolean | "indeterminate") => {
                        const next = new Set(keys)
                        for (const a of t.actions) {
                          if (c) next.add(grantKey(m.page, t.label, a))
                          else next.delete(grantKey(m.page, t.label, a))
                        }
                        setKeys(next)
                        onChange?.(keysToGrants(models, next))
                      }}
                      disabled={readonly}
                    />
                    <span className="text-sm">{t.label}</span>
                  </div>
                  <div className="ml-6 flex flex-wrap gap-2">
                    {t.actions.map((a) => (
                      <label
                        key={a}
                        className="flex items-center gap-1.5 text-xs"
                      >
                        <Checkbox
                          checked={keys.has(grantKey(m.page, t.label, a))}
                          onCheckedChange={(c: boolean | "indeterminate") =>
                            toggle(grantKey(m.page, t.label, a), !!c)
                          }
                          disabled={readonly}
                        />
                        {a}
                      </label>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="ml-6 flex flex-wrap gap-2">
              {m.actions.map((a) => (
                <label key={a} className="flex items-center gap-1.5 text-xs">
                  <Checkbox
                    checked={keys.has(grantKey(m.page, undefined, a))}
                    onCheckedChange={(c: boolean | "indeterminate") =>
                      toggle(grantKey(m.page, undefined, a), !!c)
                    }
                    disabled={readonly}
                  />
                  {a}
                </label>
              ))}
            </div>
          )}
        </div>
      ))}
    </div>
  )
}
