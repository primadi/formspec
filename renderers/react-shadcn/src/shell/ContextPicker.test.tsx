// @vitest-environment jsdom
//
// ─── Session context preference + picker contract ───
//
// A principal may hold several assignments (role × branch). The server never
// picks one — it answers 409 CONTEXT_REQUIRED with `choices` (backend §8.7).
// These tests pin the two halves of the client fix:
//
//   1. the remembered choice (localStorage) prefills the picker, and falls
//      back to the FIRST option — not to nothing — on a new device;
//   2. the picker submits the chosen id and never auto-submits, so the
//      boundary stays something the caller stated (keeps the audit answer
//      "as which role, in which branch" true).

import { afterEach, describe, expect, it, vi } from "vitest"
import { cleanup, fireEvent, render } from "@testing-library/react"
import "@testing-library/jest-dom/vitest"

import { ContextPicker } from "./ContextPicker"
import {
  defaultContextChoice,
  readContextPreference,
  writeContextPreference,
} from "@/lib/session-context"
import type { ContextChoice } from "@/types/manifest"

const choices: ContextChoice[] = [
  { id: "sales@B1", role: "sales", dimension: "branch_id", value: "B1" },
  { id: "admin@B2", role: "admin", dimension: "branch_id", value: "B2" },
]

afterEach(() => {
  cleanup()
  localStorage.clear()
})

describe("session context preference", () => {
  it("returns undefined when this device has no remembered choice", () => {
    expect(readContextPreference("kafe", "pos")).toBeUndefined()
  })

  it("round-trips a choice per workspace and App", () => {
    writeContextPreference("kafe", "pos", "admin@B2")
    expect(readContextPreference("kafe", "pos")).toBe("admin@B2")
    // Same workspace, different App → a different boundary namespace.
    expect(readContextPreference("kafe", "crm")).toBeUndefined()
    expect(readContextPreference("other", "pos")).toBeUndefined()
  })

  it("falls back to the first choice when nothing is remembered", () => {
    expect(defaultContextChoice(choices, undefined)).toBe("sales@B1")
  })

  it("prefers the remembered choice", () => {
    expect(defaultContextChoice(choices, "admin@B2")).toBe("admin@B2")
  })

  it("falls back to the first choice when the remembered one is gone", () => {
    // The assignment was revoked — the picker must stay usable, but the
    // revoked id must never be preselected (it would fail on submit).
    expect(defaultContextChoice(choices, "sales@B9")).toBe("sales@B1")
  })

  it("returns undefined for an empty choice list", () => {
    expect(defaultContextChoice([], "sales@B1")).toBeUndefined()
  })
})

describe("ContextPicker", () => {
  it("preselects the provided default without submitting anything", () => {
    const onSubmit = vi.fn()
    const { container } = render(
      <ContextPicker
        choices={choices}
        defaultId="admin@B2"
        onSubmit={onSubmit}
      />,
    )

    const checked = container.querySelector<HTMLInputElement>(
      'input[name="assignment"]:checked',
    )
    expect(checked).toBeChecked()
    expect(checked).toHaveAttribute("value", "admin@B2")
    // Prefill is a convenience, never an automatic decision.
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it("submits the chosen id", () => {
    const onSubmit = vi.fn()
    const { container } = render(
      <ContextPicker
        choices={choices}
        defaultId="sales@B1"
        onSubmit={onSubmit}
      />,
    )

    const second = container.querySelectorAll<HTMLInputElement>(
      'input[name="assignment"]',
    )[1]
    fireEvent.click(second)
    fireEvent.click(container.querySelector("button[type=submit]")!)

    expect(onSubmit).toHaveBeenCalledWith("admin@B2")
  })

  it("labels each choice with its role and branch value", () => {
    const { container } = render(
      <ContextPicker choices={choices} onSubmit={vi.fn()} />,
    )
    expect(container.textContent).toContain("sales · B1")
    expect(container.textContent).toContain("admin · B2")
  })

  it("disables submission while busy", () => {
    const { container } = render(
      <ContextPicker choices={choices} onSubmit={vi.fn()} busy />,
    )
    expect(container.querySelector("button[type=submit]")).toBeDisabled()
  })

  it("renders nothing without choices", () => {
    const { container } = render(
      <ContextPicker choices={[]} onSubmit={vi.fn()} />,
    )
    expect(container).toBeEmptyDOMElement()
  })
})
