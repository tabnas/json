/* Copyright (c) 2026 tabnas, MIT License */

/* json.ts
 *
 * A standard JSON parser (RFC 8259 / ECMA-404). It accepts exactly the
 * JSON grammar — objects, arrays, strings, numbers, and the literals
 * true, false and null — and nothing else. There is no extended grammar:
 * no comments, no trailing commas, no unquoted keys, no single-quoted or
 * multiline strings, no implicit objects or arrays. Input that
 * JSON.parse would reject, this parser rejects too.
 */

export type JsonValue =
  | null
  | boolean
  | number
  | string
  | JsonValue[]
  | { [key: string]: JsonValue }

// Structured parse error. The `code` is a stable machine-readable
// identifier; the message is human-readable and includes the source
// position.
export class JsonError extends Error {
  code: string
  index: number
  line: number
  column: number

  constructor(
    code: string,
    detail: string,
    index: number,
    line: number,
    column: number,
  ) {
    super(`[json/${code}] ${detail} (at line ${line}, column ${column})`)
    this.name = 'JsonError'
    this.code = code
    this.index = index
    this.line = line
    this.column = column
  }
}

// Whitespace recognised by JSON: space, tab, line feed, carriage return.
const isWs = (c: number): boolean =>
  c === 0x20 || c === 0x09 || c === 0x0a || c === 0x0d

const isDigit = (c: number): boolean => c >= 0x30 && c <= 0x39

// Escape sequences permitted inside a JSON string, excluding \u which is
// handled separately.
const ESCAPES: Record<string, string> = {
  '"': '"',
  '\\': '\\',
  '/': '/',
  b: '\b',
  f: '\f',
  n: '\n',
  r: '\r',
  t: '\t',
}

class Parser {
  private src: string
  private len: number
  private i = 0

  constructor(src: string) {
    this.src = src
    this.len = src.length
  }

  parse(): JsonValue {
    this.skipWs()
    if (this.i >= this.len) {
      this.fail('unexpected_eof', 'unexpected end of input')
    }
    const value = this.parseValue()
    this.skipWs()
    if (this.i < this.len) {
      this.fail(
        'trailing_content',
        `unexpected character ${JSON.stringify(this.src[this.i])}`,
      )
    }
    return value
  }

  private parseValue(): JsonValue {
    const c = this.src.charCodeAt(this.i)
    switch (c) {
      case 0x7b: // {
        return this.parseObject()
      case 0x5b: // [
        return this.parseArray()
      case 0x22: // "
        return this.parseString()
      case 0x74: // t
        return this.parseLiteral('true', true)
      case 0x66: // f
        return this.parseLiteral('false', false)
      case 0x6e: // n
        return this.parseLiteral('null', null)
      default:
        if (c === 0x2d || isDigit(c)) {
          return this.parseNumber()
        }
        this.fail(
          'unexpected_char',
          `unexpected character ${JSON.stringify(this.src[this.i])}`,
        )
    }
  }

  private parseObject(): { [key: string]: JsonValue } {
    this.i++ // consume {
    const obj: { [key: string]: JsonValue } = {}
    this.skipWs()
    if (this.src.charCodeAt(this.i) === 0x7d) {
      this.i++ // consume }
      return obj
    }
    for (;;) {
      this.skipWs()
      if (this.src.charCodeAt(this.i) !== 0x22) {
        if (this.i >= this.len) {
          this.fail('unexpected_eof', 'unexpected end of input in object')
        }
        this.fail('expected_key', 'expected a string key')
      }
      const key = this.parseString()
      this.skipWs()
      if (this.src.charCodeAt(this.i) !== 0x3a) {
        this.fail('expected_colon', "expected ':' after object key")
      }
      this.i++ // consume :
      this.skipWs()
      if (this.i >= this.len) {
        this.fail('unexpected_eof', 'unexpected end of input in object')
      }
      obj[key] = this.parseValue()
      this.skipWs()
      const c = this.src.charCodeAt(this.i)
      if (c === 0x2c) {
        this.i++ // consume , and continue
        continue
      }
      if (c === 0x7d) {
        this.i++ // consume }
        return obj
      }
      if (this.i >= this.len) {
        this.fail('unexpected_eof', 'unexpected end of input in object')
      }
      this.fail('expected_comma', "expected ',' or '}' in object")
    }
  }

  private parseArray(): JsonValue[] {
    this.i++ // consume [
    const arr: JsonValue[] = []
    this.skipWs()
    if (this.src.charCodeAt(this.i) === 0x5d) {
      this.i++ // consume ]
      return arr
    }
    for (;;) {
      this.skipWs()
      if (this.i >= this.len) {
        this.fail('unexpected_eof', 'unexpected end of input in array')
      }
      arr.push(this.parseValue())
      this.skipWs()
      const c = this.src.charCodeAt(this.i)
      if (c === 0x2c) {
        this.i++ // consume , and continue
        continue
      }
      if (c === 0x5d) {
        this.i++ // consume ]
        return arr
      }
      if (this.i >= this.len) {
        this.fail('unexpected_eof', 'unexpected end of input in array')
      }
      this.fail('expected_comma', "expected ',' or ']' in array")
    }
  }

  private parseString(): string {
    this.i++ // consume opening "
    let out = ''
    let start = this.i
    for (;;) {
      if (this.i >= this.len) {
        this.fail('unterminated_string', 'unterminated string')
      }
      const c = this.src.charCodeAt(this.i)
      if (c === 0x22) {
        // closing quote
        out += this.src.slice(start, this.i)
        this.i++
        return out
      }
      if (c === 0x5c) {
        // backslash escape
        out += this.src.slice(start, this.i)
        this.i++
        out += this.parseEscape()
        start = this.i
        continue
      }
      if (c < 0x20) {
        this.fail(
          'invalid_string_char',
          'control character must be escaped in a string',
        )
      }
      this.i++
    }
  }

  private parseEscape(): string {
    if (this.i >= this.len) {
      this.fail('unterminated_string', 'unterminated string')
    }
    const e = this.src[this.i]
    if (e === 'u') {
      return this.parseUnicodeEscape()
    }
    const mapped = ESCAPES[e]
    if (mapped === undefined) {
      this.fail('invalid_escape', `invalid escape sequence \\${e}`)
    }
    this.i++
    return mapped
  }

  private parseUnicodeEscape(): string {
    // At call time, src[i] === 'u'.
    const code = this.readHex4()
    if (code >= 0xd800 && code <= 0xdbff) {
      // High surrogate — must be followed by a low surrogate escape.
      if (
        this.src.charCodeAt(this.i) === 0x5c &&
        this.src.charCodeAt(this.i + 1) === 0x75 // \u
      ) {
        this.i++ // consume backslash
        const low = this.readHex4()
        if (low >= 0xdc00 && low <= 0xdfff) {
          return String.fromCharCode(code, low)
        }
        // Lone high surrogate followed by a non-low-surrogate escape.
        return String.fromCharCode(code) + String.fromCharCode(low)
      }
    }
    return String.fromCharCode(code)
  }

  // Reads `uXXXX` starting at src[i] === 'u', returning the code unit.
  private readHex4(): number {
    this.i++ // consume u
    if (this.i + 4 > this.len) {
      this.fail('invalid_unicode', 'incomplete \\u escape')
    }
    let code = 0
    for (let k = 0; k < 4; k++) {
      const h = this.src.charCodeAt(this.i)
      let d: number
      if (h >= 0x30 && h <= 0x39) d = h - 0x30
      else if (h >= 0x61 && h <= 0x66) d = h - 0x61 + 10
      else if (h >= 0x41 && h <= 0x46) d = h - 0x41 + 10
      else {
        this.fail(
          'invalid_unicode',
          `invalid hex digit ${JSON.stringify(this.src[this.i])} in \\u escape`,
        )
      }
      code = code * 16 + d
      this.i++
    }
    return code
  }

  private parseNumber(): number {
    const start = this.i
    if (this.src.charCodeAt(this.i) === 0x2d) {
      this.i++ // consume -
    }
    // integer part
    const c = this.src.charCodeAt(this.i)
    if (c === 0x30) {
      this.i++ // a single leading zero, no further digits allowed here
    } else if (isDigit(c)) {
      while (isDigit(this.src.charCodeAt(this.i))) this.i++
    } else {
      this.fail('invalid_number', 'invalid number: expected a digit')
    }
    // fraction
    if (this.src.charCodeAt(this.i) === 0x2e) {
      this.i++ // consume .
      if (!isDigit(this.src.charCodeAt(this.i))) {
        this.fail('invalid_number', 'invalid number: expected a digit after decimal point')
      }
      while (isDigit(this.src.charCodeAt(this.i))) this.i++
    }
    // exponent
    const ec = this.src.charCodeAt(this.i)
    if (ec === 0x65 || ec === 0x45) {
      this.i++ // consume e/E
      const sign = this.src.charCodeAt(this.i)
      if (sign === 0x2b || sign === 0x2d) this.i++
      if (!isDigit(this.src.charCodeAt(this.i))) {
        this.fail('invalid_number', 'invalid number: expected a digit in exponent')
      }
      while (isDigit(this.src.charCodeAt(this.i))) this.i++
    }
    return Number(this.src.slice(start, this.i))
  }

  private parseLiteral<T extends JsonValue>(word: string, value: T): T {
    if (this.src.startsWith(word, this.i)) {
      this.i += word.length
      return value
    }
    this.fail(
      'invalid_literal',
      `expected ${JSON.stringify(word)}`,
    )
  }

  private skipWs(): void {
    while (this.i < this.len && isWs(this.src.charCodeAt(this.i))) {
      this.i++
    }
  }

  private fail(code: string, detail: string): never {
    let line = 1
    let col = 1
    for (let k = 0; k < this.i && k < this.len; k++) {
      if (this.src.charCodeAt(k) === 0x0a) {
        line++
        col = 1
      } else {
        col++
      }
    }
    throw new JsonError(code, detail, this.i, line, col)
  }
}

// Parse a JSON source string and return the resulting value. Throws a
// JsonError on any input that is not valid standard JSON.
export function parse(src: string): JsonValue {
  if (typeof src !== 'string') {
    throw new JsonError('invalid_input', 'source must be a string', 0, 1, 1)
  }
  return new Parser(src).parse()
}

export default parse
