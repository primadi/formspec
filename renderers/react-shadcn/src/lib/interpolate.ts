// ─── Template Interpolation ───
//
// Shared `{dotted.path}` token substitution against a plain record/context
// object. Used by Print (header/footer/body text) and Page (title
// interpolation, e.g. `"Pasien — {patient.name}"`).

/** Resolve a dotted path (e.g. "polyclinic.name") against a context object. */
export function resolvePath(ctx: Record<string, unknown>, path: string): unknown {
  return path.split(".").reduce<unknown>((acc, key) => {
    if (acc == null || typeof acc !== "object") return undefined
    return (acc as Record<string, unknown>)[key]
  }, ctx)
}

/** Replace every `{dotted.path}` token in `template` with its resolved value. */
export function interpolate(template: string | undefined, ctx: Record<string, unknown> | null): string {
  if (!template) return ""
  if (!ctx) return template
  return template.replace(/\{([\w.]+)\}/g, (match, path: string) => {
    const value = resolvePath(ctx, path)
    return value == null ? match : String(value)
  })
}
