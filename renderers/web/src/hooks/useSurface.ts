// ─── Surface Context Hook ───
//
// Determines which surface (admin or app) the current URL is on and
// provides helper functions for surface-aware navigation.
//
// Detection logic mirrors Sidebar.tsx: pathname-based, no prop drilling.

import { useParams, useLocation } from "react-router-dom"
import { useMetaStore } from "@/stores/meta"

export interface SurfaceInfo {
  /** "admin" for /_admin/* routes, "app" for /app/* routes */
  surface: "admin" | "app"
  /** Whether the current surface is admin */
  isAdmin: boolean
  /** Base path for the current surface (includes root_url for app surface) */
  surfacePrefix: string
  /** Build an absolute path within the admin surface */
  adminPath: (...segments: string[]) => string
  /** Build an absolute path within the current surface (admin or app) */
  surfacePath: (...segments: string[]) => string
}

export function useSurface(): SurfaceInfo {
  const { workspace = "default" } = useParams<{ workspace: string }>()
  const location = useLocation()
  const bundle = useMetaStore((s) => s.bundle)

  const adminPrefix = `/${workspace}/_admin`
  const isAdmin =
    location.pathname === adminPrefix ||
    location.pathname.startsWith(`${adminPrefix}/`)
  const surfacePrefix = isAdmin
    ? adminPrefix
    : `/${workspace}${bundle?.app.root_url ?? "/app"}`

  const join = (prefix: string, segments: string[]) => {
    const path = [prefix, ...segments].join("/")
    return path.replace(/\/+/g, "/")
  }

  return {
    surface: isAdmin ? "admin" : "app",
    isAdmin,
    surfacePrefix,
    adminPath: (...segments: string[]) => join(adminPrefix, segments),
    surfacePath: (...segments: string[]) => join(surfacePrefix, segments),
  }
}
