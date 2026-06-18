# Reference: `@tabnas/json`

The complete public API and CLI surface. Dry and exhaustive. For learning
see [`tutorial.md`](tutorial.md); for recipes see [`guide.md`](guide.md);
for design see [`concepts.md`](concepts.md).

- **Package:** `@tabnas/json`
- **Entry point:** `dist/json.js` (CommonJS), types at `dist/json.d.ts`
- **Engine:** depends on `@tabnas/parser` (peer dependency `>=2`)
- **Node:** `>=24`

## Exports

| Export | Kind | Summary |
|---|---|---|
| `parse` | function | Parse a string with the default engine. Also the default export. |
| `make` | function | Build a configured parser instance. |
| `json` | plugin | Apply strict options + register the grammar on an engine. |
| `registerJsonGrammar` | function | Register only the rule set on an engine. |
| `Version` | string | Package version (`"1.0.0"`). |
| `Tabnas` | class | Re-exported engine class. |
| `TabnasError` | class | Re-exported engine error class. |
| `JsonError` | class | Alias of `TabnasError`. |
| `default` | function | Same reference as `parse`. |

### `parse(src: string): any`

Parses `src` as standard JSON using a single, lazily-created default
engine (shared across calls; building a fresh context per call makes reuse
safe). Returns the parsed value. Throws `TabnasError` on invalid input.

```js
const { parse } = require('@tabnas/json')
parse('{"a":1,"b":[2,3]}') // => { a: 1, b: [2, 3] }
```

Returned values use these JavaScript types:

| JSON | JavaScript |
|---|---|
| object | plain object with **`null` prototype** (`Object.create(null)`) |
| array | `Array` |
| string | `string` (primitive) |
| number | `number` |
| `true` / `false` | `boolean` |
| `null` | `null` |

The `null` prototype is deliberate and prototype-pollution-safe: a
`"__proto__"` key is stored as an ordinary own property. Consequence:
`Object.prototype` methods are absent on parsed objects (use
`Object.hasOwn(obj, k)`, not `obj.hasOwnProperty(k)`).

### `make(opts?: Record<string, any>): Tabnas`

Creates a fresh engine instance with the `json` plugin installed. If
`opts` is given, it is applied with `tn.options(opts)` **after** the
grammar exists, so engine options (e.g. `info`) layer on top of the strict
JSON configuration without clobbering it. Returns a reusable `Tabnas`
instance; call `.parse(src)` on it.

```js
const { make } = require('@tabnas/json')
const p = make({ info: { map: true, list: true, text: true } })
```

### `json: Plugin`

The standard plugin form, `function json(tn, _options?)`. Applies the
strict JSON options (`tn.options(JSON_OPTIONS)`) and then calls
`registerJsonGrammar(tn)`. Use it on a bare engine:

```js
const { Tabnas } = require('@tabnas/parser')
const { json } = require('@tabnas/json')
const tn = new Tabnas({ plugins: [json] })
```

### `registerJsonGrammar(tn: Tabnas): void`

Installs only the rule set (`val` / `map` / `list` / `pair` / `elem`) on
`tn` via the engine's declarative grammar spec (`tn.grammar({ v: 2, rule
})`). It does **not** apply the strict lexer options, so use it to layer
the JSON rules under your own configuration. The value tree is built
entirely by the engine's `$`-builtin actions; there are no
grammar-local closures.

### `Version: string`

The package version string, matching `package.json` (`"1.0.0"`).

### `Tabnas` (re-export)

The engine class, re-exported for convenience. Construct with
`new Tabnas({ plugins: [json] })`. Instance methods used with this
package:

| Method | Purpose |
|---|---|
| `parse(src)` | Parse a source string. |
| `options(opts)` | Merge engine options into the instance. |
| `use(plugin, opts?)` | Install a plugin. |
| `grammar(spec)` | Install a declarative grammar spec. |
| `rule(name, def?)` | Read or modify a single rule. |

### `TabnasError` / `JsonError` (re-export + alias)

The error thrown on invalid input. `JsonError` is the same class.

| Property | Type | Meaning |
|---|---|---|
| `code` | string | Machine-readable error code (see below). |
| `lineNumber` | number | 1-based line of the error. |
| `columnNumber` | number | 1-based column of the error. |
| `message` | string | Human-readable, source-pointing message. |

**Error codes** emitted by this parser (the shared parity contract with
the Go port):

| Code | When |
|---|---|
| `unexpected` | Any character/token that no active rule alternative accepts — the catch-all for malformed JSON (unquoted keys, trailing commas, comments, single quotes, bad numbers like `01`/`+1`/`.5`/`1.`, unknown escapes, empty input, trailing junk). |
| `unterminated_string` | A string literal with no closing quote (`"abc`). |
| `invalid_unicode` | A `\u` escape that is not four hex digits (`\uZ`, `\u{41}`). |

## What is accepted

Exactly standard JSON (RFC 8259 / ECMA-404):

- **Objects** `{ "key": value, ... }`, keys are double-quoted strings.
- **Arrays** `[ value, ... ]`.
- **Strings** double-quoted, with escapes `\" \\ \/ \b \f \n \r \t`
  and `\uXXXX` (including surrogate pairs).
- **Numbers** optional `-`, integer part with no leading zeros, optional
  `.` fraction, optional `e`/`E` exponent.
- **Keywords** `true`, `false`, `null`.
- **Whitespace** space, tab, line feed, carriage return between tokens.

## What is rejected

Everything outside that grammar, matching `JSON.parse`:

- comments (`//`, `/* */`)
- trailing commas (`[1,2,]`, `{"a":1,}`)
- unquoted keys (`{a:1}`)
- single-quoted or backtick strings (`'x'`, `` `x` ``)
- implicit objects / arrays (`a:1,b:2`, `x,y,z`)
- hex / octal / binary numbers (`0x10`)
- leading zeros (`01`, `00`), leading `+` (`+1`), bare `.5`, trailing `1.`
- non-standard escapes (`\x41`, `\u{41}`, `\v`, `\'`, `` \` ``, `\q`)
- empty or whitespace-only input

## Strict options (internal)

The `json` plugin applies a fixed options object that tightens the
engine's defaults to JSON-only. The load-bearing settings:

| Option | Value | Effect |
|---|---|---|
| `text.lex` | `false` | No bare/unquoted text tokens. |
| `number.hex/oct/bin` | `false` | Decimal numbers only. |
| `number.sep` | `null` | No digit separators. |
| `number.exclude` | regex | Rejects `+1`, `.5`, `1.`, `01`, `00`. |
| `string.chars` | `'"'` | Double-quoted strings only. |
| `string.multiChars` | `''` | No multiline string delimiters. |
| `string.allowUnknown` | `false` | Reject unknown escapes (`\q`). |
| `string.escapeStrict` | `true` | Disable `\xHH` and `\u{...}`. |
| `string.escape` | `{ v:'', "'":'', '`':'' }` | Drop non-standard built-in escapes. |
| `comment.lex` | `false` | No comments. |
| `map.extend` | `false` | No trailing-comma map extension. |
| `lex.empty` | `false` | Reject empty input. |
| `rule.finish` | `false` | Require a complete parse. |
| `rule.include` | `'json'` | Use only the `json`-tagged alternates. |
| `result.fail` | `[undefined, NaN]` | Treat "no value" as a parse failure. |
| `tokenSet.KEY` | `['#ST', null, null, null]` | Keys must be quoted strings. |

These are not part of the public API surface — they are documented so you
know precisely what `json` configures. Extra options passed to `make` are
applied after these.

## Grammar rules

Installed by `registerJsonGrammar`. Each rule is a small state machine of
open/close alternates; the value tree is built by the engine's
`$`-builtin actions (see [`concepts.md`](concepts.md)).

| Rule | Role | Builders |
|---|---|---|
| `val` | a value: map, list, or scalar token | `@reset$`, `@value$` |
| `map` | an object `{ ... }` | `@object$` |
| `list` | an array `[ ... ]` | `@array$` |
| `pair` | a `"key": value` entry in a map | `@key$`, `@setval$` |
| `elem` | a value element in a list | `@push$` |

The grammar's start rule is `val`. A railroad/syntax diagram is in
[`grammar.svg`](grammar.svg) (ASCII: [`grammar.txt`](grammar.txt)).

## CLI: `tabnas-json`

Installed as the `tabnas-json` bin; launcher at `bin/json`.

```
tabnas-json [json...]
```

- **With arguments:** the arguments are joined with a single space and
  parsed as one JSON source.
- **With no arguments:** all of stdin is read and parsed.

On success it writes the value re-serialized with
`JSON.stringify(value, null, 2)` (2-space indent) plus a trailing
newline to stdout, and exits `0`. On a `TabnasError` it writes the error
message to stderr and exits `1`.

```bash
tabnas-json '{"a":1,"b":[2,3]}'
echo '[1,2,3]' | npx tabnas-json
```

### CLI module (`json-cli`)

The CLI logic is exported from `dist/json-cli.js` for in-process testing:

| Export | Signature | Returns |
|---|---|---|
| `run` | `run(src, out, errOut)` | exit code (`number`); writes pretty JSON to `out` or error to `errOut` |
| `main` | `main(argv, stdin, out, errOut, exit)` | `void`; wires `run` to argv/stdin and calls `exit(code)` |
