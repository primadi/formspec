import type { EntitySchema } from "@/types/manifest"

export interface BreadcrumbEntry {
  label: string
  href: string
  isLast: boolean
}

function humanize(part: string): string {
  let decoded = part
  try {
    decoded = decodeURIComponent(part)
  } catch {
    // Preserve malformed manually-entered path segments.
  }
  return decoded.replace(/[-_]/g, " ").replace(/\b\w/g, (c) => c.toUpperCase())
}

export function buildBreadcrumbs(
  pathname: string,
  workspace: string,
  entities: EntitySchema[],
  routeIdentity?: string,
): BreadcrumbEntry[] {
  const pathParts = pathname.split("/").filter(Boolean).slice(1)
  let identifierIndex = -1

  for (let index = 0; index < pathParts.length - 2; index += 1) {
    const entityRoute = entities.find(
      (entity) =>
        entity.module === pathParts[index] &&
        entity.plural === pathParts[index + 1],
    )
    if (entityRoute) {
      identifierIndex = index + 2
      break
    }
  }

  return pathParts.map((part, index) => {
    const href = `/${workspace}/${pathParts.slice(0, index + 1).join("/")}`
    return {
      label:
        index === identifierIndex && routeIdentity
          ? routeIdentity
          : humanize(part),
      href,
      isLast: index === pathParts.length - 1,
    }
  })
}
