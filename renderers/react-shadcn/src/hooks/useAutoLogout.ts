// ─── Auto Logout (Idle Timeout) Hook ───
//
// Watches for user activity and expires the session after a configurable
// period of inactivity. The timeout is a user preference
// (prefs.sessionTimeoutMinutes, set on the login screen; 0 = disabled).
//
// On expiry it marks the session unauthenticated via expireSession(), which
// the auth guard in App.tsx turns into a redirect to the login page.

import { useEffect, useRef } from "react"
import { useSessionStore } from "@/stores/session"
import { usePrefsStore } from "@/stores/prefs"

const ACTIVITY_EVENTS = [
  "mousemove",
  "mousedown",
  "keydown",
  "touchstart",
  "scroll",
  "wheel",
] as const

/**
 * Arm the idle auto-logout timer. Pass `enabled=false` (public surface,
 * already unauthenticated, no token) to keep the session alive indefinitely.
 */
export function useAutoLogout(enabled: boolean) {
  const expireSession = useSessionStore((s) => s.expireSession)
  const sessionTimeoutMinutes = usePrefsStore((s) => s.sessionTimeoutMinutes)
  const timerRef = useRef<number | undefined>(undefined)

  useEffect(() => {
    // Disabled (public surface / no real session) or "Never" (0 minutes).
    if (!enabled || sessionTimeoutMinutes <= 0) return

    const reset = () => {
      if (timerRef.current !== undefined) {
        window.clearTimeout(timerRef.current)
      }
      timerRef.current = window.setTimeout(
        () => {
          expireSession()
        },
        sessionTimeoutMinutes * 60 * 1000,
      )
    }

    const onActivity = () => reset()

    // Start the countdown immediately, then reset on every activity event.
    reset()
    for (const ev of ACTIVITY_EVENTS) {
      window.addEventListener(ev, onActivity, { passive: true })
    }
    // Returning to the tab counts as activity (resets the timer).
    document.addEventListener("visibilitychange", onActivity)

    return () => {
      if (timerRef.current !== undefined) {
        window.clearTimeout(timerRef.current)
        timerRef.current = undefined
      }
      for (const ev of ACTIVITY_EVENTS) {
        window.removeEventListener(ev, onActivity)
      }
      document.removeEventListener("visibilitychange", onActivity)
    }
  }, [enabled, sessionTimeoutMinutes, expireSession])
}
