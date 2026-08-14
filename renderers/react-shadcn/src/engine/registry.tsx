// ─── Kind Renderer Registry ───
//
// Maps resource kinds to their React renderer components.
// Used by the router and OverlayHost to find the right renderer.

import type { ComponentType } from "react"

/**
 * Registry entry for a kind renderer.
 */
export interface RendererEntry<T = unknown> {
  kind: string
  component: ComponentType<{ spec: T; [key: string]: unknown }>
}

/**
 * Get the renderer component for a given kind.
 * Returns undefined if no renderer is registered (should show placeholder).
 */
export function getRenderer(kind: string): ComponentType<{ spec: unknown }> | undefined {
  return registry.get(kind)
}

// Simple registry map
const registry = new Map<string, ComponentType<{ spec: unknown }>>()

/**
 * Register a renderer for a kind.
 */
export function registerRenderer(kind: string, component: ComponentType<{ spec: unknown }>): void {
  registry.set(kind, component)
}

/**
 * Check if a renderer is registered for a kind.
 */
export function hasRenderer(kind: string): boolean {
  return registry.has(kind)
}
