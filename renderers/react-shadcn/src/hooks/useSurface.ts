// ─── Surface Context Hook ───
//
// Provides helpers for surface-aware navigation. There is only ONE surface
// kind: an App. (The `_admin` surface was retired — plan app-scoped-login.md
// D4 — so there is no admin/App fork to detect.)

import { useParams } from "react-router-dom"
import { useMetaStore } from "@/stores/meta"

export interface SurfaceInfo {
  /** Base path for the current App surface (its own root_url). */
  surfacePrefix: string
  /** Workspace slug from the route. Needed to build entity-scoped URLs that
   *  are not surface paths (e.g. a file download: `/{workspace}/_ui/...`). */
  workspace: string
  /** Build an absolute path within the current surface */
  surfacePath: (...segments: string[]) => string
}

export function useSurface(): SurfaceInfo {
  const { workspace = "default" } = useParams<{ workspace: string }>()
  const bundle = useMetaStore((s) => s.bundle)

  const surfacePrefix =
    `/${workspace}${bundle?.app.root_url ?? "/app"}`.replace(/\/+$/, "")

  const join = (prefix: string, segments: string[]) => {
    const path = [prefix, ...segments].join("/")
    return path.replace(/\/+/g, "/")
  }

  return {
    workspace,
    surfacePrefix,
    surfacePath: (...segments: string[]) => join(surfacePrefix, segments),
  }
}
