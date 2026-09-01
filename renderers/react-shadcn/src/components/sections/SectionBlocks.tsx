// ─── Section Block Components ───
//
// Declarative presentation sections (frontend/06-page-kinds.md §1). Reusable
// in any App (no-nav public, sidebar-nav, topnav, ...) — pure presentation,
// no data binding, no auth. Styled exclusively via kind: Theme tokens (CSS
// variables) + Tailwind; no inline styling.
//
// Section types (closed set): hero | feature_grid | card | carousel | cta.
// Rendered by kinds/page/PageRenderer when a Page block has `section:`.

import { useEffect, useState } from "react"
import { Link } from "react-router-dom"
import {
  AlertOctagon,
  AlertTriangle,
  ArrowRight,
  Check,
  CheckCircle2,
  Circle,
  Clock,
  Globe,
  Heart,
  Info,
  Lock,
  Rocket,
  Settings,
  Shield,
  Sparkles,
  Star,
  Users,
  Zap,
  type LucideIcon,
} from "lucide-react"
import { buttonVariants } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { interpolate } from "@/lib/interpolate"
import type { SectionBlock, SectionCTA } from "@/types/manifest"

// ── Context interpolation ──
//
// Section text may carry `{dotted.path}` tokens resolved against a render
// context (standard slots: `user`, `route`, plus any page-level context).
// Without a context the literal token is kept (interpolate() fallback).
function t(
  value: string | undefined,
  ctx?: Record<string, unknown>,
): string | undefined {
  return ctx ? interpolate(value, ctx) : value
}

// ── Icon map (lucide) ──
// Curated set of common section icons. Unknown names fall back to a circle.
const ICONS: Record<string, LucideIcon> = {
  sparkles: Sparkles,
  rocket: Rocket,
  shield: Shield,
  zap: Zap,
  check: Check,
  star: Star,
  heart: Heart,
  users: Users,
  clock: Clock,
  globe: Globe,
  lock: Lock,
  settings: Settings,
}

function Icon({ name, className }: { name: string; className?: string }) {
  const Cmp = ICONS[name] ?? Circle
  return <Cmp className={className} />
}

// ── Shared helpers ──

function CtaButton({
  cta,
  workspace,
  rootUrl,
}: {
  cta: SectionCTA
  workspace: string
  rootUrl: string
}) {
  // Base app path (root_url may be "/" for a public App — strip the trailing
  // slash so concatenating a leading-slash href never yields a double slash).
  const base = `/${workspace}${rootUrl}`.replace(/\/+$/, "")
  const href = cta.href.startsWith("/") ? `${base}${cta.href}` : cta.href
  const variant = cta.variant ?? "primary"
  // base-ui Button has no asChild — render a Link styled with the button
  // variants instead (variant maps to the shadcn button variants).
  const btnClass =
    variant === "ghost"
      ? buttonVariants({ variant: "ghost" })
      : variant === "secondary"
        ? buttonVariants({ variant: "secondary" })
        : buttonVariants({ variant: "default" })
  return (
    <Link to={href} className={btnClass}>
      {cta.label}
      <ArrowRight className="ml-2 size-4" />
    </Link>
  )
}

// ── Hero ──

function HeroBlock({
  block,
  workspace,
  rootUrl,
  context,
}: {
  block: SectionBlock
  workspace: string
  rootUrl: string
  context?: Record<string, unknown>
}) {
  return (
    <section className="relative overflow-hidden border-b">
      <div className="mx-auto grid max-w-6xl gap-8 px-4 py-20 md:grid-cols-2 md:py-28">
        <div className="flex flex-col justify-center gap-6">
          {block.title && (
            <h1 className="text-4xl font-bold tracking-tight md:text-5xl">
              {t(block.title, context)}
            </h1>
          )}
          {block.subtitle && (
            <p className="max-w-xl text-lg text-muted-foreground">
              {t(block.subtitle, context)}
            </p>
          )}
          {block.cta && (
            <div className="flex gap-3">
              <CtaButton
                cta={block.cta}
                workspace={workspace}
                rootUrl={rootUrl}
              />
            </div>
          )}
        </div>
        {block.image && (
          <div className="flex items-center justify-center">
            <img
              src={block.image}
              alt={block.title ?? "Hero"}
              className="max-h-96 w-full rounded-lg object-cover shadow-lg"
            />
          </div>
        )}
      </div>
    </section>
  )
}

// ── Feature Grid ──

function FeatureGridBlock({
  block,
  context,
}: {
  block: SectionBlock
  context?: Record<string, unknown>
}) {
  const columns = block.columns ?? 3
  const items = block.items ?? []
  return (
    // Stretch to grid column — no mx-auto (see CardBlock note).
    <section className={cn("max-w-6xl px-4 py-16", alignClass(block.align))}>
      {block.title && (
        <h2 className="text-3xl font-bold tracking-tight">
          {t(block.title, context)}
        </h2>
      )}
      {block.subtitle && (
        <p className="mt-2 text-muted-foreground">
          {t(block.subtitle, context)}
        </p>
      )}
      <div
        className="mt-10 grid gap-6"
        style={{ gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` }}
      >
        {items.map((item, idx) => (
          <div key={idx} className="flex flex-col gap-3 rounded-lg border p-6">
            {item.icon && (
              <Icon name={item.icon} className="size-6 text-primary" />
            )}
            {item.title && (
              <h3 className="text-lg font-semibold">
                {t(item.title, context)}
              </h3>
            )}
            {item.text && (
              <p className="text-sm text-muted-foreground">
                {t(item.text, context)}
              </p>
            )}
          </div>
        ))}
      </div>
    </section>
  )
}

// ── Card ──

// Align maps `block.align` to a text-alignment class (default left).
function alignClass(align?: string): string {
  if (align === "center") return "text-center"
  if (align === "right") return "text-right"
  return ""
}

function CardBlock({
  block,
  workspace,
  rootUrl,
  context,
}: {
  block: SectionBlock
  workspace: string
  rootUrl: string
  context?: Record<string, unknown>
}) {
  const columns = block.columns ?? 3
  const items = block.items ?? []
  const center = block.align === "center"
  return (
    // No `mx-auto` on the section element: as a grid item, auto margins
    // disable stretch sizing → shrink-to-fit → sections of different content
    // length appear centered/left inconsistently. The section stretches to
    // its grid column instead (plan section-block-align.md).
    <section className={cn("max-w-6xl px-4 py-16", alignClass(block.align))}>
      {block.title && (
        <h2 className="text-3xl font-bold tracking-tight">
          {t(block.title, context)}
        </h2>
      )}
      {block.subtitle && (
        <p className="mt-2 text-muted-foreground">
          {t(block.subtitle, context)}
        </p>
      )}
      <div
        className="mt-10 grid gap-6"
        style={{ gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` }}
      >
        {items.map((item, idx) => (
          <div
            key={idx}
            className={cn(
              "flex flex-col overflow-hidden rounded-lg border",
              center && "items-center",
            )}
          >
            {item.image && (
              <img
                src={item.image}
                alt={item.title ?? ""}
                className="h-40 w-full object-cover"
              />
            )}
            <div className="flex flex-1 flex-col gap-2 p-6">
              {item.icon && (
                <Icon name={item.icon} className="size-6 text-primary" />
              )}
              {item.title && (
                <h3 className="text-lg font-semibold">
                  {t(item.title, context)}
                </h3>
              )}
              {item.text && (
                <p className="text-sm text-muted-foreground">
                  {t(item.text, context)}
                </p>
              )}
              {item.cta && (
                <div className="mt-auto pt-4">
                  <CtaButton
                    cta={item.cta}
                    workspace={workspace}
                    rootUrl={rootUrl}
                  />
                </div>
              )}
            </div>
          </div>
        ))}
      </div>
    </section>
  )
}

// ── Carousel ──
//
// Minimal dependency-free carousel: horizontal slide with prev/next buttons.
// Auto-advance when `autoplay` is set.

function CarouselBlock({
  block,
  workspace,
  rootUrl,
  context,
}: {
  block: SectionBlock
  workspace: string
  rootUrl: string
  context?: Record<string, unknown>
}) {
  const items = block.items ?? []
  const interval = block.interval_ms ?? 5000
  const [index, setIndex] = useState(0)

  useEffect(() => {
    if (!block.autoplay || items.length <= 1) return
    const id = setInterval(
      () => setIndex((i) => (i + 1) % items.length),
      interval,
    )
    return () => clearInterval(id)
  }, [block.autoplay, items.length, interval])

  if (items.length === 0) return null

  return (
    // Stretch to grid column — no mx-auto (see CardBlock note).
    <section className={cn("max-w-6xl px-4 py-16", alignClass(block.align))}>
      {block.title && (
        <h2 className="text-3xl font-bold tracking-tight">
          {t(block.title, context)}
        </h2>
      )}
      <div className="relative mt-10 overflow-hidden rounded-lg border">
        <div
          className="flex transition-transform duration-500"
          style={{ transform: `translateX(-${index * 100}%)` }}
        >
          {items.map((item, idx) => (
            <div key={idx} className="w-full shrink-0">
              <div className="flex min-h-64 flex-col items-center justify-center gap-4 p-10 text-center">
                {item.icon && (
                  <Icon name={item.icon} className="size-10 text-primary" />
                )}
                {item.title && (
                  <h3 className="text-2xl font-semibold">
                    {t(item.title, context)}
                  </h3>
                )}
                {item.text && (
                  <p className="max-w-xl text-muted-foreground">
                    {t(item.text, context)}
                  </p>
                )}
                {item.cta && (
                  <CtaButton
                    cta={item.cta}
                    workspace={workspace}
                    rootUrl={rootUrl}
                  />
                )}
              </div>
            </div>
          ))}
        </div>
        {items.length > 1 && (
          <>
            <button
              type="button"
              aria-label="Previous slide"
              onClick={() =>
                setIndex((i) => (i - 1 + items.length) % items.length)
              }
              className="absolute left-3 top-1/2 -translate-y-1/2 rounded-full border bg-background/80 p-2 shadow-sm hover:bg-background"
            >
              <ArrowRight className="size-4 rotate-180" />
            </button>
            <button
              type="button"
              aria-label="Next slide"
              onClick={() => setIndex((i) => (i + 1) % items.length)}
              className="absolute right-3 top-1/2 -translate-y-1/2 rounded-full border bg-background/80 p-2 shadow-sm hover:bg-background"
            >
              <ArrowRight className="size-4" />
            </button>
          </>
        )}
      </div>
    </section>
  )
}

// ── CTA ──

function CtaBlock({
  block,
  workspace,
  rootUrl,
  context,
}: {
  block: SectionBlock
  workspace: string
  rootUrl: string
  context?: Record<string, unknown>
}) {
  return (
    <section className="border-t">
      <div className="mx-auto flex max-w-6xl flex-col items-center gap-6 px-4 py-16 text-center">
        {block.title && (
          <h2 className="text-3xl font-bold tracking-tight">
            {t(block.title, context)}
          </h2>
        )}
        {block.subtitle && (
          <p className="max-w-xl text-muted-foreground">
            {t(block.subtitle, context)}
          </p>
        )}
        {block.cta && (
          <CtaButton cta={block.cta} workspace={workspace} rootUrl={rootUrl} />
        )}
      </div>
    </section>
  )
}

// ── Alert / Banner / Notice ──
//
// Declarative alert-style block (5.2.7). Variant drives the icon + color:
// info | success | warning | destructive.

const ALERT_STYLES: Record<string, string> = {
  info: "border-blue-200 bg-blue-50 text-blue-800 dark:border-blue-900/40 dark:bg-blue-950/30 dark:text-blue-300",
  success:
    "border-green-200 bg-green-50 text-green-800 dark:border-green-900/40 dark:bg-green-950/30 dark:text-green-300",
  warning:
    "border-yellow-200 bg-yellow-50 text-yellow-800 dark:border-yellow-900/40 dark:bg-yellow-950/30 dark:text-yellow-300",
  destructive:
    "border-red-200 bg-red-50 text-red-800 dark:border-red-900/40 dark:bg-red-950/30 dark:text-red-300",
}

const ALERT_ICONS: Record<string, LucideIcon> = {
  info: Info,
  success: CheckCircle2,
  warning: AlertTriangle,
  destructive: AlertOctagon,
}

function AlertBlock({
  block,
  context,
}: {
  block: SectionBlock
  context?: Record<string, unknown>
}) {
  const variant = block.variant ?? "info"
  const IconCmp = ALERT_ICONS[variant] ?? Info
  return (
    <div
      className={cn(
        "flex items-start gap-3 rounded-lg border p-4",
        ALERT_STYLES[variant] ?? ALERT_STYLES.info,
      )}
    >
      <IconCmp className="mt-0.5 size-5 shrink-0" />
      <div className="space-y-1">
        {block.title && (
          <div className="text-sm font-semibold">{t(block.title, context)}</div>
        )}
        {block.subtitle && (
          <div className="text-sm">{t(block.subtitle, context)}</div>
        )}
      </div>
    </div>
  )
}

// ── Root dispatcher ──

export function SectionBlockRenderer({
  block,
  workspace,
  rootUrl,
  context,
}: {
  block: SectionBlock
  workspace: string
  rootUrl: string
  /** Render context for `{dotted.path}` token interpolation (standard
   *  slots: `user`, `route`, plus page-level context). */
  context?: Record<string, unknown>
}) {
  switch (block.type) {
    case "hero":
      return (
        <HeroBlock
          block={block}
          workspace={workspace}
          rootUrl={rootUrl}
          context={context}
        />
      )
    case "feature_grid":
      return <FeatureGridBlock block={block} context={context} />
    case "card":
      return (
        <CardBlock
          block={block}
          workspace={workspace}
          rootUrl={rootUrl}
          context={context}
        />
      )
    case "carousel":
      return (
        <CarouselBlock
          block={block}
          workspace={workspace}
          rootUrl={rootUrl}
          context={context}
        />
      )
    case "cta":
      return (
        <CtaBlock
          block={block}
          workspace={workspace}
          rootUrl={rootUrl}
          context={context}
        />
      )
    case "banner":
    case "alert":
    case "notice":
      return <AlertBlock block={block} context={context} />
    default:
      return null
  }
}
