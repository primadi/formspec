// ─── Custom Themed Select ───
//
// Drop-in replacement for native <select> that uses CSS variables
// (bg-popover, border-border, etc.) so it follows the active theme.
//
// Native <select> dropdowns are rendered by the OS on Linux and
// cannot be styled with CSS. This custom component gives us full
// control over the dropdown appearance.

import { useState, useRef, useEffect, useCallback } from "react"
import { ChevronDown } from "lucide-react"
import { cn } from "@/lib/utils"

interface SelectOption {
  value: string
  label: string
}

interface SelectProps {
  value?: string
  onChange?: (value: string) => void
  options: (string | SelectOption)[]
  placeholder?: string
  disabled?: boolean
  className?: string
  error?: boolean
}

function normalizeOption(opt: string | SelectOption): SelectOption {
  if (typeof opt === "string") {
    return {
      value: opt,
      label: opt.charAt(0).toUpperCase() + opt.slice(1).replace(/_/g, " "),
    }
  }
  return opt
}

export function Select({
  value,
  onChange,
  options,
  placeholder,
  disabled = false,
  className,
  error,
}: SelectProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [focusedIndex, setFocusedIndex] = useState(-1)
  const containerRef = useRef<HTMLDivElement>(null)
  const listRef = useRef<HTMLDivElement>(null)
  // Type-ahead search buffer (native <select> style): typing letters jumps to
  // the matching option. Reset after a short pause.
  const typeaheadRef = useRef("")
  const typeaheadTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined)

  const normalizedOptions = options.map(normalizeOption)
  const selectedOption = normalizedOptions.find((o) => o.value === value)

  // Close on click outside
  useEffect(() => {
    if (!isOpen) return
    const handleClickOutside = (e: MouseEvent) => {
      if (
        containerRef.current &&
        !containerRef.current.contains(e.target as Node)
      ) {
        setIsOpen(false)
      }
    }
    document.addEventListener("mousedown", handleClickOutside)
    return () => document.removeEventListener("mousedown", handleClickOutside)
  }, [isOpen])

  // Scroll focused item into view
  useEffect(() => {
    if (!isOpen || focusedIndex < 0 || !listRef.current) return
    const item = listRef.current.children[focusedIndex] as
      | HTMLElement
      | undefined
    item?.scrollIntoView({ block: "nearest" })
  }, [focusedIndex, isOpen])

  const handleSelect = useCallback(
    (opt: SelectOption) => {
      onChange?.(opt.value)
      setIsOpen(false)
      setFocusedIndex(-1)
      typeaheadRef.current = ""
    },
    [onChange],
  )

  const clearTypeahead = useCallback(() => {
    typeaheadRef.current = ""
    if (typeaheadTimerRef.current) {
      clearTimeout(typeaheadTimerRef.current)
      typeaheadTimerRef.current = undefined
    }
  }, [])

  const isPrintableKey = useCallback(
    (key: string) => key.length === 1 && /[a-zA-Z0-9]/.test(key),
    [],
  )

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      // Type-ahead search — typing letters jumps to the matching option,
      // whether the dropdown is open or closed.
      if (isPrintableKey(e.key) && !e.ctrlKey && !e.metaKey && !e.altKey) {
        e.preventDefault()
        if (!isOpen) {
          setIsOpen(true)
        }
        typeaheadRef.current = (typeaheadRef.current + e.key).toLowerCase()
        if (typeaheadTimerRef.current) {
          clearTimeout(typeaheadTimerRef.current)
        }
        typeaheadTimerRef.current = setTimeout(() => {
          typeaheadRef.current = ""
        }, 600)

        const buf = typeaheadRef.current
        const start = focusedIndex + 1
        for (let i = 0; i < normalizedOptions.length; i++) {
          const idx = (start + i) % normalizedOptions.length
          if (normalizedOptions[idx].label.toLowerCase().startsWith(buf)) {
            setFocusedIndex(idx)
            break
          }
        }
        return
      }

      if (!isOpen) {
        if (e.key === "Enter" || e.key === " " || e.key === "ArrowDown") {
          e.preventDefault()
          setIsOpen(true)
          setFocusedIndex(0)
        }
        return
      }

      switch (e.key) {
        case "Escape":
          e.preventDefault()
          setIsOpen(false)
          setFocusedIndex(-1)
          clearTypeahead()
          break
        case "ArrowDown":
          e.preventDefault()
          setFocusedIndex((prev) =>
            prev < normalizedOptions.length - 1 ? prev + 1 : 0,
          )
          break
        case "ArrowUp":
          e.preventDefault()
          setFocusedIndex((prev) =>
            prev > 0 ? prev - 1 : normalizedOptions.length - 1,
          )
          break
        case "Enter":
        case " ":
          e.preventDefault()
          if (focusedIndex >= 0 && focusedIndex < normalizedOptions.length) {
            handleSelect(normalizedOptions[focusedIndex])
          }
          break
      }
    },
    [
      isOpen,
      focusedIndex,
      normalizedOptions,
      handleSelect,
      isPrintableKey,
      clearTypeahead,
    ],
  )

  return (
    <div ref={containerRef} className="relative">
      {/* Trigger */}
      <button
        type="button"
        disabled={disabled}
        onClick={() => setIsOpen((prev) => !prev)}
        onKeyDown={handleKeyDown}
        aria-haspopup="listbox"
        aria-expanded={isOpen}
        className={cn(
          "flex h-9 w-full items-center justify-between rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-xs transition-colors",
          "hover:border-ring",
          "focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring",
          "disabled:cursor-not-allowed disabled:opacity-50",
          error && "border-destructive",
          className,
        )}
      >
        <span
          className={cn(
            "flex-1 truncate text-left",
            !selectedOption && "text-muted-foreground",
          )}
        >
          {selectedOption ? selectedOption.label : (placeholder ?? "Pilih...")}
        </span>
        <ChevronDown
          className={cn(
            "ml-2 size-4 shrink-0 text-muted-foreground transition-transform",
            isOpen && "rotate-180",
          )}
        />
      </button>

      {/* Dropdown */}
      {isOpen && (
        <div
          ref={listRef}
          role="listbox"
          className={cn(
            "absolute z-50 mt-1 w-full rounded-lg border border-border bg-popover py-1 shadow-md",
            "max-h-60 overflow-auto",
          )}
        >
          {normalizedOptions.length === 0 && (
            <div className="px-3 py-6 text-center text-sm text-muted-foreground">
              Tidak ada pilihan
            </div>
          )}
          {normalizedOptions.map((opt, index) => (
            <button
              key={opt.value}
              type="button"
              role="option"
              aria-selected={opt.value === value}
              className={cn(
                "flex w-full items-center px-3 py-1.5 text-left text-sm transition-colors",
                "hover:bg-accent hover:text-accent-foreground",
                opt.value === value && "bg-accent/50 font-medium",
                index === focusedIndex && "bg-accent/70",
              )}
              onClick={() => handleSelect(opt)}
              onMouseEnter={() => setFocusedIndex(index)}
            >
              <span className="flex-1 truncate">{opt.label}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
