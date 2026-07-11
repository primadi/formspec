// ─── FormaExpr Lexer ───
//
// Tokenizer for the FormaExpr expression language — a safe subset of Starlark.
// Produces a stream of Tokens consumed by the Pratt parser.
//
// Grammar (same as frontend-renderer.md §5.7):
//   - Literals: numbers, strings (double-quoted), booleans, null
//   - Identifiers: alphanumeric + underscore/dot (for member access)
//   - Operators: + - * / == != < <= > >= && || ! and or not in
//   - Functions: len, sum
//   - Delimiters: ( ) [ ] , .

export type TokenType =
  // Literals
  | "NUMBER"
  | "STRING"
  | "TRUE"
  | "FALSE"
  | "NULL"

  // Identifiers & keywords
  | "IDENTIFIER"
  | "AND"
  | "OR"
  | "NOT"
  | "IN"
  | "LEN"
  | "SUM"
  | "IF"
  | "ELSE"

  // Operators (multi-char)
  | "EQ"       // ==
  | "NEQ"      // !=
  | "GTE"      // >=
  | "LTE"      // <=
  | "AND_OP"   // &&
  | "OR_OP"    // ||

  // Operators (single-char)
  | "PLUS"
  | "MINUS"
  | "STAR"
  | "SLASH"
  | "GT"
  | "LT"
  | "BANG"
  | "DOT"
  | "COMMA"

  // Delimiters
  | "LPAREN"
  | "RPAREN"
  | "LBRACKET"
  | "RBRACKET"

  // Special
  | "EOF"
  | "ILLEGAL"

export interface Token {
  type: TokenType
  literal: string
  line: number
  col: number
}

const KEYWORDS: Record<string, TokenType> = {
  and: "AND",
  or: "OR",
  not: "NOT",
  in: "IN",
  len: "LEN",
  sum: "SUM",
  if: "IF",
  else: "ELSE",
  true: "TRUE",
  false: "FALSE",
  null: "NULL",
}

export class Lexer {
  private input: string
  private pos = 0
  private readPos = 0
  private ch: string = ""
  private line = 1
  private col = 0

  constructor(input: string) {
    this.input = input
    this.readChar()
  }

  private readChar(): void {
    if (this.readPos >= this.input.length) {
      this.ch = "\0"
    } else {
      this.ch = this.input[this.readPos]
    }
    this.pos = this.readPos
    this.readPos++
    if (this.ch === "\n") {
      this.line++
      this.col = 0
    } else {
      this.col++
    }
  }

  private peekChar(): string {
    if (this.readPos >= this.input.length) return "\0"
    return this.input[this.readPos]
  }

  private skipWhitespace(): void {
    while (this.ch === " " || this.ch === "\t" || this.ch === "\n" || this.ch === "\r") {
      this.readChar()
    }
  }

  nextToken(): Token {
    this.skipWhitespace()
    const line = this.line
    const col = this.col

    let type: TokenType = "ILLEGAL"
    let literal = this.ch

    switch (this.ch) {
      case "\0":
        type = "EOF"
        literal = ""
        break
      case "+":
        type = "PLUS"
        this.readChar()
        break
      case "-":
        type = "MINUS"
        this.readChar()
        break
      case "*":
        type = "STAR"
        this.readChar()
        break
      case "/":
        type = "SLASH"
        this.readChar()
        break
      case "(":
        type = "LPAREN"
        this.readChar()
        break
      case ")":
        type = "RPAREN"
        this.readChar()
        break
      case "[":
        type = "LBRACKET"
        this.readChar()
        break
      case "]":
        type = "RBRACKET"
        this.readChar()
        break
      case ".":
        type = "DOT"
        this.readChar()
        break
      case ",":
        type = "COMMA"
        this.readChar()
        break
      case "!":
        if (this.peekChar() === "=") {
          type = "NEQ"
          literal = "!="
          this.readChar()
          this.readChar()
        } else {
          type = "BANG"
          this.readChar()
        }
        break
      case "=":
        if (this.peekChar() === "=") {
          type = "EQ"
          literal = "=="
          this.readChar()
          this.readChar()
        } else {
          type = "ILLEGAL"
          this.readChar()
        }
        break
      case "<":
        if (this.peekChar() === "=") {
          type = "LTE"
          literal = "<="
          this.readChar()
          this.readChar()
        } else {
          type = "LT"
          this.readChar()
        }
        break
      case ">":
        if (this.peekChar() === "=") {
          type = "GTE"
          literal = ">="
          this.readChar()
          this.readChar()
        } else {
          type = "GT"
          this.readChar()
        }
        break
      case "&":
        if (this.peekChar() === "&") {
          type = "AND_OP"
          literal = "&&"
          this.readChar()
          this.readChar()
        } else {
          type = "ILLEGAL"
          this.readChar()
        }
        break
      case "|":
        if (this.peekChar() === "|") {
          type = "OR_OP"
          literal = "||"
          this.readChar()
          this.readChar()
        } else {
          type = "ILLEGAL"
          this.readChar()
        }
        break
      case '"':
        return this.readString()
      default:
        if (this.isLetter(this.ch) || this.ch === "_") {
          return this.readIdentifier()
        }
        if (this.isDigit(this.ch)) {
          return this.readNumber()
        }
        this.readChar()
        type = "ILLEGAL"
    }

    return { type, literal, line, col }
  }

  private readString(): Token {
    const line = this.line
    const col = this.col
    let value = ""
    this.readChar() // skip opening "

    while (this.ch !== '"' && this.ch !== "\0") {
      if (this.ch === "\\") {
        this.readChar()
        const c: string = this.ch
        if (c === "n") value += "\n"
        else if (c === "t") value += "\t"
        else if (c === "\\") value += "\\"
        else if (c === '"') value += '"'
        else value += c
      } else {
        value += this.ch
      }
      this.readChar()
    }

    if (this.ch === '"') {
      this.readChar() // skip closing "
    }

    return { type: "STRING", literal: value, line, col }
  }

  private readIdentifier(): Token {
    const line = this.line
    const col = this.col
    const start = this.pos
    while (this.isLetterOrDigit(this.ch) || this.ch === "_") {
      this.readChar()
    }
    const literal = this.input.slice(start, this.pos)
    const type = KEYWORDS[literal.toLowerCase()] ?? "IDENTIFIER"
    return { type, literal, line, col }
  }

  private readNumber(): Token {
    const line = this.line
    const col = this.col
    const start = this.pos
    while (this.isDigit(this.ch) || this.ch === ".") {
      this.readChar()
    }
    return { type: "NUMBER", literal: this.input.slice(start, this.pos), line, col }
  }

  private isLetter(ch: string): boolean {
    return (ch >= "a" && ch <= "z") || (ch >= "A" && ch <= "Z") || ch === "_"
  }

  private isDigit(ch: string): boolean {
    return ch >= "0" && ch <= "9"
  }

  private isLetterOrDigit(ch: string): boolean {
    return this.isLetter(ch) || this.isDigit(ch)
  }
}
