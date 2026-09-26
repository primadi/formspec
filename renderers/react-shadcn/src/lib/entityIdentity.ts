import type { EntitySchema } from "@/types/manifest"

export function getNaturalKeyField(entity: EntitySchema) {
  return entity.fields.find((field) => field.natural_key)
}

export function getEntityRouteIdentifier(
  entity: EntitySchema,
  record: Record<string, unknown>,
): string {
  const naturalKey = getNaturalKeyField(entity)
  const value = naturalKey ? record[naturalKey.name] : undefined
  if (value !== undefined && value !== null && String(value) !== "") {
    return String(value)
  }
  return String(record.id ?? "")
}

export function getEntityRouteSegment(
  entity: EntitySchema,
  record: Record<string, unknown>,
): string {
  return encodeURIComponent(getEntityRouteIdentifier(entity, record))
}
