# @tabnas/json

A standard JSON parser (RFC 8259 / ECMA-404) for TypeScript and
JavaScript — the standard-JSON **grammar plugin** for the
[`tabnas`](https://github.com/tabnas/parser) parsing engine.

Available for [TypeScript/JavaScript](#install) and [Go](../go/).

## Install

```bash
npm install @tabnas/json
```

`tabnas` (the engine) is a peer/dependency; see [Develop](#develop) for
the sibling-checkout setup used until it is published.

## Quick example

```ts
import { parse } from '@tabnas/json'

parse('{"a":1, "b":2}')              // { a: 1, b: 2 }
parse('[1, 2, 3]')                   // [1, 2, 3]
parse('{"a": {"b": [true, null]}}')  // { a: { b: [true, null] } }
```

```js
const { parse } = require('@tabnas/json')
parse('"hello"') // "hello"
```

`parse` is also the default export.

## Use it as a plugin

The package is a `tabnas` grammar plugin. Install it on your own engine
instance and layer further grammar on the shared rules:

```ts
import { Tabnas } from 'tabnas'
import { json } from '@tabnas/json'

const am = new Tabnas({ plugins: [json] })
am.parse('{"a":[1,2,3]}')
```

`registerJsonGrammar(am)` installs just the `val` / `map` / `list` /
`pair` / `elem` rules (jsonic's "Plain JSON" core) without the strict
options, for plugins that want to build on the JSON rule set.

## Reuse and options

`parse` reuses a single lazily-created instance, so you don't pay to
rebuild the grammar on every call. To customize, build your own instance
with `make(opts?)` — extra options (e.g. the `info` metadata options) are
applied on top of the strict JSON config:

```ts
import { make } from '@tabnas/json'

const p = make({ info: { map: true, list: true } })
p.parse('{"a":[1,2]}')
```

`Version` is exported as the package version string.

## What it accepts

Exactly standard JSON:

- objects `{ "key": value, ... }` with double-quoted string keys
- arrays `[ value, ... ]`
- double-quoted strings with the JSON escapes
  (`\" \\ \/ \b \f \n \r \t \uXXXX`, including surrogate pairs)
- numbers: optional `-`, integer (no leading zeros), optional fraction,
  optional `e`/`E` exponent
- `true`, `false`, `null`
- insignificant whitespace: space, tab, line feed, carriage return

It **rejects** everything outside that grammar — comments, trailing
commas, unquoted keys, single-quoted or backtick strings, multiline
strings, implicit objects/arrays, hex/octal/binary numbers, leading
zeros, a leading `+`, a bare `.5` or trailing `1.`, and empty input.
This matches the platform `JSON.parse`.

Parsed objects use a **null prototype** (`Object.create(null)`): this is
deliberate and prototype-pollution-safe — a `"__proto__"` key becomes a
normal own property rather than mutating the prototype. The only visible
difference from `JSON.parse` is the missing `Object.prototype` (so e.g.
`obj.hasOwnProperty` is `undefined`; use `Object.hasOwn(obj, k)`).

## Extending the grammar

Because this is a plain grammar plugin on the shared engine, it is a
foundation to build other parsers on. Layer options or rules on top of
the `json` plugin. For example, a JSON-with-comments (JSONC) parser is
just the JSON grammar with comment lexing re-enabled:

```ts
import { Tabnas } from 'tabnas'
import { json } from '@tabnas/json'

const jsonc = new Tabnas({ plugins: [json] })
jsonc.options({ comment: { lex: true } })
jsonc.parse('{"a":1} // ok')      // { a: 1 }
jsonc.parse('{"a":/* ok */1}')    // { a: 1 }
```

For deeper changes, call `registerJsonGrammar(am)` to install just the
rules, then use the engine's rule API (`am.rule(...)`, and the
`clearOpen` / `clearClose` / `@<rule>-<phase>/replace` hooks) to replace
or extend the shared `val` / `map` / `list` / `pair` / `elem` rules
without re-declaring the JSON core.

## Errors

On invalid input, `parse` throws a `TabnasError` (also exported as
`JsonError`) carrying `code`, `lineNumber`, and `columnNumber`:

```ts
import { parse, TabnasError } from '@tabnas/json'

try {
  parse('{a:1}')
} catch (err) {
  if (err instanceof TabnasError) {
    err.code // 'unexpected'
  }
}
```

## CLI

```bash
echo '{"a":1}' | npx tabnas-json
tabnas-json '{"a":1}'
```

Prints the re-serialized (pretty) JSON, or the error on stderr with exit
code 1.

## Develop

This package depends on the engine as a sibling checkout:

```bash
git clone https://github.com/tabnas/parser   # sibling of this repo
( cd parser/ts && npm install && npm run build )
npm install      # resolves "tabnas": "file:../../parser/ts"
npm test         # tsc build + node --test
```

See [`AGENTS.md`](AGENTS.md) for layout and conventions.

## License

MIT.
