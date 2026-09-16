// ─── MoneyInput Widget ───
//
// For `type: money` fields (gap #1 / item 2.14). Before this widget a money
// field rendered as a plain text input labelled with the field title, so a
// cashier had to type a currency amount as free text — no grouping, no currency,
// no separator conventions, and a stray character produced a 422 at submit.
//
// Three decisions worth knowing:
//
//   - The value is the canonical wire shape `{amount, currency}` (05-field-types
//     §2), so editing never drops the currency. A bare number/string (legacy or
//     hand-written payload) is accepted and upgraded on the first edit.
//   - The amount is kept as **text** while typing. Parsing to a float and back
//     would silently round (12.345 → 12.35) before the user finished, and money
//     is exact by contract (spec §2.1: never float).
//   - Formatting follows the resolved global settings (`settings.currency`,
//     `settings.locale`) — the widget never guesses a symbol or scale.

import { useEffect, useState } from "react"
import { useMetaStore } from "@/stores/meta"
import { createFormatter, moneyAmount } from "@/lib/format"
import { cn } from "@/lib/utils"

export interface MoneyInputProps {
  value?: unknown
  onChange?: (value: unknown) => void
  readonly?: boolean
  error?: string
  /** Field-level currency override; empty = settings.currency. */
  currency?: string
  /** Field-level scale override; empty = settings.currency.decimal_places. */
  decimalPlaces?: number
  /** Placeholder shown when empty (e.g. "0"). */
  placeholder?: string
  id?: string
  disabled?: boolean
  className?: string
}

/** Renders a money value's amount as plain text (digits, one separator). */
function amountText(value: unknown): string {
  const amount = moneyAmount(value)
  if (amount === undefined) return ""
  return String(amount)
}

/** Strips grouping/spaces so only a parseable decimal remains. */
function normalizeAmount(raw: string): string {
  return raw.replace(/[^\d.,-]/g, "").replace(",", ".")
}

export function MoneyInput({
  value,
  onChange,
  readonly = false,
  error,
  currency,
  decimalPlaces,
  placeholder,
  id,
  disabled,
  className,
}: MoneyInputProps) {
  const getSettings = useMetaStore((s) => s.getSettings)
  const settings = getSettings?.()
  const fmt = createFormatter(settings)
  const code =
    currency ||
    settings?.currency?.code ||
    (value as { currency?: string } | undefined)?.currency ||
    ""

  const [text, setText] = useState(() => amountText(value))

  // Keep in step with the form when the value changes from outside (load,
  // reset, a computed field writing into it) without fighting the user's
  // keystrokes: the effect only fires when the incoming amount actually
  // differs from what the text currently parses to.
  useEffect(() => {
    const incoming = amountText(value)
    if (normalizeAmount(text) === incoming || (incoming === "" && text === ""))
      return
    setText(incoming)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value])

  const handleChange = (raw: string) => {
    setText(raw)
    const amount = normalizeAmount(raw)
    if (amount === "" || amount === "-") {
      onChange?.(null)
      return
    }
    // Emit the canonical shape; the server still normalizes and validates
    // (spec.NormalizeMoneyValue), so an intermediate "12." is never rejected
    // as a hard error here.
    onChange?.({ amount, currency: code || undefined })
  }

  const previewAmount = moneyAmount(normalizeAmount(text))
  // `fmt.money` always formats with settings.currency, so it would mislabel a
  // field that declares its own currency (05-field-types.md §2 allows the
  // override). In that case show the settings-formatted number plus the field's
  // own code instead of a symbol that belongs to another currency.
  const overridden = Boolean(currency) && currency !== settings?.currency?.code
  const preview =
    previewAmount === undefined
      ? null
      : overridden
        ? // The field's own scale applies here (its currency need not share the
          // global one); display-only, the stored amount stays exact.
          `${previewAmount.toFixed(decimalPlaces ?? 2)} ${code}`
        : fmt.money(previewAmount)

  if (readonly) {
    return (
      <span className={cn("text-sm tabular-nums", className)}>
        {preview ?? "—"}
      </span>
    )
  }

  return (
    <div className={cn("space-y-1", className)}>
      <div
        className={cn(
          "flex items-center gap-2 rounded-md border bg-transparent px-3 py-1.5",
          error ? "border-destructive" : "border-input",
          disabled && "opacity-50",
        )}
      >
        {code && (
          <span className="text-xs font-medium text-muted-foreground">
            {code}
          </span>
        )}
        <input
          id={id}
          // Numpad on touch devices, and no spinner: this is an amount, not a
          // number to nudge.
          inputMode="decimal"
          autoComplete="off"
          value={text}
          placeholder={placeholder ?? "0"}
          disabled={disabled}
          onChange={(e) => handleChange(e.target.value)}
          className="w-full bg-transparent text-right tabular-nums outline-none"
        />
      </div>
      {preview && (
        <div className="text-xs text-muted-foreground tabular-nums">
          {preview}
        </div>
      )}
      {error && <div className="text-xs text-destructive">{error}</div>}
    </div>
  )
}
