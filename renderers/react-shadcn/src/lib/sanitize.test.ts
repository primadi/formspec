import { describe, it, expect } from "vitest"
import { sanitizeHTML } from "./sanitize"

// Mirrors renderers/jsonb-persist/richtext_test.go TestSanitizeHTML.
describe("sanitizeHTML", () => {
  const cases: Array<[string, string]> = [
    ["<p>Hello</p>", "<p>Hello</p>"],
    ["<p>Hello<script>alert(1)</script></p>", "<p>Hello</p>"],
    ['<p onclick="x()">Hi</p>', "<p>Hi</p>"],
    ['<a href="javascript:alert(1)">x</a>', "<a>x</a>"],
    ['<iframe src="x"></iframe>', ""],
    ["<style>body{}</style><b>ok</b>", "<b>ok</b>"],
  ]

  it.each(cases)("sanitizeHTML(%j) === %j", (input, expected) => {
    expect(sanitizeHTML(input)).toBe(expected)
  })
})
