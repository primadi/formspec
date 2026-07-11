// ─── FormaExpr Parser ───
//
// Pratt (top-down operator precedence) parser for the FormaExpr language.
// Produces an AST consumed by the tree-walking evaluator.
//
// Precedence levels (lowest to highest):
//   1. OR (||, or)
//   2. AND (&&, and)
//   3. EQ/NEQ, GT/LT/GTE/LTE (comparison)
//   4. PLUS/MINUS (addition)
//   5. STAR/SLASH (multiplication)
//   6. Unary (!, not, -)
//   7. Call, Member access (.name)

import { Lexer, type Token, type TokenType } from "./lexer"

// ── AST Node Types ──

export type NodeType =
  | "Program"
  | "ExpressionStatement"
  | "BinaryExpr"
  | "UnaryExpr"
  | "Identifier"
  | "NumberLiteral"
  | "StringLiteral"
  | "BooleanLiteral"
  | "NullLiteral"
  | "MemberExpr"
  | "CallExpr"
  | "ListLiteral"

export interface Node {
  type: NodeType
  loc?: { line: number; col: number }
}

export interface Program extends Node {
  type: "Program"
  body: ExpressionStatement[]
}

export interface ExpressionStatement extends Node {
  type: "ExpressionStatement"
  expression: Expression
}

export type Expression =
  | BinaryExpr
  | UnaryExpr
  | Identifier
  | NumberLiteral
  | StringLiteral
  | BooleanLiteral
  | NullLiteral
  | MemberExpr
  | CallExpr
  | ListLiteral

export interface BinaryExpr extends Node {
  type: "BinaryExpr"
  op: string
  left: Expression
  right: Expression
}

export interface UnaryExpr extends Node {
  type: "UnaryExpr"
  op: string
  right: Expression
}

export interface Identifier extends Node {
  type: "Identifier"
  name: string
}

export interface NumberLiteral extends Node {
  type: "NumberLiteral"
  value: number
}

export interface StringLiteral extends Node {
  type: "StringLiteral"
  value: string
}

export interface BooleanLiteral extends Node {
  type: "BooleanLiteral"
  value: boolean
}

export interface NullLiteral extends Node {
  type: "NullLiteral"
}

export interface MemberExpr extends Node {
  type: "MemberExpr"
  object: Expression
  property: string
}

export interface CallExpr extends Node {
  type: "CallExpr"
  callee: Identifier
  args: Expression[]
}

export interface ListLiteral extends Node {
  type: "ListLiteral"
  elements: Expression[]
}

// ── Precedence ──

const PRECEDENCE: Record<TokenType, number> = {
  OR_OP: 1,
  AND_OP: 2,
  OR: 1,
  AND: 2,
  IN: 3,
  EQ: 3,
  NEQ: 3,
  LT: 3,
  GT: 3,
  LTE: 3,
  GTE: 3,
  PLUS: 4,
  MINUS: 4,
  STAR: 5,
  SLASH: 5,
  LPAREN: 7,  // function call
  DOT: 7,     // member access
  // Default
  ILLEGAL: 0,
  EOF: 0,
  NUMBER: 0,
  STRING: 0,
  TRUE: 0,
  FALSE: 0,
  NULL: 0,
  IDENTIFIER: 0,
  LEN: 0,
  SUM: 0,
  IF: 0,
  ELSE: 0,
  NOT: 0,
  BANG: 0,
  RPAREN: 0,
  RBRACKET: 0,
  COMMA: 0,
  LBRACKET: 0,
}

type PrefixFn = () => Expression
type InfixFn = (left: Expression) => Expression

export class Parser {
  private lexer: Lexer
  private curToken: Token
  private peekToken: Token
  private errors: string[] = []

  private prefixFns: Map<TokenType, PrefixFn> = new Map()
  private infixFns: Map<TokenType, InfixFn> = new Map()

  constructor(input: string) {
    this.lexer = new Lexer(input)
    this.curToken = this.lexer.nextToken()
    this.peekToken = this.lexer.nextToken()

    // Register prefix parsers
    this.prefixFns.set("IDENTIFIER", this.parseIdentifier.bind(this))
    this.prefixFns.set("NUMBER", this.parseNumberLiteral.bind(this))
    this.prefixFns.set("STRING", this.parseStringLiteral.bind(this))
    this.prefixFns.set("TRUE", this.parseBooleanLiteral.bind(this))
    this.prefixFns.set("FALSE", this.parseBooleanLiteral.bind(this))
    this.prefixFns.set("NULL", this.parseNullLiteral.bind(this))
    this.prefixFns.set("MINUS", this.parseUnaryExpr.bind(this))
    this.prefixFns.set("BANG", this.parseUnaryExpr.bind(this))
    this.prefixFns.set("NOT", this.parseUnaryExpr.bind(this))
    this.prefixFns.set("LEN", this.parseIdentifier.bind(this))   // treated as identifier, resolved at call time
    this.prefixFns.set("SUM", this.parseIdentifier.bind(this))   // same
    this.prefixFns.set("LPAREN", this.parseGroupedExpr.bind(this))
    this.prefixFns.set("LBRACKET", this.parseListLiteral.bind(this))
    this.prefixFns.set("PLUS", this.parseUnaryExpr.bind(this))

    // Register infix parsers
    this.infixFns.set("PLUS", this.parseBinaryExpr.bind(this))
    this.infixFns.set("MINUS", this.parseBinaryExpr.bind(this))
    this.infixFns.set("STAR", this.parseBinaryExpr.bind(this))
    this.infixFns.set("SLASH", this.parseBinaryExpr.bind(this))
    this.infixFns.set("EQ", this.parseBinaryExpr.bind(this))
    this.infixFns.set("NEQ", this.parseBinaryExpr.bind(this))
    this.infixFns.set("LT", this.parseBinaryExpr.bind(this))
    this.infixFns.set("GT", this.parseBinaryExpr.bind(this))
    this.infixFns.set("LTE", this.parseBinaryExpr.bind(this))
    this.infixFns.set("GTE", this.parseBinaryExpr.bind(this))
    this.infixFns.set("AND_OP", this.parseBinaryExpr.bind(this))
    this.infixFns.set("OR_OP", this.parseBinaryExpr.bind(this))
    this.infixFns.set("AND", this.parseBinaryExpr.bind(this))
    this.infixFns.set("OR", this.parseBinaryExpr.bind(this))
    this.infixFns.set("IN", this.parseBinaryExpr.bind(this))
    this.infixFns.set("LPAREN", this.parseCallExpr.bind(this))
    this.infixFns.set("DOT", this.parseMemberExpr.bind(this))
  }

  parseProgram(): Program {
    const program: Program = {
      type: "Program",
      body: [],
    }

    while (!this.curTokenIs("EOF")) {
      const stmt = this.parseStatement()
      if (stmt) {
        program.body.push(stmt)
      }
      this.nextToken()
    }

    return program
  }

  getErrors(): string[] {
    return this.errors
  }

  private parseStatement(): ExpressionStatement | null {
    if (this.curTokenIs("EOF")) return null

    const expr = this.parseExpression(0)
    if (!expr) return null

    return {
      type: "ExpressionStatement",
      expression: expr,
      loc: { line: this.curToken.line, col: this.curToken.col },
    }
  }

  private parseExpression(precedence: number): Expression | null {
    const prefixFn = this.prefixFns.get(this.curToken.type)
    if (!prefixFn) {
      this.errors.push(`unexpected token: ${this.curToken.literal} at line ${this.curToken.line}:${this.curToken.col}`)
      return null
    }

    let left = prefixFn()

    while (!this.peekTokenIs("EOF") && precedence < this.peekPrecedence()) {
      const infixFn = this.infixFns.get(this.peekToken.type)
      if (!infixFn) break
      this.nextToken()
      left = infixFn(left!)
    }

    return left
  }

  private nextToken(): void {
    this.curToken = this.peekToken
    this.peekToken = this.lexer.nextToken()
  }

  private curTokenIs(type: TokenType): boolean {
    return this.curToken.type === type
  }

  private peekTokenIs(type: TokenType): boolean {
    return this.peekToken.type === type
  }

  private peekPrecedence(): number {
    return PRECEDENCE[this.peekToken.type] ?? 0
  }

  private curPrecedence(): number {
    return PRECEDENCE[this.curToken.type] ?? 0
  }

  private expectPeek(type: TokenType): boolean {
    if (this.peekTokenIs(type)) {
      this.nextToken()
      return true
    }
    this.errors.push(`expected ${type} but got ${this.peekToken.literal} at line ${this.peekToken.line}:${this.peekToken.col}`)
    return false
  }

  // ── Prefix Parsers ──

  private parseIdentifier(): Expression {
    const id: Identifier = {
      type: "Identifier",
      name: this.curToken.literal,
      loc: { line: this.curToken.line, col: this.curToken.col },
    }
    return id
  }

  private parseNumberLiteral(): Expression {
    const num: NumberLiteral = {
      type: "NumberLiteral",
      value: parseFloat(this.curToken.literal),
      loc: { line: this.curToken.line, col: this.curToken.col },
    }
    return num
  }

  private parseStringLiteral(): Expression {
    const str: StringLiteral = {
      type: "StringLiteral",
      value: this.curToken.literal,
      loc: { line: this.curToken.line, col: this.curToken.col },
    }
    return str
  }

  private parseBooleanLiteral(): Expression {
    const bool: BooleanLiteral = {
      type: "BooleanLiteral",
      value: this.curToken.type === "TRUE",
      loc: { line: this.curToken.line, col: this.curToken.col },
    }
    return bool
  }

  private parseNullLiteral(): Expression {
    return {
      type: "NullLiteral",
      loc: { line: this.curToken.line, col: this.curToken.col },
    }
  }

  private parseUnaryExpr(): Expression {
    const op = this.curToken.literal
    this.nextToken()
    const right = this.parseExpression(PRECEDENCE.BANG || 6)
    return {
      type: "UnaryExpr",
      op,
      right: right!,
      loc: { line: this.curToken.line, col: this.curToken.col },
    } as UnaryExpr
  }

  private parseGroupedExpr(): Expression {
    this.nextToken()
    const expr = this.parseExpression(0)
    if (!this.expectPeek("RPAREN")) return expr!
    return expr!
  }

  private parseListLiteral(): Expression {
    const list: ListLiteral = {
      type: "ListLiteral",
      elements: [],
      loc: { line: this.curToken.line, col: this.curToken.col },
    }

    this.nextToken()
    if (this.curTokenIs("RBRACKET")) {
      return list
    }

    const first = this.parseExpression(0)
    if (first) list.elements.push(first)

    while (this.peekTokenIs("COMMA")) {
      this.nextToken() // consume COMMA
      this.nextToken() // move to next element
      const el = this.parseExpression(0)
      if (el) list.elements.push(el)
    }

    this.expectPeek("RBRACKET")
    return list
  }

  // ── Infix Parsers ──

  private parseBinaryExpr(left: Expression): Expression {
    const op = this.curToken.literal
    const precedence = this.curPrecedence()
    this.nextToken()
    const right = this.parseExpression(precedence)
    return {
      type: "BinaryExpr",
      op,
      left,
      right: right!,
      loc: { line: this.curToken.line, col: this.curToken.col },
    } as BinaryExpr
  }

  private parseMemberExpr(left: Expression): Expression {
    // After DOT, expect an identifier
    this.nextToken()
    if (!this.curTokenIs("IDENTIFIER") && !this.curTokenIs("LEN") && !this.curTokenIs("SUM")) {
      this.errors.push(`expected identifier after '.' but got ${this.curToken.literal}`)
      return left
    }

    return {
      type: "MemberExpr",
      object: left,
      property: this.curToken.literal,
      loc: { line: this.curToken.line, col: this.curToken.col },
    } as MemberExpr
  }

  private parseCallExpr(callee: Expression): Expression {
    const args: Expression[] = []
    this.nextToken() // consume LPAREN

    if (!this.curTokenIs("RPAREN")) {
      const first = this.parseExpression(0)
      if (first) args.push(first)

      while (this.peekTokenIs("COMMA")) {
        this.nextToken() // COMMA
        this.nextToken()
        const arg = this.parseExpression(0)
        if (arg) args.push(arg)
      }
    }

    this.expectPeek("RPAREN")

    return {
      type: "CallExpr",
      callee: callee as Identifier,
      args,
      loc: { line: this.curToken.line, col: this.curToken.col },
    } as CallExpr
  }
}

// ── Convenience: parse once ──

export function parse(input: string): Program {
  const parser = new Parser(input)
  const program = parser.parseProgram()
  if (parser.getErrors().length > 0) {
    // Return a minimal program with expressions parsed so far
    // The evaluator will log warnings for these
  }
  return program
}
