// @vitest-environment jsdom
//
// ─── `select-multi-tag` widget + field-options helper ───
//
// The widget exists to close one failure: a field whose legal values are
// *declared* (`promo.days_of_week` = 1..7) used to render either a raw JSON
// editor or a free-text tag input, so `9` was storable and `1` never read as
// "Senin". These tests pin the four properties that make the declaration mean
// something:
//
//   1. a selected value is no longer offered (no duplicates);
//   2. the stored value keeps the field's shape (`json` → array of numbers,
//      `string` → comma-separated text);
//   3. a value outside the declaration is still shown — visible and removable,
//      never dropped on save;
//   4. a non-list value surfaces an error instead of being replaced by `[]`.

import { afterEach, describe, expect, it, vi } from "vitest"
import {
  render,
  screen,
  fireEvent,
  cleanup,
  within,
} from "@testing-library/react"
import { SelectMultiTag } from "@/widgets/SelectMultiTag"
import {
  fieldOptions,
  parseOptionValue,
  serializeOptionValue,
  optionKey,
  optionLabel,
  valueShapeOf,
} from "@/lib/field-options"
import type { Field } from "@/types/manifest"

afterEach(cleanup)

/** The kafe promo field: a closed set of days, stored as a json array. */
const daysField: Field = {
  name: "days_of_week",
  type: "json",
  options: [
    { value: 1, label: "Senin" },
    { value: 2, label: "Selasa" },
    { value: 3, label: "Rabu" },
    { value: 4, label: "Kamis" },
    { value: 5, label: "Jumat" },
  ],
}

const openPicker = () => {
  fireEvent.click(screen.getByLabelText("Hari Berlaku"))
}

describe("fieldOptions", () => {
  it("uses declared options with their captions", () => {
    const opts = fieldOptions(daysField)
    expect(opts.map((o) => o.label)).toEqual([
      "Senin",
      "Selasa",
      "Rabu",
      "Kamis",
      "Jumat",
    ])
    // The stored scalar keeps its declared type — 1, not "1".
    expect(opts[0].value).toBe(1)
  })

  it("falls back to enum_values with a humanised caption", () => {
    const field: Field = {
      name: "status",
      type: "string",
      enum_values: ["in_progress", "done"],
    }
    expect(fieldOptions(field).map((o) => o.label)).toEqual([
      "In Progress",
      "Done",
    ])
  })

  it("lists a value once when declared in both options and enum_values", () => {
    const field: Field = {
      name: "x",
      type: "string",
      enum_values: ["a"],
      options: [{ value: "a", label: "A" }],
    }
    expect(fieldOptions(field)).toHaveLength(1)
  })

  it("is empty for a field with no declaration", () => {
    expect(fieldOptions({ name: "payload", type: "json" })).toEqual([])
  })

  it('canonicalises value identity so 1 and "1" are one choice', () => {
    expect(optionKey(1)).toBe(optionKey("1"))
    expect(optionKey(true)).toBe("true")
  })
})

describe("option value shape", () => {
  it("parses a stored json array, adopting declared scalar types", () => {
    const opts = fieldOptions(daysField)
    // "1" comes back as the declared number 1, so a saved array is [1], not ["1"].
    expect(parseOptionValue(["1", 2], opts)).toEqual([1, 2])
  })

  it("parses a comma-separated string for a string field", () => {
    const field: Field = {
      name: "tags",
      type: "string",
      options: [{ value: "a" }, { value: "b" }],
    }
    expect(parseOptionValue("a, b", fieldOptions(field))).toEqual(["a", "b"])
  })

  it("keeps an undeclared value so it is visible and removable", () => {
    // Legacy data (or a spec whose options shrank) must not vanish on save.
    expect(parseOptionValue([9, 1], fieldOptions(daysField))).toEqual([9, 1])
  })

  it("drops values that cannot be drawn as one chip", () => {
    expect(parseOptionValue([{ x: 1 }, 1], fieldOptions(daysField))).toEqual([
      1,
    ])
  })

  it("serializes back into the field's declared shape", () => {
    expect(serializeOptionValue([1, 2], "array")).toEqual([1, 2])
    expect(serializeOptionValue([1, 2], "string")).toBe("1,2")
    expect(valueShapeOf(daysField)).toBe("array")
    expect(valueShapeOf({ name: "t", type: "string" })).toBe("string")
  })

  it("labels an undeclared value rather than blanking it", () => {
    expect(optionLabel(fieldOptions(daysField), 9)).toBe("9")
  })
})

describe("SelectMultiTag", () => {
  it("renders the stored days as chips in declaration order", () => {
    render(
      <SelectMultiTag
        value={[5, 1]}
        entityField={daysField}
        ariaLabel="Hari Berlaku"
      />,
    )
    // Click order was 5 then 1; declaration order (Senin before Jumat) is what
    // a reader expects from an ordered set.
    const chips = screen.getAllByText(/Senin|Jumat/)
    expect(chips.map((c) => c.textContent)).toEqual(["Senin", "Jumat"])
  })

  it("does not offer an already-selected value", () => {
    render(
      <SelectMultiTag
        value={[1]}
        entityField={daysField}
        ariaLabel="Hari Berlaku"
      />,
    )
    openPicker()

    const list = screen.getByRole("listbox")
    expect(within(list).queryByText("Senin")).toBeNull()
    // The remaining choices are still offered.
    expect(within(list).getByText("Selasa")).toBeTruthy()
  })

  it("emits an array with the declared scalar type when a value is added", () => {
    const onChange = vi.fn()
    render(
      <SelectMultiTag
        value={[1]}
        entityField={daysField}
        onChange={onChange}
        ariaLabel="Hari Berlaku"
      />,
    )
    openPicker()
    fireEvent.click(screen.getByText("Rabu"))

    // Array shape preserved (json field), and the value is the number 3 — the
    // failure this widget closes is storing "3" in a numeric set.
    expect(onChange).toHaveBeenCalledWith([1, 3])
  })

  it("emits a comma-separated string for a string field", () => {
    const field: Field = {
      name: "tags",
      type: "string",
      options: [{ value: "a" }, { value: "b" }],
    }
    const onChange = vi.fn()
    render(
      <SelectMultiTag
        value={"a"}
        entityField={field}
        onChange={onChange}
        ariaLabel="Tags"
      />,
    )
    fireEvent.click(screen.getByLabelText("Tags"))
    fireEvent.click(screen.getByText("B"))

    expect(onChange).toHaveBeenCalledWith("a,b")
  })

  it("removes a chip when its X is clicked", () => {
    const onChange = vi.fn()
    render(
      <SelectMultiTag
        value={[1, 2]}
        entityField={daysField}
        onChange={onChange}
        ariaLabel="Hari Berlaku"
      />,
    )
    fireEvent.click(screen.getByLabelText("Remove Senin"))

    expect(onChange).toHaveBeenCalledWith([2])
  })

  it("shows an undeclared stored value, marked, instead of hiding it", () => {
    render(
      <SelectMultiTag
        value={[9]}
        entityField={daysField}
        ariaLabel="Hari Berlaku"
      />,
    )
    const chip = screen.getByText("9")
    // A dashed border marks it as "not in the declared options" — dropping it
    // silently would look like the field never had a value.
    expect(chip.className).toContain("border-dashed")
  })

  it("renders an error for a value that is not a list", () => {
    render(
      <SelectMultiTag
        value={{ oops: true }}
        entityField={daysField}
        ariaLabel="Hari Berlaku"
      />,
    )
    expect(screen.getByText(/bukan daftar/)).toBeTruthy()
  })

  it("renders read-only chips without controls", () => {
    render(<SelectMultiTag value={[1, 2]} entityField={daysField} readonly />)
    expect(screen.queryByLabelText("Hari Berlaku")).toBeNull()
    expect(screen.getByText("Senin")).toBeTruthy()
    expect(screen.getByText("Selasa")).toBeTruthy()
  })

  it("shows a placeholder when nothing is selected", () => {
    render(
      <SelectMultiTag
        value={[]}
        entityField={daysField}
        placeholder="Pilih hari"
        ariaLabel="Hari Berlaku"
      />,
    )
    expect(screen.getByText("Pilih hari")).toBeTruthy()
  })
})
