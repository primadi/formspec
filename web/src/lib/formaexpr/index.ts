// ─── FormaExpr Unified API ───
//
// High-level convenience API for the FormaExpr interpreter.
// Components import this file, not the individual lexer/parser/eval modules.

import { Parser } from "./parser"
import { evaluate, getWarnings, type EvalContext, type RuntimeValue } from "./eval"

export type { EvalContext, RuntimeValue }

export interface FormaExprResult {
  value: RuntimeValue
  warnings: string[]
  valid: boolean
}

export interface ValidationResult {
  valid: boolean
  error?: string
}

/**
 * Evaluate a FormaExpr expression string against the given context.
 *
 * - Parses → evaluates → returns the result
 * - On parse error: logs warnings, returns null with `valid: false`
 * - On eval error: returns null with warnings
 *
 * @example
 * ```ts
 * evalFormaExpr('fields.status == "paid"', { fields: { status: "paid" } })
 * // → { value: true, warnings: [], valid: true }
 * ```
 */
export function evalFormaExpr(
  expr: string,
  context: EvalContext = {},
): FormaExprResult {
  if (!expr || expr.trim() === "") {
    return { value: null, warnings: [], valid: true }
  }

  const parser = new Parser(expr)
  const program = parser.parseProgram()
  const parseErrors = parser.getErrors()

  if (parseErrors.length > 0) {
    return {
      value: null,
      warnings: parseErrors,
      valid: false,
    }
  }

  const value = evaluate(program, context)
  const warnings = getWarnings()

  return {
    value,
    warnings,
    valid: warnings.length === 0,
  }
}

/**
 * Validate a FormaExpr expression string (syntax check only).
 * Returns `{ valid: true }` or `{ valid: false, error: "..." }`.
 */
export function validateFormaExpr(expr: string): ValidationResult {
  if (!expr || expr.trim() === "") {
    return { valid: true }
  }

  const parser = new Parser(expr)
  parser.parseProgram()
  const errors = parser.getErrors()

  if (errors.length > 0) {
    return { valid: false, error: errors.join("; ") }
  }

  return { valid: true }
}

// ── Convenience wrappers for common FormaExpr use-cases ──

/**
 * Evaluate a `visible_when` expression.
 * Returns `true` (visible) by default if parsing fails.
 */
export function evalVisibleWhen(
  expr: string | undefined,
  context: EvalContext = {},
): boolean {
  if (!expr) return true
  const result = evalFormaExpr(expr, context)
  if (!result.valid) return true // visible by default
  if (typeof result.value === "boolean") return result.value
  return true
}

/**
 * Evaluate a `readonly_when` expression.
 * Returns `false` (editable) by default if parsing fails.
 */
export function evalReadonlyWhen(
  expr: string | undefined,
  context: EvalContext = {},
): boolean {
  if (!expr) return false
  const result = evalFormaExpr(expr, context)
  if (!result.valid) return false // editable by default
  if (typeof result.value === "boolean") return result.value
  return false
}

/**
 * Evaluate a `required_when` expression.
 * Returns `false` (optional) by default if parsing fails.
 */
export function evalRequiredWhen(
  expr: string | undefined,
  context: EvalContext = {},
): boolean {
  if (!expr) return false
  const result = evalFormaExpr(expr, context)
  if (!result.valid) return false // optional by default
  if (typeof result.value === "boolean") return result.value
  return false
}

/**
 * Evaluate a `compute` expression and return the computed value.
 * Returns `null` if parsing/evaluation fails.
 */
export function evalCompute(
  expr: string | undefined,
  context: EvalContext = {},
): RuntimeValue {
  if (!expr) return null
  const result = evalFormaExpr(expr, context)
  if (!result.valid) return null
  return result.value
}
