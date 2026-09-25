// @vitest-environment jsdom
//
// ─── Table column align/width (kafe 10.26) ───
//
// `TableColumn.align` and `.width` are in the contract (06-page-kinds.md §3
// lists both next to `sortable`/`link`) and were declared by authors — `align:
// right` on Total columns, six `width` values in the clinic visit table — but
// READ BY NO RENDERER. Every declaration was silently discarded, the same
// class of defect as the `money → currency` hole in 10.25.
//
// Three things must hold, or the vocabulary drifts apart again:
//
//   1. the helpers map the declared values to the right classes/CSS;
//   2. BOTH table renderers apply them (a fix in one only re-creates the
//      10.25 shape: one vocabulary, two spellings, one hole);
//   3. the rendered DOM actually carries them — asserted through a real
//      ListingRenderer render, not only through the helper.

import { afterEach, describe, expect, it, vi } from "vitest"
import { readFileSync } from "node:fs"
import { fileURLToPath } from "node:url"
import { screen, cleanup } from "@testing-library/react"

import {
  columnAlignClass,
  columnJustifyClass,
  columnWidthStyle,
} from "./tableColumn"

const read = (rel: string) =>
  readFileSync(fileURLToPath(new URL(rel, import.meta.url)), "utf8")

const tableSrc = read("../kinds/table/TableRenderer.tsx")
const listingSrc = read("../kinds/listing/ListingRenderer.tsx")

afterEach(cleanup)

describe("columnAlignClass", () => {
  it("maps the closed set, and defaults to no class", () => {
    expect(columnAlignClass("left")).toBe("")
    expect(columnAlignClass("center")).toBe("text-center")
    expect(columnAlignClass("right")).toBe("text-right")
  })

  it("stays silent for undeclared or unknown values — never guesses", () => {
    expect(columnAlignClass(undefined)).toBe("")
    expect(columnAlignClass("")).toBe("")
    expect(columnAlignClass("justify")).toBe("")
  })
})

describe("columnJustifyClass", () => {
  // A sortable header wraps its label in a flex row, and `text-align` cannot
  // move a flex child — without this the header label of a right-aligned
  // column would still sit on the left.
  it("mirrors the alignment for a flex header row", () => {
    expect(columnJustifyClass("left")).toBe("")
    expect(columnJustifyClass("center")).toBe("justify-center")
    expect(columnJustifyClass("right")).toBe("justify-end")
    expect(columnJustifyClass(undefined)).toBe("")
  })
})

describe("columnWidthStyle", () => {
  it("applies the declared width to the box", () => {
    expect(columnWidthStyle("120px")).toEqual({
      width: "120px",
      minWidth: "120px",
    })
  })

  it("returns nothing when undeclared", () => {
    expect(columnWidthStyle(undefined)).toBeUndefined()
    expect(columnWidthStyle("")).toBeUndefined()
  })
})

describe("both table renderers honour the contract", () => {
  // The guard that matters: applying align in TableRenderer only would leave
  // Listing (the public catalog surface) silently ignoring it.
  it.each([
    ["TableRenderer", tableSrc],
    ["ListingRenderer", listingSrc],
  ])("%s applies align and width", (_name, src) => {
    expect(src).toContain("columnAlignClass")
    expect(src).toMatch(/columnWidthStyle|columnByField/)
  })

  it("TableRenderer aligns both the header and the body cell", () => {
    // text-align is not inherited from <th> to <td> (they are siblings), so a
    // single application would render a right-aligned header over
    // left-aligned numbers.
    const headerAlign = tableSrc.indexOf("columnAlignClass(col?.align)")
    const bodyAlign = tableSrc.indexOf("?.align,")
    expect(headerAlign).toBeGreaterThan(-1)
    expect(bodyAlign).toBeGreaterThan(-1)
  })
})

// ── DOM assertion: the behaviour, not the wiring ──

vi.mock("@/hooks/useRealtime", () => ({ useRealtime: () => 0 }))

/** Bundle + session the renderers need to mount and fetch one row.
 *  `children` receives the entity schema so each test builds its own element
 *  (ListingRenderer takes a UI-manifest entry, TableRenderer an entity). */
async function mountWith(
  children: (entity: unknown) => React.ReactNode,
  authoredTable?: unknown,
) {
  const { useMetaStore } = await import("@/stores/meta")
  const { useSessionStore } = await import("@/stores/session")

  const entity = {
    module: "catalog",
    name: "product",
    plural: "products",
    label_field: "name",
    fields: [
      { name: "name", type: "string" },
      { name: "price", type: "money" },
    ],
    actions: [],
    characteristic: "master",
  }

  useSessionStore.setState({
    workspace: "demo",
    token: "t",
    me: { user_id: "u", roles: [], permissions: ["*"] } as never,
    getClient: () =>
      ({
        get: () => ({
          json: async () => ({
            // `apiList` reads the envelope `{ data, meta }`.
            data: [{ id: "1", name: "Kopi", price: { amount: "18000" } }],
            meta: { page: 1, per_page: 20, total: 1, total_pages: 1 },
          }),
        }),
      }) as never,
  })
  useMetaStore.setState({
    bundle: {
      app: { name: "catalog", root_url: "/app", chrome: {} },
      entities: [entity],
      // Every collection the store indexes must be present as an array —
      // it builds name/plural maps from each one.
      pages: [],
      forms: [],
      tables: authoredTable ? [authoredTable] : [],
      dashboards: [],
      widgets: [],
      reports: [],
      wizards: [],
      kanbans: [],
      timelines: [],
      prints: [],
      themes: [],
      listings: [],
      settings: {
        currency: { code: "IDR", decimal_places: 0, symbol: "Rp" },
      },
    } as never,
  })

  const { MemoryRouter } = await import("react-router-dom")
  const { render: renderApp } = await import("@testing-library/react")
  renderApp(<MemoryRouter>{children(entity) as never}</MemoryRouter>)
  return entity
}

describe("ListingRenderer DOM", () => {
  it("renders a right-aligned, fixed-width column as declared", async () => {
    const { default: ListingRenderer } =
      await import("@/kinds/listing/ListingRenderer")

    await mountWith(() => (
      <ListingRenderer
        entry={
          {
            apiVersion: "formspec.dev/v1",
            kind: "Listing",
            name: "product-catalog",
            module: "catalog",
            spec: {
              entity: "catalog.product",
              columns: [
                { field: "name", label: "Nama" },
                {
                  field: "price",
                  label: "Harga",
                  align: "right",
                  width: "120px",
                },
              ],
            },
          } as never
        }
      />
    ))

    // The row arrives from the mocked list call.
    const harga = await screen.findByText("Harga")
    expect(harga.className).toContain("text-right")
    // Width is applied to the header cell — the box the browser sizes.
    expect((harga as HTMLElement).style.width).toBe("120px")
    // And the un-declared column keeps the default (no stray alignment).
    expect((await screen.findByText("Nama")).className).not.toContain(
      "text-right",
    )
  })
})

describe("TableRenderer DOM", () => {
  it("applies align and width from the authored Table to th and td", async () => {
    const { default: TableRenderer } =
      await import("@/kinds/table/TableRenderer")

    // An authored Table is what carries align/width — a derived table declares
    // none (and correctly derives no alignment).
    await mountWith(
      (ent) => <TableRenderer entity={ent as never} hideTitle />,
      {
        name: "product-table",
        module: "catalog",
        spec: {
          entity: "catalog.product",
          columns: [
            { field: "name", label: "Nama" },
            {
              field: "price",
              label: "Harga",
              format: "currency",
              align: "right",
              width: "140px",
            },
          ],
        },
      },
    )

    const harga = await screen.findByText("Harga")
    const th = harga.closest("th") as HTMLTableCellElement
    expect(th.className).toContain("text-right")
    expect(th.style.width).toBe("140px")
    expect(th.style.minWidth).toBe("140px")
    // The sortable header wraps its label in a flex row, where `text-align`
    // has no effect — the box needs justify-end or the label stays left.
    expect(harga.className).toContain("justify-end")

    // The BODY cell matters most: <td> is a sibling of <th>, so text-align is
    // not inherited. A header-only fix would show "Harga" right-aligned over
    // left-aligned numbers.
    const row = (await screen.findByText("Kopi")).closest("tr")!
    const [, priceCell] = [...row.querySelectorAll("td")] as HTMLElement[]
    expect(priceCell.className).toContain("text-right")
    // And the un-declared column keeps the default.
    const [nameCell] = [...row.querySelectorAll("td")] as HTMLElement[]
    expect(nameCell.className).not.toContain("text-right")
  })
})
