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
  // that produces null, not an error.
  return null
}

function evalUnary(node: UnaryExpr, context: EvalContext): RuntimeValue {
  const right = evalExpression(node.right, context)

  switch (node.op) {
    case "-":
      return -toNumber(right)
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
      return toNumber(left) + toNumber(right)
    case "-":
      return toNumber(left) - toNumber(right)
    case "*":
      return toNumber(left) * toNumber(right)
    case "/": {
      const r = toNumber(right)
      if (r === 0) {
        evalWarnings.push("division by zero")
        return 0
      }
      return toNumber(left) / r
    }

    // Comparison
    case "==":
      return left === right
    case "!=":
      return left !== right
    case "<":
      return toNumber(left) < toNumber(right)
    case ">":
      return toNumber(left) > toNumber(right)
    case "<=":
      return toNumber(left) <= toNumber(right)
    case ">=":
      return toNumber(left) >= toNumber(right)

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
      return arg.reduce((acc: number, item) => acc + toNumber(item), 0)
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

// ── Type Coercion Helpers ──

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
