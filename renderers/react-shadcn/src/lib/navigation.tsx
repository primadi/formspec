// ─── Page-Transition Navigation ───
//
// App.spec.page_transition (none | fade | slide) drives the page-to-page
// navigation animation via the View Transitions API (frontend/05-app-kinds
// §5). The declarative <BrowserRouter> drops the per-navigation
// `viewTransition` option (data-router only), so transitions are triggered
// manually with the documented pattern:
//
//   document.startViewTransition(() => flushSync(() => navigate(...)))
//
// Programmatic navigation goes through useAppNavigate() (drop-in for
// useNavigate); shell links go through AppLink/AppNavLink, which intercept
// clicks and run the same transitioned navigate. The active mode is also
// mirrored onto <html data-page-transition> so the ::view-transition-*
// CSS in index.css can target the configured animation.

import { forwardRef, useCallback, useEffect } from "react"
import { flushSync } from "react-dom"
import { Link, NavLink, useNavigate } from "react-router-dom"
import type {
  LinkProps,
  NavigateOptions,
  NavLinkProps,
  To,
} from "react-router-dom"
import { useMetaStore } from "@/stores/meta"

export type PageTransitionMode =
  | "none"
  | "fade"
  | "slide"
  | "slide-up"
  | "scale"

/** Resolved App.spec.page_transition — final value from the meta API
 *  (empty/unknown already normalized to "fade" server-side). */
export function usePageTransitionMode(): PageTransitionMode {
  return useMetaStore((s) => s.bundle?.app.page_transition) ?? "fade"
}

/** Mirrors the mode onto <html data-page-transition> for the CSS in
 *  index.css. Call once near the app root. */
export function usePageTransitionEffect() {
  const mode = usePageTransitionMode()
  useEffect(() => {
    document.documentElement.dataset.pageTransition = mode
  }, [mode])
}

type AppNavigate = (to: To | number, opts?: NavigateOptions) => void

function transitionedNavigate(
  mode: PageTransitionMode,
  navigate: AppNavigate,
  to: To | number,
  opts?: NavigateOptions,
) {
  // Skip the transition when: disabled by spec, navigating by delta
  // (back/forward — popstate updates land outside the transition callback,
  // so the snapshot timing is unreliable), replacing the history entry
  // (auth redirects, query-string updates), or when the API is missing.
  if (
    mode === "none" ||
    typeof to === "number" ||
    opts?.replace ||
    typeof document.startViewTransition !== "function"
  ) {
    navigate(to as To, opts)
    return
  }
  const transition = document.startViewTransition(() => {
    flushSync(() => navigate(to as To, opts))
  })
  // Skipped/aborted transitions (e.g. a newer navigation interrupts the
  // running one, or the document is hidden) reject `ready`/`finished` with
  // AbortError — expected, not an application error. Swallow to avoid
  // "Uncaught (in promise) AbortError: Transition was skipped".
  transition.ready.catch(() => {})
  transition.finished.catch(() => {})
}

/** Drop-in replacement for useNavigate() that animates page transitions
 *  according to App.spec.page_transition. Call sites keep the exact same
 *  signature. */
export function useAppNavigate(): AppNavigate {
  // NavigateFunction is overloaded ((To, opts) | (delta)); the union param
  // of AppNavigate needs an explicit widening cast.
  const navigate = useNavigate() as unknown as AppNavigate
  const mode = usePageTransitionMode()
  return useCallback(
    (to, opts) => transitionedNavigate(mode, navigate, to, opts),
    [navigate, mode],
  )
}

/** Shared click interception for AppLink/AppNavLink: modifier keys, middle
 *  clicks, blank targets and prevented defaults fall through to the
 *  browser/router default behavior untouched. */
function useTransitionClick(
  to: To,
  opts: { replace?: boolean; state?: unknown },
  onClick?: React.MouseEventHandler<HTMLAnchorElement>,
  target?: string,
) {
  // RAW navigate — NOT useAppNavigate(). transitionedNavigate already owns
  // the view-transition; wrapping again would start a nested transition
  // inside the first one's callback (outer always skipped, AbortError spam).
  const navigate = useNavigate() as unknown as AppNavigate
  const mode = usePageTransitionMode()
  return useCallback(
    (e: React.MouseEvent<HTMLAnchorElement>) => {
      onClick?.(e)
      if (e.defaultPrevented) return
      if (
        e.button !== 0 ||
        e.metaKey ||
        e.ctrlKey ||
        e.shiftKey ||
        e.altKey ||
        target === "_blank"
      ) {
        return
      }
      e.preventDefault()
      transitionedNavigate(mode, navigate, to, {
        replace: opts.replace,
        state: opts.state as NavigateOptions["state"],
      })
    },
    [onClick, to, opts.replace, opts.state, target, mode, navigate],
  )
}

/** Link with the App's page transition applied on click. Props-compatible
 *  with react-router's Link. */
export const AppLink = forwardRef<HTMLAnchorElement, LinkProps>(
  function AppLink({ onClick, to, replace, state, target, ...rest }, ref) {
    const handleClick = useTransitionClick(
      to,
      { replace, state },
      onClick,
      target,
    )
    return (
      <Link
        ref={ref}
        to={to}
        replace={replace}
        state={state}
        target={target}
        onClick={handleClick}
        {...rest}
      />
    )
  },
)

/** NavLink with the App's page transition applied on click. Active-state
 *  styling still derives from the router location, so className functions
 *  and activeClass work unchanged. Props-compatible with NavLink. */
export const AppNavLink = forwardRef<HTMLAnchorElement, NavLinkProps>(
  function AppNavLink({ onClick, to, replace, state, target, ...rest }, ref) {
    const handleClick = useTransitionClick(
      to,
      { replace, state },
      onClick,
      target,
    )
    return (
      <NavLink
        ref={ref}
        to={to}
        replace={replace}
        state={state}
        target={target}
        onClick={handleClick}
        {...rest}
      />
    )
  },
)
