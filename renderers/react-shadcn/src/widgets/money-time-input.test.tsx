// @vitest-environment jsdom
//
// ─── MoneyInput / TimeInput (gap #1, item 2.14) ───
//
// Two properties are worth pinning, because both were the reason the fields were
// unusable before:
//
//   1. Editing money emits the canonical `{amount, currency}` shape — an amount
//      typed into a bare text input used to travel as free text and lose the
//      currency (or, worse, be parsed as a float and rounded).
//   2. Editing time emits `HH:MM:SS` with seconds, so the stored value keeps the
//      field type's contract (`time` = time-of-day) instead of the control's own
//      `HH:MM`.

import { afterEach, describe, expect, it, vi } from "vitest"
import { render, screen, fireEvent, cleanup } from "@testing-library/react"
import { MoneyInput } from "@/widgets/MoneyInput"
import { TimeInput } from "@/widgets/TimeInput"
import { useMetaStore } from "@/stores/meta"
import type { MetaBundle } from "@/types/manifest"

// Without this, each render() leaves its DOM behind and the role queries below
// match several inputs.
afterEach(cleanup)

const amountBox = () => screen.getByRole("textbox")
const timeBox = (container: HTMLElement) =>
  container.querySelector("input[type=time]") as HTMLInputElement

describe("MoneyInput", () => {
  it("emits the canonical {amount, currency} shape while typing", () => {
    const onChange = vi.fn()
    render(<MoneyInput onChange={onChange} currency="IDR" />)

    fireEvent.change(amountBox(), { target: { value: "25000" } })

    expect(onChange).toHaveBeenCalledWith({ amount: "25000", currency: "IDR" })
  })

  it("keeps the amount exact — no float rounding mid-edit", () => {
    const onChange = vi.fn()
    render(<MoneyInput onChange={onChange} currency="IDR" />)

    fireEvent.change(amountBox(), { target: { value: "12.345" } })

    expect(onChange).toHaveBeenCalledWith({ amount: "12.345", currency: "IDR" })
  })

  it("treats a cleared field as no value, not zero", () => {
    const onChange = vi.fn()
    render(<MoneyInput onChange={onChange} currency="IDR" />)

    // Type, then clear: firing a change with the same (empty) value is a no-op
    // in React, so the assertion below would never see a call.
    fireEvent.change(amountBox(), { target: { value: "25000" } })
    fireEvent.change(amountBox(), { target: { value: "" } })

    expect(onChange).toHaveBeenLastCalledWith(null)
  })

  it("shows an existing amount and renders nothing editable when read-only", () => {
    render(
      <MoneyInput
        value={{ amount: "15000", currency: "IDR" }}
        readonly
        currency="IDR"
      />,
    )
    expect(screen.queryByRole("textbox")).toBeNull()
    expect(screen.getByText(/15/)).toBeTruthy()
  })

  // The preview may only use a symbol that belongs to the value's own currency.
  // `fmt.money` formats with `settings.currency`, so a value that carries a
  // different code (allowed on the wire — 05-field-types.md §2) would otherwise
  // be shown as "Rp" in front of a USD amount, or vice versa.
  it("does not label a foreign-currency value with the settings symbol", () => {
    useMetaStore.setState({
      bundle: {
        settings: {
          currency: { code: "IDR", decimal_places: 0, symbol: "Rp" },
          locale: "id-ID",
        },
      } as unknown as MetaBundle,
    })
    // No field-level `currency` prop — the manifest declared none, but the
    // stored value names its own.
    render(<MoneyInput value={{ amount: "1500", currency: "USD" }} readonly />)
    expect(screen.queryByText(/Rp/)).toBeNull()
    expect(screen.getByText(/USD/)).toBeTruthy()
  })
})

describe("TimeInput", () => {
  it("stores seconds even though the control is minute-granular", () => {
    const onChange = vi.fn()
    const { container } = render(<TimeInput onChange={onChange} />)

    fireEvent.change(timeBox(container), { target: { value: "14:30" } })

    expect(onChange).toHaveBeenCalledWith("14:30:00")
  })

  it("keeps seconds when the field is declared with them", () => {
    const onChange = vi.fn()
    const { container } = render(
      <TimeInput onChange={onChange} withSeconds value="14:30:05" />,
    )

    const input = timeBox(container)
    expect(input.value).toBe("14:30:05")
    fireEvent.change(input, { target: { value: "14:30:07" } })
    expect(onChange).toHaveBeenCalledWith("14:30:07")
  })

  it("clears rather than storing midnight", () => {
    const onChange = vi.fn()
    const { container } = render(
      <TimeInput onChange={onChange} value="14:30:00" />,
    )

    fireEvent.change(timeBox(container), { target: { value: "" } })

    expect(onChange).toHaveBeenCalledWith(null)
  })
})
