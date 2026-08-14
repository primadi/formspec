// ─── FormSpecExpr Tests ───
//
// Table-driven test suite covering the full FormSpecExpr grammar:
// literals, arithmetic, comparisons, logic, field access, string ops,
// function calls, error handling, and graceful fallback.
//
// Run with: npx vitest run src/lib/formspec-expr/formaexpr.test.ts

import { describe, it, expect } from "vitest"
import { Parser } from "./parser"
import { evaluate, clearWarnings, getWarnings } from "./eval"
import {
  evalFormSpecExpr,
  validateFormSpecExpr,
  evalVisibleWhen,
  evalReadonlyWhen,
  evalRequiredWhen,
  evalCompute,
} from "./index"
import { Lexer } from "./lexer"

// ── Lexer Tests ──

describe("FormSpecExpr Lexer", () => {
  const tokenize = (input: string) => {
    const lexer = new Lexer(input)
    const tokens = []
    let t = lexer.nextToken()
    while (t.type !== "EOF") {
      tokens.push(t)
      t = lexer.nextToken()
    }
    return tokens
  }

  it("tokenizes numbers", () => {
    const tokens = tokenize("42 3.14")
    expect(tokens[0].type).toBe("NUMBER")
    expect(tokens[0].literal).toBe("42")
    expect(tokens[1].type).toBe("NUMBER")
    expect(tokens[1].literal).toBe("3.14")
  })

  it("tokenizes strings", () => {
    const tokens = tokenize('"hello" "world"')
    expect(tokens[0].type).toBe("STRING")
    expect(tokens[0].literal).toBe("hello")
    expect(tokens[1].type).toBe("STRING")
    expect(tokens[1].literal).toBe("world")
  })

  it("tokenizes keywords", () => {
    const tokens = tokenize("true false null and or not in len sum")
    expect(tokens.map((t) => t.type)).toEqual([
      "TRUE", "FALSE", "NULL", "AND", "OR", "NOT", "IN", "LEN", "SUM",
    ])
  })

  it("tokenizes identifiers with dots", () => {
    const tokens = tokenize("fields.status user.role my_var")
    expect(tokens.map((t) => t.type)).toEqual([
      "IDENTIFIER", "DOT", "IDENTIFIER",
      "IDENTIFIER", "DOT", "IDENTIFIER",
      "IDENTIFIER",
    ])
  })
})

// ── Parser Tests ──

describe("FormSpecExpr Parser", () => {
  it("parses a number literal", () => {
    const p = new Parser("42")
    const prog = p.parseProgram()
    expect(prog.body).toHaveLength(1)
    expect(prog.body[0].expression).toMatchObject({ type: "NumberLiteral", value: 42 })
  })

  it("parses a string literal", () => {
    const p = new Parser('"hello"')
    const prog = p.parseProgram()
    expect(prog.body).toHaveLength(1)
    expect(prog.body[0].expression).toMatchObject({ type: "StringLiteral", value: "hello" })
  })

  it("parses a binary expression", () => {
    const p = new Parser("1 + 2")
    const prog = p.parseProgram()
    const expr = prog.body[0].expression
    expect(expr.type).toBe("BinaryExpr")
    expect((expr as any).op).toBe("+")
  })

  it("parses member expression", () => {
    const p = new Parser("fields.status")
    const prog = p.parseProgram()
    const expr = prog.body[0].expression
    expect(expr.type).toBe("MemberExpr")
    expect((expr as any).property).toBe("status")
  })

  it("parses comparison with ===", () => {
    const p = new Parser('fields.status == "paid"')
    const prog = p.parseProgram()
    expect(p.getErrors()).toHaveLength(0)
    const expr = prog.body[0].expression as any
    expect(expr.type).toBe("BinaryExpr")
    expect(expr.op).toBe("==")
  })

  it("parses nested member access", () => {
    const p = new Parser("fields.customer.name")
    const prog = p.parseProgram()
    expect(p.getErrors()).toHaveLength(0)
    const expr = prog.body[0].expression as any
    // fields.customer → MemberExpr(fields, customer)
    // .name → MemberExpr(MemberExpr(fields, customer), name)
    expect(expr.type).toBe("MemberExpr")
    expect(expr.property).toBe("name")
    expect(expr.object.type).toBe("MemberExpr")
    expect(expr.object.property).toBe("customer")
  })

  it("reports error for invalid syntax", () => {
    // Dangling dot (member access without property)
    const p = new Parser("fields.")
    p.parseProgram()
    expect(p.getErrors().length).toBeGreaterThan(0)
  })

  it("accepts unary plus as valid", () => {
    const p = new Parser("1 + +2")
    p.parseProgram()
    expect(p.getErrors()).toEqual([])
  })
})

// ── Evaluator Tests (table-driven) ──

describe("FormSpecExpr Evaluator", () => {
  interface EvalTestCase {
    name: string
    expr: string
    context?: Record<string, unknown>
    expected: unknown
  }

  const runCases = (cases: EvalTestCase[]) => {
    for (const { name, expr, context, expected } of cases) {
      it(name, () => {
        clearWarnings()
        const parser = new Parser(expr)
        const program = parser.parseProgram()
        const errors = parser.getErrors()
        expect(errors).toEqual([])

        const result = evaluate(program, context as any)
        expect(result).toEqual(expected)
        expect(getWarnings()).toEqual([])
      })
    }
  }

  runCases([
    // ── Literals ──
    { name: "integer literal", expr: "42", expected: 42 },
    { name: "float literal", expr: "3.14", expected: 3.14 },
    { name: "string literal", expr: '"hello"', expected: "hello" },
    { name: "boolean true", expr: "true", expected: true },
    { name: "boolean false", expr: "false", expected: false },
    { name: "null literal", expr: "null", expected: null },

    // ── Arithmetic ──
    { name: "addition", expr: "1 + 2", expected: 3 },
    { name: "subtraction", expr: "5 - 3", expected: 2 },
    { name: "multiplication", expr: "3 * 4", expected: 12 },
    { name: "division", expr: "10 / 2", expected: 5 },
    { name: "unary minus", expr: "-5", expected: -5 },
    { name: "operator precedence", expr: "1 + 2 * 3", expected: 7 },
    { name: "parentheses override", expr: "(1 + 2) * 3", expected: 9 },

    // ── Comparison ──
    { name: "eq true", expr: "1 == 1", expected: true },
    { name: "eq false", expr: "1 == 2", expected: false },
    { name: "neq true", expr: "1 != 2", expected: true },
    { name: "neq false", expr: "1 != 1", expected: false },
    { name: "lt", expr: "1 < 2", expected: true },
    { name: "gt", expr: "3 > 2", expected: true },
    { name: "lte", expr: "2 <= 2", expected: true },
    { name: "gte", expr: "3 >= 2", expected: true },

    // ── Logic ──
    { name: "and (&&)", expr: "true && false", expected: false },
    { name: "and (keyword)", expr: "true and false", expected: false },
    { name: "or (||)", expr: "true || false", expected: true },
    { name: "or (keyword)", expr: "true or false", expected: true },
    { name: "not (!)", expr: "!true", expected: false },
    { name: "not (keyword)", expr: "not false", expected: true },

    // ── Field Access ──
    {
      name: "field access",
      expr: 'fields.status == "paid"',
      context: { fields: { status: "paid" } },
      expected: true,
    },
    {
      name: "field access false",
      expr: 'fields.status == "draft"',
      context: { fields: { status: "paid" } },
      expected: false,
    },
    {
      name: "nested field access",
      expr: 'fields.customer.name == "Acme"',
      context: { fields: { customer: { name: "Acme" } } },
      expected: true,
    },
    {
      name: "boolean field",
      expr: "fields.active == true",
      context: { fields: { active: true } },
      expected: true,
    },
    {
      name: "number comparison",
      expr: "fields.total > 100",
      context: { fields: { total: 150 } },
      expected: true,
    },

    // ── User Context ──
    {
      name: "user role check",
      expr: 'user.role == "admin"',
      context: { user: { role: "admin" } },
      expected: true,
    },

    // ── len() ──
    { name: "len string", expr: 'len("hello")', expected: 5 },
    { name: "len list", expr: "len([1, 2, 3])", expected: 3 },
    {
      name: "len field",
      expr: "len(fields.name)",
      context: { fields: { name: "John" } },
      expected: 4,
    },

    // ── sum() ──
    {
      name: "sum list",
      expr: "sum([1, 2, 3])",
      expected: 6,
    },
    {
      name: "sum list of decimals",
      expr: "sum([10.5, 20.5])",
      expected: 31,
    },
    {
      name: "sum empty list",
      expr: "sum([])",
      expected: 0,
    },

    // ── List Literals ──
    {
      name: "list literal",
      expr: "[1, 2, 3]",
      expected: [1, 2, 3],
    },
    {
      name: "empty list",
      expr: "[]",
      expected: [],
    },
    {
      name: "list of strings",
      expr: '["a", "b", "c"]',
      expected: ["a", "b", "c"],
    },

    // ── Combined Expressions ──
    {
      name: "complex condition",
      expr: 'fields.status == "draft" and fields.total > 0',
      context: { fields: { status: "draft", total: 100 } },
      expected: true,
    },
    {
      name: "complex condition false",
      expr: 'fields.status == "draft" and fields.total > 0',
      context: { fields: { status: "paid", total: 100 } },
      expected: false,
    },
    {
      name: "or with field check",
      expr: 'fields.type == "premium" or fields.total > 1000',
      context: { fields: { type: "basic", total: 500 } },
      expected: false,
    },
  ])
})

// ── Error Handling & Fallback ──

describe("FormSpecExpr Error Handling", () => {
  it("returns null for empty expression", () => {
    const result = evalFormSpecExpr("")
    expect(result.value).toBeNull()
    expect(result.valid).toBe(true)
  })

  it("returns null for whitespace-only", () => {
    const result = evalFormSpecExpr("   ")
    expect(result.value).toBeNull()
    expect(result.valid).toBe(true)
  })

  it("reports invalid syntax", () => {
    const result = evalFormSpecExpr("1 + + 2")
    expect(result.valid).toBe(false)
    expect(result.warnings.length).toBeGreaterThan(0)
  })

  it("handles undefined field gracefully", () => {
    const result = evalFormSpecExpr('fields.missing == "x"')
    // fields.missing is undefined, returns null, comparison becomes null == "x" → false
    expect(result.value).toBe(false)
    expect(result.valid).toBe(true) // no parse error
  })

  it("handles missing context gracefully", () => {
    const result = evalFormSpecExpr("fields.status")
    expect(result.valid).toBe(true)
    // fields is undefined in context, returns null
  })

  it("division by zero returns 0 with warning", () => {
    const result = evalFormSpecExpr("1 / 0")
    expect(result.value).toBe(0)
    expect(result.warnings).toContain("division by zero")
  })
})

// ── Validation ──

describe("validateFormSpecExpr", () => {
  it("validates correct expression", () => {
    expect(validateFormSpecExpr("1 + 2")).toEqual({ valid: true })
  })

  it("validates expression with field access", () => {
    expect(validateFormSpecExpr('fields.status == "paid"')).toEqual({ valid: true })
  })

  it("rejects invalid expression", () => {
    const result = validateFormSpecExpr("fields.")
    expect(result.valid).toBe(false)
    expect(result.error).toBeTruthy()
  })

  it("accepts empty string", () => {
    expect(validateFormSpecExpr("")).toEqual({ valid: true })
  })
})

// ── Convenience Wrappers ──

describe("FormSpecExpr Convenience Wrappers", () => {
  const ctx = { fields: { status: "paid", total: 100 } }

  it("evalVisibleWhen returns true when expression is undefined", () => {
    expect(evalVisibleWhen(undefined, ctx)).toBe(true)
  })

  it("evalVisibleWhen returns true when expression is truthy", () => {
    expect(evalVisibleWhen('fields.status == "paid"', ctx)).toBe(true)
  })

  it("evalVisibleWhen returns false when expression is falsy", () => {
    expect(evalVisibleWhen('fields.status == "draft"', ctx)).toBe(false)
  })

  it("evalVisibleWhen defaults to true on parse error", () => {
    expect(evalVisibleWhen("1 + + 2", ctx)).toBe(true)
  })

  it("evalReadonlyWhen returns false when expression is undefined", () => {
    expect(evalReadonlyWhen(undefined, ctx)).toBe(false)
  })

  it("evalReadonlyWhen returns true when expression is truthy", () => {
    expect(evalReadonlyWhen('fields.status == "paid"', ctx)).toBe(true)
  })

  it("evalReadonlyWhen defaults to false on parse error", () => {
    expect(evalReadonlyWhen("1 + + 2", ctx)).toBe(false)
  })

  it("evalRequiredWhen defaults to false on undefined", () => {
    expect(evalRequiredWhen(undefined, ctx)).toBe(false)
  })

  it("evalCompute returns null on undefined", () => {
    expect(evalCompute(undefined, ctx)).toBeNull()
  })

  it("evalCompute computes value", () => {
    expect(evalCompute("fields.total * 0.1", ctx)).toBe(10)
  })
})

// ── Edge Cases ──

describe("FormSpecExpr Edge Cases", () => {
  it("handles string comparison with special characters", () => {
    const result = evalFormSpecExpr('"hello world" == "hello world"')
    expect(result.value).toBe(true)
  })

  it("handles escaped string", () => {
    const lexer = new Lexer('"hello\\nworld"')
    const tokens = []
    let t = lexer.nextToken()
    while (t.type !== "EOF") {
      tokens.push(t)
      t = lexer.nextToken()
    }
    expect(tokens[0].type).toBe("STRING")
    expect(tokens[0].literal).toBe("hello\nworld")
  })

  it("handles in operator with array", () => {
    const result = evalFormSpecExpr('1 in [1, 2, 3]')
    expect(result.value).toBe(true)
  })

  it("handles in operator with string", () => {
    const result = evalFormSpecExpr('"lo" in "hello"')
    expect(result.value).toBe(true)
  })

  it("handles in operator false case", () => {
    const result = evalFormSpecExpr('4 in [1, 2, 3]')
    expect(result.value).toBe(false)
  })

  it("handles very long expression", () => {
    const result = evalFormSpecExpr("1 + 2 + 3 + 4 + 5 + 6 + 7 + 8 + 9 + 10")
    expect(result.value).toBe(55)
  })

  it("handles boolean negation chain", () => {
    const result = evalFormSpecExpr("!!true")
    expect(result.value).toBe(true)
  })
})
