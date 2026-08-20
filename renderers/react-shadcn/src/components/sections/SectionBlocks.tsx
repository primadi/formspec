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
  ArrowRight,
  Check,
  Circle,
  Clock,
  Globe,
  Heart,
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
import type { SectionBlock, SectionCTA } from "@/types/manifest"

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
}: {
  block: SectionBlock
  workspace: string
  rootUrl: string
}) {
  return (
    <section className="relative overflow-hidden border-b">
      <div className="mx-auto grid max-w-6xl gap-8 px-4 py-20 md:grid-cols-2 md:py-28">
        <div className="flex flex-col justify-center gap-6">
          {block.title && (
            <h1 className="text-4xl font-bold tracking-tight md:text-5xl">
              {block.title}
            </h1>
          )}
          {block.subtitle && (
            <p className="max-w-xl text-lg text-muted-foreground">
              {block.subtitle}
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

function FeatureGridBlock({ block }: { block: SectionBlock }) {
  const columns = block.columns ?? 3
  const items = block.items ?? []
  return (
    <section className="mx-auto max-w-6xl px-4 py-16">
      {block.title && (
        <h2 className="text-3xl font-bold tracking-tight">{block.title}</h2>
      )}
      {block.subtitle && (
        <p className="mt-2 text-muted-foreground">{block.subtitle}</p>
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
              <h3 className="text-lg font-semibold">{item.title}</h3>
            )}
            {item.text && (
              <p className="text-sm text-muted-foreground">{item.text}</p>
            )}
          </div>
        ))}
      </div>
    </section>
  )
}

// ── Card ──

function CardBlock({
  block,
  workspace,
  rootUrl,
}: {
  block: SectionBlock
  workspace: string
  rootUrl: string
}) {
  const columns = block.columns ?? 3
  const items = block.items ?? []
  return (
    <section className="mx-auto max-w-6xl px-4 py-16">
      {block.title && (
        <h2 className="text-3xl font-bold tracking-tight">{block.title}</h2>
      )}
      {block.subtitle && (
        <p className="mt-2 text-muted-foreground">{block.subtitle}</p>
      )}
      <div
        className="mt-10 grid gap-6"
        style={{ gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` }}
      >
        {items.map((item, idx) => (
          <div
            key={idx}
            className="flex flex-col overflow-hidden rounded-lg border"
          >
            {item.image && (
              <img
                src={item.image}
                alt={item.title ?? ""}
                className="h-40 w-full object-cover"
              />
            )}
            <div className="flex flex-1 flex-col gap-2 p-6">
              {item.title && (
                <h3 className="text-lg font-semibold">{item.title}</h3>
              )}
              {item.text && (
                <p className="text-sm text-muted-foreground">{item.text}</p>
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
}: {
  block: SectionBlock
  workspace: string
  rootUrl: string
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
    <section className="mx-auto max-w-6xl px-4 py-16">
      {block.title && (
        <h2 className="text-3xl font-bold tracking-tight">{block.title}</h2>
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
                  <h3 className="text-2xl font-semibold">{item.title}</h3>
                )}
                {item.text && (
                  <p className="max-w-xl text-muted-foreground">{item.text}</p>
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
}: {
  block: SectionBlock
  workspace: string
  rootUrl: string
}) {
  return (
    <section className="border-t">
      <div className="mx-auto flex max-w-6xl flex-col items-center gap-6 px-4 py-16 text-center">
        {block.title && (
          <h2 className="text-3xl font-bold tracking-tight">{block.title}</h2>
        )}
        {block.subtitle && (
          <p className="max-w-xl text-muted-foreground">{block.subtitle}</p>
        )}
        {block.cta && (
          <CtaButton cta={block.cta} workspace={workspace} rootUrl={rootUrl} />
        )}
      </div>
    </section>
  )
}

// ── Root dispatcher ──

export function SectionBlockRenderer({
  block,
  workspace,
  rootUrl,
}: {
  block: SectionBlock
  workspace: string
  rootUrl: string
}) {
  switch (block.type) {
    case "hero":
      return <HeroBlock block={block} workspace={workspace} rootUrl={rootUrl} />
    case "feature_grid":
      return <FeatureGridBlock block={block} />
    case "card":
      return <CardBlock block={block} workspace={workspace} rootUrl={rootUrl} />
    case "carousel":
      return (
        <CarouselBlock block={block} workspace={workspace} rootUrl={rootUrl} />
      )
    case "cta":
      return <CtaBlock block={block} workspace={workspace} rootUrl={rootUrl} />
    default:
      return null
  }
}
