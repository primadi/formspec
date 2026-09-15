// ─── FormSpecExpr Evaluator ───
//
// Tree-walking evaluator for the FormSpecExpr AST.
// Pure TypeScript, no eval(), no external dependencies.
//
// Evaluation context provides:
//   - `fields`: current form/entity field values
//   - `user`: current user properties
//   - Additional params from route/form context

import type {
  Program,
  Expression,
  BinaryExpr,
  UnaryExpr,
  Identifier,
  NumberLiteral,
  StringLiteral,
  BooleanLiteral,
  MemberExpr,
  CallExpr,
  ListLiteral,
  ListComprehension,
} from "./parser"

// ── Runtime Values ──

export type RuntimeValue =
  | number
  | string
  | boolean
  | null
  | RuntimeValue[]
  | RuntimeObject

export interface RuntimeObject {
  [key: string]: RuntimeValue
}

export interface EvalContext {
  fields?: RuntimeObject
  user?: RuntimeObject
  [key: string]: RuntimeValue | undefined
}

// ── Evaluator ──

let evalWarnings: string[] = []

export function getWarnings(): string[] {
  return evalWarnings
}

export function clearWarnings(): void {
  evalWarnings = []
}

export function evaluate(
  program: Program,
  context: EvalContext = {},
): RuntimeValue {
  clearWarnings()

  if (program.body.length === 0) {
    return null
  }

  // Evaluate the last expression statement (FormSpecExpr is expression-oriented)
  const lastStmt = program.body[program.body.length - 1]
  return evalExpression(lastStmt.expression, context)
}

function evalExpression(node: Expression, context: EvalContext): RuntimeValue {
  switch (node.type) {
    case "NumberLiteral":
      return (node as NumberLiteral).value
    case "StringLiteral":
      return (node as StringLiteral).value
    case "BooleanLiteral":
      return (node as BooleanLiteral).value
    case "NullLiteral":
      return null
    case "Identifier":
      return evalIdentifier(node as Identifier, context)
    case "UnaryExpr":
      return evalUnary(node as UnaryExpr, context)
    case "BinaryExpr":
      return evalBinary(node as BinaryExpr, context)
    case "MemberExpr":
      return evalMember(node as MemberExpr, context)
    case "CallExpr":
      return evalCall(node as CallExpr, context)
    case "ListLiteral":
      return evalList(node as ListLiteral, context)
    case "ListComprehension":
      return evalListComprehension(node as ListComprehension, context)
    default:
      evalWarnings.push(`unknown node type: ${(node as any).type}`)
      return null
  }
}

function evalIdentifier(node: Identifier, context: EvalContext): RuntimeValue {
  const name = node.name

  // Check context first (fields, user, or custom params)
  if (name in context) {
    return context[name] as RuntimeValue
  }

  // Check if it's a top-level field name
  if (context.fields && name in context.fields) {
    return context.fields[name]
  }

  // Check user properties
  if (context.user && name in context.user) {
    return context.user[name]
  }

  // For len/sum identifiers, we return them as-is — they become call expressions
  // when followed by (...). If they appear bare, return null.
  if (name === "len" || name === "sum") {
    return null
  }

  // Unknown identifiers default to null without warning.
  // In FormSpecExpr, accessing an undefined field/property is a normal operation
  // that produces null, not an error. Schema-level field references are
  // validated at deploy time (5.11.2); the runtime defensive layer (5.11.3)
  // surfaces genuine evaluation failures (parse errors, unknown operators/
  // functions, member access on non-objects) via strictEvalFormSpecExpr.
  return null
}

function evalUnary(node: UnaryExpr, context: EvalContext): RuntimeValue {
  const right = evalExpression(node.right, context)

  switch (node.op) {
    case "-": {
      // -money → money (S7); anything non-numeric is an error, not 0.
      const m = asMoney(right)
      if (m) return moneyValue(-m.amount, m.currency, m.scale)
      if (isNonNumeric(right)) {
        evalWarnings.push(
          `cannot negate ${describeValue(right)}: expected a number or a money value`,
        )
        return null
      }
      return -toNumber(right)
    }
    case "!":
    case "not":
      return !toBoolean(right)
    default:
      evalWarnings.push(`unknown unary operator: ${node.op}`)
      return null
  }
}

function evalBinary(node: BinaryExpr, context: EvalContext): RuntimeValue {
  const left = evalExpression(node.left, context)
  const right = evalExpression(node.right, context)

  switch (node.op) {
    // Arithmetic
    case "+":
    case "-":
    case "*":
    case "/": {
      const result = evalArithmetic(node.op, left, right)
      if (result !== UNRESOLVED) return result as RuntimeValue
      break
    }

    // Comparison
    case "==":
      return deepEqual(left, right)
    case "!=":
      return !deepEqual(left, right)
    case "<":
    case ">":
    case "<=":
    case ">=": {
      const cmp = evalOrder(node.op, left, right)
      if (cmp !== UNRESOLVED) return cmp as RuntimeValue
      break
    }

    // Logical
    case "&&":
    case "and":
      return toBoolean(left) && toBoolean(right)
    case "||":
    case "or":
      return toBoolean(left) || toBoolean(right)

    // Membership
    case "in": {
      if (typeof right === "string" && typeof left === "string") {
        return (right as string).includes(left as string)
      }
      if (Array.isArray(right)) {
        return right.some((item) => deepEqual(item, left))
      }
      return false
    }

    default:
      evalWarnings.push(`unknown binary operator: ${node.op}`)
      return null
  }

  // Arithmetic/comparison returned UNRESOLVED — unreachable in practice, but
  // it keeps the return type total.
  evalWarnings.push(`unsupported operands for "${node.op}"`)
  return null
}

function evalMember(node: MemberExpr, context: EvalContext): RuntimeValue {
  const object = evalExpression(node.object, context)

  // Graceful undefined/null — just return null, no warning
  if (object == null) {
    return null
  }

  if (typeof object !== "object" || Array.isArray(object)) {
    evalWarnings.push(`cannot access property ${node.property} on non-object`)
    return null
  }

  return (object as RuntimeObject)[node.property] ?? null
}

function evalCall(node: CallExpr, context: EvalContext): RuntimeValue {
  const calleeName = node.callee.name

  switch (calleeName) {
    case "len": {
      if (node.args.length !== 1) {
        evalWarnings.push(
          `len() expects exactly 1 argument, got ${node.args.length}`,
        )
        return 0
      }
      const arg = evalExpression(node.args[0], context)
      if (typeof arg === "string") return arg.length
      if (Array.isArray(arg)) return arg.length
      return 0
    }

    case "sum": {
      if (node.args.length !== 1) {
        evalWarnings.push(
          `sum() expects exactly 1 argument, got ${node.args.length}`,
        )
        return 0
      }
      const arg = evalExpression(node.args[0], context)
      if (!Array.isArray(arg)) {
        evalWarnings.push("sum() expects an array")
        return 0
      }
      // sum() is money-aware: a list of money values sums to money, a list of
      // numbers to a number, and a mixed/illegal list is an error (S7).
      return sumValues(arg)
    }

    case "amount": {
      if (node.args.length !== 1) {
        evalWarnings.push(
          `amount() expects exactly 1 argument, got ${node.args.length}`,
        )
        return null
      }
      const arg = evalExpression(node.args[0], context)
      const m = asMoney(arg)
      if (m) return m.amount
      const scalar = asScalar(arg)
      if (scalar === UNRESOLVED) {
        evalWarnings.push(
          `amount() expects a money value or a number, got ${describeValue(arg)}`,
        )
        return null
      }
      return scalar
    }

    case "currency": {
      if (node.args.length !== 1) {
        evalWarnings.push(
          `currency() expects exactly 1 argument, got ${node.args.length}`,
        )
        return null
      }
      const arg = evalExpression(node.args[0], context)
      const m = asMoney(arg)
      if (!m) {
        evalWarnings.push(
          `currency() expects a money value, got ${describeValue(arg)}`,
        )
        return null
      }
      return m.currency
    }

    default:
      evalWarnings.push(`unknown function: ${calleeName}`)
      return null
  }
}

function evalList(node: ListLiteral, context: EvalContext): RuntimeValue {
  return node.elements.map((el) => evalExpression(el, context))
}

/**
 * Evaluate a list comprehension: [element for var in iterable].
 * The comprehension variable is bound in a child scope so it shadows any
 * same-named field — matching Starlark semantics for the subset.
 */
function evalListComprehension(
  node: ListComprehension,
  context: EvalContext,
): RuntimeValue {
  const iterable = evalExpression(node.iterable, context)
  if (!Array.isArray(iterable)) {
    evalWarnings.push("list comprehension: iterable is not an array")
    return []
  }
  const results: RuntimeValue[] = []
  for (const item of iterable) {
    const childContext: EvalContext = {
      ...context,
      [node.varName]: item,
    }
    results.push(evalExpression(node.element, childContext))
  }
  return results
}

// ── Money arithmetic (S7 / gap #28) ──
//
// `money` is a first-class type whose value is the object
// {amount, currency} (docs/spec/backend/05-field-types.md §2). The canonical
// form for arithmetic over money is the direct one — no wrapper syntax:
//
//   m + m, m - m   → money        (currencies must match; mismatch = error)
//   m * n, n * m   → money        (n scalar)
//   m / n          → money        (n scalar, not zero)
//   m / m          → number       (ratio; same currency)
//   -m             → money
//   m <op> m       → boolean      (same currency)
//   amount(x)      → number       (explicit scalar extraction)
//   currency(x)    → string
//   sum([m…])      → money
//
// Anything else — a non-money object, an array, a boolean, a non-numeric
// string — is an ERROR (a warning, which makes the evaluation invalid) rather
// than a silent 0. A wrong money number must be loud.
//
// The result amount is rendered at the operands' scale so exact results keep
// their precision (`2 * 25000` → "50000", `0.1 + 0.2` → "0.3").
//
// The server-side evaluator (internal/starlark/money.go) implements the same
// table for `computed` fields; keep the two in step.

/** Sentinel: the operation did not apply and the caller must keep looking. */
const UNRESOLVED = Symbol("unresolved")

interface MoneyParts {
  amount: number
  currency: string
  scale: number
}

/** Recognize a money value: {amount, currency} with a non-empty currency. */
function asMoney(value: RuntimeValue): MoneyParts | null {
  if (value == null || typeof value !== "object" || Array.isArray(value)) {
    return null
  }
  const obj = value as RuntimeObject
  if (!("amount" in obj) || !("currency" in obj)) return null
  const currency = typeof obj.currency === "string" ? obj.currency : ""
  if (currency === "") return null
  const raw = obj.amount
  let amount: number
  if (typeof raw === "number") amount = raw
  else if (typeof raw === "string" && raw.trim() !== "") {
    amount = Number(raw)
  } else {
    return null
  }
  if (Number.isNaN(amount)) return null
  return { amount, currency, scale: scaleOf(raw) }
}

/** Decimal places of a raw amount (string or number). */
function scaleOf(raw: unknown): number {
  const s = typeof raw === "number" ? String(raw) : String(raw ?? "")
  const dot = s.indexOf(".")
  return dot >= 0 ? s.length - dot - 1 : 0
}

/** Build a canonical money value: {amount: "<decimal string>", currency}. */
function moneyValue(
  amount: number,
  currency: string,
  scale: number,
): RuntimeObject {
  const s = Math.max(0, Math.min(20, scale))
  return { amount: trimTrailingZeros(amount.toFixed(s)), currency }
}

/**
 * Drop insignificant trailing zeros ("4000.50" → "4000.5", "12500.0" →
 * "12500"), matching the server. A computed amount must not claim precision it
 * does not have — a 0-decimal currency would reject the exact result of
 * `25000 * 0.5` for having "one decimal place".
 */
function trimTrailingZeros(s: string): string {
  if (!s.includes(".")) return s
  return s.replace(/\.?0+$/, "")
}

/**
 * Coerce a Starlark-like scalar to a number. Returns UNRESOLVED for anything
 * that is not a number, numeric string, or boolean — callers decide whether
 * that is an error.
 */
function asScalar(value: RuntimeValue): number | typeof UNRESOLVED {
  if (typeof value === "number") return value
  if (typeof value === "boolean") return value ? 1 : 0
  if (typeof value === "string") {
    const trimmed = value.trim()
    if (trimmed === "") return UNRESOLVED
    const n = Number(trimmed)
    return Number.isNaN(n) ? UNRESOLVED : n
  }
  return UNRESOLVED
}

/** True for values that can never be an arithmetic operand. */
function isNonNumeric(value: RuntimeValue): boolean {
  if (value === null) return false // unset field → null is a normal value
  return asMoney(value) === null && asScalar(value) === UNRESOLVED
}

/** Human-readable type name for an error message. */
function describeValue(value: RuntimeValue): string {
  if (value === null) return "null"
  if (Array.isArray(value)) return "a list"
  switch (typeof value) {
    case "object":
      return "an object"
    case "string":
      return "a non-numeric string"
    case "boolean":
      return "a boolean"
    default:
      return typeof value
  }
}

/** Arithmetic for +, -, *, / including money semantics. */
function evalArithmetic(
  op: string,
  left: RuntimeValue,
  right: RuntimeValue,
): RuntimeValue | typeof UNRESOLVED {
  const lm = asMoney(left)
  const rm = asMoney(right)

  // Any money operand switches to money semantics.
  if (lm || rm) {
    return moneyArithmetic(op, lm, rm, left, right)
  }

  // Neither side is money: reject non-numeric operands instead of coercing
  // them to 0 (S7 — "field non-numerik ditolak dengan error").
  for (const [side, value] of [
    ["left", left],
    ["right", right],
  ] as const) {
    if (isNonNumeric(value)) {
      evalWarnings.push(
        `cannot apply "${op}" to ${describeValue(value)} as the ${side} operand: expected a number or a money value`,
      )
      return null
    }
  }

  const l = asScalar(left)
  const r = asScalar(right)
  const ln = l === UNRESOLVED ? 0 : l
  const rn = r === UNRESOLVED ? 0 : r

  if (op === "+") return ln + rn
  if (op === "-") return ln - rn
  if (op === "*") return ln * rn
  // Division: keep the long-standing scalar behaviour (0 + warning).
  if (rn === 0) {
    evalWarnings.push("division by zero")
    return 0
  }
  return ln / rn
}

function moneyArithmetic(
  op: string,
  lm: MoneyParts | null,
  rm: MoneyParts | null,
  left: RuntimeValue,
  right: RuntimeValue,
): RuntimeValue {
  const fail = (msg: string): null => {
    evalWarnings.push(msg)
    return null
  }

  switch (op) {
    case "+":
    case "-": {
      if (!lm || !rm) {
        return fail(
          `money ${op} ${describeValue(lm ? right : left)} is not defined: money can only be combined with another money value or a number`,
        )
      }
      if (lm.currency !== rm.currency) {
        return fail(
          `money ${op} money: currency mismatch (${lm.currency} vs ${rm.currency}) — convert explicitly before combining currencies`,
        )
      }
      const amount = op === "+" ? lm.amount + rm.amount : lm.amount - rm.amount
      return moneyValue(amount, lm.currency, Math.max(lm.scale, rm.scale))
    }

    case "*": {
      if (lm && rm) {
        return fail(
          `money * money is not defined: multiplying two amounts gives ${lm.currency}·${rm.currency}, which is not a currency (use money / money for a ratio)`,
        )
      }
      const scalar = asScalar(lm ? right : left)
      if (scalar === UNRESOLVED) {
        return fail(
          `money * ${describeValue(lm ? right : left)} is not defined: money can only be multiplied by a number`,
        )
      }
      const m = (lm ?? rm)!
      return moneyValue(
        m.amount * scalar,
        m.currency,
        m.scale + scaleOf(m.amount * scalar),
      )
    }

    case "/": {
      if (lm && rm) {
        // Ratio: a plain number ("margin").
        if (lm.currency !== rm.currency) {
          return fail(
            `money / money: currency mismatch (${lm.currency} vs ${rm.currency})`,
          )
        }
        if (rm.amount === 0) return fail("money / money: division by zero")
        return lm.amount / rm.amount
      }
      // number / money has no unit meaning.
      if (!lm) {
        return fail(
          `number / money is not defined: money can only divide by a number`,
        )
      }
      const divisor = asScalar(right)
      if (divisor === UNRESOLVED) {
        return fail(
          `money / ${describeValue(right)} is not defined: money can only be divided by a number`,
        )
      }
      if (divisor === 0) return fail("money / number: division by zero")
      return moneyValue(lm.amount / divisor, lm.currency, lm.scale)
    }
  }
  return null
}

/** Ordered comparison for <, >, <=, >= including money semantics. */
function evalOrder(
  op: string,
  left: RuntimeValue,
  right: RuntimeValue,
): boolean | null | typeof UNRESOLVED {
  const lm = asMoney(left)
  const rm = asMoney(right)

  if (lm || rm) {
    if (!lm || !rm) {
      evalWarnings.push(
        `cannot compare money with ${describeValue(lm ? right : left)}: both operands must be money of the same currency`,
      )
      return null
    }
    if (lm.currency !== rm.currency) {
      evalWarnings.push(
        `money comparison: currency mismatch (${lm.currency} vs ${rm.currency})`,
      )
      return null
    }
    const a = lm.amount
    const b = rm.amount
    if (op === "<") return a < b
    if (op === ">") return a > b
    if (op === "<=") return a <= b
    return a >= b
  }

  for (const value of [left, right]) {
    if (isNonNumeric(value)) {
      evalWarnings.push(
        `cannot apply "${op}" to ${describeValue(value)}: expected a number or a money value`,
      )
      return null
    }
  }
  const l = asScalar(left)
  const r = asScalar(right)
  const ln = l === UNRESOLVED ? 0 : l
  const rn = r === UNRESOLVED ? 0 : r
  if (op === "<") return ln < rn
  if (op === ">") return ln > rn
  if (op === "<=") return ln <= rn
  return ln >= rn
}

// ── Type Coercion Helpers ──

/**
 * Money-aware sum: a list of money values sums to money, a list of numbers to
 * a number. Mixing the two, or summing anything non-numeric, is an error.
 */
function sumValues(items: RuntimeValue[]): RuntimeValue {
  const money = items.map(asMoney)
  const anyMoney = money.some((m) => m !== null)

  if (anyMoney) {
    let total = 0
    let currency = ""
    let scale = 0
    for (const [i, m] of money.entries()) {
      if (m === null) {
        if (items[i] === null) continue
        evalWarnings.push(
          `sum(): cannot mix money and ${describeValue(items[i])} in one sum — a total must be all money or all numbers`,
        )
        return null
      }
      if (currency === "") currency = m.currency
      else if (m.currency !== currency) {
        evalWarnings.push(
          `sum(): currency mismatch (${currency} vs ${m.currency}) — convert explicitly before totalling`,
        )
        return null
      }
      total += m.amount
      scale = Math.max(scale, m.scale)
    }
    if (currency === "") return 0
    return moneyValue(total, currency, scale)
  }

  let total = 0
  for (const item of items) {
    if (item === null) continue
    const scalar = asScalar(item)
    if (scalar === UNRESOLVED) {
      evalWarnings.push(
        `sum(): ${describeValue(item)} is not a number — expected a list of numbers or money values`,
      )
      return null
    }
    total += scalar
  }
  return total
}

function toNumber(value: RuntimeValue): number {
  if (typeof value === "number") return value
  if (typeof value === "string") {
    const n = parseFloat(value)
    return isNaN(n) ? 0 : n
  }
  if (typeof value === "boolean") return value ? 1 : 0
  return 0
}

function toBoolean(value: RuntimeValue): boolean {
  if (typeof value === "boolean") return value
  if (typeof value === "number") return value !== 0
  if (typeof value === "string") return value !== ""
  if (value === null) return false
  if (Array.isArray(value)) return value.length > 0
  if (typeof value === "object") return true
  return false
}

function deepEqual(a: RuntimeValue, b: RuntimeValue): boolean {
  if (a === b) return true
  if (a == null || b == null) return a === b
  if (Array.isArray(a) && Array.isArray(b)) {
    if (a.length !== b.length) return false
    return a.every((item, idx) => deepEqual(item, b[idx]))
  }
  if (
    typeof a === "object" &&
    typeof b === "object" &&
    !Array.isArray(a) &&
    !Array.isArray(b)
  ) {
    const aKeys = Object.keys(a as RuntimeObject)
    const bKeys = Object.keys(b as RuntimeObject)
    if (aKeys.length !== bKeys.length) return false
    return aKeys.every((key) =>
      deepEqual((a as RuntimeObject)[key], (b as RuntimeObject)[key]),
    )
  }
  return a === b
}
