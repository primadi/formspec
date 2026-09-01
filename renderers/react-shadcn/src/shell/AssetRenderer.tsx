// ─── AssetRenderer — loads & mounts a custom asset component (todo 5.9.1) ───
//
// Loads an ES module from `asset` (spec-root-relative path like
// "billing/assets/payment-timeline.js"), calls its default export's
// `mount(el, props, formspec)` on mount and `unmount(el)` on unmount. The
// `formspec` client is injected per the asset contract
// (07-component-kinds.md §4).

import { useEffect, useRef, useState } from "react"
import { useNavigate, useParams } from "react-router-dom"
import { Loader2 } from "lucide-react"
import type { AssetNeeds } from "@/types/manifest"
import {
  createFormspecClient,
  type FormspecClient,
} from "@/lib/formspec-client"

interface AssetComponent {
  mount: (
    el: HTMLElement,
    props: Record<string, unknown>,
    formspec: FormspecClient,
  ) => void
  unmount?: (el: HTMLElement) => void
}

interface AssetRendererProps {
  asset: string
  props?: Record<string, unknown>
  needs?: AssetNeeds
}

export function AssetRenderer({
  asset,
  props = {},
  needs,
}: AssetRendererProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [error, setError] = useState<string | null>(null)
  const navigate = useNavigate()
  const { workspace = "default" } = useParams<{ workspace: string }>()
  const propsRef = useRef(props)
  propsRef.current = props
  const needsRef = useRef(needs)
  needsRef.current = needs

  useEffect(() => {
    let disposed = false
    let component: AssetComponent | null = null
    const el = containerRef.current
    if (!el) return

    // Resolve spec-root-relative asset path:
    // "modules/x/assets/y.js" → "/{ws}/_ui/assets/modules/x/assets/y.js"
    const url = `/${workspace}/_ui/assets/${asset}`

    const formspec = createFormspecClient({
      navigate: (page, params) => {
        const q = params ? "?" + new URLSearchParams(params).toString() : ""
        navigate(page + q)
      },
      needs: needsRef.current,
    })

    // Shadow DOM host — scopes the component's CSS so it never leaks to the
    // surrounding chrome (todo 5.9.8).
    const shadow = el.attachShadow({ mode: "open" })
    const host = document.createElement("div")
    host.style.cssText = "all: initial; display: block;"
    shadow.appendChild(host)

    import(/* @vite-ignore */ url)
      .then((mod) => {
        if (disposed) return
        component = (mod.default ?? mod) as AssetComponent
        if (typeof component.mount !== "function") {
          setError("Asset does not export a mount() function")
          return
        }
        component.mount(host, propsRef.current, formspec)
      })
      .catch((e) => {
        if (!disposed) {
          setError(e instanceof Error ? e.message : "Failed to load asset")
        }
      })

    return () => {
      disposed = true
      component?.unmount?.(host)
      el.innerHTML = ""
    }
  }, [asset, workspace, navigate])

  if (error) {
    return (
      <div className="rounded-md border border-destructive/50 p-4 text-sm text-destructive">
        Failed to load asset: {error}
      </div>
    )
  }

  return <div ref={containerRef} className="min-h-8" />
}

// Loading indicator used while the module is being fetched.
export function AssetLoading() {
  return (
    <div className="flex items-center justify-center p-6 text-muted-foreground">
      <Loader2 className="size-5 animate-spin" />
    </div>
  )
}
