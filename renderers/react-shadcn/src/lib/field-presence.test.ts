import { describe, expect, it } from "vitest"
import type { Field } from "@/types/manifest"
import {
  fieldIsUserRequired,
  isServerMintedKey,
  isUserEnterableKey,
  resolveNaturalKeyEntry,
} from "@/lib/field-presence"
import { buildZodField } from "@/lib/zod-schema"

const generatedKey = {
  name: "number",
  type: "string",
  required: true,
  natural_key: true,
  natural_key_rule: { strategy: "sequence", format: "ORD-{seq:05d}" },
} as Field

const customKey = {
  name: "code",
  type: "string",
  required: true,
  natural_key: true,
  natural_key_rule: { strategy: "custom" },
} as Field

const autoOnlyKey = {
  name: "number",
  type: "string",
  required: true,
  natural_key: true,
  natural_key_entry: "auto_generated",
  natural_key_rule: { strategy: "sequence", format: "INV-{seq:05d}" },
} as Field

describe("resolveNaturalKeyEntry", () => {
  it("applies the same convention as the Go validator", () => {
    expect(resolveNaturalKeyEntry({ natural_key: true } as Field)).toBe(
      "user_entry",
    )
    expect(
      resolveNaturalKeyEntry({
        natural_key: true,
        natural_key_rule: { strategy: "sequence" },
      } as Field),
    ).toBe("auto_generated_if_empty")
    expect(
      resolveNaturalKeyEntry({
        natural_key: true,
        natural_key_rule: { strategy: "custom" },
      } as Field),
    ).toBe("user_entry")
  })

  it("honours an explicit declaration over the convention", () => {
    expect(resolveNaturalKeyEntry(autoOnlyKey)).toBe("auto_generated")
  })

  it("says nothing for a field that is not a natural key", () => {
    expect(resolveNaturalKeyEntry({ name: "x", type: "string" } as Field)).toBe(
      undefined,
    )
  })
})

describe("isServerMintedKey", () => {
  it("recognises a sequence-generated natural key", () => {
    expect(isServerMintedKey(generatedKey)).toBe(true)
    expect(isServerMintedKey(autoOnlyKey)).toBe(true)
  })

  it("does not claim a custom-strategy key is generated", () => {
    expect(isServerMintedKey(customKey)).toBe(false)
  })
})

describe("isUserEnterableKey", () => {
  it("keeps every ordinary field as an input", () => {
    // Regression pin: treating "not a natural key" as "not an input" emptied
    // every derived form of its ordinary fields.
    expect(isUserEnterableKey({ name: "note", type: "text" } as Field)).toBe(
      true,
    )
  })

  it("drops the engine-authored key and keeps the two input modes", () => {
    expect(isUserEnterableKey(autoOnlyKey)).toBe(false)
    expect(isUserEnterableKey(generatedKey)).toBe(true) // may be overridden
    expect(isUserEnterableKey(customKey)).toBe(true)
  })
})

describe("fieldIsUserRequired", () => {
  it("relaxes presence for a key the server mints", () => {
    // The entity validator sets required:true on every natural key — the key
    // must be on the row — but the user still owes nothing.
    expect(fieldIsUserRequired(generatedKey)).toBe(false)
  })

  it("keeps presence for a key the user must supply", () => {
    expect(fieldIsUserRequired(customKey)).toBe(true)
    expect(
      fieldIsUserRequired({
        name: "x",
        type: "string",
        required: true,
      } as Field),
    ).toBe(true)
  })
})

describe("buildZodField on a generated natural key", () => {
  it("accepts an empty value instead of demanding the server's key", () => {
    // Regression pin: before this, a Create form for an entity with a
    // server-generated key refused to submit until the user typed a number the
    // server was about to create.
    expect(buildZodField(generatedKey).safeParse("").success).toBe(true)
    expect(buildZodField(generatedKey).safeParse(undefined).success).toBe(true)
  })

  it("still refuses an empty value for a user-supplied key", () => {
    expect(buildZodField(customKey).safeParse("").success).toBe(false)
  })
})
