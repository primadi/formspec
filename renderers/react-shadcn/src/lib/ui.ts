// ─── formspec.ui — centralized UI service (todo 5.9.3) ───
//
// Single entry point for renderer UI chrome: toast (wrapping sonner),
// confirm/dialog/drawer (promise-based, rendered by shell/UiHost). Asset
// components will receive this as `formspec.ui` (todo 5.9.2) — routing UI
// calls through here keeps the renderer and assets on one implementation.

import type { ReactNode } from "react"
import { toast } from "sonner"

export { toast }

export type UiVariant = "info" | "warning" | "destructive"

export interface UiDialogOptions {
  title: string
  message?: string
  variant?: UiVariant
  confirmLabel?: string
  cancelLabel?: string
}

export interface UiDrawerOptions {
  title: string
  content: ReactNode
  side?: "left" | "right"
}

interface ConfirmRequest {
  id: number
  options: UiDialogOptions
  resolve: (value: boolean) => void
}

interface DrawerRequest {
  id: number
  options: UiDrawerOptions
  resolve: (value: boolean) => void
}

const confirmListeners = new Set<(req: ConfirmRequest) => void>()
const drawerListeners = new Set<(req: DrawerRequest) => void>()
let nextId = 1

/** Resolve a confirmation. Returns true when confirmed. */
export function confirm(options: UiDialogOptions): Promise<boolean> {
  return new Promise((resolve) => {
    const id = nextId++
    confirmListeners.forEach((l) => l({ id, options, resolve }))
  })
}

/** Alias of confirm — a modal dialog with confirm/cancel. */
export const dialog = confirm

/** Open a slide-in drawer. Resolves true when closed. */
export function drawer(options: UiDrawerOptions): Promise<boolean> {
  return new Promise((resolve) => {
    const id = nextId++
    drawerListeners.forEach((l) => l({ id, options, resolve }))
  })
}

/** Subscribe to confirm requests (used by shell/UiHost). */
export function onConfirm(listener: (req: ConfirmRequest) => void): () => void {
  confirmListeners.add(listener)
  return () => confirmListeners.delete(listener)
}

/** Subscribe to drawer requests (used by shell/UiHost). */
export function onDrawer(listener: (req: DrawerRequest) => void): () => void {
  drawerListeners.add(listener)
  return () => drawerListeners.delete(listener)
}

/** The full formspec.ui surface (what asset components receive). */
export const ui = { toast, confirm, dialog, drawer }
