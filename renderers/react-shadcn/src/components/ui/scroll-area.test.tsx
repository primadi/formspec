// @vitest-environment jsdom
//
// ─── Sidebar nav must be scrollable (kafe 10.31) ───
//
// The sidebar's nav already sat inside `<ScrollArea className="flex-1 py-2">`,
// so every declaration *looked* right: `flex-1` (`flex: 1 1 0%`) asks for the
// remaining height, and the Viewport already carries `overflow: scroll`.
// Measured on `/kafe/app/pos/cafe-stock/waste-entries` before the fix, none of
// that mattered:
//
//   aside          clientHeight  560   scrollHeight 1383
//   └ ScrollArea   clientHeight 1354   ← grew to its CONTENT, not clamped
//     └ viewport   clientHeight 1338   scrollHeight 1338  → nothing to scroll
//
// `flex-1` only sets the flex *basis*; the default `min-height: auto` of a flex
// item still floors the box at its content height, so the tree kept overflowing
// `aside` (`overflow: visible`) and the last menu item ended at y=1398 in a
// 560px viewport — unreachable, with base-ui hiding the Scrollbar because
// `hasOverflowY` was false (`shouldRender` → null).
//
// jsdom has no layout engine, so these assertions pin the contract that
// produces the behaviour (same approach as `tableColumn.test.tsx`): the
// primitive can shrink, keeps its other responsibilities, and is actually the
// box the sidebar nav lives in.

import { beforeEach, describe, expect, it } from "vitest"
import { render, cleanup } from "@testing-library/react"
import { MemoryRouter } from "react-router-dom"

import { Sidebar } from "@/shell/Sidebar"
import { useMetaStore } from "@/stores/meta"
import { useSessionStore } from "@/stores/session"
import type { MetaBundle, MeResponse } from "@/types/manifest"

const MENU = Array.from({ length: 24 }, (_, i) => ({
  label: `Item ${i + 1}`,
  route: `/item-${i + 1}`,
}))

function seedStores() {
  useMetaStore.setState({
    bundle: {
      app: { name: "kafe-pos", title: "Kafe POS", root_url: "/app" },
      entities: [],
      menu: MENU,
    } as unknown as MetaBundle,
  })
  useSessionStore.setState({
    workspace: "kafe",
    me: {
      user_id: "u1",
      workspace: "kafe",
      roles: [],
      permissions: [],
    } as MeResponse,
  })
}

function renderSidebar(mobile: boolean) {
  const { container } = render(
    <MemoryRouter initialEntries={["/kafe/app/pos/cafe-stock/waste-entries"]}>
      <Sidebar collapsed={false} mobile={mobile} mobileOpen={mobile} />
    </MemoryRouter>,
  )
  return container
}

beforeEach(() => {
  cleanup()
  seedStores()
})

describe("Sidebar scroll area", () => {
  it.each([
    ["desktop", false],
    ["mobile overlay", true],
  ])("clamps the nav so it can overflow — %s", (_variant, mobile) => {
    const container = renderSidebar(mobile as boolean)
    const roots = container.querySelectorAll('[data-slot="scroll-area"]')
    expect(roots).toHaveLength(1)

    const root = roots[0] as HTMLElement
    // `min-h-0` is what lets `flex-1` clamp the box to the aside's remaining
    // height instead of growing to the 24 menu items.
    expect(root.classList.contains("min-h-0")).toBe(true)
    expect(root.classList.contains("flex-1")).toBe(true)
    // The nav must live inside the scroll area — otherwise the shrink contract
    // applies to an empty box and the surplus items stay unreachable.
    expect(root.querySelector("nav")).not.toBeNull()
    expect(root.querySelectorAll("nav a").length).toBeGreaterThan(12)
  })

  it("keeps the sidebar a flex column, so flex-1 has a height to divide", () => {
    const container = renderSidebar(false)
    const aside = container.querySelector("aside") as HTMLElement
    expect(aside.classList.contains("flex")).toBe(true)
    expect(aside.classList.contains("flex-col")).toBe(true)
  })

  it("does not lose the primitive's own positioning when it shrinks", () => {
    const container = renderSidebar(false)
    const root = container.querySelector(
      '[data-slot="scroll-area"]',
    ) as HTMLElement
    expect(root.classList.contains("relative")).toBe(true)
    const viewport = root.querySelector('[data-slot="scroll-area-viewport"]')
    expect(viewport).not.toBeNull()
    // The Viewport must fill whatever height the Root got — if it stops being
    // `size-full`, clamping the Root alone changes nothing.
    expect(viewport?.classList.contains("size-full")).toBe(true)
  })
})
